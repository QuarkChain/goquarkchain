package shard

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/params"
	"golang.org/x/sync/errgroup"
	"math/big"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
)

var (
	EmptyErrTemplate                 = "empty result when call %s, params: %v\n"
	AllowedFutureBlocksTimeBroadcast = 15
)

// Wrapper over master connection, used by synchronizer.
type peer struct {
	cm     ConnManager
	peerID string
}

// ######################## peer Methods ###############################
func (p *peer) GetMinorBlockHeaderList(gReq *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	return p.cm.GetMinorBlockHeaderList(gReq)
}

func (p *peer) GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	return p.cm.GetMinorBlocks(hashes, p.peerID, branch)
}

func (p *peer) PeerID() string {
	return p.peerID
}

// ######################## txs and logs Methods #######################
func (s *ShardBackend) GetTransactionListByAddress(address *account.Address, transferTokenID *uint64,
	start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	return s.MinorBlockChain.GetTransactionByAddress(*address, transferTokenID, start, limit)
}

func (s *ShardBackend) GetAllTx(start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	return s.MinorBlockChain.GetAllTx(start, limit)
}

func (s *ShardBackend) GenTx(genTxs *rpc.GenTxRequest) error {
	var g errgroup.Group
	for index := 0; index < len(s.txGenerator); index++ {
		i := index
		g.Go(func() error {
			err := s.txGenerator[i].Generate(genTxs, s.AddTxList)
			if err != nil {
				log.Error(s.logInfo, "GenTx err", err)
			}
			return err
		})
	}
	if err := g.Wait(); err != nil {
		panic(err)
	}
	return nil
}

func (s *ShardBackend) GetLogs(hash common.Hash) ([][]*types.Log, error) {
	return s.MinorBlockChain.GetLogs(hash), nil
}

func (s *ShardBackend) GetLogsByFilterQuery(args *qrpc.FilterQuery) ([]*types.Log, error) {
	return s.MinorBlockChain.GetLogsByFilterQuery(args)
}

func (s *ShardBackend) GetReceiptsByHash(hash common.Hash) (types.Receipts, error) {
	receipts := s.MinorBlockChain.GetReceiptsByHash(hash)
	if qcom.IsNil(receipts) {
		return nil, fmt.Errorf(EmptyErrTemplate, "GetReceiptsByHash", hash.Hex())
	}
	return receipts, nil
}

// ######################## root block Methods #########################
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

// ######################## minor block Methods ########################
func (s *ShardBackend) GetHeaderByNumber(height qrpc.BlockNumber) (*types.MinorBlockHeader, error) {
	var iHeader types.IHeader
	if height == qrpc.LatestBlockNumber {
		iHeader = s.MinorBlockChain.CurrentHeader()
	} else {
		iHeader = s.MinorBlockChain.GetHeaderByNumber(height.Uint64())
	}
	if qcom.IsNil(iHeader) {
		return nil, fmt.Errorf(EmptyErrTemplate, "GetHeaderByNumber", height)
	}
	return iHeader.(*types.MinorBlockHeader), nil
}

func (s *ShardBackend) GetHeaderByHash(blockHash common.Hash) (*types.MinorBlockHeader, error) {
	iHeader := s.MinorBlockChain.GetHeaderByHash(blockHash)
	if qcom.IsNil(iHeader) {
		return nil, fmt.Errorf(EmptyErrTemplate, "GetHeaderByNumber", blockHash.Hex())
	}
	return iHeader.(*types.MinorBlockHeader), nil
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
	err := s.synchronizer.AddTask(qsync.NewMinorChainTask(peer, mBHeader))
	if err != nil {
		log.Error("Failed to add minor chain task,", "hash", mBHeader.Hash(), "height", mBHeader.Number)
	}

	log.Info("Handle new tip received new tip with height", "shard height", mBHeader.Number)
	return nil
}

func (s *ShardBackend) GetUnconfirmedHeaderList() ([]*types.MinorBlockHeader, error) {
	headers := s.MinorBlockChain.GetUnconfirmedHeaderList()
	return headers, nil
}

func (s *ShardBackend) GetMinorBlock(mHash common.Hash, height *uint64) (*types.MinorBlock, error) {
	if mHash != (common.Hash{}) {
		return s.MinorBlockChain.GetMinorBlock(mHash), nil
	} else if height != nil {
		block := s.MinorBlockChain.GetBlockByNumber(*height)
		if block == nil {
			return nil, errors.New("minor block not found")
		}
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
		return errors.New("time is not right")
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
	if err = s.conn.BroadcastMinorBlock(block, s.branch.Value); err != nil {
		return err
	}
	return s.AddMinorBlock(block)
}

// Returns true if block is successfully added. False on any error.
// called by 1. local miner (will not run if syncing) 2. SyncTask
func (s *ShardBackend) AddMinorBlock(block *types.MinorBlock) error {
	var (
		oldTip = s.MinorBlockChain.CurrentBlock().Header()
	)

	if commitStatus := s.getBlockCommitStatusByHash(block.Header().Hash()); commitStatus == BLOCK_COMMITTED {
		return nil
	}
	//TODO support BLOCK_COMMITTING
	currHead := s.MinorBlockChain.CurrentBlock().Header()
	_, xshardLst, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block}, false)
	if err != nil || len(xshardLst) != 1 {
		log.Error("Failed to add minor block", "err", err)
		return err
	}
	// only remove from pool if the block successfully added to state,
	// this may cache failed blocks but prevents them being broadcasted more than needed
	s.mBPool.delBlockInPool(block.Header())

	// block has been added to local state, broadcast tip so that peers can sync if needed
	if oldTip.Hash() != s.MinorBlockChain.CurrentBlock().Hash() {
		if err = s.broadcastNewTip(); err != nil {
			s.setHead(currHead.Number)
			return err
		}
	}

	if xshardLst[0] == nil {
		log.Info("add minor block has been added...", "branch", s.branch.Value, "height", block.Number())
		return nil
	}

	prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.Header().PrevRootBlockHash).Header().Number
	if err := s.conn.BroadcastXshardTxList(block, xshardLst[0], prevRootHeight); err != nil {
		s.setHead(currHead.Number)
		return err
	}
	status, err := s.MinorBlockChain.GetShardStats()
	if err != nil {
		s.setHead(currHead.Number)
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
		s.setHead(currHead.Number)
		return err
	}
	s.MinorBlockChain.CommitMinorBlockByHash(block.Header().Hash())
	s.mBPool.delBlockInPool(block.Header())
	if s.MinorBlockChain.CurrentBlock().Hash() != currHead.Hash() {
		go s.miner.HandleNewTip()
	}

	return nil
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
		if block.Header().Branch.GetFullShardID() != s.branch.Value {
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

// ######################## miner Methods ##############################
func (s *ShardBackend) GetWork(coinbaseAddr *account.Address) (*consensus.MiningWork, error) {
	return s.miner.GetWork(coinbaseAddr)
}

func (s *ShardBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) error {
	if ok := s.miner.SubmitWork(nonce, headerHash, mixHash, nil); ok {
		return nil
	}
	return errors.New("submit mined work failed")
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

// ######################## subscribe Methods ##########################
func (s *ShardBackend) SubscribeChainHeadEvent(ch chan<- core.MinorChainHeadEvent) event.Subscription {
	return s.MinorBlockChain.SubscribeChainHeadEvent(ch)
}

func (s *ShardBackend) SubscribeLogsEvent(ch chan<- core.LoglistEvent) event.Subscription {
	return s.MinorBlockChain.SubReorgLogsEvent(ch)
}

func (s *ShardBackend) SubscribeChainEvent(ch chan<- core.MinorChainEvent) event.Subscription {
	return s.MinorBlockChain.SubscribeChainEvent(ch)
}

func (s *ShardBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return s.MinorBlockChain.SubscribeNewTxsEvent(ch)
}

func (s *ShardBackend) SubscribeSyncEvent(ch chan<- *qsync.SyncingResult) event.Subscription {
	return s.synchronizer.SubscribeSyncEvent(ch)
}

func (s *ShardBackend) broadcastNewTip() (err error) {
	var (
		rootTip  = s.MinorBlockChain.GetRootTip()
		minorTip = s.MinorBlockChain.CurrentBlock().Header()
	)

	err = s.conn.BroadcastNewTip([]*types.MinorBlockHeader{minorTip}, rootTip, s.branch.Value)
	return
}

func (s *ShardBackend) setHead(head uint64) {
	if err := s.MinorBlockChain.SetHead(head); err != nil {
		panic(err)
	}
}

func (s *ShardBackend) AddTxList(txs []*types.Transaction, peerID string) error {
	ts := time.Now()
	if err := s.MinorBlockChain.AddTxList(txs); err != nil {
		return err
	}
	go func() {
		span := len(txs) / params.NEW_TRANSACTION_LIST_LIMIT
		for index := 0; index < span; index++ {
			if err := s.conn.BroadcastTransactions(txs[index*params.NEW_TRANSACTION_LIST_LIMIT:(index+1)*params.NEW_TRANSACTION_LIST_LIMIT], s.branch.Value, peerID); err != nil {
				log.Error(s.logInfo, "broadcastTransaction err", err)
			}
		}
		if len(txs)%params.NEW_TRANSACTION_LIST_LIMIT != 0 {
			if err := s.conn.BroadcastTransactions(txs[span*params.NEW_TRANSACTION_LIST_LIMIT:], s.branch.Value, peerID); err != nil {
				log.Error(s.logInfo, "broadcastTransaction err", err)
			}
		}

	}()
	log.Info("time-tx-insert-end", "time", time.Now().Sub(ts).Seconds(), "len(tx)", len(txs))
	return nil
}

func (s *ShardBackend) GetDefaultCoinbaseAddress() account.Address {
	addr := s.Config.CoinbaseAddress
	if !s.branch.IsInBranch(addr.FullShardKey) {
		addr = addr.AddressInBranch(s.branch)
	}
	return addr
}

// miner api
func (s *ShardBackend) CreateBlockToMine(addr *account.Address) (types.IBlock, *big.Int, uint64, error) {
	coinbaseAddress := s.Config.CoinbaseAddress
	if addr != nil {
		coinbaseAddress = *addr
	}
	minorBlock, err := s.MinorBlockChain.CreateBlockToMine(nil, &coinbaseAddress, nil, nil, nil)
	if err != nil {
		return nil, nil, 0, err
	}
	diff := minorBlock.Difficulty()
	header := minorBlock.Header()
	if s.posw.IsPoSWEnabled(header) {
		balances, err := s.MinorBlockChain.GetBalance(header.GetCoinbase().Recipient, nil)
		if err != nil {
			return nil, nil, 0, err
		}
		balance := balances.GetTokenBalance(s.MinorBlockChain.GetGenesisToken())
		adjustedDifficulty, err := s.posw.PoSWDiffAdjust(header, balance)
		if err != nil {
			log.Error("PoSW", "failed to compute PoSW difficulty", err)
			return nil, nil, 0, err
		}
		return minorBlock, adjustedDifficulty, 1, nil
	}
	return minorBlock, diff, 1, nil
}

func (s *ShardBackend) CheckMinorBlock(header *types.MinorBlockHeader) error {
	block := s.MinorBlockChain.GetBlock(header.Hash())
	if qcom.IsNil(block) {
		return fmt.Errorf("block %v cannot be found", header.Hash())
	}
	if header.Number == 0 {
		return nil
	}
	_, _, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block}, true)
	return err
}

func (s *ShardBackend) GetRootChainStakes(address account.Address, lastMinor common.Hash) (*big.Int,
	*account.Recipient, error) {

	if s.Config.ChainID != 0 || s.Config.ShardID != 0 {
		return nil, nil, errors.New("not chain 0 shard 0")
	}
	return s.MinorBlockChain.GetRootChainStakes(address.Recipient, lastMinor)
}
