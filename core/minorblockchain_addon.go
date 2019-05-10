package core

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/serialize"
	"math/big"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

func (m *MinorBlockChain) getLastConfirmedMinorBlockHeaderAtRootBlock(hash common.Hash) *types.MinorBlockHeader {
	rMinorHeaderHash := rawdb.ReadLastConfirmedMinorBlockHeaderAtRootBlock(m.db, hash)
	if rMinorHeaderHash == qkcCommon.EmptyHash {
		return nil
	}
	return rawdb.ReadMinorBlockHeader(m.db, rMinorHeaderHash)
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
	rawdb.WriteMinorBlock(m.db, mBlock)
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
			return nil, ErrRootBlockIsNil
		}
	}
	if evmState.GetGasUsed().Cmp(evmState.GetGasLimit()) > 0 {
		return nil, errors.New("runCrossShardTxList err:gasUsed > GasLimit")
	}
	return txList, nil
}

func (m *MinorBlockChain) validateTx(tx *types.Transaction, evmState *state.StateDB, fromAddress *account.Address, gas *uint64) (*types.Transaction, error) {
	if tx.TxType != types.EvmTx {
		return nil, errors.New("unexpected tx type")
	}
	evmTx := tx.EvmTx
	if fromAddress != nil {
		if evmTx.FromFullShardKey() != fromAddress.FullShardKey {
			return nil, errors.New("from full shard id not match")
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
			return nil, errors.New("shard is not initialized yet")
		}
	}

	if evmTx.IsCrossShard() && !m.isNeighbor(toBranch, nil) {
		return nil, ErrNotNeighbor
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
func (m *MinorBlockChain) InitGenesisState(rBlock *types.RootBlock) (*types.MinorBlock, error) {
	m.mu.Lock() // used in initFromRootBlock and addRootBlock, so should lock
	defer m.mu.Unlock()
	var err error
	gBlock, err := NewGenesis(m.clusterConfig.Quarkchain).CreateMinorBlock(rBlock, m.branch.Value, m.db)
	if err != nil {
		return nil, err
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
	evmState, err := m.getDefaultEvmState(height)
	if err != nil {
		return 0, err
	}
	return evmState.GetNonce(recipient), nil
}
func (m *MinorBlockChain) isSameRootChain(long types.IHeader, short types.IHeader) bool {
	return isSameRootChain(m.db, long, short)
}

func (m *MinorBlockChain) isMinorBlockLinkedToRootTip(mBlock *types.MinorBlock) bool {
	confirmed := m.confirmedHeaderTip
	if confirmed == nil {
		return true
	}
	if mBlock.Header().Number <= confirmed.Number {
		return false
	}
	return m.isSameMinorChain(mBlock.Header(), confirmed)
}
func (m *MinorBlockChain) isNeighbor(remoteBranch account.Branch, rootHeight *uint32) bool {
	if rootHeight == nil {
		rootHeight = &m.rootTip.Number
	}
	shardSize := len(m.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(*rootHeight))
	return account.IsNeighbor(m.branch, remoteBranch, uint32(shardSize))
}

func (m *MinorBlockChain) putRootBlock(rBlock *types.RootBlock, minorHeader *types.MinorBlockHeader) {
	rBlockHash := rBlock.Hash()
	rawdb.WriteRootBlock(m.db, rBlock)
	var mHash common.Hash
	if minorHeader != nil {
		mHash = minorHeader.Hash()
	}
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
func (m *MinorBlockChain) GetPOSWCoinbaseBlockCnt(headerHash common.Hash, length *uint32) (map[account.Recipient]uint32, error) {
	panic(-1)
}

func (m *MinorBlockChain) getCoinbaseAddressUntilBlock(headerHash common.Hash, length uint32) ([]account.Recipient, error) {
	panic(-1)
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
	confirmedHeaderTip := m.getLastConfirmedMinorBlockHeaderAtRootBlock(rBlock.Hash())
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
	panic(-1)
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

func (m *MinorBlockChain) computeGasLimit(parentGasLimit, parentGasUsed, gasLimitFloor uint64) (*big.Int, error) {
	// no need to lock
	shardConfig := m.shardConfig
	if gasLimitFloor < shardConfig.GasLimitMinimum {
		return nil, errors.New("gas limit floor is too low")
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
		return new(big.Int).SetUint64(parentGasLimit + decay), nil
	}
	return new(big.Int).SetUint64(gasLimit), nil
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
	panic(-1)
}

// GetCode get code for addr
func (m *MinorBlockChain) GetCode(recipient account.Recipient, height *uint64) ([]byte, error) {
	panic(-1)
}

// GetStorageAt get storage for addr
func (m *MinorBlockChain) GetStorageAt(recipient account.Recipient, key common.Hash, height *uint64) (common.Hash, error) {
	panic(-1)
}

// ExecuteTx execute tx
func (m *MinorBlockChain) ExecuteTx(tx *types.Transaction, fromAddress *account.Address, height *uint64) ([]byte, error) {
	panic(-1)
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
	panic(-1)
}

func (m *MinorBlockChain) getMaxBlocksInOneRootBlock() uint64 {
	return uint64(m.shardConfig.MaxBlocksPerShardInOneRootBlock())
}

// GetUnconfirmedHeadersCoinbaseAmount get unconfirmed headers coinbase amount
func (m *MinorBlockChain) GetUnconfirmedHeadersCoinbaseAmount() uint64 {
	panic(-1)
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
	toShardSize := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.ToChainID())
	if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
		return ErrorTxContinue
	}
	fromShardSize := m.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
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
	m.mu.Lock() // to lock txpool and getEvmStateForNewBlock
	defer m.mu.Unlock()
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
		gasLimit, err = m.computeGasLimit(prevBlock.Header().GetGasLimit().Uint64(), prevBlock.GetMetaData().GasUsed.Value.Uint64(), m.shardConfig.Genesis.GasLimit)
	}
	//newGasLimit, err := m.computeGasLimit(prevBlock.Header().GetGasLimit().Uint64(), prevBlock.GetMetaData().GasUsed.Value.Uint64(), m.shardConfig.Genesis.GasLimit)

	block := prevBlock.CreateBlockToAppend(&realCreateTime, difficulty, address, nil, gasLimit, nil, nil)
	evmState, err := m.getEvmStateForNewBlock(block, true)
	//if gasLimit != nil {
	//	evmState.SetGasLimit(gasLimit)
	//}

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
	panic(-1)
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
			if !m.HasBlock(h) {
				return ErrMinorBlockIsNil
			}
			shardHeaders = append(shardHeaders, mHeader)
			continue
		}
		prevRootHeader := m.GetRootBlockByHash(mHeader.PrevRootBlockHash)
		prevHeaderNumber := uint32(0)
		if prevRootHeader != nil {
			prevHeaderNumber = prevRootHeader.Number()
		}

		// prev_root_header can be None when the shard is not created at root height 0
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
	m.putRootBlock(rBlock, shardHeader)
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
			newGenesis := rawdb.ReadGenesis(m.db, genesisRootHeader.Hash()) // genesisblock key is rootblock hash
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
	panic(-1)
}

// GetTransactionReceipt get tx receipt by hash for slave
func (m *MinorBlockChain) GetTransactionReceipt(hash common.Hash) (*types.MinorBlock, uint32, *types.Receipt) {
	panic(-1)
}

// GetTransactionListByAddress get txList by addr
func (m *MinorBlockChain) GetTransactionListByAddress(address account.Address, start, limit uint64) {
	panic(-1)
}

// GetShardStatus show shardStatus
func (m *MinorBlockChain) GetShardStatus() (*rpc.ShardStatus, error) {
	// getBlockCountByHeight have lock
	cblock := m.CurrentBlock()
	cutoff := cblock.IHeader().GetTime() - 60

	txCount := uint32(0)
	blockCount := uint32(0)
	staleBlockCount := uint32(0)
	lastBlockTime := uint32(0)
	for cblock.IHeader().NumberU64() > 0 && cblock.IHeader().GetTime() > cutoff {
		txCount += uint32(len(cblock.GetTransactions()))
		blockCount++
		staleBlockCount = uint32(m.getBlockCountByHeight(cblock.Header().Number))
		cblock = m.GetMinorBlock(cblock.IHeader().GetParentHash())
		if cblock == nil {
			return nil, ErrMinorBlockIsNil
		}
		if lastBlockTime == 0 {
			lastBlockTime = uint32(m.CurrentBlock().IHeader().GetTime() - cblock.IHeader().GetTime())
		}
	}
	if staleBlockCount < 0 {
		return nil, errors.New("staleBlockCount should >=0")
	}
	return &rpc.ShardStatus{
		Branch:             m.branch,
		Height:             cblock.IHeader().NumberU64(),
		Difficulty:         cblock.IHeader().GetDifficulty(),
		CoinbaseAddress:    cblock.IHeader().GetCoinbase(),
		Timestamp:          cblock.IHeader().GetTime(),
		TxCount60s:         txCount,
		PendingTxCount:     uint32(len(m.txPool.pending)),
		TotalTxCount:       *m.getTotalTxCount(cblock.Hash()),
		BlockCount60s:      blockCount,
		StaleBlockCount60s: staleBlockCount,
		LastBlockTime:      lastBlockTime,
	}, nil
}

// EstimateGas estimate gas for this tx
func (m *MinorBlockChain) EstimateGas(tx *types.Transaction, fromAddress account.Address) (uint32, error) {
	panic(-1)
}

// GasPrice gas price
func (m *MinorBlockChain) GasPrice() *uint64 {
	panic(-1)
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

// POSWDiffAdjust POSW diff calc,already locked by insertChain
// TODO to finish it later
func (m *MinorBlockChain) POSWDiffAdjust(block types.IBlock) (uint64, error) {
	panic(-1)
}

func (m *MinorBlockChain) GetBranch() account.Branch {
	panic(-1)
}

func (m *MinorBlockChain) GetRootTip() *types.RootBlockHeader {
	return m.rootTip
}
