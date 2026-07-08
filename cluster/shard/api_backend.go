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

	// ErrBodyDeleted is returned when the block body was deleted before the
	// commit marker could be written. The block stays UNCOMMITTED and a later
	// sync round re-fetches and commits it. Informational only: s.mu serializes
	// the commit paths against body deletion, so this is not expected in practice.
	ErrBodyDeleted = errors.New("block body deleted before commit")
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

func (s *ShardBackend) GetTransactionByHash(hash common.Hash) (*types.MinorBlock, uint32) {
	return s.MinorBlockChain.GetTransactionByHash(hash)
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *ShardBackend) GetTransactionCount(address common.Address, blockNrOrHash qrpc.BlockNumberOrHash) (*uint64, error) {
	// Ask transaction pool for the nonce which includes pending transactions
	hash := blockNrOrHash.BlockHash
	if blockNr, ok := blockNrOrHash.Number(); ok {
		if blockNr == qrpc.PendingBlockNumber {
			nonce, err := s.MinorBlockChain.GetPoolNonce(address)
			if err != nil {
				return nil, err
			}
			return &nonce, nil
		}

		header := s.MinorBlockChain.GetHeaderByNumber(blockNr.Uint64())
		if header == nil {
			return nil, errors.New("block not found")
		}
		h := header.Hash()
		hash = &h
	}

	nonce, err := s.MinorBlockChain.GetTransactionCount(address, hash)
	return &nonce, err
}

func (s *ShardBackend) ExecuteTx(tx *types.Transaction, fromAddress *account.Address, height *uint64) ([]byte, error) {
	return s.MinorBlockChain.ExecuteTx(tx, fromAddress, height)
}

func (s *ShardBackend) EstimateGas(tx *types.Transaction, fromAddress *account.Address) (uint32, error) {
	return s.MinorBlockChain.EstimateGas(tx, fromAddress)
}

func (s *ShardBackend) GasPrice(tokenID uint64) (uint64, error) {
	return s.MinorBlockChain.GasPrice(tokenID)
}

func (s *ShardBackend) GetCode(recipient account.Recipient, height *uint64) ([]byte, error) {
	var hash *common.Hash = nil
	if height != nil {
		header := s.MinorBlockChain.GetHeaderByNumber(*height)
		if header == nil {
			return nil, errors.New(fmt.Sprintf("Cannot get %d", *height))
		}
		h := header.Hash()
		hash = &h
	}

	return s.MinorBlockChain.GetCode(recipient, hash)
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
	// Serialize root-block handling against AddMinorBlock / AddBlockListForSync on
	// the same shard, so their broadcast + master-report side effects cannot
	// interleave. The core-layer data race (currentBlock/canonical-index rewrite
	// vs. the insertChain pipeline) is handled separately inside
	// MinorBlockChain.AddRootBlock, which now holds m.chainmu across the reorg.
	s.mu.Lock()
	defer s.mu.Unlock()
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

func (s *ShardBackend) AddTransaction(tx *types.Transaction) error {
	return s.MinorBlockChain.AddTx(tx)
}

func (s *ShardBackend) AddTransactionAndBroadcast(tx *types.Transaction) error {
	err := s.MinorBlockChain.AddTx(tx)
	go func() {
		if err := s.conn.BroadcastTransactions("", s.branch.Value, []*types.Transaction{tx}); err != nil {
			log.Error(s.logInfo, "broadcastTransaction err", err)
		}
	}()
	return err
}

func (s *ShardBackend) GetEthChainID() uint32 {
	return s.MinorBlockChain.EthChainID()
}

func (s *ShardBackend) GetNetworkId() uint32 {
	return s.MinorBlockChain.Config().NetworkID
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
		mBlock = s.MinorBlockChain.CurrentBlock()
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
	if s.MinorBlockChain.HasCommittedBlock(block.Hash()) {
		log.Debug("add minor block, Known minor block", "branch", block.Branch(), "height", block.Number())
		return
	}

	// Allow children of committed parents, plus body-only sidechain parents
	// whose state is pruned so body accumulation can continue.
	if !s.hasCommittedOrBodyWithoutState(block.ParentHash()) {
		if err := s.commitUncommittedAncestorsIfPresent(block.ParentHash()); err != nil {
			return err
		}
		if !s.hasCommittedOrBodyWithoutState(block.ParentHash()) {
			log.Debug("prarent block hash not be included", "parent hash: ", block.ParentHash().Hex())
			return
		}
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

	// force=true: any block reaching here is uncommitted (committed blocks
	// returned at the HasCommittedBlock check above). A body may already be
	// present but uncommitted - a prior AddMinorBlock inserted it, then failed
	// before writing the commit marker. force=false would reject that as
	// ErrKnownBlock and return before AddMinorBlock's force=true recovery runs,
	// stranding the body permanently. For a fresh block force has no effect.
	if err := s.MinorBlockChain.Validator().ValidateBlock(block, true); err != nil {
		// Only recover pruned-parent imports when the parent is committed or is
		// body-only with missing state. Do not advance past a body+state parent
		// that still lacks its commit marker.
		if err == core.ErrPrunedAncestor && s.hasCommittedOrBodyWithoutState(block.ParentHash()) {
			return s.AddMinorBlock(block)
		}
		log.Warn(s.logInfo+" ValidateBlock", "err", err)
		return nil // next time to handle
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

	blockHash := block.Hash()

	// Fast path (lock-free): skip if already durably committed.
	if s.getBlockCommitStatusByHash(blockHash) == BLOCK_COMMITTED {
		return nil
	}

	// Acquire lock and perform the actual addition. Callers block here until
	// any in-flight processing of this block finishes, then re-check.
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wg.Add(1)
	defer s.wg.Done()

	// Double-check after acquiring lock.
	if s.getBlockCommitStatusByHash(blockHash) == BLOCK_COMMITTED {
		return nil
	}

	currHead := s.MinorBlockChain.CurrentBlock().Header()
	log.Debug(s.logInfo, "currHead", currHead.Number)
	_, xshardBlocks, err := s.MinorBlockChain.InsertChainForDepositsWithBlocks([]types.IBlock{block}, true)
	if err != nil {
		log.Error(s.logInfo+" Failed to add minor block", "err", err, "len", len(xshardBlocks))
		return err
	}

	// only remove from pool if the block successfully added to state,
	// this may cache failed blocks but prevents them being broadcasted more than needed
	s.mBPool.delBlockInPool(blockHash)

	if handled, err := s.commitSidechainReplayIfNeeded(currHead, blockHash, xshardBlocks); handled {
		return err
	}

	xshardLst := xshardBlocks[0].XShardList
	if xshardLst == nil {
		log.Info(s.logInfo+" add minor block has been added...", "branch", s.branch.Value, "height", block.Number())
		return nil
	}

	prevRootBlock := s.MinorBlockChain.GetRootBlockByHash(block.PrevRootBlockHash())
	if prevRootBlock == nil {
		log.Error(s.logInfo+" prev root block not found", "hash", block.PrevRootBlockHash().Hex())
		return fmt.Errorf("prev root block not found: %s", block.PrevRootBlockHash().Hex())
	}
	prevRootHeight := prevRootBlock.Number()
	if err := s.conn.BroadcastXshardTxList(block, xshardLst, prevRootHeight); err != nil {
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
		XShardTxCount:     uint32(len(xshardLst)),
		ShardStats:        status,
		CoinbaseAmountMap: block.CoinbaseAmount(),
	}
	err = s.conn.SendMinorBlockHeaderToMaster(requests)
	if err != nil {
		s.setHead(currHead.Number)
		return err
	}

	// Commit to DB; false means the body was deleted. See ErrBodyDeleted.
	if !s.MinorBlockChain.CommitMinorBlockByHash(block.Hash()) {
		log.Warn(s.logInfo+" block body removed, cancelling commit", "hash", block.Hash().Hex())
		return fmt.Errorf("%w: %s", ErrBodyDeleted, block.Hash().Hex())
	}

	return s.notifyMinorTipChanged(currHead)
}

func (s *ShardBackend) hasUncommittedBodyWithoutState(hash common.Hash) bool {
	return !s.MinorBlockChain.HasCommittedBlock(hash) && s.MinorBlockChain.HasBodyWithoutState(hash)
}

func (s *ShardBackend) hasCommittedOrBodyWithoutState(hash common.Hash) bool {
	return s.MinorBlockChain.HasCommittedBlock(hash) || s.hasUncommittedBodyWithoutState(hash)
}

func (s *ShardBackend) commitUncommittedAncestorsIfPresent(hash common.Hash) error {
	blocks := make([]*types.MinorBlock, 0)
	for !s.MinorBlockChain.HasCommittedBlock(hash) && s.MinorBlockChain.HasBlockAndState(hash) {
		block := s.MinorBlockChain.GetMinorBlock(hash)
		if block == nil {
			return nil
		}
		blocks = append(blocks, block)
		hash = block.ParentHash()
	}
	if len(blocks) == 0 || !s.MinorBlockChain.HasCommittedBlock(hash) {
		return nil
	}
	for i := len(blocks) - 1; i >= 0; i-- {
		if err := s.AddMinorBlock(blocks[i]); err != nil {
			return err
		}
		if !s.MinorBlockChain.HasCommittedBlock(blocks[i].Hash()) {
			return fmt.Errorf("failed to commit ancestor block: %s", blocks[i].Hash().Hex())
		}
	}
	return nil
}

func (s *ShardBackend) commitSidechainReplayIfNeeded(currHead *types.MinorBlockHeader, requestedHash common.Hash, xShardBlocks []core.XShardListWithBlock) (bool, error) {
	if len(xShardBlocks) == 0 {
		return true, nil
	}
	if len(xShardBlocks) == 1 && xShardBlocks[0].Block.Hash() == requestedHash {
		return false, nil
	}

	// A single incoming block can trigger pruned sidechain reconstruction.
	// In that case core returns x-shard lists for the replayed blocks, so each
	// list must be committed with its own block/header instead of using the
	// single-block path below.
	if err := s.broadcastAndCommitXShardBlocks(xShardBlocks); err != nil {
		s.setHead(currHead.Number)
		return true, err
	}
	return true, s.notifyMinorTipChanged(currHead)
}

func (s *ShardBackend) notifyMinorTipChanged(currHead *types.MinorBlockHeader) error {
	if s.MinorBlockChain.CurrentBlock().Hash() == currHead.Hash() {
		return nil
	}
	go s.miner.HandleNewTip()
	return s.broadcastNewTip()
}

func (s *ShardBackend) broadcastAndCommitXShardBlocks(xShardBlocks []core.XShardListWithBlock) error {
	blockHashToXShardList := make(map[common.Hash]*XshardListTuple, len(xShardBlocks))
	uncommittedBlockHeaderList := make([]*types.MinorBlockHeader, 0, len(xShardBlocks))

	for _, xShardBlock := range xShardBlocks {
		block := xShardBlock.Block
		blockHash := block.Hash()
		if s.getBlockCommitStatusByHash(blockHash) == BLOCK_COMMITTED {
			continue
		}
		prevRootBlock := s.MinorBlockChain.GetRootBlockByHash(block.PrevRootBlockHash())
		if prevRootBlock == nil {
			log.Error(s.logInfo+" prev root block not found", "hash", block.PrevRootBlockHash().Hex())
			return fmt.Errorf("prev root block not found: %s", block.PrevRootBlockHash().Hex())
		}
		blockHashToXShardList[blockHash] = &XshardListTuple{
			XshardTxList:   xShardBlock.XShardList,
			PrevRootHeight: prevRootBlock.Number(),
		}
		uncommittedBlockHeaderList = append(uncommittedBlockHeaderList, block.Header())
	}
	if len(uncommittedBlockHeaderList) == 0 {
		return nil
	}

	if err := s.conn.BatchBroadcastXshardTxList(blockHashToXShardList, s.branch); err != nil {
		return err
	}
	req := &rpc.AddMinorBlockHeaderListRequest{
		MinorBlockHeaderList: uncommittedBlockHeaderList,
	}
	if err := s.conn.SendMinorBlockHeaderListToMaster(req); err != nil {
		return err
	}
	for _, header := range uncommittedBlockHeaderList {
		blockHash := header.Hash()
		if !s.MinorBlockChain.CommitMinorBlockByHash(blockHash) {
			log.Warn(s.logInfo+" block body removed, cancelling commit", "hash", blockHash.Hex())
			return fmt.Errorf("%w: %s", ErrBodyDeleted, blockHash.Hex())
		}
		s.mBPool.delBlockInPool(blockHash)
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

	seenInBatch := make(map[common.Hash]struct{})
	chain := make([]types.IBlock, 0, len(blockLst))

	for _, block := range blockLst {
		blockHash := block.Hash()
		if block.Branch().Value != s.branch.Value {
			continue
		}

		if s.getBlockCommitStatusByHash(blockHash) == BLOCK_COMMITTED {
			log.Debug(s.logInfo+" minor block to sync is already committed", "hash", blockHash.Hex())
			continue
		}
		if _, dup := seenInBatch[blockHash]; dup {
			log.Debug(s.logInfo+" duplicate block in sync batch, skipping", "hash", blockHash.Hex())
			continue
		}
		seenInBatch[blockHash] = struct{}{}

		if len(chain) > 0 {
			prev := chain[len(chain)-1]
			if block.NumberU64() != prev.NumberU64()+1 || block.ParentHash() != prev.Hash() {
				if err := s.insertAndCommitSyncSegment(chain); err != nil {
					return err
				}
				chain = chain[:0]
			}
		}
		chain = append(chain, block)
	}
	return s.insertAndCommitSyncSegment(chain)
}

func (s *ShardBackend) insertAndCommitSyncSegment(chain []types.IBlock) error {
	if len(chain) == 0 {
		return nil
	}
	_, xShardBlocks, err := s.MinorBlockChain.InsertChainForDepositsWithBlocks(chain, true)
	if err != nil {
		log.Error(s.logInfo+" Failed to add minor block segment", "err", err)
		return err
	}
	for _, block := range chain {
		s.mBPool.delBlockInPool(block.Hash())
	}
	return s.broadcastAndCommitXShardBlocks(xShardBlocks)
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
		stakePreBlock := s.MinorBlockChain.DecayByHeightAndTime(minorBlock.NumberU64(), minorBlock.Time())
		adjustedDifficulty, err := s.posw.PoSWDiffAdjust(header, balance, stakePreBlock)
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
