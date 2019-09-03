package shard

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	synchronizer "github.com/QuarkChain/goquarkchain/cluster/sync"
	qkccommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var (
	AllowedFutureBlocksTimeBroadcast = 15
)

// Wrapper over master connection, used by synchronizer.
type peer struct {
	cm     ConnManager
	peerID string
}

func (p *peer) GetMinorBlockHeaderList(gReq *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	return p.cm.GetMinorBlockHeaderList(gReq)
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

func (s *ShardBackend) setHead(head uint64) {
	if err := s.MinorBlockChain.SetHead(head); err != nil {
		panic(err)
	}
}

// Returns true if block is successfully added. False on any error.
// called by 1. local miner (will not run if syncing) 2. SyncTask
func (s *ShardBackend) AddMinorBlock(block *types.MinorBlock) error {
	var (
		oldTip = s.MinorBlockChain.CurrentHeader()
	)

	if commitStatus := s.getBlockCommitStatusByHash(block.Header().Hash()); commitStatus == BLOCK_COMMITTED {
		return nil
	}
	//TODO support BLOCK_COMMITTING
	currHead := s.MinorBlockChain.CurrentBlock().Number()
	_, xshardLst, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block}, false)
	if err != nil || len(xshardLst) != 1 {
		log.Error("Failed to add minor block", "err", err)
		return err
	}
	// only remove from pool if the block successfully added to state,
	// this may cache failed blocks but prevents them being broadcasted more than needed
	s.mBPool.delBlockInPool(block.Header())

	// block has been added to local state, broadcast tip so that peers can sync if needed
	if oldTip.Hash() != s.MinorBlockChain.CurrentHeader().Hash() {
		if err = s.broadcastNewTip(); err != nil {
			s.setHead(currHead)
			return err
		}
	}

	if xshardLst[0] == nil {
		log.Info("add minor block has been added...", "branch", s.fullShardId, "height", block.Number())
		return nil
	}

	prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash).Header().Number
	if err := s.conn.BroadcastXshardTxList(block, xshardLst[0], prevRootHeight); err != nil {
		s.setHead(currHead)
		return err
	}
	status, err := s.MinorBlockChain.GetShardStats()
	if err != nil {
		s.setHead(currHead)
		return err
	}

	requests := &rpc.AddMinorBlockHeaderRequest{
		MinorBlockHeader:  block.Header(),
		TxCount:           uint32(block.Transactions().Len()),
		XShardTxCount:     uint32(len(xshardLst[0])),
		ShardStats:        status,
		CoinbaseAmountMap: block.Header().CoinbaseAmount,
	}
	err = s.conn.SendMinorBlockHeaderToMaster(requests)
	if err != nil {
		s.setHead(currHead)
		return err
	}
	s.MinorBlockChain.CommitMinorBlockByHash(block.Header().Hash())
	s.mBPool.delBlockInPool(block.Header())
	go s.miner.HandleNewTip()
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
func (s *ShardBackend) AddBlockListForSync(blockLst []*types.MinorBlock) (map[common.Hash]*types.TokenBalances, error) {
	blockHashToXShardList := make(map[common.Hash]*XshardListTuple)

	coinbaseAmountList := make(map[common.Hash]*types.TokenBalances, 0)
	if len(blockLst) == 0 {
		return coinbaseAmountList, nil
	}

	uncommittedBlockHeaderList := make([]*types.MinorBlockHeader, 0)
	for _, block := range blockLst {
		blockHash := block.Header().Hash()
		if block.Header().Branch.GetFullShardID() != s.fullShardId {
			continue
		}
		if s.getBlockCommitStatusByHash(blockHash) == BLOCK_COMMITTED {
			continue
		}
		//TODO:support BLOCK_COMMITTING
		coinbaseAmountList[block.Header().Hash()] = block.Header().CoinbaseAmount
		_, xshardLst, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block}, false)
		if err != nil || len(xshardLst) != 1 {
			log.Error("Failed to add minor block", "err", err)
			return nil, err
		}
		s.mBPool.delBlockInPool(block.Header())
		prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash)
		blockHashToXShardList[blockHash] = &XshardListTuple{XshardTxList: xshardLst[0], PrevRootHeight: prevRootHeight.Number()}
		uncommittedBlockHeaderList = append(uncommittedBlockHeaderList, block.Header())
	}
	// interrupt the current miner and restart
	if err := s.conn.BatchBroadcastXshardTxList(blockHashToXShardList, blockLst[0].Header().Branch); err != nil {
		return nil, err
	}

	req := &rpc.AddMinorBlockHeaderListRequest{
		MinorBlockHeaderList: uncommittedBlockHeaderList,
	}
	if err := s.conn.SendMinorBlockHeaderListToMaster(req); err != nil {
		return nil, err
	}
	for _, header := range uncommittedBlockHeaderList {
		s.MinorBlockChain.CommitMinorBlockByHash(header.Hash())
		s.mBPool.delBlockInPool(header)
	}
	return coinbaseAmountList, nil
}

func (s *ShardBackend) GetTransactionListByAddress(address *account.Address, transferTokenID *uint64,
	start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	return s.MinorBlockChain.GetTransactionByAddress(*address, transferTokenID, start, limit)
}

func (s *ShardBackend) GetAllTx(start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	return s.MinorBlockChain.GetAllTx(start, limit)
}

func (s *ShardBackend) GetLogs(start uint64, end uint64, address []account.Address, topics [][]common.Hash) ([]*types.Log, error) {
	return s.MinorBlockChain.GetLogsByAddressAndTopic(start, end, address, topics)
}

func (s *ShardBackend) GetWork() (*consensus.MiningWork, error) {
	return s.miner.GetWork()
}

func (s *ShardBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) error {
	if ok := s.miner.SubmitWork(nonce, headerHash, mixHash, nil); ok {
		return nil
	}
	return errors.New("submit mined work failed")
}

func (s *ShardBackend) HandleNewTip(rBHeader *types.RootBlockHeader, mBHeader *types.MinorBlockHeader, peerID string) error {
	if s.MinorBlockChain.CurrentHeader().NumberU64() >= mBHeader.Number {
		return nil
	}

	if s.MinorBlockChain.GetRootBlockByHash(mBHeader.PrevRootBlockHash) == nil {
		log.Warn(s.logInfo, "preRootBlockHash do not have height ,no need to add task", mBHeader.Number, "preRootHash", mBHeader.PrevRootBlockHash.String())
		return nil
	}
	if s.MinorBlockChain.CurrentBlock().Number() >= mBHeader.Number {
		log.Info(s.logInfo, "no need t sync curr height", s.MinorBlockChain.CurrentBlock().Number(), "tipHeight", mBHeader.Number)
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

func (s *ShardBackend) GetMinorBlock(mHash common.Hash, height *uint64) (*types.MinorBlock, error) {
	if mHash != (common.Hash{}) {
		return s.MinorBlockChain.GetMinorBlock(mHash), nil
	} else if height != nil {
		return s.MinorBlockChain.GetBlockByNumber(*height).(*types.MinorBlock), nil
	}
	return nil, errors.New("invalied params in GetMinorBlock")
}

func (s *ShardBackend) NewMinorBlock(block *types.MinorBlock) (err error) {
	log.Info(s.logInfo, "NewMinorBlock height", block.Header().Number, "hash", block.Header().Hash().String())
	defer log.Info(s.logInfo, "NewMinorBlock", "end")
	// TODO synchronizer.running
	mHash := block.Header().Hash()
	if s.mBPool.getBlockInPool(mHash) != nil {
		return
	}
	if s.MinorBlockChain.HasBlock(block.Hash()) {
		log.Info("add minor block, Known minor block", "branch", block.Header().Branch, "height", block.Number())
		return
	}

	if !s.MinorBlockChain.HasBlock(block.Header().ParentHash) && s.mBPool.getBlockInPool(block.ParentHash()) == nil {
		log.Info("prarent block hash not be included", "parent hash: ", block.Header().ParentHash.Hex())
		return
	}

	//Sanity check on timestamp and block height
	if block.Header().Time > uint64(time.Now().Unix())+uint64(AllowedFutureBlocksTimeBroadcast) {
		log.Warn(s.logInfo, "HandleNewMinorBlock err time is not right,height", block.Header().Number, "time", block.Header().Time,
			"now", time.Now().Unix(), "Max", AllowedFutureBlocksTimeBroadcast)
		return
	}

	if s.MinorBlockChain.CurrentBlock() != nil && s.MinorBlockChain.CurrentBlock().NumberU64() > block.NumberU64() &&
		s.MinorBlockChain.CurrentBlock().NumberU64()-block.NumberU64() >
			s.MinorBlockChain.Config().GetShardConfigByFullShardID(s.MinorBlockChain.GetBranch().Value).MaxStaleMinorBlockHeightDiff() {
		log.Info(s.logInfo, "HandleNewMinorBlock err:old blocks, height", block.NumberU64(),
			"currTip", s.MinorBlockChain.CurrentBlock().NumberU64())
	}

	if s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash) == nil {
		log.Warn(s.logInfo, "add minor block:preRootBlock have not exist", block.Header().PrevRootBlockHash.String())
		return nil
	}

	if err := s.MinorBlockChain.Validator().ValidateBlock(block, false); err != nil {
		return err
	}

	s.mBPool.setBlockInPool(block.Header())
	if err = s.conn.BroadcastMinorBlock(block, s.fullShardId); err != nil {
		return err
	}
	return s.AddMinorBlock(block)
}

func (s *ShardBackend) addTxList(txs []*types.Transaction) error {
	ts := time.Now()
	for index := range txs {
		if err := s.MinorBlockChain.AddTx(txs[index]); err != nil {
			return err //TODO ? need return err?
		}
		if index%1000 == 0 {
			log.Info("time-tx-insert-loop", "time", time.Now().Sub(ts).Seconds(), "index", index)
			ts = time.Now()
		}
	}
	go func() {
		if err := s.conn.BroadcastTransactions(txs, s.fullShardId); err != nil {
			log.Error(s.logInfo, "broadcastTransaction err", err)
		}
	}()
	log.Info("time-tx-insert-end", "time", time.Now().Sub(ts).Seconds(), "len(tx)", len(txs))
	return nil
}

func (s *ShardBackend) GenTx(genTxs *rpc.GenTxRequest) error {
	go func() {
		err := s.txGenerator.Generate(genTxs, s.addTxList)
		if err != nil {
			log.Error(s.logInfo, "GenTx err", err)
		}
	}()
	return nil
}

// miner api
func (s *ShardBackend) CreateBlockToMine() (types.IBlock, *big.Int, error) {
	minorBlock, err := s.MinorBlockChain.CreateBlockToMine(nil, &s.Config.CoinbaseAddress, nil, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	diff := minorBlock.Difficulty()
	if s.posw.IsPoSWEnabled() {
		header := minorBlock.Header()
		balances, err := s.MinorBlockChain.GetBalance(header.GetCoinbase().Recipient, nil)
		if err != nil {
			return nil, nil, err
		}
		balance := balances.GetTokenBalance(s.MinorBlockChain.GetGenesisToken())
		adjustedDifficulty, err := s.posw.PoSWDiffAdjust(header, balance)
		if err != nil {
			log.Error("[PoSW]Failed to compute PoSW difficulty.", err)
			return nil, nil, err
		}
		log.Info("[PoSW]CreateBlockToMine", "number", header.Number, "diff", header.Difficulty, "adjusted to", adjustedDifficulty)
		return minorBlock, adjustedDifficulty, nil
	}
	return minorBlock, diff, nil
}

func (s *ShardBackend) InsertMinedBlock(block types.IBlock) error {
	return s.NewMinorBlock(block.(*types.MinorBlock))
}
func (s *ShardBackend) GetTip() uint64 {
	return s.MinorBlockChain.CurrentBlock().NumberU64()
}

func (s *ShardBackend) IsSyncIng() bool {
	return s.synchronizer.IsSyncing()
}

func (s *ShardBackend) CheckMinorBlock(header *types.MinorBlockHeader) error {
	block := s.MinorBlockChain.GetBlock(header.Hash())
	if qkccommon.IsNil(block) {
		return fmt.Errorf("block %v cannot be found", header.Hash())
	}
	if header.Number == 0 {
		return nil
	}
	_, _, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block}, true)
	return err
}
