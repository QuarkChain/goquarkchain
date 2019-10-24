package core

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/qkcdb"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	MAX_FUTURE_TX_NONCE                   = uint64(64)
	ALLOWED_FUTURE_BLOCKS_TIME_VALIDATION = uint64(15)
	addressTxKey                          = []byte("iaddr")
	allTxKey                              = []byte("iall")
	ErrorTxContinue                       = errors.New("apply tx continue")
	ErrorTxBreak                          = errors.New("apply tx break")
)

type GetTxDetailType byte

const (
	GetAllTransaction GetTxDetailType = iota
	GetTxFromAddress
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

func powerBigInt(data *big.Int, p uint64) *big.Int {
	t := new(big.Int).Set(data)
	if p == 0 {
		return new(big.Int).SetUint64(1)
	}
	for index := 0; index < int(p)-1; index++ {
		t.Mul(t, t)
	}
	return t
}

func (m *MinorBlockChain) getCoinbaseAmount(height uint64) *types.TokenBalances {
	epoch := height / m.shardConfig.EpochInterval
	balances, ok := m.coinbaseAmountCache[epoch]
	if !ok {
		decayNumerator := powerBigInt(m.clusterConfig.Quarkchain.BlockRewardDecayFactor.Num(), epoch)
		decayDenominator := powerBigInt(m.clusterConfig.Quarkchain.BlockRewardDecayFactor.Denom(), epoch)
		coinbaseAmount := new(big.Int).Mul(m.shardConfig.CoinbaseAmount, m.clusterConfig.Quarkchain.LocalFeeRate.Num())
		coinbaseAmount = new(big.Int).Mul(coinbaseAmount, decayNumerator)
		coinbaseAmount = new(big.Int).Div(coinbaseAmount, m.clusterConfig.Quarkchain.LocalFeeRate.Denom())
		coinbaseAmount = new(big.Int).Div(coinbaseAmount, decayDenominator)
		data := make(map[uint64]*big.Int)
		data[m.clusterConfig.Quarkchain.GetDefaultChainTokenID()] = coinbaseAmount
		balances = types.NewTokenBalancesWithMap(data)
		m.coinbaseAmountCache[epoch] = balances
	}
	return balances.Copy()
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

	hashList := new(rawdb.HashList)
	hashList.HList = make([]common.Hash, 0)
	for _, tx := range xShardReceiveTxList {
		hashList.HList = append(hashList.HList, tx.TxHash)
	}
	m.putXShardDepositHashList(mBlock.Hash(), hashList)
	return nil
}

func (m *MinorBlockChain) updateTip(state *state.StateDB, block *types.MinorBlock) (bool, error) {
	preRootHeader := m.getRootBlockHeaderByHash(block.PrevRootBlockHash())
	if preRootHeader == nil {
		return false, errors.New("missing prev block")
	}

	tipPrevRootHeader := m.getRootBlockHeaderByHash(m.CurrentBlock().PrevRootBlockHash())
	// Don't update tip if the block depends on a root block that is not root_tip or root_tip's ancestor
	if !m.isSameRootChain(m.rootTip, preRootHeader) {
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
			updateTip = preRootHeader.Number > tipPrevRootHeader.Number
		}
	}
	if updateTip {
		m.currentEvmState = state
	}
	return updateTip, nil
}

func (m *MinorBlockChain) validateTx(tx *types.Transaction, evmState *state.StateDB, fromAddress *account.Address, gas, xShardGasLimit *uint64) (*types.Transaction, error) {
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

	if xShardGasLimit == nil {
		x := m.xShardGasLimit.Uint64()
		xShardGasLimit = &x
	}
	if evmTx.IsCrossShard() && evmTx.Gas() > *xShardGasLimit {
		return nil, fmt.Errorf("xshard evm tx exceeds xshard gasLimit %v %v", evmTx.Gas(), xShardGasLimit)
	}

	if evmState.GetTimeStamp() < m.clusterConfig.Quarkchain.EnableEvmTimeStamp {
		if evmTx.To() == nil || len(evmTx.Data()) != 0 {
			return nil, errors.New("smart contract tx is not allowed before evm is enabled")
		}
	}
	var sender account.Recipient
	if fromAddress == nil {
		sender, err = tx.Sender(types.NewEIP155Signer(m.clusterConfig.Quarkchain.NetworkID))
		if err != nil {
			return nil, err
		}
	} else {
		sender = fromAddress.Recipient
	}

	tx = &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	reqNonce := evmState.GetNonce(sender)
	if reqNonce < evmTx.Nonce() && evmTx.Nonce() < reqNonce+MAX_FUTURE_TX_NONCE { //TODO fix
		return tx, nil
	}
	evmState.SetQuarkChainConfig(m.clusterConfig.Quarkchain)
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

	if err := m.putMinorBlock(gBlock, []*types.CrossShardTransactionDeposit{}); err != nil {
		return nil, err
	}
	m.putRootBlock(rBlock, nil)
	rawdb.WriteGenesisBlock(m.db, rBlock.Hash(), gBlock) // key:rootBlockHash value:minorBlock
	m.CommitMinorBlockByHash(gBlock.Hash())
	if m.initialized {
		return gBlock, nil
	}
	m.rootTip = rBlock.Header()
	m.confirmedHeaderTip = nil
	m.currentEvmState, err = m.StateAt(gBlock.Meta().Root)
	if err != nil {
		return nil, err
	}

	m.initialized = true
	return gBlock, nil
}

// GetTransactionCount get txCount for addr
func (m *MinorBlockChain) GetTransactionCount(recipient account.Recipient, hash *common.Hash) (uint64, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(hash)
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
	fmt.Println("????", m.branch.Value, confirmedHeaderTip == nil)
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
	fmt.Println("rBlock.hahs", m.branch.Value, rBlock.Hash().String(), rBlock.NumberU64(), confirmedHeaderTip.Hash().String(), confirmedHeaderTip.Number)

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
	m.currentEvmState, err = m.StateAt(block.Meta().Root)
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
		return nil, ErrMinorBlockIsNil
	}
	recipient := mHeader.GetCoinbase().Recipient
	evmState, err := m.stateAtWithSenderDisallowMap(preMinorBlock, &recipient)
	if err != nil {
		return nil, err
	}
	if ephemeral {
		evmState = evmState.Copy()
	}
	m.setEvmStateWithHeader(evmState, mHeader.(*types.MinorBlockHeader))
	return evmState, nil
}

func (m *MinorBlockChain) setEvmStateWithHeader(evmState *state.StateDB, header *types.MinorBlockHeader) {
	evmState.SetTimeStamp(header.GetTime())
	evmState.SetBlockNumber(header.NumberU64())
	evmState.SetBlockCoinbase(header.GetCoinbase().Recipient)
	evmState.SetGasLimit(header.GetGasLimit())
	evmState.SetQuarkChainConfig(m.clusterConfig.Quarkchain)
}
func (m *MinorBlockChain) runBlock(block *types.MinorBlock) (*state.StateDB, types.Receipts, []*types.Log, uint64,
	[]*types.CrossShardTransactionDeposit, error) {

	parent := m.GetMinorBlock(block.ParentHash())
	if qkcCommon.IsNil(parent) {
		log.Error(m.logInfo, "err-runBlock", ErrRootBlockIsNil, "parentHash", block.ParentHash().String())
		return nil, nil, nil, 0, nil, ErrRootBlockIsNil
	}
	xShardReceiveTxList := make([]*types.CrossShardTransactionDeposit, 0)
	preEvmState, err := m.getEvmStateForNewBlock(block.Header(), false)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}
	evmState := preEvmState.Copy()
	xTxList, txCursorInfo, xShardReceipts, err := m.RunCrossShardTxWithCursor(evmState, block)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}
	evmState.SetTxCursorInfo(txCursorInfo)
	xShardReceiveTxList = append(xShardReceiveTxList, xTxList...)
	xShardGasLimit := block.Meta().XShardGasLimit.Value
	if evmState.GetGasUsed().Cmp(xShardGasLimit) == -1 {
		left := new(big.Int).Sub(xShardGasLimit, evmState.GetGasUsed())
		evmState.SetGasLimit(new(big.Int).Sub(evmState.GetGasLimit(), left))
	}
	receipts, logs, usedGas, err := m.processor.Process(block, evmState, m.vmConfig)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}
	receipts = append(receipts, xShardReceipts...)
	return evmState, receipts, logs, usedGas, xShardReceiveTxList, nil
}

// FinalizeAndAddBlock finalize minor block and add it to chain
// only used in test now
func (m *MinorBlockChain) FinalizeAndAddBlock(block *types.MinorBlock) (*types.MinorBlock, types.Receipts, error) {
	evmState, receipts, _, _, _, err := m.runBlock(block) // will lock
	if err != nil {
		return nil, nil, err
	}
	coinbaseAmount := m.getCoinbaseAmount(block.Header().NumberU64())
	coinbaseAmount.Add(evmState.GetBlockFee())

	block.Finalize(receipts, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), coinbaseAmount, evmState.GetTxCursorInfo())
	_, err = m.InsertChain([]types.IBlock{block}, false) // will lock
	if err != nil {
		return nil, nil, err
	}
	return block, receipts, nil
}

// AddTx add tx to txPool
func (m *MinorBlockChain) AddTx(tx *types.Transaction) error {
	return m.txPool.AddLocal(tx)
}

func (m *MinorBlockChain) getEvmStateByHeight(hash *common.Hash) (*state.StateDB, error) {
	if hash == nil {
		t := m.CurrentBlock().Hash()
		hash = &t
	}

	mBlock := m.GetMinorBlock(*hash)
	if mBlock == nil {
		return nil, fmt.Errorf("no such block:hash %v", (*hash).String())
	}

	evmState, err := m.StateAt(mBlock.GetMetaData().Root)
	if err != nil {
		return nil, err
	}
	return evmState, nil
}

// GetBalance get balance for address
func (m *MinorBlockChain) GetBalance(recipient account.Recipient, hash *common.Hash) (*types.TokenBalances, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(hash)
	if err != nil {
		return nil, err
	}
	return evmState.GetBalances(recipient), nil
}

// GetCode get code for addr
func (m *MinorBlockChain) GetCode(recipient account.Recipient, hash *common.Hash) ([]byte, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(hash)
	if err != nil {
		return nil, err
	}
	return evmState.GetCode(recipient), nil
}

// GetStorageAt get storage for addr
func (m *MinorBlockChain) GetStorageAt(recipient account.Recipient, key common.Hash, hash *common.Hash) (common.Hash, error) {
	// no need to lock
	evmState, err := m.getEvmStateByHeight(hash)
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
	evmState, err := m.stateAtWithSenderDisallowMap(mBlock, nil)
	if err != nil {
		return nil, err
	}
	state := evmState.Copy()
	state.SetGasUsed(new(big.Int).SetUint64(0))
	var gas uint64
	if tx.EvmTx.Gas() != 0 {
		gas = tx.EvmTx.Gas()
	} else {
		gas = state.GetGasLimit().Uint64()
	}
	evmTx, err := m.validateTx(tx, state, fromAddress, &gas, nil)
	if err != nil {
		return nil, err
	}
	gp := new(GasPool).AddGas(mBlock.Header().GetGasLimit().Uint64())

	to := evmTx.EvmTx.To()
	toFullShardKey := tx.EvmTx.ToFullShardKey()
	msg := types.NewMessage(fromAddress.Recipient, to, evmTx.EvmTx.Nonce(), evmTx.EvmTx.Value(), evmTx.EvmTx.Gas(),
		evmTx.EvmTx.GasPrice(), evmTx.EvmTx.Data(), false, tx.EvmTx.FromFullShardKey(), &toFullShardKey,
		tx.EvmTx.TransferTokenID(), tx.EvmTx.GasTokenID())
	state.SetFullShardKey(tx.EvmTx.ToFullShardKey())
	state.SetQuarkChainConfig(m.clusterConfig.Quarkchain)

	context := NewEVMContext(msg, m.CurrentBlock().IHeader().(*types.MinorBlockHeader), m)
	evmEnv := vm.NewEVM(context, state, m.ethChainConfig, m.vmConfig)
	ret, _, _, err := ApplyMessage(evmEnv, msg, gp)
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

	header = m.CurrentBlock().Header()
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
	//TODO-master
	//headers := m.GetUnconfirmedHeaderList() // have lock
	//for _, header := range headers {
	//	//amount += header.CoinbaseAmount.Value.Uint64()
	//}
	return amount
}

func (m *MinorBlockChain) addTransactionToBlock(block *types.MinorBlock, evmState *state.StateDB) (*types.MinorBlock, types.Receipts, error) {
	// have locked by upper call
	pending, err := m.txPool.Pending() // txpool already locked
	if err != nil {
		return nil, nil, err
	}
	txs, err := types.NewTransactionsByPriceAndNonce(types.NewEIP155Signer(uint32(m.Config().NetworkID)), pending)
	if err != nil {
		return nil, nil, err
	}
	gp := new(GasPool).AddGas(block.Header().GetGasLimit().Uint64())
	usedGas := new(uint64)

	receipts := make([]*types.Receipt, 0)
	txsInBlock := make([]*types.Transaction, 0)

	stateT := evmState
	txIndex := 0
	for stateT.GetGasUsed().Cmp(stateT.GetGasLimit()) < 0 {
		tx := txs.Peek()
		// Pop skip all txs about this account
		//Shift skip this tx ,goto next tx about this account
		if err := m.checkTxBeforeApply(stateT, tx, block.Header()); err != nil {
			if err == ErrorTxBreak {
				break
			} else if err == ErrorTxContinue {
				txs.Pop()
				continue
			}

		}
		stateT.Prepare(tx.Hash(), block.Hash(), txIndex)
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
			txIndex++
		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			if err := txs.Shift(); err != nil {
				return nil, nil, errors.New("txs.Shift error")
			}
		}

	}
	bHeader := block.Header()
	return types.NewMinorBlock(bHeader, block.Meta(), txsInBlock, receipts, nil), receipts, nil
}

func (m *MinorBlockChain) checkTxBeforeApply(stateT *state.StateDB, tx *types.Transaction, header *types.MinorBlockHeader) error {
	if tx == nil {
		return ErrorTxBreak
	}
	diff := new(big.Int).Sub(stateT.GetGasLimit(), stateT.GetGasUsed())
	if tx.EvmTx.Gas() > diff.Uint64() {
		return ErrorTxContinue
	}
	if tx.EvmTx.GasPrice().Cmp(m.clusterConfig.Quarkchain.MinMiningGasPrice) < 0 {
		return ErrorTxContinue
	}
	if header.Time < m.clusterConfig.Quarkchain.EnableEvmTimeStamp {
		if tx.EvmTx.To() == nil || len(tx.EvmTx.Data()) != 0 {
			return ErrorTxContinue
		}
	}
	return nil
}

// CreateBlockToMine create block to mine
func (m *MinorBlockChain) CreateBlockToMine(createTime *uint64, address *account.Address, gasLimit, xShardGasLimit *big.Int,
	includeTx *bool) (*types.MinorBlock, error) {

	if includeTx == nil {
		t := true
		includeTx = &t
	}
	realCreateTime := uint64(time.Now().Unix())
	if createTime == nil {
		if realCreateTime < m.CurrentBlock().IHeader().GetTime()+1 {
			realCreateTime = m.CurrentBlock().IHeader().GetTime() + 1
		}
	} else {
		realCreateTime = *createTime
	}
	difficulty, err := m.engine.CalcDifficulty(m, realCreateTime, m.CurrentBlock().Header())
	if err != nil {
		return nil, err
	}
	prevBlock := m.CurrentBlock()
	if gasLimit == nil {
		gasLimit = m.gasLimit
	}
	if xShardGasLimit == nil {
		xShardGasLimit = m.xShardGasLimit
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
	currRootTipHash := m.rootTip.Hash()
	block := prevBlock.CreateBlockToAppend(&realCreateTime, difficulty, address, nil, gasLimit, xShardGasLimit,
		nil, nil, &currRootTipHash)
	evmState, err := m.getEvmStateForNewBlock(block.IHeader(), true)
	if err != nil {
		return nil, err
	}
	ancestorRootHeader := m.GetRootBlockByHash(m.CurrentBlock().Header().PrevRootBlockHash).Header()
	if !m.isSameRootChain(m.rootTip, ancestorRootHeader) {
		return nil, ErrNotSameRootChain
	}
	_, txCursor, xShardReceipts, err := m.RunCrossShardTxWithCursor(evmState, block)
	if err != nil {
		return nil, err
	}
	evmState.SetTxCursorInfo(txCursor)
	//Adjust inshard tx limit if xshard gas limit is not exhausted
	if evmState.GetGasUsed().Cmp(xShardGasLimit) == -1 {
		// ensure inshard gasLimit = 1/2 default gasLimit
		left := new(big.Int).Sub(xShardGasLimit, evmState.GetGasUsed())
		evmState.SetGasLimit(new(big.Int).Sub(evmState.GetGasLimit(), left))
	}
	receipts := make(types.Receipts, 0)
	if *includeTx {
		block, receipts, err = m.addTransactionToBlock(block, evmState)
		if err != nil {
			return nil, err
		}
	}
	receipts = append(receipts, xShardReceipts...)

	pureCoinbaseAmount := m.getCoinbaseAmount(block.Header().Number)
	bMap := pureCoinbaseAmount.GetBalanceMap()
	for k, v := range bMap {
		evmState.AddBalance(evmState.GetBlockCoinbase(), v, k)
	}
	pureCoinbaseAmount.Add(evmState.GetBlockFee())
	block.Finalize(receipts, evmState.IntermediateRoot(true), evmState.GetGasUsed(),
		evmState.GetXShardReceiveGasUsed(), pureCoinbaseAmount, evmState.GetTxCursorInfo())
	return block, nil
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

	if rBlock.Header().Version != 0 {
		return false, errors.New("incorrect root block version")
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
			log.Error(m.logInfo, "addrootBlock err-2", errXshardListNotHave, "h", h.String())
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
	if rBlock.Header().ToTalDifficulty.Cmp(m.rootTip.ToTalDifficulty) <= 0 {
		if !m.isSameRootChain(m.rootTip, m.GetRootBlockByHash(m.CurrentBlock().IHeader().(*types.MinorBlockHeader).GetPrevRootBlockHash()).Header()) {
			return false, ErrNotSameRootChain
		}
		return false, nil
	}

	m.mu.Lock()
	m.rootTip = rBlock.Header()
	m.confirmedHeaderTip = shardHeader
	m.mu.Unlock()
	origHeaderTip := m.CurrentBlock()
	if shardHeader != nil {
		origBlock := m.GetBlockByNumber(shardHeader.Number)
		if qkcCommon.IsNil(origBlock) || origBlock.Hash() != shardHeader.Hash() {
			log.Warn(m.logInfo, "ready to set current header height", shardHeader.Number, "hash", shardHeader.Hash().String(), "status", qkcCommon.IsNil(origBlock))
			m.currentBlock.Store(m.GetBlock(shardHeader.Hash()).(*types.MinorBlock))
		}
	}

	for !m.isSameRootChain(m.rootTip, m.getRootBlockHeaderByHash(m.CurrentBlock().Header().GetPrevRootBlockHash())) {
		if m.CurrentBlock().NumberU64() == 0 {
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
		preBlock := m.GetMinorBlock(m.CurrentBlock().Header().GetParentHash())
		log.Warn(m.logInfo, "ready to set currentHeader height", preBlock.Number(), "hash", preBlock.Hash().String())
		m.currentBlock.Store(preBlock)
	}

	if m.CurrentBlock().Hash() != origHeaderTip.Hash() {
		headerTipHash := m.CurrentBlock().Hash()
		origBlock := m.GetMinorBlock(origHeaderTip.Hash())
		newBlock := m.GetMinorBlock(headerTipHash)
		log.Warn("reWrite", "orig_number", origBlock.Number(), "orig_hash", origBlock.Hash().String(), "new_number", newBlock.Number(), "new_hash", newBlock.Hash().String())
		var err error
		if err = m.reWriteBlockIndexTo(origBlock, newBlock); err != nil {
			return false, err
		}
		m.currentEvmState, err = m.StateAt(newBlock.Meta().Root)
		if err != nil {
			return false, err
		}
	}
	return true, nil
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

// GetShardStats show shardStatus
func (m *MinorBlockChain) GetShardStats() (*rpc.ShardStatus, error) {
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
	preBlock := m.GetBlock(m.CurrentBlock().ParentHash())
	var preCoinbase *account.Recipient
	if qkcCommon.IsNil(preBlock) {
		if m.CurrentBlock().Number() != 0 {
			panic("bug fix")
		}
	} else {
		preCoinbase = new(account.Recipient)
		*preCoinbase = preBlock.IHeader().GetCoinbase().Recipient
	}
	currentState, err := m.stateAtWithSenderDisallowMap(m.CurrentBlock(), preCoinbase)
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
		evmTx, err := m.validateTx(tx, evmState, &fromAddress, &uint64Gas, nil)
		if err != nil {
			return err
		}

		gp := new(GasPool).AddGas(evmState.GetGasLimit().Uint64())
		to := evmTx.EvmTx.To()
		toFullShardKey := tx.EvmTx.ToFullShardKey()
		msg := types.NewMessage(fromAddress.Recipient, to, evmTx.EvmTx.Nonce(), evmTx.EvmTx.Value(), evmTx.EvmTx.Gas(),
			evmTx.EvmTx.GasPrice(), evmTx.EvmTx.Data(), false, tx.EvmTx.FromFullShardKey(), &toFullShardKey,
			tx.EvmTx.TransferTokenID(), tx.EvmTx.GasTokenID())
		evmState.SetFullShardKey(tx.EvmTx.ToFullShardKey())
		context := NewEVMContext(msg, m.CurrentBlock().IHeader().(*types.MinorBlockHeader), m)
		evmEnv := vm.NewEVM(context, evmState, m.ethChainConfig, m.vmConfig)

		_, _, _, err = ApplyMessage(evmEnv, msg, gp)
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
func (m *MinorBlockChain) GasPrice(tokenID uint64) (uint64, error) {
	if !m.clusterConfig.Quarkchain.IsAllowedTokenID(tokenID) {
		return 0, fmt.Errorf("no support tokenID %v", tokenID)
	}

	currHead := m.CurrentBlock().Hash()
	if data, ok := m.gasPriceSuggestionOracle.cache.Get(gasPriceKey{
		currHead: currHead,
		tokenID:  tokenID,
	}); ok {
		return data.(uint64), nil
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
			if tx.EvmTx.GasTokenID() == tokenID {
				tempPreBlockPrices = append(tempPreBlockPrices, tx.EvmTx.GasPrice().Uint64())
			}
		}
		prices = append(prices, tempPreBlockPrices...)
	}
	if len(prices) == 0 {
		return m.clusterConfig.Quarkchain.MinTXPoolGasPrice.Uint64(), nil
	}

	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	price := prices[(len(prices)-1)*int(m.gasPriceSuggestionOracle.Percentile)/100]
	m.gasPriceSuggestionOracle.cache.Add(gasPriceKey{
		currHead: currHead,
		tokenID:  tokenID,
	}, price)
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
	return m.reorg(oldBlock, newBlock)
}
func (m *MinorBlockChain) GetBranch() account.Branch {
	return m.branch
}

func (m *MinorBlockChain) GetMinorTip() *types.MinorBlockHeader {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.confirmedHeaderTip
}

func (m *MinorBlockChain) GetRootTip() *types.RootBlockHeader {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.rootTip
}

func getCrossShardBytesIndb(crossShard bool) []byte {
	crossShardBytes := make([]byte, 0)
	if crossShard {
		crossShardBytes = append(crossShardBytes, byte(0))
	} else {
		crossShardBytes = append(crossShardBytes, byte(1))
	}
	return crossShardBytes
}

func encodeAddressTxKey(addr account.Recipient, height uint64, index int, crossShard bool) []byte {
	crossShardBytes := getCrossShardBytesIndb(crossShard)

	addressBytes, err := serialize.SerializeToBytes(addr)
	if err != nil {
		panic(err)
	}
	heightBytes := qkcCommon.Uint32ToBytes(uint32(height))
	indexBytes := qkcCommon.Uint32ToBytes(uint32(index))

	rs := make([]byte, 0)
	rs = append(rs, addressTxKey...)
	rs = append(rs, addressBytes...)
	rs = append(rs, heightBytes...)
	rs = append(rs, crossShardBytes...)
	rs = append(rs, indexBytes...)
	return rs
}

func decodeTxKey(data []byte, keyLen int, addrLen int) (uint64, bool, uint32, error) {
	if len(data) != keyLen+addrLen+4+1+4 {
		return 0, false, 0, errors.New("input err")
	}
	height := qkcCommon.BytesToUint32(data[keyLen+addrLen : keyLen+addrLen+4])
	flag := false
	if data[keyLen+addrLen+4] == byte(0) {
		flag = true
	}
	index := qkcCommon.BytesToUint32(data[keyLen+addrLen+4+1:])
	return uint64(height), flag, index, nil
}

func encodeAllTxKey(height uint64, index int, crossShard bool) []byte {
	crossShardBytes := getCrossShardBytesIndb(crossShard)
	heightBytes := qkcCommon.Uint32ToBytes(uint32(height))
	indexBytes := qkcCommon.Uint32ToBytes(uint32(index))
	rs := make([]byte, 0)
	rs = append(rs, allTxKey...)
	rs = append(rs, heightBytes...)
	rs = append(rs, crossShardBytes...)
	rs = append(rs, indexBytes...)
	return rs

}

func (m *MinorBlockChain) putTxIndexFromBlock(batch rawdb.DatabaseWriter, block types.IBlock) error {
	deposit := m.getXShardDepositHashList(block.Hash())
	if deposit == nil {
		log.Error("impossible err", "please fix it", "getXshardDepositHashList err")
		return errors.New("xShardDepositHashList err")
	}
	rawdb.WriteBlockContentLookupEntriesWithCrossShardHashList(batch, block, deposit)
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

func (m *MinorBlockChain) removeTxIndexFromBlock(db rawdb.DatabaseDeleter, block *types.MinorBlock) error {
	blockTxs := block.Transactions()
	for index, tx := range blockTxs {
		if err := m.removeTxHistoryIndex(db, tx, block.NumberU64(), index); err != nil {
			return err
		}
	}
	depositHList := m.getXShardDepositHashList(block.Hash())
	if depositHList == nil {
		log.Error(m.logInfo, "impossible err", "please fix it removeTxIndexFromBlock")
	} else {
		for _, hash := range depositHList.HList {
			rawdb.DeleteBlockContentLookupEntry(db, hash)
		}
	}
	return m.removeTxHistoryIndexFromBlock(block)
}

func bytesSubOne(data []byte) []byte {
	bigData := new(big.Int).SetBytes(data)
	return bigData.Sub(bigData, new(big.Int).SetUint64(1)).Bytes()
}
func bytesAddOne(data []byte) []byte {
	bigData := new(big.Int).SetBytes(data)
	return bigData.Add(bigData, new(big.Int).SetUint64(1)).Bytes()
}

func (m *MinorBlockChain) getPendingTxByAddress(address account.Address, transferTokenID *uint64) ([]*rpc.TransactionDetail, []byte, error) {
	txList := make([]*rpc.TransactionDetail, 0)
	txs := m.txPool.GetAllTxInPool()

	needStore := func(sender account.Recipient, tx *types.Transaction) bool {
		if account.IsSameReceipt(sender, address.Recipient) || (tx.EvmTx.To() != nil && tx.EvmTx.To().String() == address.Recipient.String()) {
			if transferTokenID == nil || *transferTokenID == tx.EvmTx.TransferTokenID() {
				return true
			}
		}
		return false
	}

	//TODO: could also show incoming pending tx????? need check later
	for _, tx := range txs {
		sender, err := types.Sender(types.MakeSigner(m.clusterConfig.Quarkchain.NetworkID), tx.EvmTx)
		if err != nil {
			return nil, nil, err
		}
		if needStore(sender, tx) {
			to := new(account.Address)
			if tx.EvmTx.To() == nil {
				to = nil
			} else {
				to.Recipient = *tx.EvmTx.To()
				to.FullShardKey = tx.EvmTx.ToFullShardKey()
			}
			txList = append(txList, &rpc.TransactionDetail{
				TxHash:          tx.Hash(),
				FromAddress:     address,
				ToAddress:       to,
				Value:           serialize.Uint256{Value: tx.EvmTx.Value()},
				BlockHeight:     0,
				Timestamp:       0,
				Success:         false,
				GasTokenID:      tx.EvmTx.GasTokenID(),
				TransferTokenID: tx.EvmTx.TransferTokenID(),
				IsFromRootChain: false,
			})
		}

	}
	return txList, []byte{}, nil
}

func (m *MinorBlockChain) getTransactionDetails(start, end []byte, limit uint32, getTxType GetTxDetailType, skipCoinbaseRewards bool, transferTokenID *uint64) ([]*rpc.TransactionDetail, []byte, error) {
	qkcDB, ok := m.db.(*qkcdb.RDBDatabase)
	if !ok {
		return nil, nil, errors.New("only support qkcdb now")
	}

	skipXShard := func(xShardTx *types.CrossShardTransactionDeposit) bool {
		if xShardTx.IsFromRootChain {
			return skipCoinbaseRewards || xShardTx.Value.Value.Uint64() == 0
		}
		return transferTokenID != nil && xShardTx.TransferTokenID != *transferTokenID
	}

	skipTx := func(tx *types.EvmTransaction) bool {
		return transferTokenID != nil && tx.TransferTokenID() != *transferTokenID
	}

	var (
		next       = end
		txList     = make([]*rpc.TransactionDetail, 0)
		it         = qkcDB.NewIterator()
		height     uint64
		crossShard bool
		err        error
		index      uint32
		txHashes   = make(map[common.Hash]struct{})
	)

	it.SeekForPrev(start)
	for it.Valid() {
		if bytes.Compare(it.Key().Data(), end) < 0 {
			break
		}
		if getTxType == GetTxFromAddress {
			height, crossShard, index, err = decodeTxKey(it.Key().Data(), len(addressTxKey), 20)
		} else if getTxType == GetAllTransaction {
			height, crossShard, index, err = decodeTxKey(it.Key().Data(), len(allTxKey), 0)
		} else {
			return nil, nil, errors.New("not support yet")
		}
		if err != nil {
			return nil, nil, err
		}
		mBlock, ok := m.GetBlockByNumber(height).(*types.MinorBlock)
		if !ok {
			log.Error(m.logInfo, "get minor block fialed height", height)
			return nil, nil, errors.New("get minBlock failed")
		}
		if crossShard {
			xShardReceiveTxList := rawdb.ReadConfirmedCrossShardTxList(m.db, mBlock.Hash())
			if index >= uint32(len(xShardReceiveTxList.TXList)) {
				return nil, nil, errors.New("tx's index bigger than txs's len ")
			}
			tx := xShardReceiveTxList.TXList[index]
			_, ok := txHashes[tx.TxHash]
			if !ok && !skipXShard(tx) {
				limit--
				txHashes[tx.TxHash] = struct{}{}
				txList = append(txList, &rpc.TransactionDetail{
					TxHash:          tx.TxHash,
					FromAddress:     tx.From,
					ToAddress:       &tx.To,
					Value:           serialize.Uint256{Value: tx.Value.Value},
					BlockHeight:     height,
					Timestamp:       mBlock.IHeader().GetTime(),
					Success:         true,
					GasTokenID:      tx.GasTokenID,
					TransferTokenID: tx.TransferTokenID,
					IsFromRootChain: tx.IsFromRootChain,
				})
			}
		} else {
			tx := mBlock.Transactions()[index]
			txHash := tx.Hash()
			evmTx := tx.EvmTx
			_, ok := txHashes[txHash]
			if !ok && !skipTx(evmTx) {
				limit--
				receipt, _, _ := rawdb.ReadReceipt(m.db, tx.Hash())
				sender, err := types.Sender(types.MakeSigner(m.clusterConfig.Quarkchain.NetworkID), evmTx)
				if err != nil {
					return nil, nil, err
				}
				toAddr := new(account.Address)
				if evmTx.To() == nil {
					toAddr = nil
				} else {
					toAddr.Recipient = *evmTx.To()
					toAddr.FullShardKey = evmTx.ToFullShardKey()
				}

				succFlag := false
				if receipt.Status == 1 {
					succFlag = true
				}

				txList = append(txList, &rpc.TransactionDetail{
					TxHash: txHash,
					FromAddress: account.Address{
						Recipient:    sender,
						FullShardKey: evmTx.FromFullShardKey(),
					},
					ToAddress:       toAddr,
					Value:           serialize.Uint256{Value: evmTx.Value()},
					BlockHeight:     height,
					Timestamp:       mBlock.IHeader().GetTime(),
					Success:         succFlag,
					GasTokenID:      evmTx.GasTokenID(),
					TransferTokenID: evmTx.TransferTokenID(),
					IsFromRootChain: false,
				})
			}

		}
		next = bytesSubOne(it.Key().Data())
		if limit == 0 {
			break
		}
		it.Prev()
	}
	return txList, next, nil
}

func (m *MinorBlockChain) GetTransactionByAddress(address account.Address, transferTokenID *uint64, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	if !m.clusterConfig.EnableTransactionHistory {
		return []*rpc.TransactionDetail{}, []byte{}, nil
	}

	if bytes.Equal(start, []byte{1}) { //get pending tx
		return m.getPendingTxByAddress(address, transferTokenID)
	}
	end := make([]byte, 0)
	end = append(end, addressTxKey...)
	tAdd, err := serialize.SerializeToBytes(address.Recipient)
	if err != nil {
		panic(err)
	}
	end = append(end, tAdd...)
	originalStartBytes := bytesAddOne(end)

	if len(start) == 0 || bytes.Compare(start, originalStartBytes) > 0 {
		start = originalStartBytes
	}
	return m.getTransactionDetails(start, end, limit, GetTxFromAddress, false, transferTokenID)
}

func (m *MinorBlockChain) GetAllTx(start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	if !m.clusterConfig.EnableTransactionHistory {
		return []*rpc.TransactionDetail{}, []byte{}, nil
	}
	end := make([]byte, 0)
	end = append(end, allTxKey...)
	originalStartBytes := bytesAddOne(end)
	if len(start) == 0 || bytes.Compare(start, originalStartBytes) > 0 {
		start = originalStartBytes
	}
	return m.getTransactionDetails(start, end, limit, GetAllTransaction, true, nil)
}

func (m *MinorBlockChain) GetLogsByFilterQuery(args *qrpc.FilterQuery) ([]*types.Log, error) {
	filter := NewRangeFilter(m, args.FromBlock.Uint64(), args.ToBlock.Uint64(), args.Addresses, args.Topics)
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
	if !m.clusterConfig.EnableTransactionHistory {
		return nil
	}
	evmtx := tx.EvmTx
	key := encodeAllTxKey(height, index, false)
	if err := f(key); err != nil {
		return err
	}

	sender, err := types.Sender(types.MakeSigner(m.clusterConfig.Quarkchain.NetworkID), evmtx)
	if err != nil {
		return err
	}
	key = encodeAddressTxKey(sender, height, index, false)
	if err := f(key); err != nil {
		return err
	}

	if evmtx.To() != nil && m.branch.IsInBranch(evmtx.ToFullShardKey()) {
		key := encodeAddressTxKey(*evmtx.To(), height, index, false)
		if err := f(key); err != nil {
			return err
		}
	}
	return nil
}
func (m *MinorBlockChain) putTxHistoryIndex(tx *types.Transaction, height uint64, index int) error {
	return m.updateTxHistoryIndex(tx, height, index, m.putTxIndexDB)
}
func (m *MinorBlockChain) removeTxHistoryIndex(db rawdb.DatabaseDeleter, tx *types.Transaction, height uint64, index int) error {
	rawdb.DeleteBlockContentLookupEntry(db, tx.Hash())
	return m.updateTxHistoryIndex(tx, height, index, m.deleteTxIndexDB)
}

func (m *MinorBlockChain) updateTxHistoryIndexFromBlock(block *types.MinorBlock, f func([]byte) error) error {
	if !m.clusterConfig.EnableTransactionHistory {
		return nil
	}
	xShardReceiveTxList := rawdb.ReadConfirmedCrossShardTxList(m.db, block.Hash())
	for index, tx := range xShardReceiveTxList.TXList {
		//ignore dummy coinbase reward deposits
		if tx.IsFromRootChain && tx.Value.Value.Uint64() == 0 {
			continue
		}
		key := encodeAddressTxKey(tx.To.Recipient, block.Number(), index, true)
		if err := f(key); err != nil {
			return err
		}
		if !tx.IsFromRootChain {
			key := encodeAllTxKey(block.Number(), index, true)
			if err := f(key); err != nil {
				return err
			}
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

func (m *MinorBlockChain) RunCrossShardTxWithCursor(evmState *state.StateDB,
	mBlock *types.MinorBlock) ([]*types.CrossShardTransactionDeposit, *types.XShardTxCursorInfo, types.Receipts, error) {

	preMinorBlock := m.GetMinorBlock(mBlock.ParentHash())
	if preMinorBlock == nil {
		return nil, nil, nil, errors.New("no pre block")
	}
	cursor := NewXShardTxCursor(m, mBlock.Header(), preMinorBlock.Meta().XShardTxCursorInfo)
	var receipts types.Receipts
	txList := make([]*types.CrossShardTransactionDeposit, 0)
	evmState.SetQuarkChainConfig(m.clusterConfig.Quarkchain)
	gasUsed := new(uint64)
	for true {
		xShardDepositTx, err := cursor.getNextTx()
		if err != nil {
			return nil, nil, nil, err
		}
		if xShardDepositTx == nil {
			break
		}
		checkIsFromRootChain := cursor.rBlock.Header().NumberU64() >= m.clusterConfig.Quarkchain.XShardGasDDOSFixRootHeight
		txIndex := 0
		receipt, err := ApplyCrossShardDeposit(m.ethChainConfig, m, mBlock.Header(),
			*m.GetVMConfig(), evmState, xShardDepositTx, gasUsed, checkIsFromRootChain, txIndex)
		if err != nil {
			return nil, nil, nil, err
		}
		txIndex++
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
		txList = append(txList, xShardDepositTx)

		if evmState.GetGasUsed().Cmp(mBlock.Meta().XShardGasLimit.Value) >= 0 {
			break
		}
	}
	evmState.SetXShardReceiveGasUsed(evmState.GetGasUsed())
	return txList, cursor.getCursorInfo(), receipts, nil
}

func CountAddressFromSlice(lists []account.Recipient, recipient account.Recipient) uint64 {
	cnt := uint64(0)
	for _, v := range lists {
		if v.String() == recipient.String() {
			cnt++
		}
	}
	return cnt
}
func (m *MinorBlockChain) PoswInfo(mBlock *types.MinorBlock) (*rpc.PoSWInfo, error) {
	if mBlock == nil {
		return nil, errors.New("get powInfo err:mBlock is full")
	}
	header := mBlock.Header()
	if !m.posw.IsPoSWEnabled(header) {
		return nil, nil
	}
	evmState, err := m.getEvmStateForNewBlock(header, true)
	if err != nil {
		return nil, err
	}
	stakes := evmState.GetBalance(header.Coinbase.Recipient, m.clusterConfig.Quarkchain.GetDefaultChainTokenID())
	diff, minable, mined, err := m.posw.GetPoSWInfo(header, stakes)
	if err != nil {
		return nil, err
	}
	return &rpc.PoSWInfo{
		EffectiveDifficulty: diff,
		PoswMineableBlocks:  minable,
		PoswMinedBlocks:     mined}, nil
}

func (m *MinorBlockChain) putXShardDepositHashList(h common.Hash, hList *rawdb.HashList) {
	rawdb.PutXShardDepositHashList(m.db, h, hList)
}

func (m *MinorBlockChain) getXShardDepositHashList(h common.Hash) *rawdb.HashList {
	return rawdb.GetXShardDepositHashList(m.db, h)
}

func (m *MinorBlockChain) IsMinorBlockCommittedByHash(h common.Hash) bool {
	return rawdb.HasCommitMinorBlock(m.db, h)
}
func (m *MinorBlockChain) CommitMinorBlockByHash(h common.Hash) {
	rawdb.WriteCommitMinorBlock(m.db, h)
}

func (m *MinorBlockChain) GetMiningInfo(address account.Recipient, stake *types.TokenBalances) (uint64, uint64, error) {
	//TODO @DL to fix
	return 0, 0, nil
}
