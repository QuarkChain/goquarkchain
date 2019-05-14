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
	panic("GetUnconfirmedHeaderList")
	//headers := s.MinorBlockChain.GetAllUnconfirmedHeaderList()
	//return headers[0:s.maxBlocks], nil
}

func (s *ShardBackend) broadcastNewTip() (err error) {

	var (
		rootTip  = s.MinorBlockChain.GetRootTip()
		minorTip = s.MinorBlockChain.CurrentHeader().(*types.MinorBlockHeader)
	)

	err = s.conn.BroadcastNewTip([]*types.MinorBlockHeader{minorTip}, rootTip, s.fullShardId)
	return
}

// Returns true if block is successfully added. False on any error.
// called by 1. local miner (will not run if syncing) 2. SyncTask
func (s *ShardBackend) AddMinorBlock(block *types.MinorBlock) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var (
		oldTip = s.MinorBlockChain.CurrentHeader()
	)

	_, xshardLst, err := s.MinorBlockChain.InsertChain([]types.IBlock{block})
	if err != nil || len(xshardLst) != 1 {
		log.Error("Failed to add minor block, err %v", err)
		return err
	}
	// only remove from pool if the block successfully added to state,
	// this may cache failed blocks but prevents them being broadcasted more than needed
	delete(s.newBlockPool, block.Header().Hash())

	// block has been added to local state, broadcast tip so that peers can sync if needed
	if !reflect.DeepEqual(oldTip, s.MinorBlockChain.CurrentHeader()) {
		if err = s.broadcastNewTip(); err != nil {
			return err
		}
	}

	if xshardLst[0] == nil {
		log.Info("add minor block has been added...", "branch", s.fullShardId, "height", block.Number())
		return nil
	}

	prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash).Header().Number
	s.conn.BroadcastXshardTxList(block, xshardLst[0], prevRootHeight)
	status, err := s.MinorBlockChain.GetShardStatus()
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
		return s.MinorBlockChain.InitFromRootBlock(rBlock)
	}
	if rBlock.Header().Number == s.genesisRootHeight {
		return s.initGenesisState(rBlock)
	}
	return nil
}

func (s *ShardBackend) AddRootBlock(rBlock *types.RootBlock) error {
	if rBlock.Header().Number > s.genesisRootHeight {
		if err := s.MinorBlockChain.AddRootBlock(rBlock); err != nil {
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

	blockHashToXShardList := make(map[common.Hash]*XshardListTuple)
	if blockLst == nil {
		return errors.New(fmt.Sprintf("empty root block list in %d", s.Config.ShardID))
	}

	for _, block := range blockLst {
		blockHash := block.Header().Hash()
		if block.Header().Branch.GetFullShardID() != s.fullShardId || s.MinorBlockChain.HasBlock(block.Hash()) {
			continue
		}
		_, xshardLst, err := s.MinorBlockChain.InsertChain([]types.IBlock{block})
		if err != nil || len(xshardLst) != 1 {
			log.Error("Failed to add minor block, err %v", err)
			return err
		}
		prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash)
		blockHashToXShardList[blockHash] = &XshardListTuple{XshardTxList: xshardLst[0], PrevRootHeight: prevRootHeight.Number()}
	}
	s.conn.BatchBroadcastXshardTxList(blockHashToXShardList, blockLst[0].Header().Branch)

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

	// TODO sync.add_task
	if s.MinorBlockChain.CurrentHeader().NumberU64() >= mBHeader.Number {
		return nil
	}

	log.Info("handle new tip", "received new tip with height", "shard height", mBHeader.Number)
	return nil
}

func (s *ShardBackend) GetMinorBlock(mHash common.Hash, height *uint64) *types.MinorBlock {
	if mHash != (common.Hash{}) {
		return s.MinorBlockChain.GetMinorBlock(mHash)
	} else if height != nil {
		return s.MinorBlockChain.GetBlockByNumber(*height).(*types.MinorBlock)
	}
	return nil
}

func (s *ShardBackend) NewMinorBlock(block *types.MinorBlock) (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	mHash := block.Header().Hash()
	if _, ok := s.newBlockPool[mHash]; ok {
		return
	}
	if s.MinorBlockChain.HasBlock(block.Hash()) {
		log.Info("add minor block, Known minor block", "branch", block.Header().Branch, "height", block.Number())
		return
	}
	if !s.MinorBlockChain.HasBlock(block.Header().ParentHash) {
		return fmt.Errorf("prarent block hash be included, parent hash: %s", block.Header().ParentHash.Hex())
	}

	s.newBlockPool[mHash] = block
	if err = s.conn.BroadcastMinorBlock(block, s.fullShardId); err != nil {
		return err
	}
	return s.AddMinorBlock(block)
}
