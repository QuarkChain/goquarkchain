package core

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/qkcdb"
	"github.com/QuarkChain/goquarkchain/serialize"

	"github.com/QuarkChain/goquarkchain/account"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

func (m *MinorBlockChain) ReadLastConfirmedMinorBlockHeaderAtRootBlock(hash common.Hash) common.Hash {
	if data, ok := m.lastConfirmCache.Get(hash); ok {
		return data.(common.Hash)
	}
	data := rawdb.ReadLastConfirmedMinorBlockHeaderAtRootBlock(m.db, hash)
	if !bytes.Equal(data.Bytes(), common.Hash{}.Bytes()) {
		m.lastConfirmCache.Add(hash, data)
	}

	return data
}
func (m *MinorBlockChain) getLastConfirmedMinorBlockHeaderAtRootBlock(hash common.Hash) *types.MinorBlockHeader {
	rMinorHeaderHash := m.ReadLastConfirmedMinorBlockHeaderAtRootBlock(hash)
	if rMinorHeaderHash == qkcCommon.EmptyHash {
		return nil
	}
	return m.GetHeader(rMinorHeaderHash).(*types.MinorBlockHeader)
}

func getLocalFeeRate(qkcConfig *config.QuarkChainConfig) *big.Rat {
	ret := new(big.Rat).SetInt64(1)
	return ret.Sub(ret, qkcConfig.RewardTaxRate)
}
func (m *MinorBlockChain) getCoinbaseAmount() *big.Int {
	// no need to lock
	localFeeRate := getLocalFeeRate(m.clusterConfig.Quarkchain)
	coinbaseAmount := qkcCommon.BigIntMulBigRat(m.clusterConfig.Quarkchain.GetShardConfigByFullShardID(m.branch.Value).CoinbaseAmount, localFeeRate)
	return coinbaseAmount
}

func (m *MinorBlockChain) putMinorBlock(mBlock *types.MinorBlock, xShardReceiveTxList []*types.CrossShardTransactionDeposit) error {
	if _, ok := m.heightToMinorBlockHashes[mBlock.NumberU64()]; !ok {
		m.heightToMinorBlockHashes[mBlock.NumberU64()] = make(map[common.Hash]struct{})
	}
	m.heightToMinorBlockHashes[mBlock.NumberU64()][mBlock.Hash()] = struct{}{}
	if !m.HasBlock(mBlock.Hash()) {
		rawdb.WriteMinorBlock(m.db, mBlock)
	}

	if err := m.putTotalTxCount(mBlock); err != nil {
		return err
	}

	if err := m.putConfirmedCrossShardTransactionDepositList(mBlock.Hash(), xShardReceiveTxList); err != nil {
		return err
	}
	return nil
}
func (m *MinorBlockChain) updateTip(state *state.StateDB, block *types.MinorBlock) (bool, error) {
	// Don't update tip if the block depends on a root block that is not root_tip or root_tip's ancestor
	if !m.isSameRootChain(m.rootTip, m.getRootBlockHeaderByHash(block.Header().PrevRootBlockHash)) {
		return false, nil
	}
	updateTip := false
	currentTip := m.CurrentBlock().IHeader().(*types.MinorBlockHeader)
	if block.Header().ParentHash == currentTip.Hash() {
		updateTip = true
	} else if m.isMinorBlockLinkedToRootTip(block) {
		if block.Header().Number > currentTip.NumberU64() {
			updateTip = true
		} else if block.Header().Number == currentTip.NumberU64() {
			updateTip = m.getRootBlockHeaderByHash(block.Header().PrevRootBlockHash).Number > m.getRootBlockHeaderByHash(currentTip.PrevRootBlockHash).Number
		}
	}

	if updateTip {
		m.currentEvmState = state
	}
	return updateTip, nil
}

func (m *MinorBlockChain) runCrossShardTxList(evmState *state.StateDB, descendantRootHeader *types.RootBlockHeader, ancestorRootHeader *types.RootBlockHeader) ([]*types.CrossShardTransactionDeposit, error) {
	txList := make([]*types.CrossShardTransactionDeposit, 0)
	rHeader := descendantRootHeader
	for rHeader.Hash() != ancestorRootHeader.Hash() {
		if rHeader.Number == ancestorRootHeader.Number {
			return nil, errors.New("incorrect ancestor root header")
		}
		if evmState.GetGasUsed().Cmp(evmState.GetGasLimit()) > 0 {
			return nil, errors.New("gas consumed by cross-shard tx exceeding limit")
		}
		onTxList, err := m.runOneCrossShardTxListByRootBlockHash(rHeader.Hash(), evmState)
		if err != nil {
			return nil, err
		}
		txList = append(txList, onTxList...)
		rHeader = m.getRootBlockHeaderByHash(rHeader.ParentHash)
		if rHeader == nil {
			log.Info(m.logInfo, "err-runCrossShardTxList", ErrRootBlockIsNil, "parentHash", rHeader.ParentHash.String())
			return nil, ErrRootBlockIsNil
		}
	}
	if evmState.GetGasUsed().Cmp(evmState.GetGasLimit()) > 0 {
		return nil, errors.New("runCrossShardTxList err:gasUsed > GasLimit")
	}
	return txList, nil
}

func (m *MinorBlockChain) validateTx(tx *types.Transaction, evmState *state.StateDB, fromAddress *account.Address, gas *uint64) (*types.Transaction, error) {
	if evmState == nil && fromAddress != nil {
		return nil, errors.New("validateTx params err")
	}
	if tx.TxType != types.EvmTx {
		return nil, errors.New("unexpected tx type")
	}
	evmTx := tx.EvmTx
	if fromAddress != nil {
		nonce := evmState.GetNonce(fromAddress.Recipient)
		evmTx.SetNonce(nonce)
		if evmTx.FromFullShardKey() != fromAddress.FullShardKey {
			return nil, errors.New("from full shard id not match")
		}
		evmTxGas := evmTx.Gas()
		if gas != nil {
			evmTxGas = *gas
		}
		evmTx.SetGas(evmTxGas)
	}

	toShardSize, err := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.ToChainID())
	if err != nil {
		return nil, err
	}
	if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
		return nil, err
	}
	fromShardSize, err := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err != nil {
		return nil, err
	}
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return nil, err
	}

	if evmTx.NetworkId() != m.clusterConfig.Quarkchain.NetworkID {
		return nil, ErrNetWorkID
	}
	if !m.branch.IsInBranch(evmTx.FromFullShardId()) {
		return nil, ErrBranch
	}

	toBranch := account.Branch{Value: evmTx.ToFullShardId()}

	if evmTx.IsCrossShard() {
		initializedFullShardIDs := m.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(m.rootTip.Number)
		hasInit := false
		for _, v := range initializedFullShardIDs {
			if toBranch.GetFullShardID() == v {
				hasInit = true
			}
		}
		if !hasInit {
			return nil, errors.New("shard is not initialized yet")
		}
	}

	if evmTx.IsCrossShard() && !m.isNeighbor(toBranch, nil) {
		return nil, ErrNotNeighbor
	}
	tx = &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	if evmState == nil {
		return tx, nil //txpool.validateTx,validateTransaction will add in txpool,to avoid write and read txpool.currentEvmState frequently
	}
	if err := ValidateTransaction(evmState, tx, fromAddress); err != nil {
		return nil, err
	}
	return tx, nil
}
func (m *MinorBlockChain) InitGenesisState(rBlock *types.RootBlock) (*types.MinorBlock, error) {
	log.Info(m.logInfo, "InitGenesisState number", rBlock.Number(), "hash", rBlock.Hash().String())
	defer log.Info(m.logInfo, "InitGenesisState", "end")
	m.mu.Lock() // used in initFromRootBlock and addRootBlock, so should lock
	defer m.mu.Unlock()
	var err error
	gBlock := m.genesisBlock
	if m.genesisBlock.PrevRootBlockHash() != rBlock.Hash() {
		gBlock, err = NewGenesis(m.clusterConfig.Quarkchain).CreateMinorBlock(rBlock, m.branch.Value, m.db)
		if err != nil {
			return nil, err
		}
	}

	rawdb.WriteTd(m.db, gBlock.Hash(), gBlock.Difficulty())
	height := m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)
	if rBlock.Header().Number != height {
		return nil, errors.New("header number not match")
	}

	if err := m.putMinorBlock(gBlock, nil); err != nil {
		return nil, err
	}
	m.putRootBlock(rBlock, nil)
	rawdb.WriteGenesisBlock(m.db, rBlock.Hash(), gBlock) // key:rootBlockHash value:minorBlock
	if m.initialized {
		return gBlock, nil
	}
	m.rootTip = rBlock.Header()
	m.confirmedHeaderTip = nil
	m.currentEvmState, err = m.createEvmState(gBlock.Meta().Root, gBlock.Hash())
	if err != nil {
		return nil, err
	}

	m.initialized = true
	return gBlock, nil
}

// GetTransactionCount get txCount for addr
func (m *MinorBlockChain) GetTransactionCount(recipient account.Recipient, height *uint64) (uint64, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(height)
	if err != nil {
		return 0, err
	}
	return evmState.GetNonce(recipient), nil
}

func (m *MinorBlockChain) isSameRootChain(long types.IHeader, short types.IHeader) bool {
	return isSameChain(m.db, long, short)
}

func (m *MinorBlockChain) isMinorBlockLinkedToRootTip(mBlock *types.MinorBlock) bool {
	confirmed := m.confirmedHeaderTip
	if confirmed == nil {
		return true
	}
	if mBlock.Header().Number <= confirmed.Number {
		return false
	}
	return isSameChain(m.db, mBlock.Header(), confirmed)
}
func (m *MinorBlockChain) isNeighbor(remoteBranch account.Branch, rootHeight *uint32) bool {
	if rootHeight == nil {
		rootHeight = &m.rootTip.Number
	}
	shardSize := len(m.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(*rootHeight))
	return account.IsNeighbor(m.branch, remoteBranch, uint32(shardSize))
}

func (m *MinorBlockChain) putRootBlock(rBlock *types.RootBlock, minorHeader *types.MinorBlockHeader) {
	log.Info(m.logInfo, "putRootBlock number", rBlock.Number(), "hash", rBlock.Hash().String(), "lenMinor", len(rBlock.MinorBlockHeaders()))
	rBlockHash := rBlock.Hash()
	rawdb.WriteRootBlock(m.db, rBlock)
	var mHash common.Hash
	if minorHeader != nil {
		mHash = minorHeader.Hash()
	}
	if _, ok := m.rootHeightToHashes[rBlock.NumberU64()]; !ok {
		m.rootHeightToHashes[rBlock.NumberU64()] = make(map[common.Hash]common.Hash)
	}
	log.Info("putRootBlock", "rBlock", rBlock.NumberU64(), "rHash", rBlock.Hash().String(), "mHash", mHash.String())
	m.rootHeightToHashes[rBlock.NumberU64()][rBlock.Hash()] = mHash
	rawdb.WriteLastConfirmedMinorBlockHeaderAtRootBlock(m.db, rBlockHash, mHash)
}

func (m *MinorBlockChain) createEvmState(trieRootHash common.Hash, headerHash common.Hash) (*state.StateDB, error) {
	evmState, err := m.StateAt(trieRootHash)
	if err != nil {
		return nil, err
	}
	evmState.SetShardConfig(m.shardConfig)
	return evmState, nil
}

func (m *MinorBlockChain) putTotalTxCount(mBlock *types.MinorBlock) error {
	prevCount := uint32(0)
	if mBlock.Header().Number > 1 {
		dbPreCount := m.getTotalTxCount(mBlock.Header().ParentHash)
		if dbPreCount == nil {
			return errors.New("get totalTx failed")
		}
		prevCount += *dbPreCount
	}
	prevCount += uint32(len(mBlock.Transactions()))
	rawdb.WriteTotalTx(m.db, mBlock.Header().Hash(), prevCount)
	return nil
}

func (m *MinorBlockChain) getTotalTxCount(hash common.Hash) *uint32 {
	return rawdb.ReadTotalTx(m.db, hash) //cache?
}

func (m *MinorBlockChain) putConfirmedCrossShardTransactionDepositList(hash common.Hash, xShardReceiveTxList []*types.CrossShardTransactionDeposit) error {
	if !m.clusterConfig.EnableTransactionHistory {
		return nil
	}
	data := types.CrossShardTransactionDepositList{TXList: xShardReceiveTxList}
	rawdb.WriteConfirmedCrossShardTxList(m.db, hash, data)
	return nil
}

// InitFromRootBlock init minorBlockChain from rootBlock
func (m *MinorBlockChain) InitFromRootBlock(rBlock *types.RootBlock) error {
	log.Info(m.logInfo, "InitFromRootBlock number", rBlock.Number(), "hash", rBlock.Hash().String())
	defer log.Info(m.logInfo, "InitFromRootBlock", "end")
	if rBlock.Header().Number <= uint32(m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)) {
		return errors.New("rootBlock height small than config's height")
	}
	if m.initialized == true {
		log.Warn("ReInitFromBlock", "InitFromBlock", rBlock.Number())
	}
	m.initialized = true
	confirmedHeaderTip := m.getLastConfirmedMinorBlockHeaderAtRootBlock(rBlock.Hash())
	if confirmedHeaderTip == nil {
		m.rootTip = m.getRootBlockHeaderByHash(rBlock.ParentHash())
		_, err := m.AddRootBlock(rBlock)
		if err != nil {
			m.Stop()
			log.Error(m.logInfo, "InitFromRootBlock-addRootBlock number", rBlock.NumberU64())
			return err
		}
		confirmedHeaderTip = m.getLastConfirmedMinorBlockHeaderAtRootBlock(rBlock.Hash())
	}

	headerTip := confirmedHeaderTip
	if headerTip == nil {
		headerTip = m.GetBlockByNumber(0).IHeader().(*types.MinorBlockHeader)
	}

	m.rootTip = rBlock.Header()
	m.confirmedHeaderTip = confirmedHeaderTip
	headerTipHash := headerTip.Hash()
	block := rawdb.ReadMinorBlock(m.db, headerTipHash)
	if block == nil {
		return ErrMinorBlockIsNil
	}
	log.Info(m.logInfo, "tipMinor", block.Number(), "hash", block.Hash().String(),
		"mete.root", block.Meta().Root.String(), "rootBlock", rBlock.NumberU64(), "rootTip", m.rootTip.Number)
	var err error
	m.currentEvmState, err = m.createEvmState(block.Meta().Root, block.Hash())
	if err != nil {
		return err
	}
	return m.reWriteBlockIndexTo(nil, block)
}

// getEvmStateForNewBlock get evmState for new block.should have locked
func (m *MinorBlockChain) getEvmStateForNewBlock(mHeader types.IHeader, ephemeral bool) (*state.StateDB, error) {
	prevHash := mHeader.GetParentHash()
	preMinorBlock := m.GetMinorBlock(prevHash)
	if preMinorBlock == nil {
		return nil, errInsufficientBalanceForGas
	}
	recipient := mHeader.GetCoinbase().Recipient
	rootHash := preMinorBlock.GetMetaData().Root
	evmState, err := m.createEvmState(rootHash, prevHash)
	if err != nil {
		return nil, err
	}
	if ephemeral {
		evmState = evmState.Copy()
	}
	evmState.SetBlockCoinbase(recipient)
	header := mHeader.(*types.MinorBlockHeader)
	evmState.SetGasLimit(header.GetGasLimit())
	evmState.SetQuarkChainConfig(m.clusterConfig.Quarkchain)
	senderDisallowMap, err := m.posw.BuildSenderDisallowMap(prevHash, &recipient)
	if err != nil {
		return nil, err
	}
	evmState.SetSenderDisallowMap(senderDisallowMap)
	return evmState, nil
}

func (m *MinorBlockChain) getEvmStateFromHeight(height *uint64) (*state.StateDB, error) {
	if height == nil || *height == m.CurrentBlock().NumberU64() {
		return m.State()
	}
	header := m.GetHeaderByNumber(*height + 1)
	if header != nil {
		return nil, ErrMinorBlockIsNil
	}
	return m.getEvmStateForNewBlock(header, true)
}

func (m *MinorBlockChain) runBlock(block *types.MinorBlock) (*state.StateDB, types.Receipts, error) {
	parent := m.GetMinorBlock(block.ParentHash())
	if qkcCommon.IsNil(parent) {
		log.Error(m.logInfo, "err-runBlock", ErrRootBlockIsNil, "parentHash", block.ParentHash().String())
		return nil, nil, ErrRootBlockIsNil
	}

	preEvmState, err := m.StateAt(parent.GetMetaData().Root)
	if err != nil {
		return nil, nil, err
	}
	evmState := preEvmState.Copy()
	coinbase := block.Coinbase().Recipient
	senderDisallowMap, err := m.posw.BuildSenderDisallowMap(block.ParentHash(), &coinbase)
	if err != nil {
		return nil, nil, err
	}
	evmState.SetSenderDisallowMap(senderDisallowMap)
	receipts, _, _, err := m.processor.Process(block, evmState, m.vmConfig, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	return evmState, receipts, nil
}

// FinalizeAndAddBlock finalize minor block and add it to chain
// only used in test now
func (m *MinorBlockChain) FinalizeAndAddBlock(block *types.MinorBlock) (*types.MinorBlock, types.Receipts, error) {
	evmState, receipts, err := m.runBlock(block) // will lock
	if err != nil {
		return nil, nil, err
	}
	coinbaseAmount := new(big.Int).Add(m.getCoinbaseAmount(), evmState.GetBlockFee())
	block.Finalize(receipts, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), coinbaseAmount)
	_, err = m.InsertChain([]types.IBlock{block}) // will lock
	if err != nil {
		return nil, nil, err
	}
	return block, receipts, nil
}

func absUint32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
func minBigInt(a, b *big.Int) *big.Int {
	if a.Cmp(b) < 0 {
		return a
	}
	return b
}

// AddTx add tx to txPool
func (m *MinorBlockChain) AddTx(tx *types.Transaction) error {
	return m.txPool.AddLocal(tx)
}

func (m *MinorBlockChain) getCrossShardTxListByRootBlockHash(hash common.Hash) ([]*types.CrossShardTransactionDeposit, error) {
	// no need to lock
	rBlock := m.GetRootBlockByHash(hash)
	if rBlock == nil {
		log.Error(m.logInfo, "err-getCrossShardTxListByRootBlockHash", ErrRootBlockIsNil, "parenthash", hash.String())
		return nil, ErrRootBlockIsNil
	}
	txList := make([]*types.CrossShardTransactionDeposit, 0)
	for _, mHeader := range rBlock.MinorBlockHeaders() {
		if mHeader.Branch == m.branch {
			continue
		}
		prevRootHeader := m.getRootBlockHeaderByHash(mHeader.PrevRootBlockHash)
		if prevRootHeader == nil {
			return nil, errors.New("not get pre root header")
		}
		if !m.isNeighbor(mHeader.Branch, &prevRootHeader.Number) {
			continue
		}
		xShardTxList := m.ReadCrossShardTxList(mHeader.Hash())

		if prevRootHeader.Number <= uint32(m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)) {
			if xShardTxList != nil {
				return nil, errors.New("get xShard tx list err")
			}
			continue
		}
		txList = append(txList, xShardTxList.TXList...)
	}
	if m.branch.IsInBranch(rBlock.Header().GetCoinbase().FullShardKey) { // Apply root block coinbase
		txList = append(txList, &types.CrossShardTransactionDeposit{
			TxHash:   common.Hash{},
			From:     account.CreatEmptyAddress(0),
			To:       rBlock.Header().Coinbase,
			Value:    rBlock.Header().CoinbaseAmount,
			GasPrice: &serialize.Uint256{Value: new(big.Int).SetUint64(0)},
		})
	}
	return txList, nil
}

func (m *MinorBlockChain) getEvmStateByHeight(height *uint64) (*state.StateDB, error) {
	mBlock := m.CurrentBlock()
	if height != nil {
		var ok bool
		mBlock, ok = m.GetBlockByNumber(*height).(*types.MinorBlock)
		if !ok {
			return nil, fmt.Errorf("no such block:height %v", *height)
		}
	}
	evmState, err := m.StateAt(mBlock.GetMetaData().Root)
	if err != nil {
		return nil, err
	}
	senderDisallowMap, err := m.posw.BuildSenderDisallowMap(mBlock.Hash(), nil)
	if err != nil {
		return nil, err
	}
	evmState.SetSenderDisallowMap(senderDisallowMap)
	return evmState, nil
}

// GetBalance get balance for address
func (m *MinorBlockChain) GetBalance(recipient account.Recipient, height *uint64) (*big.Int, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(height)
	if err != nil {
		return nil, err
	}
	return evmState.GetBalance(recipient), nil
}

// GetCode get code for addr
func (m *MinorBlockChain) GetCode(recipient account.Recipient, height *uint64) ([]byte, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(height)
	if err != nil {
		return nil, err
	}
	return evmState.GetCode(recipient), nil
}

// GetStorageAt get storage for addr
func (m *MinorBlockChain) GetStorageAt(recipient account.Recipient, key common.Hash, height *uint64) (common.Hash, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(height)
	if err != nil {
		return common.Hash{}, err
	}
	return evmState.GetState(recipient, key), nil
}

// ExecuteTx execute tx
func (m *MinorBlockChain) ExecuteTx(tx *types.Transaction, fromAddress *account.Address, height *uint64) ([]byte, error) {
	if height == nil {
		temp := m.CurrentBlock().NumberU64()
		height = &temp
	}
	if fromAddress == nil {
		return nil, errors.New("from address should not empty")
	}
	mBlock, ok := m.GetBlockByNumber(*height).(*types.MinorBlock)
	if !ok {
		return nil, ErrMinorBlockIsNil
	}

	evmState, err := m.StateAt(mBlock.GetMetaData().Root)
	if err != nil {
		return nil, err
	}

	senderDisallowMap, err := m.posw.BuildSenderDisallowMap(mBlock.Hash(), nil)
	if err != nil {
		return nil, err
	}
	evmState.SetSenderDisallowMap(senderDisallowMap)
	state := evmState.Copy()
	state.SetGasUsed(new(big.Int).SetUint64(0))
	var gas uint64
	if tx.EvmTx.Gas() != 0 {
		gas = tx.EvmTx.Gas()
	} else {
		gas = state.GetGasLimit().Uint64()
	}
	evmTx, err := m.validateTx(tx, state, fromAddress, &gas)
	if err != nil {
		return nil, err
	}
	gp := new(GasPool).AddGas(mBlock.Header().GetGasLimit().Uint64())

	to := evmTx.EvmTx.To()
	msg := types.NewMessage(fromAddress.Recipient, to, evmTx.EvmTx.Nonce(), evmTx.EvmTx.Value(), evmTx.EvmTx.Gas(), evmTx.EvmTx.GasPrice(), evmTx.EvmTx.Data(), false, tx.EvmTx.FromShardID(), tx.EvmTx.ToShardID())
	evmState.SetFullShardKey(tx.EvmTx.ToFullShardKey())

	context := NewEVMContext(msg, m.CurrentBlock().IHeader().(*types.MinorBlockHeader), m)
	evmEnv := vm.NewEVM(context, evmState, m.ethChainConfig, m.vmConfig)

	localFee := getLocalFeeRate(m.clusterConfig.Quarkchain)
	ret, _, _, err := ApplyMessage(evmEnv, msg, gp, localFee)
	return ret, err

}

func checkEqual(a, b types.IHeader) bool {
	if qkcCommon.IsNil(a) && qkcCommon.IsNil(b) {
		return true
	}
	if qkcCommon.IsNil(a) && !qkcCommon.IsNil(b) {
		return false
	}
	if !qkcCommon.IsNil(a) && qkcCommon.IsNil(b) {
		return false
	}
	if a.Hash() != b.Hash() {
		return false
	}
	return true
}
func (m *MinorBlockChain) getAllUnconfirmedHeaderList() []*types.MinorBlockHeader {
	var (
		header *types.MinorBlockHeader
		ok     bool
	)

	confirmedHeaderTip := m.confirmedHeaderTip

	header, ok = m.CurrentHeader().(*types.MinorBlockHeader)
	if !ok {
		panic(errors.New("current not exist"))
	}
	startHeight := int64(-1)
	if confirmedHeaderTip != nil {
		startHeight = int64(confirmedHeaderTip.Number)
	}

	allHeight := int(header.NumberU64()) - int(startHeight)
	if allHeight < 0 {
		allHeight = 0
	}
	headerList := make([]*types.MinorBlockHeader, allHeight)
	for index := allHeight - 1; index >= 0; index-- {
		headerList[index] = header
		header, ok = m.GetHeaderByHash(header.GetParentHash()).(*types.MinorBlockHeader)
		if !ok {
			if index == 0 {
				continue // 0's pre
			} else {
				panic(errors.New("not in order"))
			}
		}
	}

	if !checkEqual(header, confirmedHeaderTip) {
		return nil
	}
	return headerList
}

// GetUnconfirmedHeaderList get unconfirmed headerList
func (m *MinorBlockChain) GetUnconfirmedHeaderList() []*types.MinorBlockHeader {
	m.mu.Lock()
	defer m.mu.Unlock()
	headers := m.getAllUnconfirmedHeaderList() // have lock
	maxBlocks := m.getMaxBlocksInOneRootBlock()
	if len(headers) < int(maxBlocks) {
		return headers
	}
	return headers[0:maxBlocks]
}

func (m *MinorBlockChain) getMaxBlocksInOneRootBlock() uint64 {
	return uint64(m.shardConfig.MaxBlocksPerShardInOneRootBlock())
}

// GetUnconfirmedHeadersCoinbaseAmount get unconfirmed Headers coinbase amount
func (m *MinorBlockChain) GetUnconfirmedHeadersCoinbaseAmount() uint64 {
	amount := uint64(0)
	headers := m.GetUnconfirmedHeaderList() // have lock
	for _, header := range headers {
		amount += header.CoinbaseAmount.Value.Uint64()
	}
	return amount
}

func (m *MinorBlockChain) getXShardTxLimits(rBlock *types.RootBlock) map[uint32]uint32 {
	// no need to lock
	results := make(map[uint32]uint32, 0)
	for _, mHeader := range rBlock.MinorBlockHeaders() {
		results[mHeader.Branch.GetFullShardID()] = uint32(mHeader.GasLimit.Value.Uint64()) / uint32(params.GtxxShardCost.Uint64()) / m.clusterConfig.Quarkchain.MaxNeighbors / uint32(m.getMaxBlocksInOneRootBlock())
	}
	return results
}

func (m *MinorBlockChain) addTransactionToBlock(rootBlockHash common.Hash, block *types.MinorBlock, evmState *state.StateDB) (*types.MinorBlock, types.Receipts, error) {
	// have locked by upper call
	pending, err := m.txPool.Pending() // txpool already locked
	if err != nil {
		return nil, nil, err
	}
	txs, err := types.NewTransactionsByPriceAndNonce(types.NewEIP155Signer(uint32(m.Config().NetworkID)), pending)

	xShardTxCounters := make(map[uint32]uint32, 0)
	xShardTxLimits := m.getXShardTxLimits(m.GetRootBlockByHash(rootBlockHash))
	gp := new(GasPool).AddGas(block.Header().GetGasLimit().Uint64())
	usedGas := new(uint64)

	receipts := make([]*types.Receipt, 0)
	txsInBlock := make([]*types.Transaction, 0)

	stateT := evmState
	for stateT.GetGasUsed().Cmp(stateT.GetGasLimit()) < 0 {
		tx := txs.Peek()
		// Pop skip all txs about this account
		//Shift skip this tx ,goto next tx about this account
		if err := m.checkTxBeforeApply(stateT, tx, xShardTxCounters, xShardTxLimits); err != nil {
			if err == ErrorTxBreak {
				break
			} else if err == ErrorTxContinue {
				txs.Pop()
				continue
			}
		}
		_, receipt, _, err := ApplyTransaction(m.ethChainConfig, m, gp, stateT, block.IHeader().(*types.MinorBlockHeader), tx, usedGas, *m.GetVMConfig())
		switch err {
		case ErrGasLimitReached:
			txs.Pop()
		case ErrNonceTooLow:
			// New head notification data race between the transaction pool and miner, shift
			if err := txs.Shift(); err != nil {
				return nil, nil, errors.New("txs.Shift error")
			}
		case ErrNonceTooHigh:
			// Reorg notification data race between the transaction pool and miner, skip account =
			txs.Pop()
		case nil:
			if err := txs.Shift(); err != nil {
				return nil, nil, errors.New("txs.Shift error")
			}
			receipts = append(receipts, receipt)
			txsInBlock = append(txsInBlock, tx)
			xShardTxCounters[tx.EvmTx.ToFullShardId()]++
		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			if err := txs.Shift(); err != nil {
				return nil, nil, errors.New("txs.Shift error")
			}
		}

	}
	bHeader := block.Header()
	bHeader.PrevRootBlockHash = rootBlockHash
	return types.NewMinorBlock(bHeader, block.Meta(), txsInBlock, receipts, nil), receipts, nil
}

var (
	ErrorTxContinue = errors.New("apply tx continue")
	ErrorTxBreak    = errors.New("apply tx break")
)

func (m *MinorBlockChain) checkTxBeforeApply(stateT *state.StateDB, tx *types.Transaction, xShardTxCounters, xShardTxLimits map[uint32]uint32) error {
	diff := new(big.Int).Sub(stateT.GetGasLimit(), stateT.GetGasUsed())
	if tx == nil {
		return ErrorTxBreak
	}

	if tx.EvmTx.Gas() > diff.Uint64() {
		return ErrorTxContinue
	}

	toShardSize, err := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.ToChainID())
	if err != nil {
		return err
	}
	if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
		return ErrorTxContinue
	}

	toBranch := account.Branch{Value: tx.EvmTx.ToFullShardId()}
	if toBranch.Value != m.branch.Value {
		if !m.isNeighbor(toBranch, nil) {
			return ErrorTxContinue
		}
		if xShardTxCounters[tx.EvmTx.ToFullShardId()]+1 > xShardTxLimits[tx.EvmTx.ToFullShardId()] {
			return ErrorTxContinue
		}
	}
	return nil
}

// CreateBlockToMine create block to mine
func (m *MinorBlockChain) CreateBlockToMine(createTime *uint64, address *account.Address, gasLimit *big.Int) (*types.MinorBlock, error) {
	realCreateTime := uint64(time.Now().Unix())
	if createTime == nil {
		if realCreateTime < m.CurrentBlock().IHeader().GetTime()+1 {
			realCreateTime = m.CurrentBlock().IHeader().GetTime() + 1
		}
	} else {
		realCreateTime = *createTime
	}
	difficulty, err := m.engine.CalcDifficulty(m, realCreateTime, m.CurrentHeader().(*types.MinorBlockHeader))
	if err != nil {
		return nil, err
	}
	prevBlock := m.CurrentBlock()
	if gasLimit == nil {
		gasLimit = m.gasLimit
	}

	if address == nil {
		t := account.CreatEmptyAddress(0)
		address = &t
	}

	fullShardID, err := m.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	if err != nil {
		return nil, err
	}
	if fullShardID != m.branch.Value {
		t := address.AddressInBranch(m.branch)
		address = &t
	}
	block := prevBlock.CreateBlockToAppend(&realCreateTime, difficulty, address, nil, gasLimit, nil, nil)
	evmState, err := m.getEvmStateForNewBlock(block.IHeader(), true)
	prevHeader := m.CurrentBlock()
	ancestorRootHeader := m.GetRootBlockByHash(prevHeader.Header().PrevRootBlockHash).Header()
	if !m.isSameRootChain(m.rootTip, ancestorRootHeader) {
		return nil, ErrNotSameRootChain
	}

	rootHeader, err := m.includeCrossShardTxList(evmState, m.rootTip, ancestorRootHeader)

	if err != nil {
		return nil, err
	}
	newBlock, recipiets, err := m.addTransactionToBlock(rootHeader.Hash(), block, evmState)
	if err != nil {
		return nil, err
	}
	pureCoinbaseAmount := m.getCoinbaseAmount()
	evmState.AddBalance(evmState.GetBlockCoinbase(), pureCoinbaseAmount)
	coinbaseAmount := new(big.Int).Add(pureCoinbaseAmount, evmState.GetBlockFee())
	newBlock.Finalize(recipiets, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), coinbaseAmount)
	return newBlock, nil
}

//Cross-Shard transaction handling

// AddCrossShardTxListByMinorBlockHash add crossShardTxList by slave
func (m *MinorBlockChain) AddCrossShardTxListByMinorBlockHash(h common.Hash, txList types.CrossShardTransactionDepositList) {
	rawdb.WriteCrossShardTxList(m.db, h, txList)
}

// AddRootBlock add root block for minorBlockChain
func (m *MinorBlockChain) AddRootBlock(rBlock *types.RootBlock) (bool, error) {
	if rBlock.Number() <= uint32(m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)) {
		errRootBlockHeight := errors.New("rBlock is small than config")
		log.Error(m.logInfo, "add rootBlock", errRootBlockHeight, "block's height", rBlock.Number(), "config's height", m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value))
		return false, errRootBlockHeight
	}

	if m.GetRootBlockByHash(rBlock.Hash()) != nil {
		return false, nil
	}

	if m.GetRootBlockByHash(rBlock.ParentHash()) == nil {
		log.Error(m.logInfo, "add rootBlock err", ErrRootBlockIsNil, "parentHash", rBlock.ParentHash(), "height", rBlock.Number()-1)
		return false, ErrRootBlockIsNil
	}

	shardHeaders := make([]*types.MinorBlockHeader, 0)
	for _, mHeader := range rBlock.MinorBlockHeaders() {
		h := mHeader.Hash()
		if mHeader.Branch == m.branch {
			if !m.HasBlock(h) {
				log.Error(m.logInfo, "add rootBlock err", "block not exist", "height", mHeader.Number, "hash", mHeader.Hash().String(), "blockNumber", rBlock.NumberU64())
				return false, ErrMinorBlockIsNil
			}
			shardHeaders = append(shardHeaders, mHeader)
			continue
		}
		prevRootHeader := m.GetRootBlockByHash(mHeader.PrevRootBlockHash)

		// prev_root_header can be None when the shard is not created at root height 0
		if prevRootHeader == nil || prevRootHeader.Number() == uint32(m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)) || !m.isNeighbor(mHeader.Branch, &prevRootHeader.Header().Number) {
			if data := m.ReadCrossShardTxList(h); data != nil {
				errXshardListAlreadyHave := errors.New("already have")
				log.Error(m.logInfo, "addrootBlock err-1", errXshardListAlreadyHave)
				return false, errXshardListAlreadyHave
			}
			continue
		}

		if data := m.ReadCrossShardTxList(h); data == nil {
			errXshardListNotHave := errors.New("not have")
			log.Error(m.logInfo, "addrootBlock err-2", errXshardListNotHave)
			return false, errXshardListNotHave
		}

	}
	if uint64(len(shardHeaders)) > m.getMaxBlocksInOneRootBlock() {
		errShardHeaders := errors.New("shardHeaders big than config")
		log.Error(m.logInfo, "add root block err", errShardHeaders)
		return false, errShardHeaders
	}

	lastMinorHeaderInPrevRootBlock := m.getLastConfirmedMinorBlockHeaderAtRootBlock(rBlock.Header().ParentHash)

	var shardHeader *types.MinorBlockHeader
	if len(shardHeaders) > 0 {
		if shardHeaders[0].Number == 0 || shardHeaders[0].ParentHash == lastMinorHeaderInPrevRootBlock.Hash() {
			shardHeader = shardHeaders[len(shardHeaders)-1]
		} else {
			return false, errors.New("master should assure this check will not fail")
		}
	} else {
		shardHeader = lastMinorHeaderInPrevRootBlock
	}
	m.putRootBlock(rBlock, shardHeader)
	if shardHeader != nil {
		if !m.isSameRootChain(rBlock.Header(), m.getRootBlockHeaderByHash(shardHeader.PrevRootBlockHash)) {
			return false, ErrNotSameRootChain
		}
	}

	// No change to root tip
	if rBlock.Header().Number <= m.rootTip.Number {
		if !m.isSameRootChain(m.rootTip, m.GetRootBlockByHash(m.CurrentBlock().IHeader().(*types.MinorBlockHeader).GetPrevRootBlockHash()).Header()) {
			return false, ErrNotSameRootChain
		}
		return false, nil
	}

	m.mu.Lock()
	m.rootTip = rBlock.Header()
	m.confirmedHeaderTip = shardHeader
	m.mu.Unlock()
	origHeaderTip := m.CurrentHeader().(*types.MinorBlockHeader)
	if shardHeader != nil {
		origBlock := m.GetBlockByNumber(shardHeader.Number)
		if qkcCommon.IsNil(origBlock) || origBlock.Hash() != shardHeader.Hash() {
			log.Warn(m.logInfo, "ready to set current header height", shardHeader.Number, "hash", shardHeader.Hash().String(), "status", qkcCommon.IsNil(origBlock))
			m.hc.SetCurrentHeader(shardHeader)

			newTipBlock := m.GetBlock(shardHeader.Hash())
			if err := m.reorg(m.CurrentBlock(), newTipBlock); err != nil {
				return false, err
			}
		}
	}

	for !m.isSameRootChain(m.rootTip, m.getRootBlockHeaderByHash(m.CurrentHeader().(*types.MinorBlockHeader).GetPrevRootBlockHash())) {
		if m.CurrentHeader().NumberU64() == 0 {
			genesisRootHeader := m.rootTip
			genesisHeight := m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)
			if genesisRootHeader.Number < uint32(genesisHeight) {
				return false, errors.New("genesis root height small than config")
			}
			for genesisRootHeader.Number != uint32(genesisHeight) {
				genesisRootHeader = m.getRootBlockHeaderByHash(genesisRootHeader.ParentHash)
				if genesisRootHeader == nil {
					return false, ErrMinorBlockIsNil
				}
			}
			newGenesis := rawdb.ReadGenesis(m.db, genesisRootHeader.Hash()) // genesisblock key is rootblock hash
			if newGenesis == nil {
				panic(errors.New("get genesis block is nil"))
			}
			m.genesisBlock = newGenesis
			log.Warn(m.logInfo, "ready to resrt genesis number", m.genesisBlock.Number(), "hash", m.genesisBlock.Hash().String())
			if err := m.Reset(); err != nil {
				return false, err
			}
			break
		}
		preBlock := m.GetBlock(m.CurrentHeader().GetParentHash()).(*types.MinorBlock)
		log.Warn(m.logInfo, "ready to set currentHeader height", preBlock.Number(), "hash", preBlock.Hash().String())
		m.hc.SetCurrentHeader(preBlock.Header())
		m.currentBlock.Store(preBlock)
	}

	if m.CurrentHeader().Hash() != origHeaderTip.Hash() {
		headerTipHash := m.CurrentHeader().Hash()
		origBlock := m.GetMinorBlock(origHeaderTip.Hash())
		newBlock := m.GetMinorBlock(headerTipHash)
		log.Warn("reWrite", "orig_number", origBlock.Number(), "orig_hash", origBlock.Hash().String(), "new_number", newBlock.Number(), "new_hash", newBlock.Hash().String())
		var err error
		m.currentEvmState, err = m.createEvmState(newBlock.Meta().Root, headerTipHash)
		if err != nil {
			return false, err
		}
		if err = m.reWriteBlockIndexTo(origBlock, newBlock); err != nil {
			return false, err
		}
	}
	return true, nil
}

// includeCrossShardTxList already locked
func (m *MinorBlockChain) includeCrossShardTxList(evmState *state.StateDB, descendantRootHeader *types.RootBlockHeader, ancestorRootHeader *types.RootBlockHeader) (*types.RootBlockHeader, error) {
	if descendantRootHeader == ancestorRootHeader {
		return ancestorRootHeader, nil
	}
	rHeader := descendantRootHeader
	headerList := make([]*types.RootBlockHeader, 0)
	for rHeader.Hash() != ancestorRootHeader.Hash() {
		if rHeader.Number <= ancestorRootHeader.Number {
			return nil, errors.New("root height small than ancestor root height")
		}
		headerList = append(headerList, rHeader)
		rHeader = m.getRootBlockHeaderByHash(rHeader.ParentHash)
	}

	for index := len(headerList) - 1; index >= 0; index-- {
		_, err := m.runOneCrossShardTxListByRootBlockHash(headerList[index].Hash(), evmState)
		if err != nil {
			return nil, err
		}
		if evmState.GetGasUsed().Cmp(evmState.GetGasLimit()) == 0 {
			return headerList[index], nil
		}
	}
	return descendantRootHeader, nil
}

func (m *MinorBlockChain) runOneCrossShardTxListByRootBlockHash(hash common.Hash, evmState *state.StateDB) ([]*types.CrossShardTransactionDeposit, error) {
	// already locked
	txList, err := m.getCrossShardTxListByRootBlockHash(hash)
	if err != nil {
		return nil, err
	}

	localFeeRate := getLocalFeeRate(evmState.GetQuarkChainConfig())
	for _, tx := range txList {
		evmState.AddBalance(tx.To.Recipient, tx.Value.Value)

		if tx.GasPrice.Value.Uint64() != 0 {
			evmState.SetGasUsed(new(big.Int).Add(evmState.GetGasUsed(), params.GtxxShardCost))
		}
		if evmState.GetGasUsed().Cmp(evmState.GetGasLimit()) > 0 {
			evmState.SetGasUsed(evmState.GetGasLimit())
		}

		xShardFee := new(big.Int).Mul(params.GtxxShardCost, tx.GasPrice.Value)
		xShardFee = qkcCommon.BigIntMulBigRat(xShardFee, localFeeRate)
		evmState.AddBlockFee(xShardFee)
		evmState.AddBalance(evmState.GetBlockCoinbase(), xShardFee)
	}
	evmState.SetXShardReceiveGasUsed(evmState.GetGasUsed())
	return txList, nil
}

// GetTransactionByHash get tx by hash
func (m *MinorBlockChain) GetTransactionByHash(hash common.Hash) (*types.MinorBlock, uint32) {
	_, mHash, txIndex := rawdb.ReadTransaction(m.db, hash)
	if mHash == qkcCommon.EmptyHash { //TODO need? for test???
		txs := make([]*types.Transaction, 0)
		tx := m.txPool.all.Get(hash)
		if tx == nil {
			return nil, 0
		}
		txs = append(txs, tx)
		temp := types.NewMinorBlock(&types.MinorBlockHeader{}, &types.MinorBlockMeta{}, txs, nil, nil)
		return temp, 0
	}
	return m.GetMinorBlock(mHash), txIndex
}

// GetTransactionReceipt get tx receipt by hash for slave
func (m *MinorBlockChain) GetTransactionReceipt(hash common.Hash) (*types.MinorBlock, uint32, *types.Receipt) {
	block, index := m.GetTransactionByHash(hash)
	if block == nil {
		return nil, 0, nil
	}
	receipts := m.GetReceiptsByHash(block.Hash())
	for _, receipt := range receipts {
		if receipt.TxHash == hash {
			return block, index, receipt
		}
	}
	return nil, 0, nil
}

// GetShardStatus show shardStatus
func (m *MinorBlockChain) GetShardStatus() (*rpc.ShardStatus, error) {
	// getBlockCountByHeight have lock
	cblock := m.CurrentBlock()
	cutoff := cblock.IHeader().GetTime() - 60

	txCount := uint32(0)
	blockCount := uint32(0)
	staleBlockCount := uint32(0)
	lastBlockTime := uint64(0)
	for cblock.IHeader().NumberU64() > 0 && cblock.IHeader().GetTime() > cutoff {
		txCount += uint32(len(cblock.GetTransactions()))
		blockCount++
		tStale := uint32(m.getBlockCountByHeight(cblock.Header().Number))
		staleBlockCount += tStale
		if tStale >= 1 {
			staleBlockCount--
		}
		cblock = m.GetMinorBlock(cblock.IHeader().GetParentHash())
		if cblock == nil {
			return nil, ErrMinorBlockIsNil
		}
		if lastBlockTime == 0 {
			lastBlockTime = m.CurrentBlock().IHeader().GetTime() - cblock.IHeader().GetTime()
		}
	}
	if staleBlockCount < 0 {
		return nil, errors.New("staleBlockCount should >=0")
	}
	pendingCount := m.txPool.PendingCount()
	cblock = m.CurrentBlock()
	return &rpc.ShardStatus{
		Branch:             m.branch,
		Height:             cblock.IHeader().NumberU64(),
		Difficulty:         cblock.IHeader().GetDifficulty(),
		CoinbaseAddress:    cblock.IHeader().GetCoinbase(),
		Timestamp:          cblock.IHeader().GetTime(),
		TxCount60s:         txCount,
		PendingTxCount:     uint32(pendingCount),
		TotalTxCount:       *m.getTotalTxCount(cblock.Hash()),
		BlockCount60s:      blockCount,
		StaleBlockCount60s: staleBlockCount,
		LastBlockTime:      lastBlockTime,
	}, nil
}

// EstimateGas estimate gas for this tx
func (m *MinorBlockChain) EstimateGas(tx *types.Transaction, fromAddress account.Address) (uint32, error) {
	// no need to locks
	if tx.EvmTx.Gas() > math.MaxUint32 {
		return 0, errors.New("gas > maxInt31")
	}
	evmTxStartGas := uint32(tx.EvmTx.Gas())
	lo := uint32(21000 - 1)
	currentState, err := m.State()
	if err != nil {
		return 0, err
	}
	if currentState.GetGasLimit().Uint64() > math.MaxInt32 {
		return 0, errors.New("gasLimit > MaxInt32")
	}
	hi := uint32(currentState.GetGasLimit().Uint64())
	if evmTxStartGas > 21000 {
		hi = evmTxStartGas
	}
	cap := hi

	runTx := func(gas uint32) error {
		evmState := currentState.Copy()
		evmState.SetGasUsed(new(big.Int).SetUint64(0))
		uint64Gas := uint64(gas)
		evmTx, err := m.validateTx(tx, evmState, &fromAddress, &uint64Gas)
		if err != nil {
			return err
		}

		gp := new(GasPool).AddGas(evmState.GetGasLimit().Uint64())
		to := evmTx.EvmTx.To()
		msg := types.NewMessage(fromAddress.Recipient, to, evmTx.EvmTx.Nonce(), evmTx.EvmTx.Value(), evmTx.EvmTx.Gas(), evmTx.EvmTx.GasPrice(), evmTx.EvmTx.Data(), false, tx.EvmTx.FromShardID(), tx.EvmTx.ToShardID())
		evmState.SetFullShardKey(tx.EvmTx.ToFullShardKey())
		context := NewEVMContext(msg, m.CurrentBlock().IHeader().(*types.MinorBlockHeader), m)
		evmEnv := vm.NewEVM(context, evmState, m.ethChainConfig, m.vmConfig)

		localFee := getLocalFeeRate(m.clusterConfig.Quarkchain)
		_, _, _, err = ApplyMessage(evmEnv, msg, gp, localFee)
		return err
	}

	for lo+1 < hi {
		mid := (lo + hi) / 2
		if runTx(mid) == nil {
			hi = mid
		} else {
			lo = mid
		}
	}
	if hi == cap && runTx(hi) == nil {
		return 0, nil
	}
	return hi, nil
}

// GasPrice gas price
func (m *MinorBlockChain) GasPrice() (uint64, error) {
	// no need to lock
	currHead := m.CurrentBlock().Hash()
	if currHead == m.gasPriceSuggestionOracle.LastHead {
		return m.gasPriceSuggestionOracle.LastPrice, nil
	}
	currHeight := m.CurrentBlock().NumberU64()
	startHeight := int64(currHeight) - int64(m.gasPriceSuggestionOracle.CheckBlocks) + 1
	if startHeight < 3 {
		startHeight = 3
	}
	prices := make([]uint64, 0)
	for index := startHeight; index < int64(currHeight+1); index++ {
		block, ok := m.GetBlockByNumber(uint64(index)).(*types.MinorBlock)
		if !ok {
			log.Error(m.logInfo, "failed to get block", index)
			return 0, errors.New("failed to get block")
		}
		tempPreBlockPrices := make([]uint64, 0)
		for _, tx := range block.GetTransactions() {
			tempPreBlockPrices = append(tempPreBlockPrices, tx.EvmTx.GasPrice().Uint64())
		}
		prices = append(prices, tempPreBlockPrices...)
	}
	if len(prices) == 0 {
		return 0, errors.New("len(prices)==0")
	}

	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	price := prices[(len(prices)-1)*int(m.gasPriceSuggestionOracle.Percentile)/100]
	m.gasPriceSuggestionOracle.LastPrice = price
	m.gasPriceSuggestionOracle.LastHead = currHead
	return price, nil
}

func (m *MinorBlockChain) getBlockCountByHeight(height uint64) uint64 {
	m.mu.RLock() //to lock heightToMinorBlockHashes
	defer m.mu.RUnlock()
	rs, ok := m.heightToMinorBlockHashes[height]
	if !ok {
		return 0
	}
	return uint64(len(rs))
}

// reWriteBlockIndexTo : already locked
func (m *MinorBlockChain) reWriteBlockIndexTo(oldBlock *types.MinorBlock, newBlock *types.MinorBlock) error {
	if oldBlock == nil {
		oldBlock = m.CurrentBlock()
	}
	if oldBlock.NumberU64() < newBlock.NumberU64() {
		return m.reorg(oldBlock, newBlock)
	}
	return m.SetHead(newBlock.NumberU64())
}
func (m *MinorBlockChain) GetBranch() account.Branch {
	return m.branch
}

func (m *MinorBlockChain) GetRootTip() *types.RootBlockHeader {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rootTip
}
func encodeAddressTxKey(adddress account.Address, height uint64, index int, crossShard bool) []byte {
	crossShardBytes := make([]byte, 0)
	if crossShard {
		crossShardBytes = append(crossShardBytes, byte(0))
	} else {
		crossShardBytes = append(crossShardBytes, byte(1))
	}
	addressBytes, err := serialize.SerializeToBytes(adddress)
	if err != nil {
		panic(err)
	}
	heightBytes := qkcCommon.Uint32ToBytes(uint32(height))
	indexBytes := qkcCommon.Uint32ToBytes(uint32(index))

	rs := make([]byte, 0)
	rs = append(rs, []byte("addr_")...)
	rs = append(rs, addressBytes...)
	rs = append(rs, heightBytes...)
	rs = append(rs, crossShardBytes...)
	rs = append(rs, indexBytes...)
	return rs
}
func decodeAddressTxKey(data []byte) (uint64, bool, uint32, error) {
	// 38=5+24+4+1+4
	// 5="addr_"
	// 24=len(account.Address)
	// 4=height
	// 1=isCrossShard
	// 4=index
	if len(data) != 38 {
		return 0, false, 0, errors.New("input err")
	}
	height := qkcCommon.BytesToUint32(data[5+24 : 5+24+4])
	flag := false
	if data[5+24+4] == byte(0) {
		flag = true
	}
	index := qkcCommon.BytesToUint32(data[5+24+4+1:])
	return uint64(height), flag, index, nil

}

func (m *MinorBlockChain) putTxIndexFromBlock(batch rawdb.DatabaseWriter, block types.IBlock) error {
	rawdb.WriteBlockContentLookupEntries(batch, block) // put eth's tx lookup
	if !m.clusterConfig.EnableTransactionHistory {
		return nil
	}
	minorBlock, ok := block.(*types.MinorBlock)
	if !ok {
		return errors.New("minor block is nil")
	}
	for index, tx := range minorBlock.Transactions() { // put qkc's inshard tx
		if err := m.putTxHistoryIndex(tx, minorBlock.Number(), index); err != nil {
			return err
		}
	}
	return m.putTxHistoryIndexFromBlock(minorBlock) // put qkc's xshard tx
}
func (m *MinorBlockChain) removeTxIndexFromBlock(db rawdb.DatabaseDeleter, txs types.Transactions) error {
	slovedBlock := make(map[common.Hash]bool)
	for _, tx := range txs {
		blockHash, _ := rawdb.ReadBlockContentLookupEntry(m.db, tx.Hash())
		rawdb.DeleteBlockContentLookupEntry(db, tx.Hash()) //delete eth's tx lookup

		if !m.clusterConfig.EnableTransactionHistory {
			continue
		}

		if _, ok := slovedBlock[blockHash]; ok {
			continue
		}
		slovedBlock[blockHash] = true
		block, ok := m.GetBlock(blockHash).(*types.MinorBlock) // find old block
		if !ok {
			return errors.New("get minor block err")
		}
		for oldBlockTxIndex, oldBlockTx := range block.Transactions() { // delete qkc's oldBlock's tx
			if err := m.removeTxHistoryIndex(oldBlockTx, block.Number(), oldBlockTxIndex); err != nil {
				return err
			}
		}
		if err := m.removeTxHistoryIndexFromBlock(block); err != nil { //delete qkc's crossShard tx
			return err
		}
	}
	return nil
}

func bytesSubOne(data []byte) []byte {
	bigData := new(big.Int).SetBytes(data)
	return bigData.Sub(bigData, new(big.Int).SetUint64(1)).Bytes()
}
func bytesAddOne(data []byte) []byte {
	bigData := new(big.Int).SetBytes(data)
	return bigData.Add(bigData, new(big.Int).SetUint64(1)).Bytes()
}

func (m *MinorBlockChain) getPendingTxByAddress(address account.Address) ([]*rpc.TransactionDetail, []byte, error) {
	txList := make([]*rpc.TransactionDetail, 0)
	txs := m.txPool.GetPendingTxsFromAddress(address.Recipient)
	txs = append(txs, m.txPool.GetQueueTxsFromAddress(address.Recipient)...)

	for _, tx := range txs {
		to := new(account.Address)
		if tx.EvmTx.To() == nil {
			to = nil
		} else {
			to.Recipient = *tx.EvmTx.To()
			to.FullShardKey = tx.EvmTx.ToFullShardKey()
		}
		txList = append(txList, &rpc.TransactionDetail{
			TxHash:      tx.EvmTx.Hash(),
			FromAddress: address,
			ToAddress:   to,
			Value:       serialize.Uint256{Value: tx.EvmTx.Value()},
			BlockHeight: 0,
			Timestamp:   0,
			Success:     false,
		})
	}
	return txList, []byte{}, nil
}
func (m *MinorBlockChain) GetTransactionByAddress(address account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	if !m.clusterConfig.EnableTransactionHistory {
		return []*rpc.TransactionDetail{}, []byte{}, nil
	}

	if bytes.Equal(start, []byte{1}) { //get pending tx
		return m.getPendingTxByAddress(address)
	}
	endEncodeAddressTxKey := make([]byte, 0)
	endEncodeAddressTxKey = append(endEncodeAddressTxKey, []byte("addr_")...)
	tAdd, err := serialize.SerializeToBytes(address)
	if err != nil {
		panic(err)
	}
	endEncodeAddressTxKey = append(endEncodeAddressTxKey, tAdd...)
	originalStartBytes := bytesAddOne(endEncodeAddressTxKey)

	next := make([]byte, 0)
	next = append(next, endEncodeAddressTxKey...)

	if len(start) == 0 || bytes.Compare(start, originalStartBytes) > 0 {
		start = originalStartBytes
	}

	qkcDB, ok := m.db.(*qkcdb.RDBDatabase)
	if !ok {
		return nil, nil, errors.New("only support qkcdb now")
	}

	txList := make([]*rpc.TransactionDetail, 0)
	it := qkcDB.NewIterator()
	it.SeekForPrev(start)
	for it.Valid() {

		if bytes.Compare(it.Key().Data(), endEncodeAddressTxKey) < 0 {
			break
		}

		height, crossShard, index, err := decodeAddressTxKey(it.Key().Data())
		if err != nil {
			return nil, nil, err
		}
		if crossShard {
			mBlock, ok := m.GetBlockByNumber(height).(*types.MinorBlock)
			if !ok {
				log.Error(m.logInfo, "get minor block fialed height", height)
				return nil, nil, errors.New("get minBlock failed")
			}
			xShardReceiveTxList := rawdb.ReadConfirmedCrossShardTxList(m.db, mBlock.Hash())
			if index >= uint32(len(xShardReceiveTxList.TXList)) {
				return nil, nil, errors.New("tx's index bigger than txs's len ")
			}
			tx := xShardReceiveTxList.TXList[index]
			txList = append(txList, &rpc.TransactionDetail{
				TxHash:      tx.TxHash,
				FromAddress: tx.From,
				ToAddress:   &tx.To,
				Value:       serialize.Uint256{Value: tx.Value.Value},
				BlockHeight: height,
				Timestamp:   mBlock.IHeader().GetTime(),
				Success:     true,
			})
		} else {
			mBlock, ok := m.GetBlockByNumber(height).(*types.MinorBlock)
			if !ok {
				log.Error(m.logInfo, "get minor block fialed height", height)
				return nil, nil, errors.New("get minBlock failed")
			}
			tx := mBlock.Transactions()[index]
			receipt, _, _ := rawdb.ReadReceipt(m.db, tx.Hash())
			evmTx := tx.EvmTx
			sender, err := types.Sender(types.MakeSigner(m.clusterConfig.Quarkchain.NetworkID), evmTx)
			if err != nil {
				return nil, nil, err
			}
			to := account.Address{
				FullShardKey: evmTx.ToFullShardKey(),
			}
			if tx.EvmTx.To() != nil {
				to.Recipient = *tx.EvmTx.To()
			}
			succFlag := false
			if receipt.Status == 1 {
				succFlag = true
			}
			txList = append(txList, &rpc.TransactionDetail{
				TxHash: tx.Hash(),
				FromAddress: account.Address{
					Recipient:    sender,
					FullShardKey: evmTx.FromFullShardKey(),
				},
				ToAddress:   &to,
				Value:       serialize.Uint256{Value: evmTx.Value()},
				BlockHeight: height,
				Timestamp:   mBlock.IHeader().GetTime(),
				Success:     succFlag,
			})
		}
		next = bytesSubOne(it.Key().Data())
		limit--
		if limit == 0 {
			break
		}
		it.Prev()
	}
	return txList, next, nil
}

func (m *MinorBlockChain) GetLogsByAddressAndTopic(start uint64, end uint64, addresses []account.Address, topics [][]common.Hash) ([]*types.Log, error) {
	addressValue := make([]common.Address, 0)
	mapFullShardKey := make(map[uint32]bool)
	for _, v := range addresses {
		mapFullShardKey[v.FullShardKey] = true
		addressValue = append(addressValue, v.Recipient)
	}
	if len(mapFullShardKey) != 1 {
		return nil, errors.New("should have same full_shard_key for the given addresses")
	}
	fullShardID, err := m.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(addresses[0].FullShardKey)
	if err != nil {
		return nil, err
	}
	if fullShardID != m.branch.Value {
		return nil, errors.New("not in this branch")
	}
	topicsValue := make([][]common.Hash, 0)
	for _, v := range topics {
		topicsValue = append(topicsValue, v)
	}
	filter := NewRangeFilter(m, start, end, addressValue, topicsValue)
	return filter.Logs()
}
func (m *MinorBlockChain) putTxIndexDB(key []byte) error {
	err := m.db.Put(key, []byte("1")) //TODO????
	return err
}
func (m *MinorBlockChain) deleteTxIndexDB(key []byte) error {
	return m.db.Delete(key)
}
func (m *MinorBlockChain) updateTxHistoryIndex(tx *types.Transaction, height uint64, index int, f func(key []byte) error) error {
	evmtx := tx.EvmTx
	sender, err := types.Sender(types.MakeSigner(m.clusterConfig.Quarkchain.NetworkID), evmtx)
	if err != nil {
		return err
	}
	addr := account.Address{
		Recipient:    sender,
		FullShardKey: evmtx.FromFullShardKey(),
	}
	key := encodeAddressTxKey(addr, height, index, false)
	if err := f(key); err != nil {
		return err
	}
	if evmtx.To() != nil && m.branch.IsInBranch(evmtx.ToFullShardKey()) {
		toAddr := account.Address{
			Recipient:    *evmtx.To(),
			FullShardKey: evmtx.ToFullShardKey(),
		}
		key := encodeAddressTxKey(toAddr, height, index, false)
		if err := f(key); err != nil {
			return err
		}
	}
	return nil
}
func (m *MinorBlockChain) putTxHistoryIndex(tx *types.Transaction, height uint64, index int) error {
	return m.updateTxHistoryIndex(tx, height, index, m.putTxIndexDB)
}
func (m *MinorBlockChain) removeTxHistoryIndex(tx *types.Transaction, height uint64, index int) error {
	return m.updateTxHistoryIndex(tx, height, index, m.deleteTxIndexDB)
}

func (m *MinorBlockChain) updateTxHistoryIndexFromBlock(block *types.MinorBlock, f func([]byte) error) error {
	xShardReceiveTxList := rawdb.ReadConfirmedCrossShardTxList(m.db, block.Hash())
	for index, tx := range xShardReceiveTxList.TXList {
		if bytes.Equal(tx.TxHash.Bytes(), common.Hash{}.Bytes()) {
			continue // coinbase reward for root block miner
		}
		key := encodeAddressTxKey(tx.To, block.Number(), index, true)
		if err := f(key); err != nil {
			return err
		}
	}
	return nil
}
func (m *MinorBlockChain) putTxHistoryIndexFromBlock(block *types.MinorBlock) error {
	return m.updateTxHistoryIndexFromBlock(block, m.putTxIndexDB)
}
func (m *MinorBlockChain) removeTxHistoryIndexFromBlock(block *types.MinorBlock) error {
	return m.updateTxHistoryIndexFromBlock(block, m.deleteTxIndexDB)
}

func (m *MinorBlockChain) ReadCrossShardTxList(hash common.Hash) *types.CrossShardTransactionDepositList {
	if data, ok := m.crossShardTxListCache.Get(hash); ok {
		return data.(*types.CrossShardTransactionDepositList)
	}
	data := rawdb.ReadCrossShardTxList(m.db, hash)
	if data != nil {
		m.crossShardTxListCache.Add(hash, data)
		return data
	}
	return nil
}
