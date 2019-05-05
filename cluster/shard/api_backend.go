package shard

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"reflect"
)

func (s *ShardBackend) GetUnconfirmedHeaderList() ([]*types.MinorBlockHeader, error) {
	headers := s.State.GetAllUnconfirmedHeaderList()
	return headers[0:s.maxBlocks], nil
}

func (s *ShardBackend) broadcastNewTip() (err error) {

	var (
		rootTip  = s.State.GetRootTip()
		minorTip = s.State.CurrentHeader().(*types.MinorBlockHeader)
	)
	if s.bstRHObserved != nil {
		if rootTip.Number < s.bstRHObserved.Number {
			return
		}
		if reflect.DeepEqual(rootTip, s.bstRHObserved) {
			if uint64(rootTip.Number) < s.bstMHObserved.Number {
				return
			}
			if reflect.DeepEqual(minorTip, s.bstMHObserved) {
				return
			}
		}
	}

	err = s.conn.BroadcastNewTip([]*types.MinorBlockHeader{minorTip}, rootTip, s.fullShardId)
	return
}

// Returns true if block is successfully added. False on any error.
// called by 1. local miner (will not run if syncing) 2. SyncTask
func (s *ShardBackend) AddMinorBlock(block *types.MinorBlock) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		oldTip = s.State.CurrentHeader()
	)

	if s.State.HasBlock(block.Hash()) {
		log.Info("add minor block", "Known minor block", "branch", block.Header().Branch, "height", block.Number())
		return nil
	}
	if block.Header().ParentHash != oldTip.Hash() {
		// Tip changed, don't bother creating a fork
		log.Info("add minor block", "dropped stale block mined locally",
			"branch", block.Header().Branch.Value, "minor height", block.Header().Number)
		return nil
	}

	_, xshardLst, err := s.State.InsertChain([]types.IBlock{block})
	if err != nil || len(xshardLst) != 1 {
		log.Error("Failed to add minor block, err %v", err)
		return err
	}
	// only remove from pool if the block successfully added to state,
	// this may cache failed blocks but prevents them being broadcasted more than needed
	delete(s.newBlockPool, block.Header().Hash())

	// block has been added to local state, broadcast tip so that peers can sync if needed
	if !reflect.DeepEqual(oldTip, s.State.CurrentHeader()) {
		if err = s.broadcastNewTip(); err != nil {
			return err
		}
	}

	if xshardLst[0] == nil {
		log.Info("add minor block", "has been added...", "branch", s.fullShardId, "height", block.Number())
		return nil
	}

	prevRootHeight := s.State.GetRootBlockByHash(block.Header().Hash()).Header().Number
	s.conn.BroadcastXshardTxList(block, xshardLst[0], prevRootHeight)
	status, err := s.State.GetShardStatus()
	if err != nil {
		return err
	}
	err = s.conn.SendMinorBlockHeaderToMaster(
		block.Header(),
		uint32(block.Transactions().Len()),
		uint32(len(xshardLst[0])),
		status,
	)
	if err != nil {
		return err
	}
	return nil
}

// Either recover state from local db or create genesis state based on config
func (s *ShardBackend) InitFromRootBlock(rBlock *types.RootBlock) error {
	if rBlock.Header().Number > s.genesisRootHeight {
		return s.State.InitFromRootBlock(rBlock)
	}
	if rBlock.Header().Number == s.genesisRootHeight {
		return s.initGenesisState(rBlock)
	}
	return nil
}

func (s *ShardBackend) AddRootBlock(rBlock *types.RootBlock) error {
	if rBlock.Header().Number > s.genesisRootHeight {
		if err := s.State.AddRootBlock(rBlock); err != nil {
			return err
		}
	}
	if rBlock.Header().Number == s.genesisRootHeight {
		return s.initGenesisState(rBlock)
	}
	return nil
}

// Add blocks in batch to reduce RPCs. Will NOT broadcast to peers.
//
// Returns true if blocks are successfully added. False on any error.
// This function only adds blocks to local and propagate xshard list to other shards.
// It does NOT notify master because the master should already have the minor header list,
// and will add them once this function returns successfully.
func (s *ShardBackend) AddBlockListForSync(blockLst []*types.MinorBlock) error {

	hashToXShardList := make(map[common.Hash]*XshardListTuple)
	if blockLst == nil {
		return errors.New(fmt.Sprintf("empty root block list in %d", s.Config.ShardID))
	}

	for _, block := range blockLst {
		if block.Header().Branch.GetFullShardID() != s.fullShardId || s.State.HasBlock(block.Hash()) {
			continue
		}
		if _, _, err := s.State.InsertChain([]types.IBlock{block}); err != nil {
			return err
		}
	}
	s.conn.BatchBroadcastXshardTxList(hashToXShardList, blockLst[0].Header().Branch)

	return nil
}

// TODO 当前版本暂不添加
func (s *ShardBackend) GetTransactionListByAddress(address *account.Address,
	start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	panic("not implemented")
}

// TODO 当前版本暂不添加
func (s *ShardBackend) GetLogs() ([]*types.Log, error) { panic("not implemented") }

func (s *ShardBackend) PoswDiffAdjust(block *types.MinorBlock) (*big.Int, error) { panic("not implemented") }

func (s *ShardBackend) GetWork() (*consensus.MiningWork, error) {
	return s.engine.GetWork()
}

func (s *ShardBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) error {
	if ok := s.engine.SubmitWork(nonce, headerHash, mixHash); ok {
		return nil
	}
	return errors.New("submit mined work failed")
}

func (s *ShardBackend) HandleNewTip(rBHeader *types.RootBlockHeader, mBHeader *types.MinorBlockHeader) error {

	if s.bstRHObserved != nil {
		if rBHeader.Number < s.bstRHObserved.Number {
			return errors.New(fmt.Sprintf("best observed root header height is decreasing %d < %d", rBHeader.Number, s.bstRHObserved.Number))
		}

		if rBHeader.Number == s.bstRHObserved.Number {
			if !reflect.DeepEqual(rBHeader, s.bstRHObserved) {
				return errors.New(fmt.Sprintf("best observed root header changed with same height %d", s.bstRHObserved.Number))
			}
			if mBHeader.Number < s.bstMHObserved.Number {
				return errors.New(fmt.Sprintf("best observed minor header is decreasing %d < %d", mBHeader.Number, s.bstMHObserved.Number))
			}
		}
	}

	s.bstRHObserved = rBHeader
	s.bstMHObserved = mBHeader
	if s.State.CurrentHeader().NumberU64() >= mBHeader.Number {
		return nil
	}

	log.Info("handle new tip", "received new tip with height", "shard height", mBHeader.Number)
	return nil
}

func (s *ShardBackend) GetMinorBlock(mHash common.Hash, height *uint64) *types.MinorBlock {
	return nil
}
