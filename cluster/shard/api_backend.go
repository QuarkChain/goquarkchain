package shard

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	synchronizer "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
)

var (
	directionToGenesis = uint8(0)
	directionToTip     = uint8(1)
)

// Wrapper over master connection, used by synchronizer.
type peer struct {
	cm     ConnManager
	peerID string
}

func (p *peer) GetMinorBlockHeaderList(hash common.Hash, limit, branch uint32, reverse bool) ([]*types.MinorBlockHeader, error) {
	req := &rpc.GetMinorBlockHeaderListRequest{
		Branch:    branch,
		BlockHash: hash,
		Limit:     limit,
		Direction: directionToGenesis,
		PeerID:    p.peerID,
	}
	return p.cm.GetMinorBlockHeaders(req)
}

func (p *peer) GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	return p.cm.GetMinorBlocks(hashes, p.peerID, branch)
}

func (p *peer) PeerID() string {
	return p.peerID
}

func (s *ShardBackend) GetUnconfirmedHeaderList() ([]*types.MinorBlockHeader, error) {
	headers := s.MinorBlockChain.GetUnconfirmedHeaderList()
	return headers, nil
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
	log.Info(s.logInfo, "shardBackend addminorBlock number", block.Number(), "hash", block.Hash().String())
	defer log.Info(s.logInfo, "shardBackend addminorBlock", "end")
	var (
		oldTip = s.MinorBlockChain.CurrentHeader()
	)

	_, xshardLst, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block})
	if err != nil || len(xshardLst) != 1 {
		log.Error("Failed to add minor block", "err", err)
		return err
	}
	// only remove from pool if the block successfully added to state,
	// this may cache failed blocks but prevents them being broadcasted more than needed
	s.mBPool.delBlockInPool(block)

	// block has been added to local state, broadcast tip so that peers can sync if needed
	if oldTip.Hash() != s.MinorBlockChain.CurrentHeader().Hash() {
		if err = s.broadcastNewTip(); err != nil {
			return err
		}
	}

	if xshardLst[0] == nil {
		log.Info("add minor block has been added...", "branch", s.fullShardId, "height", block.Number())
		return nil
	}

	prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash).Header().Number
	if err := s.conn.BroadcastXshardTxList(block, xshardLst[0], prevRootHeight); err != nil {
		return err
	}
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

func (s *ShardBackend) AddRootBlock(rBlock *types.RootBlock) (switched bool, err error) {
	log.Info(s.logInfo, "AddRootBlock height", rBlock.Number(), "hash", rBlock.Hash().String())
	defer log.Info(s.logInfo, "AddRootBlock ", "end")
	switched = false
	if rBlock.Header().Number > s.genesisRootHeight {
		switched, err = s.MinorBlockChain.AddRootBlock(rBlock)
	}
	if rBlock.Header().Number == s.genesisRootHeight {
		err = s.initGenesisState(rBlock)
	}
	return
}

// Add blocks in batch to reduce RPCs. Will NOT broadcast to peers.
//
// Returns true if blocks are successfully added. False on any error.
// This function only adds blocks to local and propagate xshard list to other shards.
// It does NOT notify master because the master should already have the minor header list,
// and will add them once this function returns successfully.
func (s *ShardBackend) AddBlockListForSync(blockLst []*types.MinorBlock) error {
	blockHashToXShardList := make(map[common.Hash]*XshardListTuple)
	if len(blockLst) == 0 {
		return errors.New(fmt.Sprintf("empty root block list in %d", s.Config.ShardID))
	}

	for _, block := range blockLst {
		blockHash := block.Header().Hash()
		if block.Header().Branch.GetFullShardID() != s.fullShardId || s.MinorBlockChain.HasBlock(block.Hash()) {
			continue
		}
		_, xshardLst, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block})
		if err != nil || len(xshardLst) != 1 {
			log.Error("Failed to add minor block", "err", err)
			return err
		}
		s.mBPool.delBlockInPool(block)
		prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash)
		blockHashToXShardList[blockHash] = &XshardListTuple{XshardTxList: xshardLst[0], PrevRootHeight: prevRootHeight.Number()}
	}
	// interrupt the current miner and restart
	return s.conn.BatchBroadcastXshardTxList(blockHashToXShardList, blockLst[0].Header().Branch)
}

func (s *ShardBackend) GetTransactionListByAddress(address *account.Address,
	start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	return s.MinorBlockChain.GetTransactionByAddress(*address, start, limit)
}

// TODO 当前版本暂不添加
func (s *ShardBackend) GetLogs() ([]*types.Log, error) { panic("not implemented") }

func (s *ShardBackend) PoswDiffAdjust(block *types.MinorBlock) (*big.Int, error) {
	panic("not implemented")
}

func (s *ShardBackend) GetWork() (*consensus.MiningWork, error) {
	return s.miner.GetWork()
}

func (s *ShardBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) error {
	if ok := s.miner.SubmitWork(nonce, headerHash, mixHash); ok {
		return nil
	}
	return errors.New("submit mined work failed")
}

func (s *ShardBackend) HandleNewTip(rBHeader *types.RootBlockHeader, mBHeader *types.MinorBlockHeader, peerID string) error {
	if s.MinorBlockChain.CurrentHeader().NumberU64() >= mBHeader.Number {
		return nil
	}

	peer := &peer{cm: s.conn, peerID: peerID}
	err := s.synchronizer.AddTask(synchronizer.NewMinorChainTask(peer, mBHeader))
	if err != nil {
		log.Error("Failed to add minor chain task,", "hash", mBHeader.Hash(), "height", mBHeader.Number)
	}

	log.Info("Handle new tip received new tip with height", "shard height", mBHeader.Number)
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
	log.Info(s.logInfo, "NewMinorBlock height", block.Header().Number, "hash", block.Header().Hash().String())
	defer log.Info(s.logInfo, "NewMinorBlock", "end")
	// TODO
	mHash := block.Header().Hash()
	if s.mBPool.getBlockInPool(mHash) != nil {
		return
	}
	if s.MinorBlockChain.HasBlock(block.Hash()) {
		log.Info("add minor block, Known minor block", "branch", block.Header().Branch, "height", block.Number())
		return
	}
	if !s.MinorBlockChain.HasBlock(block.Header().ParentHash) && s.mBPool.getBlockInPool(block.ParentHash()) == nil {
		log.Info("prarent block hash be included", "parent hash: ", block.Header().ParentHash.Hex())
		return
	}

	header := block.Header()
	diff := header.Difficulty
	diffDivider := big.NewInt(int64(s.Config.PoswConfig.DiffDivider))
	if s.Config.PoswConfig.Enabled {
		diff = diff.Div(diff, diffDivider)
	}
	if err = s.engine.VerifySeal(s.MinorBlockChain, header, diff); err != nil {
		log.Error("got block with bad seal in handle_new_block", "branch", header.Branch.Value, "err", err)
		return err
	}

	timeNow := uint64(time.Now().UnixNano())
	if header.Time > timeNow+30 {
		return
	}

	s.mBPool.setBlockInPool(block)
	if err = s.conn.BroadcastMinorBlock(block, s.fullShardId); err != nil {
		return err
	}
	return s.AddMinorBlock(block)
}

func (s *ShardBackend) addTxList(txs []*types.Transaction) error {
	var g errgroup.Group
	for index := range txs {
		idx := index
		g.Go(func() error {
			return s.MinorBlockChain.AddTx(txs[idx])
		})
	}
	return g.Wait()
}

func (s *ShardBackend) GenTx(genTxs *rpc.GenTxRequest) error {
	return s.txGenerator.Generate(genTxs, s.MinorBlockChain.GetTransactionCount, s.addTxList)
}

// miner api
func (s *ShardBackend) CreateBlockToMine() (types.IBlock, error) {
	return s.MinorBlockChain.CreateBlockToMine(nil, &s.Config.CoinbaseAddress, nil)
}

func (s *ShardBackend) InsertMinedBlock(block types.IBlock) error {
	return s.AddMinorBlock(block.(*types.MinorBlock))
}
