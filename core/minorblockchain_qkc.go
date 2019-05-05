package core

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math"
	"math/big"
	"sort"
	"time"
)

// ShardStatus shard status for api
type ShardStatus struct {
	Branch             account.Branch
	Height             uint64
	Difficulty         *big.Int
	CoinbaseAddress    account.Address
	TimeStamp          uint64
	TxCount60s         uint32
	PendingTxCount     uint32
	TotalTxCount       uint32
	BlockCount60s      uint32
	StaleBlockCount60s uint32
	LastBlockTime      uint32
}

func (m *MinorBlockChain) getPOSWCoinbaseBlockCnt(headerHash common.Hash, length *uint32) ([]account.Recipient, error) {
	// already locked
	if length == nil {
		length = &m.shardConfig.PoswConfig.WindowSize
	}
	coinbaseAddrList, err := m.getCoinbaseAddressUntilBlock(headerHash, *length)
	if err != nil {
		return nil, err
	}
	return coinbaseAddrList, nil
}

func (m *MinorBlockChain) getCoinbaseAddressUntilBlock(headerHash common.Hash, length uint32) ([]account.Recipient, error) {
	// already locked
	currBlock := m.GetMinorBlock(headerHash)
	if currBlock == nil {
		return nil, ErrMinorBlockIsNil
	}
	header := currBlock.IHeader()
	height := header.NumberU64()
	prevHash := header.GetParentHash()
	addrLists := make([]account.Recipient, 0)
	if data, ok := m.coinbaseAddrCache[prevHash]; ok {
		addrLists = append(addrLists, data.addrs...)
		if len(addrLists) == int(length) && len(addrLists) >= 1 {
			addrLists = addrLists[1:]
		}
		addrLists = append(addrLists, header.GetCoinbase().Recipient)
	} else {
		tempList := make([]account.Recipient, 0)
		for index := int(length - 1); index >= 0; index-- {
			tempList = append(tempList, header.GetCoinbase().Recipient)
			if header.NumberU64() == 0 {
				break
			}
			header = m.GetHeaderByHash(header.GetParentHash())
			if qkcCommon.IsNil(header) {
				panic(errors.New("mysteriously missing block"))
			}
		}
		addrLists = append(tempList, addrLists...)
	}
	m.coinbaseAddrCache[headerHash] = heightAndAddrs{
		height: height,
		addrs:  addrLists,
	}
	if len(m.coinbaseAddrCache) > 128 {
		for k, v := range m.coinbaseAddrCache {
			if v.height <= height-16 {
				delete(m.coinbaseAddrCache, k)
			}

		}
	}
	return addrLists, nil
}

func (m *MinorBlockChain) putTotalTxCount(mBlock *types.MinorBlock) error {
	prevCount := uint32(0)
	if mBlock.Header().Number > 2 {
		dbPreCount := rawdb.ReadTotalTx(m.db, mBlock.Header().ParentHash)
		if dbPreCount == nil {
			return errors.New("get totalTx failed")
		}
		prevCount += *dbPreCount
	}
	rawdb.WriteTotalTx(m.db, mBlock.Header().Hash(), prevCount)
	return nil
}

func (m *MinorBlockChain) getTotalTxCount(hash common.Hash) *uint32 {
	return rawdb.ReadTotalTx(m.db, hash)
}

func (m *MinorBlockChain) putConfirmedCrossShardTransactionDepositList(hash common.Hash, xShardReceiveTxList []*types.CrossShardTransactionDeposit) error {
	if !m.clusterConfig.EnableTransactionHistory {
		return nil
	}
	data := types.CrossShardTransactionDepositList{TXList: xShardReceiveTxList}
	rawdb.WriteConfirmedCrossShardTxList(m.db, hash, data)
	return nil

}

func (m *MinorBlockChain) getConfirmedCrossShardTransactionDepositList(hash common.Hash) *types.MinorBlockHeader {
	rMinorHeaderHash := rawdb.ReadLastConfirmedMinorBlockHeaderAtRootBlock(m.db, hash)
	if rMinorHeaderHash == qkcCommon.EmptyHash {
		return nil
	}
	return rawdb.ReadMinorBlockHeader(m.db, rMinorHeaderHash)
}

func (m *MinorBlockChain) getLastConfirmedMinorBlockHeaderAtRootBlock(hash common.Hash) *types.MinorBlockHeader {
	rMinorHeaderHash := rawdb.ReadLastConfirmedMinorBlockHeaderAtRootBlock(m.db, hash)
	if rMinorHeaderHash == qkcCommon.EmptyHash {
		return nil
	}
	return rawdb.ReadMinorBlockHeader(m.db, rMinorHeaderHash)
}

// putMinorBlock: put minor block into db ,already locked by upper call
func (m *MinorBlockChain) putMinorBlock(mBlock *types.MinorBlock, xShardReceiveTxList []*types.CrossShardTransactionDeposit) error {
	if _, ok := m.heightToMinorBlockHashes[mBlock.NumberU64()]; !ok {
		m.heightToMinorBlockHashes[mBlock.NumberU64()] = make(map[common.Hash]struct{})
	}
	m.heightToMinorBlockHashes[mBlock.NumberU64()][mBlock.Hash()] = struct{}{}
	rawdb.WriteMinorBlock(m.db, mBlock)
	if err := m.putTotalTxCount(mBlock); err != nil {
		return err
	}

	if err := m.putConfirmedCrossShardTransactionDepositList(mBlock.Hash(), xShardReceiveTxList); err != nil {
		return err
	}
	return nil
}

func (m *MinorBlockChain) putRootBlock(rBlock *types.RootBlock, minorHeader *types.MinorBlockHeader, rBlockHash common.Hash) {
	// no need to lock
	if rBlockHash == qkcCommon.EmptyHash {
		rBlockHash = rBlock.Hash()
	}
	rawdb.WriteRootBlock(m.db, rBlock)
	var mHash common.Hash
	if minorHeader != nil {
		mHash = minorHeader.Hash()
	}
	rawdb.WriteLastConfirmedMinorBlockHeaderAtRootBlock(m.db, rBlockHash, mHash)
}

func (m *MinorBlockChain) putGenesisBlock(rBlockHash common.Hash, genesisBlock *types.MinorBlock) {
	rawdb.WriteGenesisBlock(m.db, rBlockHash, genesisBlock)
}

func (m *MinorBlockChain) getGenesisBlock(hash common.Hash) *types.MinorBlock {
	return rawdb.ReadGenesis(m.db, hash)
}

// InitFromRootBlock init minorBlockChain from rootBlock
func (m *MinorBlockChain) InitFromRootBlock(rBlock *types.RootBlock) error {
	m.mu.Lock() // to lock rootTip  confirmedHeaderTip...
	defer m.mu.Unlock()
	if rBlock.Header().Number <= uint32(m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)) {
		return errors.New("rootBlock height small than config's height")
	}
	if m.initialized == true {
		return errors.New("already initialized")
	}
	m.initialized = true
	confirmedHeaderTip := m.getConfirmedCrossShardTransactionDepositList(rBlock.Hash())
	headerTip := confirmedHeaderTip
	if headerTip == nil {
		headerTip = m.GetBlockByNumber(0).IHeader().(*types.MinorBlockHeader)
	}

	m.rootTip = rBlock.Header()
	m.confirmedHeaderTip = confirmedHeaderTip

	block := rawdb.ReadMinorBlock(m.db, headerTip.Hash())
	if block == nil {
		return ErrMinorBlockIsNil
	}
	var err error
	m.currentEvmState, err = m.createEvmState(block.Meta().Root, block.Hash())
	if err != nil {
		return err
	}
	return m.reWriteBlockIndexTo(nil, block)
}

// createEvmState already locked
func (m *MinorBlockChain) createEvmState(trieRootHash common.Hash, headerHash common.Hash) (*state.StateDB, error) {
	evmState, err := m.StateAt(trieRootHash)
	if err != nil {
		return nil, err
	}
	evmState.SetShardConfig(m.shardConfig)
	if m.shardConfig.PoswConfig.Enabled && headerHash == qkcCommon.EmptyHash {
		powsAddr, err := m.getPOSWCoinbaseBlockCnt(headerHash, nil)
		if err != nil {
			return nil, err
		}
		evmState.SetSenderDisallowList(powsAddr)
	}
	return evmState, nil
}

// getEvmStateForNewBlock get evmState for new block.should have locked
func (m *MinorBlockChain) getEvmStateForNewBlock(mBlock types.IBlock, ephemeral bool) (*state.StateDB, error) {
	block := mBlock.(*types.MinorBlock)
	preMinorBlock := m.GetMinorBlock(block.IHeader().GetParentHash())
	if preMinorBlock == nil {
		return nil, errInsufficientBalanceForGas
	}
	rootHash := preMinorBlock.GetMetaData().Root
	evmState, err := m.createEvmState(rootHash, block.IHeader().GetParentHash())
	if err != nil {
		return nil, err
	}

	if ephemeral {
		evmState = evmState.Copy()
	}
	evmState.SetBlockCoinbase(block.IHeader().GetCoinbase().Recipient)
	evmState.SetGasLimit(block.Header().GetGasLimit())
	evmState.SetQuarkChainConfig(m.clusterConfig.Quarkchain)
	return evmState, nil
}

func (m *MinorBlockChain) getEvmStateFromHeight(height *uint64) (*state.StateDB, error) {
	if height == nil || *height == m.CurrentBlock().NumberU64() {
		return m.State()
	}
	block := m.GetBlockByNumber(*height + 1)
	if block != nil {
		return nil, ErrMinorBlockIsNil
	}
	return m.getEvmStateForNewBlock(block, true)

}

// InitGenesisState init genesis stateDB from rootBlock
func (m *MinorBlockChain) InitGenesisState(rBlock *types.RootBlock) (*types.MinorBlock, error) {
	m.mu.Lock() // used in init_from_rootBlock and add_rootBlock,so should lock
	defer m.mu.Unlock()
	var err error
	gBlock, err := NewGenesis(m.clusterConfig.Quarkchain).CreateMinorBlock(rBlock, m.branch.Value, m.db)
	if err != nil {
		return nil, err
	}
	rawdb.WriteTd(m.db, gBlock.Hash(), gBlock.Difficulty())
	height := m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)
	if rBlock.Header().Number != height {
		return nil, errors.New("header number is not match")
	}

	if err := m.putMinorBlock(gBlock, nil); err != nil {
		return nil, err
	}
	m.putRootBlock(rBlock, nil, common.Hash{})
	m.putGenesisBlock(rBlock.Hash(), gBlock)
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

func (m *MinorBlockChain) runBlock(block *types.MinorBlock) (*state.StateDB, types.Receipts, error) {
	m.mu.Lock() // to lock Process
	defer m.mu.Unlock()
	parent := m.GetMinorBlock(block.ParentHash())
	if qkcCommon.IsNil(parent) {
		return nil, nil, ErrRootBlockIsNil
	}

	preEvmState, err := m.StateAt(parent.GetMetaData().Root)
	if err != nil {
		return nil, nil, err
	}
	evmState := preEvmState.Copy()
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
	_, _, err = m.InsertChain([]types.IBlock{block}) // will lock
	if err != nil {
		return nil, nil, err
	}
	return block, receipts, nil
}
func (m *MinorBlockChain) validateTx(tx *types.Transaction, evmState *state.StateDB, fromAddress *account.Address, gas *uint64) (*types.Transaction, error) {
	// no need to lock
	if tx.TxType != types.EvmTx {
		return nil, errors.New("tx type is not match")
	}
	evmTx := tx.EvmTx
	if fromAddress != nil {
		if evmTx.FromFullShardKey() != fromAddress.FullShardKey {
			return nil, errors.New("from full shard id is not match")
		}
		evmTxGas := evmTx.Gas()
		if gas != nil {
			evmTxGas = *gas
		}
		evmTx.SetGas(evmTxGas)
	}

	toShardSize := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.ToChainID())
	if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
		return nil, err
	}
	fromShardSize := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
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

	initializedFullShardIDs := m.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(m.rootTip.Number)

	if evmTx.IsCrossShard() {
		hasInit := false
		for _, v := range initializedFullShardIDs {
			if toBranch.GetFullShardID() == v {
				hasInit = true
			}
		}
		if !hasInit {
			return nil, errors.New("is not initialized yet")
		}
	}

	if evmTx.IsCrossShard() && !m.isNeighbor(toBranch, nil) {
		return nil, ErrNotNeighbir
	}
	if err := ValidateTransaction(evmState, tx, fromAddress); err != nil {
		return nil, err
	}

	tx = &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	return tx, nil
}

func (m *MinorBlockChain) isNeighbor(remoteBranch account.Branch, rootHeight *uint32) bool {
	// no need to lock
	if rootHeight == nil {
		rootHeight = &m.rootTip.Number
	}
	shardSize := len(m.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(*rootHeight))
	return isNeighbor(m.branch, remoteBranch, uint32(shardSize))
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
func isNeighbor(b1, b2 account.Branch, shardSize uint32) bool {
	if shardSize <= 32 {
		return true
	}
	if b1.GetChainID() == b2.GetChainID() {
		return qkcCommon.IsP2(absUint32(b1.GetShardID(), b2.GetShardID()))
	}
	if b1.GetShardID() == b2.GetShardID() {
		return qkcCommon.IsP2(absUint32(b1.GetChainID(), b2.GetChainID()))
	}
	return false
}

// AddTx add tx to txPool
func (m *MinorBlockChain) AddTx(tx *types.Transaction) error {
	if m.txPool.all.Count() > int(m.clusterConfig.Quarkchain.TransactionQueueSizeLimitPerShard) {
		return errors.New("txpool queue full")
	}
	txHash := tx.Hash()
	txInDB, _, _ := rawdb.ReadTransaction(m.db, txHash)
	if txInDB != nil {
		return errors.New("tx already have")
	}
	evmState, err := m.State()
	if err != nil {
		return err
	}
	evmState = evmState.Copy()
	evmState.SetGasUsed(new(big.Int).SetUint64(0))
	if _, err = evmState.Commit(true); err != nil {
		return err
	}

	if tx, err = m.validateTx(tx, evmState, nil, nil); err != nil {
		return err
	}

	err = m.txPool.AddLocal(tx) // txpool.addTx have lock already
	if err != nil {
		return err
	}
	return nil
}

func (m *MinorBlockChain) computerGasLimit(parentGasLimit, parentGasUsed, gasLimitFloor uint64) (uint64, error) {
	// no need to lock
	shardConfig := m.shardConfig
	if gasLimitFloor < shardConfig.GasLimitMinimum {
		return 0, errors.New("gas limit floor is too low")
	}
	decay := parentGasLimit / uint64(shardConfig.GasLimitEmaDenominator)
	usageIncrease := uint64(0)
	if parentGasUsed != 0 {
		usageIncrease = parentGasUsed * uint64(shardConfig.GasLimitUsageAdjustmentNumerator) / uint64(shardConfig.GasLimitUsageAdjustmentDenominator) / uint64(shardConfig.GasLimitEmaDenominator)
	}
	gasLimit := shardConfig.GasLimitMinimum
	if gasLimit < parentGasLimit-decay+usageIncrease {
		gasLimit = parentGasLimit - decay + usageIncrease
	}

	if gasLimit < gasLimitFloor {
		return parentGasLimit + decay, nil
	}
	return gasLimit, nil

}

// runCrossShardTxList already locked by insertChain
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
			return nil, ErrRootBlockIsNil
		}
	}
	if evmState.GetGasUsed().Cmp(evmState.GetGasLimit()) > 0 {
		return nil, errors.New("runCrossShardTxList err:gasUsed > GasLimit")
	}
	return txList, nil
}

func getLocalFeeRate(qkcConfig *config.QuarkChainConfig) *big.Rat {
	localFeeRate := big.NewRat(1, 1)
	if qkcConfig != nil {
		num := qkcConfig.RewardTaxRate.Num().Int64()
		demo := qkcConfig.RewardTaxRate.Denom().Int64()
		localFeeRate = big.NewRat(demo-num, demo)
	}
	return localFeeRate
}

func (m *MinorBlockChain) ruOneCrossShardTxListByRootBlockHash(hash common.Hash, evmState *state.StateDB) ([]*types.CrossShardTransactionDeposit, error) {
	// no need to lock
	txList, err := m.getCrossShardTxListByRootBlockHash(hash)
	if err != nil {
		return nil, err
	}
	localFeeRate := getLocalFeeRate(evmState.GetQuarkChainConfig())
	for _, tx := range txList {
		evmState.AddBalance(tx.To.Recipient, tx.Value.Value)
		addGasUsed := new(big.Int).SetUint64(0)
		if tx.GasPrice.Value.Uint64() != 0 {
			addGasUsed = params.GtxxShardCost
		}
		gasUsed := minBigInt(evmState.GetGasLimit(), new(big.Int).Add(evmState.GetGasUsed(), addGasUsed))
		evmState.SetGasUsed(gasUsed)
		xShardFee := new(big.Int).Mul(params.GtxxShardCost, tx.GasPrice.Value)
		xShardFee = qkcCommon.BigIntMulBigRat(xShardFee, localFeeRate)
		evmState.AddBlockFee(xShardFee)
		evmState.AddBalance(evmState.GetBlockCoinbase(), xShardFee)
	}
	evmState.SetXShardReceiveGasUsed(evmState.GetGasUsed())
	return txList, nil
}

func (m *MinorBlockChain) getCrossShardTxListByRootBlockHash(hash common.Hash) ([]*types.CrossShardTransactionDeposit, error) {
	// no need to lock
	rBlock := m.GetRootBlockByHash(hash)
	if rBlock == nil {
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
		xShardTxList := rawdb.ReadCrossShardTxList(m.db, mHeader.Hash())

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
func (m *MinorBlockChain) getCoinbaseAmount() *big.Int {
	// no need to lock
	localFeeRate := getLocalFeeRate(m.clusterConfig.Quarkchain)
	coinbaseAmount := qkcCommon.BigIntMulBigRat(m.clusterConfig.Quarkchain.GetShardConfigByFullShardID(m.branch.Value).CoinbaseAmount, localFeeRate)
	return coinbaseAmount
}

func (m *MinorBlockChain) isMinorBlockLinkedToRootTip(mBlock *types.MinorBlock) bool {
	// already lock
	if m.confirmedHeaderTip == nil {
		return true
	}
	if mBlock.Header().Number <= m.confirmedHeaderTip.Number {
		return false
	}
	return m.isSameMinorChain(mBlock.Header(), m.confirmedHeaderTip)
}

func (m *MinorBlockChain) isSameMinorChain(long types.IHeader, short types.IHeader) bool {
	// already locked by insertChain
	if short.NumberU64() > long.NumberU64() {
		return false
	}
	header := long
	for index := 0; index < int(long.NumberU64()-short.NumberU64()); index++ {
		header = m.GetHeaderByHash(header.GetParentHash())
	}
	return header.Hash() == short.Hash()
}
func (m *MinorBlockChain) isSameRootChain(long types.IHeader, short types.IHeader) bool {
	// no need to lock
	return consensus.IsSameRootChain(m.db, long, short)
}

func (m *MinorBlockChain) getDefaultEvmState(height *uint64) (*state.StateDB, error) {
	if height == nil {
		temp := m.CurrentBlock().NumberU64()
		height = &temp
	}
	mBlock := m.GetBlockByNumber(*height)
	if qkcCommon.IsNil(mBlock) {
		return nil, ErrMinorBlockIsNil
	}

	evmState, err := m.StateAt(mBlock.(*types.MinorBlock).GetMetaData().Root)
	if err != nil {
		return nil, err
	}
	return evmState, nil
}

// GetBalance get balance for address
func (m *MinorBlockChain) GetBalance(recipient account.Recipient, height *uint64) (*big.Int, error) {
	// no need to lock
	evmState, err := m.getDefaultEvmState(height)
	if err != nil {
		return nil, err
	}
	return evmState.GetBalance(recipient), nil
}

// GetTransactionCount get txCount for addr
func (m *MinorBlockChain) GetTransactionCount(recipient account.Recipient, height *uint64) (uint64, error) {
	// no need to lock
	evmState, err := m.getDefaultEvmState(height)
	if err != nil {
		return 0, err
	}
	return evmState.GetNonce(recipient), nil
}

// GetCode get code for addr
func (m *MinorBlockChain) GetCode(recipient account.Recipient, height *uint64) ([]byte, error) {
	// no need to lock
	evmState, err := m.getDefaultEvmState(height)
	if err != nil {
		return nil, err
	}
	return evmState.GetCode(recipient), nil
}

// GetStorageAt get storage for addr
func (m *MinorBlockChain) GetStorageAt(recipient account.Recipient, key common.Hash, height *uint64) (common.Hash, error) {
	// no need to lock
	evmState, err := m.getDefaultEvmState(height)
	if err != nil {
		return common.Hash{}, err
	}
	return evmState.GetState(recipient, key), nil
}

// ExecuteTx execute tx
func (m *MinorBlockChain) ExecuteTx(tx *types.Transaction, fromAddress *account.Address, height *uint64) ([]byte, error) {
	// no need to lock
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
	usedGas := new(uint64)
	ret, _, _, err := ApplyTransaction(m.ethChainConfig, m, gp, state, mBlock.IHeader().(*types.MinorBlockHeader), evmTx, usedGas, *m.GetVMConfig())
	return ret, err
}

// GetNextBlockDifficulty get next block difficulty
func (m *MinorBlockChain) GetNextBlockDifficulty(createTime *uint64) (*big.Int, error) {
	// no need to lock
	realTime := uint64(0)
	if createTime == nil {
		realTime = uint64(time.Now().Unix())
		if realTime < m.CurrentBlock().IHeader().NumberU64()+1 {
			realTime = m.CurrentBlock().IHeader().NumberU64() + 1
		}
	} else {
		realTime = *createTime
	}
	return m.engine.CalcDifficulty(m, realTime, m.CurrentHeader().(*types.MinorBlockHeader))
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
	headerList := make([]*types.MinorBlockHeader, allHeight)
	for index := allHeight - 1; index >= 0; index-- {
		headerList[index] = header
		header, ok = m.GetHeaderByHash(header.GetParentHash()).(*types.MinorBlockHeader)
		if !ok && index == 0 {
			break // 0's pre
		}
	}

	if !checkEqual(header, confirmedHeaderTip) {
		return nil
	}
	return headerList
}

// GetUnconfirmedHeaderList get unconfirmed headerList
func (m *MinorBlockChain) GetUnconfirmedHeaderList() []*types.MinorBlockHeader {
	headers := m.getAllUnconfirmedHeaderList() // have lock
	maxBlocks := m.getMaxBlocksInOneRootBlock()
	return headers[0:maxBlocks]
}

func (m *MinorBlockChain) getMaxBlocksInOneRootBlock() uint64 {
	return uint64(m.shardConfig.MaxBlocksPerShardInOneRootBlock())
}

// GetUnconfirmedHeadersCoinbaseAmount get unconfirmed headers coinbase amount
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
		diff := new(big.Int).Sub(stateT.GetGasLimit(), stateT.GetGasUsed())
		tx := txs.Peek()

		if tx == nil {
			break
		}

		if tx.EvmTx.Gas() > diff.Uint64() {
			txs.Pop()
			continue
		}
		toShardSize := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.ToChainID())
		if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
			return nil, nil, err
		}
		fromShardSize := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
		if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
			return nil, nil, err
		}

		toBranch := account.Branch{Value: tx.EvmTx.ToFullShardId()}
		if toBranch.Value != m.branch.Value {
			if !m.isNeighbor(toBranch, nil) {
				txs.Pop()
				continue
			}
			if xShardTxCounters[tx.EvmTx.ToFullShardId()]+1 > xShardTxLimits[tx.EvmTx.ToFullShardId()] {
				txs.Pop()
				continue
			}

		}

		_, receipt, _, err := ApplyTransaction(m.ethChainConfig, m, gp, stateT, block.IHeader().(*types.MinorBlockHeader), tx, usedGas, *m.GetVMConfig())
		if err != nil {
			return nil, nil, err
		}
		receipts = append(receipts, receipt)
		txsInBlock = append(txsInBlock, tx)
		xShardTxCounters[tx.EvmTx.ToFullShardId()]++
		txs.Pop()
	}
	bHeader := block.Header()
	bHeader.PrevRootBlockHash = rootBlockHash
	return types.NewMinorBlock(bHeader, block.Meta(), txsInBlock, receipts, nil), receipts, nil
}

// CreateBlockToMine create block to mine
func (m *MinorBlockChain) CreateBlockToMine(createTime *uint64, address *account.Address, gasLimit *big.Int) (*types.MinorBlock, error) {
	m.mu.Lock() // to lock txpool and getEvmStateForNewBlock
	defer m.mu.Unlock()
	startTime := time.Now()
	realCreateTime := uint64(startTime.Unix())
	if createTime == nil {
		realCreateTime = uint64(startTime.Unix())
		if realCreateTime < m.CurrentBlock().IHeader().GetTime()+1 {
			realCreateTime = m.CurrentBlock().IHeader().GetTime() + 1
		}
	} else {
		realCreateTime = *createTime
	}
	difficulty, err := m.GetNextBlockDifficulty(&realCreateTime)
	if err != nil {
		return nil, err
	}
	prevBlock := m.CurrentBlock()
	newGasLimit, err := m.computerGasLimit(prevBlock.Header().GetGasLimit().Uint64(), prevBlock.GetMetaData().GasUsed.Value.Uint64(), m.shardConfig.Genesis.GasLimit)

	block := prevBlock.CreateBlockToAppend(&realCreateTime, difficulty, address, nil, new(big.Int).SetUint64(newGasLimit), nil, nil)
	evmState, err := m.getEvmStateForNewBlock(block, true)
	if gasLimit != nil {
		evmState.SetGasLimit(gasLimit)
	}

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
	_, err = evmState.Commit(true)
	if err != nil {
		return nil, err
	}

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
func (m *MinorBlockChain) AddRootBlock(rBlock *types.RootBlock) error {
	m.mu.Lock() // Ensure insertion continuity
	defer m.mu.Unlock()
	if rBlock.Number() <= uint32(m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)) {
		return errors.New("rBlock is small than config")
	}

	if m.GetRootBlockByHash(rBlock.ParentHash()) == nil {
		return ErrRootBlockIsNil
	}

	shardHeaders := make([]*types.MinorBlockHeader, 0)
	for _, mHeader := range rBlock.MinorBlockHeaders() {
		h := mHeader.Hash()
		if mHeader.Branch == m.branch {
			if m.GetMinorBlock(h) == nil {
				return ErrMinorBlockIsNil
			}
			shardHeaders = append(shardHeaders, mHeader)
			continue
		}
		prevRootHeader := m.GetRootBlockByHash(mHeader.PrevRootBlockHash)
		prevHeaderNumber := prevRootHeader.Number()
		if prevRootHeader == nil || prevRootHeader.Number() == uint32(m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)) || !m.isNeighbor(mHeader.Branch, &prevHeaderNumber) {
			if data := rawdb.ReadCrossShardTxList(m.db, h); data != nil {
				return errors.New("already have")
			}
			continue
		}

		if data := rawdb.ReadCrossShardTxList(m.db, h); data == nil {
			return errors.New("not have")
		}

	}
	if uint64(len(shardHeaders)) > m.getMaxBlocksInOneRootBlock() {
		return errors.New("shardHeaders big than config")
	}

	lastMinorHeaderInPrevRootBlock := m.getLastConfirmedMinorBlockHeaderAtRootBlock(rBlock.Header().ParentHash)

	var shardHeader *types.MinorBlockHeader
	if len(shardHeaders) > 0 {
		if shardHeaders[0].Number == 0 || shardHeaders[0].ParentHash == lastMinorHeaderInPrevRootBlock.Hash() {
			shardHeader = shardHeaders[len(shardHeaders)-1]
		} else {
			return errors.New("master should assure this check will not fail")
		}
	} else {
		shardHeader = lastMinorHeaderInPrevRootBlock
	}
	m.putRootBlock(rBlock, shardHeader, common.Hash{})
	if shardHeader != nil {
		if !m.isSameRootChain(rBlock.Header(), m.getRootBlockHeaderByHash(shardHeader.PrevRootBlockHash)) {
			return ErrNotSameRootChain
		}
	}

	if rBlock.Header().Number <= m.rootTip.Number {
		if !m.isSameRootChain(m.rootTip, m.GetRootBlockByHash(m.CurrentBlock().IHeader().(*types.MinorBlockHeader).GetPrevRootBlockHash()).Header()) {
			return ErrNotSameRootChain
		}
		return nil
	}

	m.rootTip = rBlock.Header()
	m.confirmedHeaderTip = shardHeader

	origHeaderTip := m.CurrentHeader().(*types.MinorBlockHeader)
	if shardHeader != nil {
		origBlock := m.GetBlockByNumber(shardHeader.Number)
		if qkcCommon.IsNil(origBlock) || origBlock.Hash() != shardHeader.Hash() {
			m.hc.SetCurrentHeader(shardHeader)
			block := m.GetMinorBlock(shardHeader.Hash())
			m.currentBlock.Store(block)
		}
	}

	for !m.isSameRootChain(m.rootTip, m.getRootBlockHeaderByHash(m.CurrentHeader().(*types.MinorBlockHeader).GetPrevRootBlockHash())) {
		if m.CurrentHeader().NumberU64() == 0 {
			genesisRootHeader := m.rootTip
			genesisHeight := m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)
			if genesisRootHeader.Number < uint32(genesisHeight) {
				return errors.New("genesis root height small than config")
			}
			for genesisRootHeader.Number != uint32(genesisHeight) {
				genesisRootHeader = m.getRootBlockHeaderByHash(genesisRootHeader.ParentHash)
				if genesisRootHeader == nil {
					return ErrMinorBlockIsNil
				}
			}
			newGenesis := m.getGenesisBlock(genesisRootHeader.Hash())
			if newGenesis == nil {
				panic(errors.New("get genesis block is nil"))
			}
			m.genesisBlock = newGenesis
			if err := m.Reset(); err != nil {
				return err
			}
			break
		}
		preBlock := m.GetBlock(m.CurrentHeader().GetParentHash()).(*types.MinorBlock)
		m.hc.SetCurrentHeader(preBlock.Header())
		m.currentBlock.Store(preBlock)
	}

	if m.CurrentHeader().Hash() != origHeaderTip.Hash() {
		origBlock := m.GetMinorBlock(origHeaderTip.Hash())
		newBlock := m.GetMinorBlock(m.CurrentHeader().Hash())
		return m.reWriteBlockIndexTo(origBlock, newBlock)
	}
	return nil
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
		if evmState.GetGasUsed() == evmState.GetGasLimit() {
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
		addGasUsed := new(big.Int).SetUint64(0)
		if tx.GasPrice.Value.Uint64() != 0 {
			addGasUsed = params.GtxxShardCost
		}
		gasUsed := evmState.GetGasLimit()
		addGasUsedT := new(big.Int).Add(evmState.GetGasUsed(), addGasUsed)
		if gasUsed.Cmp(addGasUsedT) > 0 {
			gasUsed = addGasUsedT
		}
		evmState.SetGasUsed(gasUsed)
		xShardFee := new(big.Int).Mul(params.GtxxShardCost, tx.GasPrice.Value)
		xShardFee = qkcCommon.BigIntMulBigRat(xShardFee, localFeeRate)
		evmState.AddBlockFee(xShardFee)
		evmState.AddBalance(evmState.GetBlockCoinbase(), new(big.Int).SetUint64(xShardFee.Uint64()))
	}
	evmState.SetXShardReceiveGasUsed(evmState.GetGasUsed())
	return txList, nil
}

// GetTransactionByHash get tx by hash
func (m *MinorBlockChain) GetTransactionByHash(hash common.Hash) (*types.MinorBlock, uint32) {
	_, mHash, txIndex := rawdb.ReadTransaction(m.db, hash)
	if mHash == qkcCommon.EmptyHash {
		txs := make([]*types.Transaction, 0)
		m.txPool.mu.Lock() // to lock txpool.all
		txs = append(txs, m.txPool.all.all[hash])
		m.txPool.mu.Unlock()
		temp := types.NewMinorBlock(&types.MinorBlockHeader{}, &types.MinorBlockMeta{}, txs, nil, nil)
		return temp, 0
	}
	return m.GetMinorBlock(mHash), txIndex
}

// GetTransactionReceipt get tx receipt by hash for slave
func (m *MinorBlockChain) GetTransactionReceipt(hash common.Hash) (*types.MinorBlock, uint32, types.Receipts) {
	block, index := m.GetTransactionByHash(hash)
	receipts := m.GetReceiptsByHash(block.Hash())
	return block, index, receipts
}

// GetTransactionListByAddress get txList by addr
func (m *MinorBlockChain) GetTransactionListByAddress(address account.Address, start, limit uint64) {
	panic(errors.New("dad"))
}

// GetShardStatus show shardStatus
func (m *MinorBlockChain) GetShardStatus() (*ShardStatus, error) {
	// getBlockCountByHeight have lock
	cutoff := m.CurrentBlock().IHeader().GetTime() - 60
	block := m.CurrentBlock()

	txCount := uint32(0)
	blockCount := uint32(0)
	staleBlockCount := uint32(0)
	lastBlockTime := uint32(0)
	for block.IHeader().NumberU64() > 0 && block.IHeader().GetTime() > cutoff {
		txCount += uint32(len(block.GetTransactions()))
		blockCount++
		staleBlockCount = uint32(m.getBlockCountByHeight(block.Header().Number))
		block = m.GetMinorBlock(block.IHeader().GetParentHash())
		if block == nil {
			return nil, ErrMinorBlockIsNil
		}
		if lastBlockTime == 0 {
			lastBlockTime = uint32(m.CurrentBlock().IHeader().GetTime() - block.IHeader().GetTime())
		}
	}
	if staleBlockCount < 0 {
		return nil, errors.New("staleBlockCount should >=0")
	}
	return &ShardStatus{
		Branch:             m.branch,
		Height:             m.CurrentBlock().IHeader().NumberU64(),
		Difficulty:         m.CurrentBlock().IHeader().GetDifficulty(),
		CoinbaseAddress:    m.CurrentBlock().IHeader().GetCoinbase(),
		TimeStamp:          m.CurrentBlock().IHeader().GetTime(),
		TxCount60s:         txCount,
		PendingTxCount:     uint32(len(m.txPool.pending)),
		TotalTxCount:       *m.getTotalTxCount(m.CurrentBlock().Hash()),
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

		localFee := big.NewRat(1, 1)
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
func (m *MinorBlockChain) GasPrice() *uint64 {
	// no need to lock
	currHead := m.CurrentBlock().Hash()
	if currHead == m.gasPriceSuggestionOracle.LastHead {
		return &m.gasPriceSuggestionOracle.LastPrice
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
			return nil
		}
		tempPreBlockPrices := make([]uint64, 0)
		for _, v := range block.GetTransactions() {
			tempPreBlockPrices = append(tempPreBlockPrices, v.EvmTx.GasPrice().Uint64())
		}
		prices = append(prices, tempPreBlockPrices...)
	}
	if len(prices) == 0 {
		return nil
	}

	sort.Slice(prices, func(i, j int) bool { return prices[i] < prices[j] })
	price := prices[(len(prices)-1)*int(m.gasPriceSuggestionOracle.Percentile)/100]
	m.gasPriceSuggestionOracle.LastPrice = price
	m.gasPriceSuggestionOracle.LastHead = currHead
	return &price
}

func (m *MinorBlockChain) getBlockCountByHeight(height uint64) uint64 {
	m.mu.RLock() //to lock heightToMinorBlockHashes
	defer m.mu.RUnlock()
	if _, ok := m.heightToMinorBlockHashes[height]; ok == false {
		return 0
	}
	return uint64(len(m.heightToMinorBlockHashes[height]))
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

// updateTip : already have locked
func (m *MinorBlockChain) updateTip(state *state.StateDB, block *types.MinorBlock) (bool, error) {
	updateTip := false
	if !m.isSameRootChain(m.rootTip, m.getRootBlockHeaderByHash(block.Header().PrevRootBlockHash)) {
		updateTip = false
	} else if block.Header().ParentHash.String() == m.CurrentBlock().Hash().String() {
		updateTip = true
	} else if m.isMinorBlockLinkedToRootTip(block) {
		if block.Header().Number > m.CurrentBlock().NumberU64() {
			updateTip = true
		} else if block.Header().Number == m.CurrentBlock().NumberU64() {
			updateTip = m.getRootBlockHeaderByHash(block.Header().PrevRootBlockHash).Number > m.getRootBlockHeaderByHash(m.CurrentBlock().IHeader().(*types.MinorBlockHeader).GetPrevRootBlockHash()).Number
		}
	}

	if updateTip {
		if m.clusterConfig.Quarkchain.GetShardConfigByFullShardID(m.branch.Value).PoswConfig.Enabled {
			disallowList, err := m.getPOSWCoinbaseBlockCnt(block.Hash(), nil)
			if err != nil {
				return updateTip, err
			}
			state.SetSenderDisallowList(disallowList)
		}
		m.currentEvmState = state
	}
	return updateTip, nil
}

// POSWDiffAdjust POSW diff calc,already locked by insertChain
func (m *MinorBlockChain) POSWDiffAdjust(block types.IBlock) (uint64, error) {
	startTime := time.Now()
	header := block.IHeader() //already check
	diff := uint32(header.GetDifficulty().Uint64())
	coinbaseAddress := header.GetCoinbase().Recipient

	evmState, err := m.getEvmStateForNewBlock(block, true)
	if err != nil {
		return 0, err
	}
	config := m.shardConfig.PoswConfig
	stakes := evmState.GetBalance(coinbaseAddress)

	blockThreShold := stakes.Uint64() / config.TotalStakePerBlock.Uint64()
	if config.WindowSize < uint32(blockThreShold) {
		blockThreShold = uint64(config.WindowSize)
	}

	windowSize := config.WindowSize
	blockCnt, err := m.getPOSWCoinbaseBlockCnt(header.GetParentHash(), &windowSize)
	log.Info(m.logInfo, blockCnt)
	//TODO ---block_cnt.get()
	var cnt uint64 = 1
	if cnt < blockThreShold {
		diff /= config.DiffDivider
	}
	passedMs := (time.Now().Sub(startTime)) * 1000
	log.Info(m.logInfo, "adjust posw diff took milliseconds", passedMs)
	return uint64(diff), nil
}

func (m *MinorBlockChain) GetBranch() account.Branch {
	return m.branch
}
