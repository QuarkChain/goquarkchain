package core

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/QuarkChain/goquarkchain/account"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
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
	localFeeRate := big.NewRat(1, 1)
	if qkcConfig != nil {
		num := qkcConfig.RewardTaxRate.Num().Int64()
		demo := qkcConfig.RewardTaxRate.Denom().Int64()
		localFeeRate = big.NewRat(demo-num, demo)
	}
	return localFeeRate
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
	//TODO add logic
	return nil
}
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
		m.currentEvmState = state
	}
	return updateTip, nil
}

func (m *MinorBlockChain) runCrossShardTxList(evmState *state.StateDB, descendantRootHeader *types.RootBlockHeader, ancestorRootHeader *types.RootBlockHeader) ([]*types.CrossShardTransactionDeposit, error) {
	//TODO
	ans := make([]*types.CrossShardTransactionDeposit, 0)
	return ans, nil
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
func (m *MinorBlockChain) GetTransactionCount(recipient account.Recipient, height *uint64) (uint64, error) {
	panic("not implemented")
}
func (m *MinorBlockChain) isSameRootChain(long types.IHeader, short types.IHeader) bool {
	// no need to lock
	return consensus.IsSameRootChain(m.db, long, short)
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
func (m *MinorBlockChain) isNeighbor(remoteBranch account.Branch, rootHeight *uint32) bool {
	// no need to lock
	if rootHeight == nil {
		rootHeight = &m.rootTip.Number
	}
	shardSize := len(m.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(*rootHeight))
	return account.IsNeighbor(m.branch, remoteBranch, uint32(shardSize))
}

func (m *MinorBlockChain) putRootBlock(rBlock *types.RootBlock, minorHeader *types.MinorBlockHeader) {
	// no need to lock
	rBlockHash := rBlock.Hash()
	rawdb.WriteRootBlock(m.db, rBlock)
	var mHash common.Hash
	if minorHeader != nil {
		mHash = minorHeader.Hash()
	}
	rawdb.WriteLastConfirmedMinorBlockHeaderAtRootBlock(m.db, rBlockHash, mHash)
}

// createEvmState already locked
func (m *MinorBlockChain) createEvmState(trieRootHash common.Hash, headerHash common.Hash) (*state.StateDB, error) {
	evmState, err := m.StateAt(trieRootHash)
	if err != nil {
		return nil, err
	}
	evmState.SetShardConfig(m.shardConfig)
	return evmState, nil
}
