package core

import (
	"errors"
	"math/big"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/rawdb"

	"github.com/ethereum/go-ethereum/common"

	"github.com/QuarkChain/goquarkchain/account"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
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
	// TODO 1: put total tx count.
	// TODO 2: put confirmed xshard tx deposits.
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
	// TODO
	return nil, nil
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
func (m *MinorBlockChain) GetTransactionCount(recipient account.Recipient, height *uint64) (uint64, error) {
	panic("not implemented")
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
