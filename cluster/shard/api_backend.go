package shard

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/params"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
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

func (s *ShardBackend) GenTx(genTxs rpc.GenTxRequest) error {
	log.Info(s.logInfo, "ready to genTx txNumber", genTxs.NumTxPerShard, "XShardPercent", genTxs.XShardPercent)
	allTxNumber := genTxs.NumTxPerShard
	for allTxNumber > 0 {
		pendingCnt := s.MinorBlockChain.GetPendingCount()
		log.Info(s.logInfo, "last tx to add", allTxNumber, "pendingCnt", pendingCnt)
		if pendingCnt >= 36000 {
			time.Sleep(2 * time.Second)
			continue
		}
		genTxs.NumTxPerShard = allTxNumber
		if allTxNumber > 12000 {
			genTxs.NumTxPerShard = 12000
		}

		if err := s.genTx(genTxs); err != nil {
			log.Error(s.logInfo, "genTx err", err)
			return err
		}
		allTxNumber -= genTxs.NumTxPerShard
	}
	return nil
}

func (s *ShardBackend) AccountForTPSReady() bool {
	for _, v := range s.txGenerator {
		if len(v.accounts) == 0 {
			return false
		}
	}
	if len(s.txGenerator) == 0 {
		return false
	}
	return true
}

func (s *ShardBackend) genTx(genTxs rpc.GenTxRequest) error {
	genTxs.NumTxPerShard = genTxs.NumTxPerShard / uint32(len(s.txGenerator))
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
	return g.Wait()
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
	s.wg.Add(1)
	defer s.wg.Done()
	if rBlock.Number() > s.genesisRootHeight {
		return s.MinorBlockChain.InitFromRootBlock(rBlock)
	}
	if rBlock.Number() == s.genesisRootHeight {
		return s.initGenesisState(rBlock)
	}
	return nil
}

func (s *ShardBackend) AddRootBlock(rBlock *types.RootBlock) (switched bool, err error) {
	s.wg.Add(1)
	defer s.wg.Done()
	switched = false
	if rBlock.Number() > s.genesisRootHeight {
		switched, err = s.MinorBlockChain.AddRootBlock(rBlock)
	}
	if rBlock.Number() == s.genesisRootHeight {
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

func (s *ShardBackend) HandleNewTip(rBHeader *types.RootBlockHeader, mBHeader *types.MinorBlockHeader, peerID string) error {
	s.wg.Add(1)
	defer s.wg.Done()
	if s.MinorBlockChain.GetRootBlockByHash(mBHeader.PrevRootBlockHash) == nil {
		log.Debug(s.logInfo, "preRootBlockHash do not have height ,no need to add task", mBHeader.Number, "preRootHash", mBHeader.PrevRootBlockHash.String())
		return nil
	}
	if s.MinorBlockChain.CurrentBlock().Number() >= mBHeader.Number {
		log.Debug(s.logInfo, "no need t sync curr height", s.MinorBlockChain.CurrentBlock().Number(), "tipHeight", mBHeader.Number)
		return nil
	}
	if s.mBPool.getBlockInPool(mBHeader.Hash()) {
		return nil
	}
	peer := &peer{cm: s.conn, peerID: peerID}
	err := s.synchronizer.AddTask(qsync.NewMinorChainTask(peer, mBHeader))
	if err != nil {
		log.Error("Failed to add minor chain task,", "hash", mBHeader.Hash(), "height", mBHeader.Number)
	}
	return nil
}

func (s *ShardBackend) GetUnconfirmedHeaderList() ([]*types.MinorBlockHeader, error) {
	headers := s.MinorBlockChain.GetUnconfirmedHeaderList()
	return headers, nil
}

func (s *ShardBackend) GetMinorBlock(mHash common.Hash, height *uint64) (mBlock *types.MinorBlock, err error) {
	if mHash != (common.Hash{}) {
		mBlock = s.MinorBlockChain.GetMinorBlock(mHash)
	} else if height != nil {
		block := s.MinorBlockChain.GetBlockByNumber(*height)
		if !qcom.IsNil(block) {
			mBlock = block.(*types.MinorBlock)
		}
	} else {
		return nil, errors.New("invalied params in GetMinorBlock")
	}
	if mBlock != nil {
		return
	}
	return nil, errors.New("minor block not found")
}

func (s *ShardBackend) NewMinorBlock(peerId string, block *types.MinorBlock) (err error) {
	s.wg.Add(1)
	defer s.wg.Done()
	log.Debug(s.logInfo+" NewMinorBlock", "height", block.NumberU64(), "hash", block.Hash().String())
	defer log.Debug(s.logInfo+" NewMinorBlock end", "height", block.NumberU64())
	// TODO synchronizer.running
	mHash := block.Hash()
	if s.mBPool.getBlockInPool(mHash) {
		return
	}
	if s.MinorBlockChain.HasBlock(block.Hash()) {
		log.Debug("add minor block, Known minor block", "branch", block.Branch(), "height", block.Number())
		return
	}

	if !s.MinorBlockChain.HasBlock(block.ParentHash()) {
		log.Debug("prarent block hash not be included", "parent hash: ", block.ParentHash().Hex())
		return
	}

	//Sanity check on timestamp and block height
	if block.Time() > uint64(time.Now().Unix())+uint64(AllowedFutureBlocksTimeBroadcast) {
		log.Warn(s.logInfo, "HandleNewMinorBlock err time is not right,height", block.NumberU64(), "time", block.Time(),
			"now", time.Now().Unix(), "Max", AllowedFutureBlocksTimeBroadcast)
		return errors.New("time is not right")
	}

	if s.MinorBlockChain.CurrentBlock() != nil && s.MinorBlockChain.CurrentBlock().NumberU64() > block.NumberU64() &&
		s.MinorBlockChain.CurrentBlock().NumberU64()-block.NumberU64() >
			s.MinorBlockChain.Config().GetShardConfigByFullShardID(s.MinorBlockChain.GetBranch().Value).MaxStaleMinorBlockHeightDiff() {
		log.Info(s.logInfo, "HandleNewMinorBlock err:old blocks, height", block.NumberU64(),
			"currTip", s.MinorBlockChain.CurrentBlock().NumberU64())
	}

	if s.MinorBlockChain.GetRootBlockByHash(block.PrevRootBlockHash()) == nil {
		log.Warn(s.logInfo, "add minor block:preRootBlock have not exist", block.PrevRootBlockHash().String())
		return nil
	}

	if err := s.MinorBlockChain.Validator().ValidateBlock(block, false); err != nil {
		log.Warn(s.logInfo+" ValidateBlock", "err", err)
		return err
	}

	s.mBPool.setBlockInPool(block.Hash())
	go func() {
		if err := s.conn.BroadcastMinorBlock(peerId, block); err != nil {
			log.Error("failed to broadcast new minor block", "err", err)
		}
	}()

	return s.AddMinorBlock(block)
}

// Returns true if block is successfully added. False on any error.
// called by 1. local miner (will not run if syncing) 2. SyncTask
func (s *ShardBackend) AddMinorBlock(block *types.MinorBlock) error {
	log.Debug(s.logInfo+" AddMinorBlock", "height", block.NumberU64(), "hash", block.Hash().String())
	defer log.Debug(s.logInfo+" AddMinorBlock end", "height", block.NumberU64())

	s.mu.Lock()
	defer s.mu.Unlock()
	s.wg.Add(1)
	defer s.wg.Done()
	if commitStatus := s.getBlockCommitStatusByHash(block.Hash()); commitStatus == BLOCK_COMMITTED {
		return nil
	}
	//TODO support BLOCK_COMMITTING
	currHead := s.MinorBlockChain.CurrentBlock().Header()
	log.Debug(s.logInfo, "currHead", currHead.Number)
	_, xshardLst, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block}, false)
	if err != nil {
		log.Error(s.logInfo+" Failed to add minor block", "err", err, "len", len(xshardLst))
		return err
	}

	if len(xshardLst) != 1 {
		log.Warn(s.logInfo+" already have this block", "number", block.NumberU64(), "hash", block.Hash().String())
		return nil
	}

	// only remove from pool if the block successfully added to state,
	// this may cache failed blocks but prevents them being broadcasted more than needed
	s.mBPool.delBlockInPool(block.Hash())

	if xshardLst[0] == nil {
		log.Info(s.logInfo+" add minor block has been added...", "branch", s.branch.Value, "height", block.Number())
		return nil
	}

	prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.PrevRootBlockHash()).Number()
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
		CoinbaseAmountMap: block.CoinbaseAmount(),
	}
	err = s.conn.SendMinorBlockHeaderToMaster(requests)
	if err != nil {
		s.setHead(currHead.Number)
		return err
	}
	s.MinorBlockChain.CommitMinorBlockByHash(block.Hash())
	if s.MinorBlockChain.CurrentBlock().Hash() != currHead.Hash() {
		go s.miner.HandleNewTip()
	}
	// block has been added to local state, broadcast tip so that peers can sync if needed
	if currHead.Hash() != s.MinorBlockChain.CurrentBlock().Hash() {
		if err = s.broadcastNewTip(); err != nil {
			s.setHead(currHead.Number)
			return err
		}
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wg.Add(1)
	defer s.wg.Done()

	if len(blockLst) == 0 {
		return nil
	}

	var (
		blockHashToXShardList      = make(map[common.Hash]*XshardListTuple)
		uncommittedBlockHeaderList = make([]*types.MinorBlockHeader, 0, len(blockLst))
	)
	for _, block := range blockLst {
		blockHash := block.Hash()
		if block.Branch().Value != s.branch.Value {
			continue
		}
		if s.getBlockCommitStatusByHash(blockHash) == BLOCK_COMMITTED {
			continue
		}
		//TODO:support BLOCK_COMMITTING
		_, xshardLst, err := s.MinorBlockChain.InsertChainForDeposits([]types.IBlock{block}, false)
		if err != nil {
			log.Error(s.logInfo+" Failed to add minor block", "err", err)
			return err
		}
		if len(xshardLst) != 1 {
			log.Warn(s.logInfo+" already have this block", "number", block.NumberU64(), "hash", block.Hash().String())
			return nil
		}
		s.mBPool.delBlockInPool(block.Hash())
		prevRootHeight := s.MinorBlockChain.GetRootBlockByHash(block.PrevRootBlockHash())
		blockHashToXShardList[blockHash] = &XshardListTuple{XshardTxList: xshardLst[0], PrevRootHeight: prevRootHeight.Number()}
		uncommittedBlockHeaderList = append(uncommittedBlockHeaderList, block.Header())
	}
	// interrupt the current miner and restart
	if err := s.conn.BatchBroadcastXshardTxList(blockHashToXShardList, blockLst[0].Branch()); err != nil {
		return err
	}

	req := &rpc.AddMinorBlockHeaderListRequest{
		MinorBlockHeaderList: uncommittedBlockHeaderList,
	}
	if err := s.conn.SendMinorBlockHeaderListToMaster(req); err != nil {
		return err
	}
	for _, header := range uncommittedBlockHeaderList {
		s.MinorBlockChain.CommitMinorBlockByHash(header.Hash())
		s.mBPool.delBlockInPool(header.Hash())
	}
	return nil
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
	s.wg.Add(1)
	defer s.wg.Done()
	return s.NewMinorBlock("", block.(*types.MinorBlock))
}

func (s *ShardBackend) GetTip() uint64 {
	return s.MinorBlockChain.CurrentBlock().NumberU64()
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

	err = s.conn.BroadcastNewTip([]*types.MinorBlockHeader{minorTip}, rootTip.Header(), s.branch.Value)
	return
}

func (s *ShardBackend) setHead(head uint64) {
	if err := s.MinorBlockChain.SetHead(head); err != nil {
		panic(err)
	}
}

func (s *ShardBackend) AddTxList(txs []*types.Transaction) error {
	errList := s.MinorBlockChain.AddTxList(txs)
	for _, err := range errList {
		if err != nil {
			return err
		}
	}

	go func() {
		span := len(txs) / params.NEW_TRANSACTION_LIST_LIMIT
		for index := 0; index < span; index++ {
			if err := s.conn.BroadcastTransactions("", s.branch.Value, txs[index*params.NEW_TRANSACTION_LIST_LIMIT:(index+1)*params.NEW_TRANSACTION_LIST_LIMIT]); err != nil {
				log.Error(s.logInfo, "broadcastTransaction err", err)
			}
		}
		if len(txs)%params.NEW_TRANSACTION_LIST_LIMIT != 0 {
			if err := s.conn.BroadcastTransactions("", s.branch.Value, txs[span*params.NEW_TRANSACTION_LIST_LIMIT:]); err != nil {
				log.Error(s.logInfo, "broadcastTransaction err", err)
			}
		}
	}()
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
	if s.posw.IsPoSWEnabled(header.Time, header.NumberU64()) {
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
