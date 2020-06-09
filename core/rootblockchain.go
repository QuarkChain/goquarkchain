// Modified from go-ethereum under GNU Lesser General Public License

package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/big"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qkccom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/posw"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/encoder"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
)

var (
	ErrNoGenesis = errors.New("Genesis not found in chain")
)

const (
	blockCacheLimit           = 1024 // TODO really need 1024?
	receiptsCacheLimit        = 32
	maxFutureBlocks           = 32
	maxTimeFutureBlocks       = 30
	triesInMemory             = 256
	triesInRootBlock          = uint64(32)
	validatedMinorBlockHashes = 128
)

// CacheConfig contains the configuration values for the trie caching/pruning
// that's resident in a blockchain.
type CacheConfig struct {
	Disabled       bool          // Whether to disable trie write caching (archive node)
	TrieCleanLimit int           // Memory allowance (MB) to use for caching trie nodes in memory
	TrieDirtyLimit int           // Memory limit (MB) at which to start flushing dirty trie nodes to disk
	TrieTimeLimit  time.Duration // Time limit after which to flush the current in-memory trie to disk
}

// RootBlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
//
// Importing blocks in to the block chain happens according to the set of rules
// defined by the two stage Validator. Processing of blocks is done using the
// Processor which processes the included transaction. The validation of the state
// is done in the second part of the Validator. Failing results in aborting of
// the import.
//
// The RootBlockChain also helps in returning blocks from **any** chain included
// in the database as well as blocks that represents the canonical chain. It's
// important to note that GetBlock can return any block and does not need to be
// included in the canonical one where as GetBlockByNumber always represents the
// canonical chain.
type RootBlockChain struct {
	chainConfig *config.QuarkChainConfig // Chain & network configuration

	db     ethdb.Database // Low level persistent database to store final content in
	triegc *prque.Prque   // Priority queue mapping block numbers to tries to gc
	gcproc time.Duration  // Accumulates canonical block processing for trie dumping

	//	headerChain              *RootHeaderChain
	validatedMinorBlockCache *lru.Cache // Cache for the most recent validated Minor Block hash
	rmLogsFeed               event.Feed
	chainFeed                event.Feed
	chainSideFeed            event.Feed
	chainHeadFeed            event.Feed
	logsFeed                 event.Feed
	scope                    event.SubscriptionScope
	genesisBlock             *types.RootBlock

	mu      sync.RWMutex // global mutex for locking chain operations
	chainmu sync.RWMutex // blockchain insertion lock
	procmu  sync.RWMutex // block processor lock

	checkpoint   int          // checkpoint counts towards the new checkpoint
	currentBlock atomic.Value // Current head of the block chain

	blockCache          *lru.Cache // Cache for the most recent entire blocks
	futureBlocks        *lru.Cache // future blocks are blocks added for later processing
	coinbaseAmountCache map[uint64]*big.Int

	quit    chan struct{} // blockchain quit channel
	running int32         // running must be called atomically
	// procInterrupt must be atomically called
	procInterrupt int32          // interrupt signaler for block processing
	wg            sync.WaitGroup // chain processing wait group for shutting down

	engine    consensus.Engine
	validator Validator // block and state validator interface

	countMinorBlocks    bool
	addBlockAndBroad    func(block *types.RootBlock) error
	isCheckDB           bool
	posw                consensus.PoSWCalculator
	rootChainStakesFunc func(address account.Address, lastMinor common.Hash) (*big.Int, *account.Recipient, error)
}

// NewBlockChain returns a fully initialized block chain using information
// available in the database. It initializes the default Ethereum Validator and
// Processor.
func NewRootBlockChain(db ethdb.Database, chainConfig *config.QuarkChainConfig, engine consensus.Engine) (*RootBlockChain, error) {
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)
	validatedMinorBlockHashCache, _ := lru.New(validatedMinorBlockHashes)

	bc := &RootBlockChain{
		chainConfig:              chainConfig,
		db:                       db,
		triegc:                   prque.New(nil),
		quit:                     make(chan struct{}),
		blockCache:               blockCache,
		coinbaseAmountCache:      make(map[uint64]*big.Int),
		futureBlocks:             futureBlocks,
		engine:                   engine,
		validatedMinorBlockCache: validatedMinorBlockHashCache,
		isCheckDB:                false,
	}
	bc.SetValidator(NewRootBlockValidator(chainConfig, bc, engine))
	bc.posw = posw.NewPoSW(bc, chainConfig.Root.PoSWConfig)
	var err error
	if err != nil {
		return nil, err
	}
	genesisBlock := bc.GetBlockByNumber(0)
	if genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	bc.genesisBlock = genesisBlock.(*types.RootBlock)

	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
	// Take ownership of this particular state
	go bc.update()
	return bc, nil
}

func (bc *RootBlockChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&bc.procInterrupt) == 1
}

// will set it when check db
func (bc *RootBlockChain) SetIsCheckDB(isCheckDB bool) {
	bc.isCheckDB = isCheckDB
}

func (bc *RootBlockChain) IsCheckDB() bool {
	return bc.isCheckDB
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (bc *RootBlockChain) loadLastState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadBlockHash(bc.db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		log.Warn("Empty database, resetting chain")
		return bc.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := bc.GetBlock(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		log.Warn("Head block missing, resetting chain", "hash", head)
		return bc.Reset()
	}
	// Everything seems to be fine, set as the head block
	bc.currentBlock.Store(currentBlock)
	return nil
}

// SetHead rewinds the local chain to a new head. In the case of Headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
func (bc *RootBlockChain) SetHead(head uint64) error {
	log.Warn("Rewinding blockchain", "target", head)

	bc.mu.Lock()
	defer bc.mu.Unlock()

	batch := bc.db.NewBatch()
	for block := bc.CurrentBlock(); !qkccom.IsNil(block) && block.NumberU64() > head; block = bc.CurrentBlock() {
		rawdb.DeleteRootBlock(batch, block.Hash())
		rawdb.DeleteCanonicalHash(batch, rawdb.ChainTypeRoot, block.NumberU64())
		bc.currentBlock.Store(bc.GetBlock(block.ParentHash()))
	}
	batch.Write()
	// Clear out any stale content from the caches
	bc.blockCache.Purge()
	bc.futureBlocks.Purge()

	// If either blocks reached nil, reset to the genesis state
	if currentBlock := bc.CurrentBlock(); qkccom.IsNil(currentBlock) {
		bc.currentBlock.Store(bc.genesisBlock)
	}

	rawdb.WriteHeadBlockHash(bc.db, bc.CurrentBlock().Hash())

	return bc.loadLastState()
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (bc *RootBlockChain) CurrentBlock() *types.RootBlock {
	return bc.currentBlock.Load().(*types.RootBlock)
}

// SetValidator sets the validator which is used to validate incoming blocks.
func (bc *RootBlockChain) SetValidator(validator Validator) {
	bc.procmu.Lock()
	defer bc.procmu.Unlock()
	bc.validator = validator
}

// Validator returns the current validator.
func (bc *RootBlockChain) Validator() Validator {
	bc.procmu.RLock()
	defer bc.procmu.RUnlock()
	return bc.validator
}

// Reset purges the entire blockchain, restoring it to its genesis state.
func (bc *RootBlockChain) Reset() error {
	return bc.ResetWithGenesisBlock(bc.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (bc *RootBlockChain) ResetWithGenesisBlock(genesis *types.RootBlock) error {
	// Dump the entire block chain and purge the caches
	if err := bc.SetHead(0); err != nil {
		return err
	}
	bc.mu.Lock()
	defer bc.mu.Unlock()

	rawdb.WriteRootBlock(bc.db, genesis)
	bc.genesisBlock = genesis
	bc.insert(bc.genesisBlock)
	bc.currentBlock.Store(bc.genesisBlock)

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (bc *RootBlockChain) repair(head **types.RootBlock) error {
	for {
		block := bc.GetBlock((*head).ParentHash())
		if block == nil {
			return fmt.Errorf("missing block %d [%x]", (*head).NumberU64()-1, (*head).ParentHash())
		}
		(*head) = block.(*types.RootBlock)
	}
}

// Export writes the active chain to the given writer.
func (bc *RootBlockChain) Export(w io.Writer) error {
	return bc.ExportN(w, uint64(0), bc.CurrentBlock().NumberU64())
}

// ExportN writes a subset of the active chain to the given writer.
func (bc *RootBlockChain) ExportN(w io.Writer, first uint64, last uint64) error {
	bc.mu.RLock()
	defer bc.mu.RUnlock()

	if first > last {
		return fmt.Errorf("export failed: first (%d) is greater than last (%d)", first, last)
	}
	log.Info("Exporting batch of blocks", "count", last-first+1)

	start, reported := time.Now(), time.Now()
	for nr := first; nr <= last; nr++ {
		block := bc.GetBlockByNumber(nr)
		if block == nil {
			return fmt.Errorf("export failed on #%d: not found", nr)
		}
		data, err := serialize.SerializeToBytes(block)
		if err != nil {
			return err
		}
		w.Write(data)
		if time.Since(reported) >= statsReportLimit {
			log.Info("Exporting blocks", "exported", block.NumberU64()-first, "elapsed", common.PrettyDuration(time.Since(start)))
			reported = time.Now()
		}
	}

	return nil
}

// insert injects a new head block into the current block chain. This method
// assumes that the block is indeed a true head. It will also reset the head
// header and the head fast sync block to this very same block if they are older
// or if they are on a different side chain.
//
// Note, this function assumes that the `mu` mutex is held!
func (bc *RootBlockChain) insert(block *types.RootBlock) {
	// Add the block to the canonical chain number scheme and mark as the head
	if err := bc.PutRootBlockIndex(block); err != nil {
		//TODO need delete later?
		panic(err)
	}

	rawdb.WriteHeadBlockHash(bc.db, block.Hash())
	bc.currentBlock.Store(block)
}

// Genesis retrieves the chain's genesis block.
func (bc *RootBlockChain) Genesis() *types.RootBlock {
	return bc.genesisBlock
}

// HasBlock checks if a block is fully present in the database or not.
func (bc *RootBlockChain) HasBlock(hash common.Hash) bool {
	if bc.blockCache.Contains(hash) {
		return true
	}
	return rawdb.HasBlock(bc.db, hash)
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (bc *RootBlockChain) GetBlock(hash common.Hash) types.IBlock {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.RootBlock)
	}
	block := rawdb.ReadRootBlock(bc.db, hash)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	bc.blockCache.Add(block.Hash(), block)
	return block
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (bc *RootBlockChain) GetBlockByNumber(number uint64) types.IBlock {
	hash := rawdb.ReadCanonicalHash(bc.db, rawdb.ChainTypeRoot, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return bc.GetBlock(hash)
}

// Stop stops the blockchain service. If any imports are currently in progress
// it will abort them using the procInterrupt.
func (bc *RootBlockChain) Stop() {
	if !atomic.CompareAndSwapInt32(&bc.running, 0, 1) {
		return
	}
	// Unsubscribe all subscriptions registered from blockchain
	bc.scope.Close()
	close(bc.quit)
	atomic.StoreInt32(&bc.procInterrupt, 1)

	bc.wg.Wait()
	log.Info("Blockchain manager stopped")
}

func (bc *RootBlockChain) procFutureBlocks() {
	blocks := make([]types.IBlock, 0, bc.futureBlocks.Len())
	for _, hash := range bc.futureBlocks.Keys() {
		if block, exist := bc.futureBlocks.Peek(hash); exist {
			blocks = append(blocks, block.(*types.RootBlock))
		}
	}
	if len(blocks) > 0 {
		sort.Slice(blocks, func(i, j int) bool { return blocks[i].NumberU64() < blocks[j].NumberU64() })

		// Insert one by one as chain insertion needs contiguous ancestry between blocks
		for i := range blocks {
			bc.InsertChain(blocks[i : i+1])
		}
	}
}

// WriteStatus status of write
type WriteStatus byte

const (
	NonStatTy WriteStatus = iota
	CanonStatTy
	SideStatTy
)

// Rollback is designed to remove a chain of links from the database that aren't
// certain enough to be valid.
func (bc *RootBlockChain) Rollback(chain []common.Hash) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		if currentBlock := bc.CurrentBlock(); currentBlock.Hash() == hash {
			newBlock := bc.GetBlock(currentBlock.ParentHash())
			bc.currentBlock.Store(newBlock)
			rawdb.WriteHeadBlockHash(bc.db, newBlock.Hash())
		}
	}
}

// WriteBlockWithoutState writes only the block and its metadata to the database,
// but does not write any state. This is used to construct competing side forks
// up to the point where they exceed the canonical total difficulty.
func (bc *RootBlockChain) WriteBlockWithoutState(block types.IBlock) (err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()
	rawdb.WriteRootBlock(bc.db, block.(*types.RootBlock))
	bc.blockCache.Add(block.Hash(), block)

	return nil
}

//todo
// WriteBlockWithState writes the block and all associated state to the database.
func (bc *RootBlockChain) WriteBlockWithState(block *types.RootBlock) (status WriteStatus, err error) {
	bc.wg.Add(1)
	defer bc.wg.Done()

	// Make sure no inconsistent state is leaked during insertion
	bc.mu.Lock()
	defer bc.mu.Unlock()

	currentBlock := bc.CurrentBlock()
	localTd := currentBlock.TotalDifficulty()
	externTd := block.TotalDifficulty()

	rawdb.WriteRootBlock(bc.db, block)
	bc.blockCache.Add(block.Hash(), block)

	// Write other block data using a batch.
	batch := bc.db.NewBatch()

	// If the total difficulty is higher than our known, add it to the canonical chain
	// Second clause in the if statement reduces the vulnerability to selfish mining.
	// Please refer to http://www.cs.cornell.edu/~ie53/publications/btcProcFC.pdf
	reorg := externTd.Cmp(localTd) > 0
	currentBlock = bc.CurrentBlock()
	if !reorg && externTd.Cmp(localTd) == 0 {
		// Split same-difficulty blocks by number, then preferentially select
		// the block generated by the local miner as the canonical block.
		if block.NumberU64() < currentBlock.NumberU64() {
			reorg = true
		}
	}
	if reorg {
		// Reorganise the chain if the parent is not the head block
		if block.ParentHash() != currentBlock.Hash() {
			if err := bc.reorg(currentBlock, block); err != nil {
				return NonStatTy, err
			}
		}
		// Write the positional metadata for transaction/receipt lookups and preimages
		rawdb.WriteBlockContentLookupEntriesWithCrossShardHashList(batch, block, nil)

		status = CanonStatTy
	} else {
		status = SideStatTy
	}
	if err := batch.Write(); err != nil {
		return NonStatTy, err
	}

	// Set new head.
	if status == CanonStatTy {
		bc.insert(block)
	}
	bc.futureBlocks.Remove(block.Hash())
	return status, nil
}

// addFutureBlock checks if the block is within the max allowed window to get
// accepted for future processing, and returns an error if the block is too far
// ahead and was not added.
func (bc *RootBlockChain) addFutureBlock(block types.IBlock) error {
	max := big.NewInt(time.Now().Unix() + maxTimeFutureBlocks).Uint64()
	if block.Time() > max {
		return fmt.Errorf("future block timestamp %v > allowed %v", block.Time(), max)
	}
	bc.futureBlocks.Add(block.Hash(), block)
	return nil
}

// InsertChain attempts to insert the given batch of blocks in to the canonical
// chain or, otherwise, create a fork. If an error is returned it will return
// the index number of the failing block as well an error describing what went
// wrong.
//
// After insertion is done, all accumulated events will be fired.
func (bc *RootBlockChain) InsertChain(chain []types.IBlock) (int, error) {
	// Sanity check that we have something meaningful to import
	if len(chain) == 0 {
		return 0, nil
	}
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].NumberU64() != chain[i-1].NumberU64()+1 || chain[i].ParentHash() != chain[i-1].Hash() {
			// Chain broke ancestry, log a message (programming error) and skip insertion
			log.Error("Non contiguous block insert", "number", chain[i].NumberU64(), "hash", chain[i].Hash(),
				"parent", chain[i].ParentHash(), "prevnumber", chain[i-1].NumberU64(), "prevhash", chain[i-1].Hash())

			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].NumberU64(),
				chain[i-1].Hash().Bytes()[:4], i, chain[i].NumberU64(), chain[i].Hash().Bytes()[:4], chain[i].ParentHash().Bytes()[:4])
		}
	}
	// Pre-checks passed, start the full block imports
	bc.wg.Add(1)
	bc.chainmu.Lock()
	n, events, err := bc.insertChain(chain, true)
	bc.chainmu.Unlock()
	bc.wg.Done()

	bc.PostChainEvents(events)
	return n, err
}

func absUint64(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return b - a
}

// insertChain is the internal implementation of insertChain, which assumes that
// 1) chains are contiguous, and 2) The chain mutex is held.
//
// This method is split out so that import batches that require re-injecting
// historical blocks can do so without releasing the lock, which could lead to
// racey behaviour. If a sidechain import is in progress, and the historic state
// is imported, but then new canon-head is added before the actual sidechain
// completes, then the historic state could be pruned again
func (bc *RootBlockChain) insertChain(chain []types.IBlock, verifySeals bool) (int, []interface{}, error) {
	// If the chain is terminating, don't even bother starting u
	if atomic.LoadInt32(&bc.procInterrupt) == 1 {
		return 0, nil, nil
	}

	// A queued approach to delivering events. This is generally
	// faster than direct delivery and requires much less mutex
	// acquiring.
	var (
		stats     = insertStats{startTime: mclock.Now()}
		events    = make([]interface{}, 0, len(chain))
		lastCanon *types.RootBlock
	)

	// Peek the error for the first block to decide the directing import logic
	it := newInsertIterator(chain, bc.Validator(), bc.isCheckDB)

	block, err := it.next()
	switch {
	// First block is pruned, insert as sidechain and reorg only if TD grows enough
	case err == ErrPrunedAncestor:
		return bc.insertSidechain(it)

	// First block is future, shove it (and all children) to the future queue (unknown ancestor)
	case err == ErrFutureBlock || (err == ErrUnknownAncestor && bc.futureBlocks.Contains(it.first().ParentHash())):
		for block != nil && (it.index == 0 || err == ErrUnknownAncestor) {
			if err := bc.addFutureBlock(block); err != nil {
				return it.index, events, err
			}
			block, err = it.next()
		}
		stats.queued += it.processed()
		stats.ignored += it.remaining()

		// If there are any still remaining, mark as ignored
		return it.index, events, err

	// First block (and state) is known
	//   1. We did a roll-back, and should now do a re-import
	//   2. The block is stored as a sidechain, and is lying about it's stateroot, and passes a stateroot
	// 	    from the canonical chain, which has not been verified.
	case err == ErrKnownBlock:
		// Skip all known blocks that behind us
		current := bc.CurrentBlock().NumberU64()

		for block != nil && err == ErrKnownBlock && current >= block.NumberU64() {
			stats.ignored++
			block, err = it.next()
		}
		// Falls through to the block import

	// Some other error occurred, abort
	case err != nil:
		stats.ignored += len(it.chain)
		bc.reportBlock(block, err)
		return it.index, events, err
	}
	// No validation errors for the first block (or chain prefix skipped)
	for ; block != nil && err == nil; block, err = it.next() {
		// If the chain is terminating, stop processing blocks
		if atomic.LoadInt32(&bc.procInterrupt) == 1 {
			log.Debug("Premature abort during blocks processing")
			break
		}
		// Retrieve the parent block and it's state to execute on top
		start := time.Now()

		parent := it.previous()
		if parent == nil {
			parent = bc.GetBlock(block.ParentHash())
		}
		if err != nil {
			bc.reportBlock(block, err)
			return it.index, events, err
		}
		if !bc.isCheckDB && absUint64(bc.CurrentBlock().NumberU64(), block.NumberU64()) > bc.Config().Root.MaxStaleRootBlockHeightDiff {
			log.Warn("Insert Root Block", "drop block height", block.NumberU64(), "tip height", bc.CurrentBlock().NumberU64())
			return it.index, events, fmt.Errorf("block is too old %v %v", block.NumberU64(), bc.CurrentBlock().NumberU64())
		}
		proctime := time.Since(start)

		// Write the block to the chain and get the status.
		status, err := bc.WriteBlockWithState(block.(*types.RootBlock))
		if err != nil {
			return it.index, events, err
		}
		switch status {
		case CanonStatTy:
			log.Debug("Inserted new block", "number", block.NumberU64(), "hash", block.Hash(),
				"minorHeaderd", len(block.Content()), "elapsed", common.PrettyDuration(time.Since(start)))
			lastCanon = block.(*types.RootBlock)
			events = append(events, RootChainEvent{lastCanon, block.Hash()})

			// Only count canonical blocks for GC processing time
			bc.gcproc += proctime

		case SideStatTy:
			log.Debug("Inserted forked block", "number", block.NumberU64(), "hash", block.Hash(),
				"diff", block.IHeader().GetDifficulty(), "elapsed", common.PrettyDuration(time.Since(start)),
				"headblock", len(block.Content()))
			events = append(events, RootChainSideEvent{block.(*types.RootBlock)})
		}
		stats.processed++
		//stats.report(chain, it.index)
	}
	// Any blocks remaining here? The only ones we care about are the future ones
	if block != nil && err == ErrFutureBlock {
		if err := bc.addFutureBlock(block); err != nil {
			return it.index, events, err
		}
		block, err = it.next()

		for ; block != nil && err == ErrUnknownAncestor; block, err = it.next() {
			if err := bc.addFutureBlock(block); err != nil {
				return it.index, events, err
			}
			stats.queued++
		}
	}
	stats.ignored += it.remaining()

	// Append a single chain head event if we've progressed the chain
	if lastCanon != nil && bc.CurrentBlock().Hash() == lastCanon.Hash() {
		events = append(events, RootChainHeadEvent{lastCanon})
	}
	return it.index, events, err
}

// insertSidechain is called when an import batch hits upon a pruned ancestor
// error, which happens when a sidechain with a sufficiently old fork-block is
// found.
//
// The method writes all (header-and-body-valid) blocks to disk, then tries to
// switch over to the new chain if the TD exceeded the current chain.
func (bc *RootBlockChain) insertSidechain(it *insertIterator) (int, []interface{}, error) {
	var (
		externTd *big.Int
		current  = bc.CurrentBlock().NumberU64()
	)
	// The first sidechain block error is already verified to be ErrPrunedAncestor.
	// Since we don't import them here, we expect ErrUnknownAncestor for the remaining
	// ones. Any other errors means that the block is invalid, and should not be written
	// to disk.
	block, err := it.current(), ErrPrunedAncestor
	for ; block != nil && (err == ErrPrunedAncestor); block, err = it.next() {
		// Check the canonical state root for that number
		if number := block.NumberU64(); current >= number {
			if canonical := bc.GetBlockByNumber(number); canonical != nil && canonical.Hash() == block.Hash() {
				// This is most likely a shadow-state attack. When a fork is imported into the
				// database, and it eventually reaches a block height which is not pruned, we
				// just found that the state already exist! This means that the sidechain block
				// refers to a state which already exists in our canon chain.
				//
				// If left unchecked, we would now proceed importing the blocks, without actually
				// having verified the state of the previous blocks.
				log.Warn("Sidechain ghost-state attack detected", "number", block.NumberU64(), "sideroot")

				// If someone legitimately side-mines blocks, they would still be imported as usual. However,
				// we cannot risk writing unverified blocks to disk when they obviously target the pruning
				// mechanism.
				return it.index, nil, errors.New("sidechain ghost-state attack")
			}
		}
		externTd = block.IHeader().GetTotalDifficulty()

		if !bc.HasBlock(block.Hash()) {
			start := time.Now()
			if err := bc.WriteBlockWithoutState(block); err != nil {
				return it.index, nil, err
			}
			log.Debug("Inserted sidechain block", "number", block.NumberU64(), "hash", block.Hash(),
				"diff", block.IHeader().GetDifficulty(), "elapsed", common.PrettyDuration(time.Since(start)),
				"Headers", len(block.Content()))
		}
	}
	// At this point, we've written all sidechain blocks to database. Loop ended
	// either on some other error or all were processed. If there was some other
	// error, we can ignore the rest of those blocks.
	//
	// If the externTd was larger than our local TD, we now need to reimport the previous
	// blocks to regenerate the required state
	localTd := bc.CurrentBlock().TotalDifficulty()
	if localTd.Cmp(externTd) > 0 {
		log.Info("Sidechain written to disk", "start", it.first().NumberU64(), "end", it.previous().NumberU64(), "sidetd", externTd, "localtd", localTd)
		return it.index, nil, err
	}
	// Gather all the sidechain hashes (full blocks may be memory heavy)
	var (
		hashes []common.Hash
	)
	parent := bc.GetHeader(it.previous().Hash())
	for parent != nil {
		hashes = append(hashes, parent.Hash())
		parent = bc.GetHeader(parent.GetParentHash())
	}
	if parent == nil {
		return it.index, nil, errors.New("missing parent")
	}
	// Import all the pruned blocks to make the state available
	var (
		blocks []types.IBlock
	)
	for i := len(hashes) - 1; i >= 0; i-- {
		// Append the next block to our batch
		block := bc.GetBlock(hashes[i])

		blocks = append(blocks, block)

		// If memory use grew too large, import and continue. Sadly we need to discard
		// all raised events and logs from notifications since we're too heavy on the
		// memory here.
		if len(blocks) >= 2048 {
			log.Info("Importing heavy sidechain segment", "blocks", len(blocks), "start", blocks[0].NumberU64(), "end", block.NumberU64())
			if _, _, err := bc.insertChain(blocks, false); err != nil {
				return 0, nil, err
			}
			blocks = blocks[:0]

			// If the chain is terminating, stop processing blocks
			if atomic.LoadInt32(&bc.procInterrupt) == 1 {
				log.Debug("Premature abort during blocks processing")
				return 0, nil, nil
			}
		}
	}
	if len(blocks) > 0 {
		log.Info("Importing sidechain segment", "start", blocks[0].NumberU64(), "end", blocks[len(blocks)-1].NumberU64())
		return bc.insertChain(blocks, false)
	}
	return 0, nil, nil
}

// reorgs takes two blocks, an old chain and a new chain and will reconstruct the blocks and inserts them
// to be part of the new canonical chain and accumulates potential missing transactions and post an
// event about them
func (bc *RootBlockChain) reorg(oldBlock, newBlock types.IBlock) error {
	log.Debug("reorg", "oldBlock", oldBlock.NumberU64(), "newBlock", newBlock.NumberU64())
	log.Debug("reorg done")
	var (
		newChain       []types.IBlock
		oldChain       []types.IBlock
		commonBlock    types.IBlock
		deletedHeaders types.MinorBlockHeaders
	)

	// first reduce whoever is higher bound
	if oldBlock.NumberU64() > newBlock.NumberU64() {
		// reduce old chain
		for ; oldBlock != nil && oldBlock.NumberU64() != newBlock.NumberU64(); oldBlock = bc.GetBlock(oldBlock.ParentHash()) {
			oldChain = append(oldChain, oldBlock)
			deletedHeaders = append(deletedHeaders, oldBlock.(*types.RootBlock).MinorBlockHeaders()...)
		}
	} else {
		// reduce new chain and append new chain blocks for inserting later on
		for ; newBlock != nil && newBlock.NumberU64() != oldBlock.NumberU64(); newBlock = bc.GetBlock(newBlock.ParentHash()) {
			newChain = append(newChain, newBlock)
		}
	}
	if oldBlock == nil {
		return fmt.Errorf("Invalid old chain")
	}
	if newBlock == nil {
		return fmt.Errorf("Invalid new chain")
	}

	for {
		if oldBlock.Hash() == newBlock.Hash() {
			commonBlock = oldBlock
			break
		}

		oldChain = append(oldChain, oldBlock)
		newChain = append(newChain, newBlock)

		oldBlock, newBlock = bc.GetBlock(oldBlock.ParentHash()), bc.GetBlock(newBlock.ParentHash())
		if oldBlock == nil {
			return fmt.Errorf("Invalid old chain")
		}
		if newBlock == nil {
			return fmt.Errorf("Invalid new chain")
		}
	}
	// Ensure the user sees large reorgs
	if len(oldChain) > 0 && len(newChain) > 0 {
		logFn := log.Debug
		if len(oldChain) > 63 {
			logFn = log.Warn
		}
		logFn("Chain split detected", "number", commonBlock.NumberU64(), "hash", commonBlock.Hash(),
			"drop", len(oldChain), "dropfrom", oldChain[0].Hash(), "add", len(newChain), "addfrom", newChain[0].Hash())
	} else {
		log.Error("Impossible reorg, please file an issue", "oldnum", oldBlock.NumberU64(), "oldhash", oldBlock.Hash(), "newnum", newBlock.NumberU64(), "newhash", newBlock.Hash())
	}
	// Insert the new chain, taking care of the proper incremental order
	var addedHeaders types.MinorBlockHeaders
	for i := len(newChain) - 1; i >= 0; i-- {
		// insert the block in the canonical way, re-writing history
		bc.insert(newChain[i].(*types.RootBlock))
		// write lookup entries for hash based transaction/receipt searches
		rawdb.WriteBlockContentLookupEntriesWithCrossShardHashList(bc.db, newChain[i].(*types.RootBlock), nil)
		addedHeaders = append(addedHeaders, newChain[i].(*types.RootBlock).MinorBlockHeaders()...)
	}
	// calculate the difference between deleted and added transactions
	diff := types.MinorHeaderDifference(deletedHeaders, addedHeaders)
	// When transactions get deleted from the database that means the
	// receipts that were created in the fork must also be deleted
	batch := bc.db.NewBatch()
	for _, item := range diff {
		rawdb.DeleteBlockContentLookupEntry(batch, item.Hash())
	}
	batch.Write()

	if len(oldChain) > 0 {
		go func() {
			for _, block := range oldChain {
				bc.chainSideFeed.Send(RootChainSideEvent{Block: block.(*types.RootBlock)})
			}
		}()
	}

	return nil
}

// PostChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
// TODO: Should not expose PostChainEvents. The chain events should be posted in WriteBlock.
func (bc *RootBlockChain) PostChainEvents(events []interface{}) {
	for _, event := range events {
		switch ev := event.(type) {
		case RootChainEvent:
			bc.chainFeed.Send(ev)

		case RootChainHeadEvent:
			bc.chainHeadFeed.Send(ev)

		case RootChainSideEvent:
			bc.chainSideFeed.Send(ev)
		}
	}
}

func (bc *RootBlockChain) update() {
	futureTimer := time.NewTicker(5 * time.Second)
	defer futureTimer.Stop()
	for {
		select {
		case <-futureTimer.C:
			bc.procFutureBlocks()
		case <-bc.quit:
			return
		}
	}
}

// reportBlock logs a bad block error.
func (bc *RootBlockChain) reportBlock(block types.IBlock, err error) {

	log.Error(fmt.Sprintf(`
########## BAD BLOCK #########
Chain config: %#v

Number: %v
Hash: 0x%x

Error: %v
##############################
`, bc.chainConfig, block.NumberU64(), block.Hash(), err))
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the RootHeaderChain's internal cache.
func (bc *RootBlockChain) CurrentHeader() types.IHeader {
	return bc.CurrentBlock().IHeader()
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (bc *RootBlockChain) GetHeader(hash common.Hash) types.IHeader {
	block := bc.GetBlock(hash)
	if qkccom.IsNil(block) {
		return nil
	}
	return block.IHeader()
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (bc *RootBlockChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	// Get the origin header from which to fetch
	block := bc.GetBlock(hash)
	if qkccom.IsNil(block) {
		return nil
	}
	// Iterate the Headers until enough is collected or the genesis reached
	chain := make([]common.Hash, 0, max)
	for i := uint64(0); i < max; i++ {
		next := block.ParentHash()
		if block = bc.GetBlock(next); block == nil {
			break
		}
		chain = append(chain, next)
		if block.NumberU64() == 0 {
			break
		}
	}
	return chain
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (bc *RootBlockChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	if ancestor > number {
		return common.Hash{}, 0
	}
	if ancestor == 1 {
		// in this case it is cheaper to just read the header
		if block := bc.GetBlock(hash); block != nil {
			return block.ParentHash(), number - 1
		} else {
			return common.Hash{}, 0
		}
	}
	for ancestor != 0 {
		if rawdb.ReadCanonicalHash(bc.db, rawdb.ChainTypeRoot, number) == hash {
			number -= ancestor
			return rawdb.ReadCanonicalHash(bc.db, rawdb.ChainTypeRoot, number), number
		}
		if *maxNonCanonical == 0 {
			return common.Hash{}, 0
		}
		*maxNonCanonical--
		ancestor--
		block := bc.GetBlock(hash)
		if block == nil {
			return common.Hash{}, 0
		}
		hash = block.ParentHash()
		number--
	}
	return hash, number
}

func (bc *RootBlockChain) GetParentHashByHash(hash common.Hash) common.Hash {
	if b := bc.GetBlock(hash); b == nil {
		return common.Hash{}
	} else {
		return b.(*types.RootBlock).ParentHash()
	}
}

func (bc *RootBlockChain) isSameChain(longerChainHeader, shorterChainHeader types.IBlock) bool {
	return isSameChain(bc.GetParentHashByHash, longerChainHeader, shorterChainHeader)
}

func (bc *RootBlockChain) AddValidatedMinorBlockHeader(hash common.Hash, coinbaseToken *types.TokenBalances) {
	bc.PutMinorBlockCoinbase(hash, coinbaseToken)
}

func (bc *RootBlockChain) GetLatestMinorBlockHeaders(hash common.Hash) map[uint32]*types.MinorBlockHeader {
	headerMap := make(map[uint32]*types.MinorBlockHeader)
	headers := rawdb.ReadLatestMinorBlockHeaders(bc.db, hash)
	for _, header := range headers {
		headerMap[header.Branch.GetFullShardID()] = header
	}

	return headerMap
}

func (bc *RootBlockChain) SetLatestMinorBlockHeaders(hash common.Hash, headerMap map[uint32]*types.MinorBlockHeader) {
	headers := make([]*types.MinorBlockHeader, 0, len(headerMap))
	for _, header := range headerMap {
		headers = append(headers, header)
	}

	rawdb.WriteLatestMinorBlockHeaders(bc.db, hash, headers)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (bc *RootBlockChain) GetHeaderByNumber(number uint64) types.IHeader {
	block := bc.GetBlockByNumber(number)
	if qkccom.IsNil(block) {
		return nil
	}
	return block.IHeader()
}

// Config retrieves the blockchain's chain configuration.
func (bc *RootBlockChain) Config() *config.QuarkChainConfig { return bc.chainConfig }

// Engine retrieves the blockchain's consensus engine.
func (bc *RootBlockChain) Engine() consensus.Engine { return bc.engine }

func (bc *RootBlockChain) SkipDifficultyCheck() bool {
	return bc.Config().SkipRootDifficultyCheck
}

//For remote miner to getWork, no signature verified
func (bc *RootBlockChain) GetAdjustedDifficultyToMine(header types.IHeader) (*big.Int, uint64, error) {
	rHeader := header.(*types.RootBlockHeader)
	if crypto.VerifySignature(bc.Config().GuardianPublicKey, rHeader.SealHash().Bytes(), rHeader.Signature[:64]) {
		guardianAdjustedDiff := new(big.Int).Div(rHeader.GetDifficulty(), new(big.Int).SetUint64(1000))
		return guardianAdjustedDiff, 1, nil
	}
	if bc.posw.IsPoSWEnabled(header.GetTime(), header.NumberU64()) {
		stakes, err := bc.getPoSWStakes(header)
		if err != nil {
			log.Debug("get PoSW stakes", "err", err, "coinbase", header.GetCoinbase().ToHex())
		}
		poswAdjusted, err := bc.posw.PoSWDiffAdjust(header, stakes, *bc.chainConfig.Root.PoSWConfig.TotalStakePerBlock)
		if err != nil {
			log.Debug("PoSW diff adjust", "err", err, "coinbase", header.GetCoinbase().ToHex())
		}
		if poswAdjusted != nil && poswAdjusted.Cmp(rHeader.Difficulty) == -1 {
			log.Debug("PoSW applied", "from", rHeader.Difficulty, "to", poswAdjusted, "coinbase", header.GetCoinbase().ToHex())
			return header.GetDifficulty(), bc.Config().Root.PoSWConfig.DiffDivider, nil
		}
		log.Debug("PoSW not satisfied", "stakes", stakes, "coinbase", header.GetCoinbase().ToHex())
	}
	return rHeader.GetDifficulty(), 1, nil
}

func (bc *RootBlockChain) getPoSWStakes(header types.IHeader) (*big.Int, error) {
	// get chain 0 shard 0's last confirmed block header
	lastConfirmedMinorBlockHeader := bc.GetLastConfirmedMinorBlockHeader(header.GetParentHash(), uint32(1))
	if lastConfirmedMinorBlockHeader == nil {
		return nil, errors.New("no shard block has been confirmed")
	}
	getStakes := bc.GetRootChainStakesFunc()
	stakes, _, err := getStakes(header.GetCoinbase(), lastConfirmedMinorBlockHeader.Hash())
	if err != nil {
		return nil, err
	}
	return stakes, nil
}

func (bc *RootBlockChain) GetAdjustedDifficulty(header types.IHeader) (*big.Int, uint64, error) {
	rHeader := header.(*types.RootBlockHeader)
	if crypto.VerifySignature(bc.Config().GuardianPublicKey, rHeader.SealHash().Bytes(), rHeader.Signature[:64]) {
		guardianAdjustedDiff := new(big.Int).Div(rHeader.GetDifficulty(), new(big.Int).SetUint64(1000))
		return guardianAdjustedDiff, 1, nil
	}
	if bc.posw.IsPoSWEnabled(header.GetTime(), header.NumberU64()) {
		poswAdjusted, err := bc.getPoSWAdjustedDiff(header)
		if err != nil {
			log.Debug("PoSW not applied", "reason", err, "coinbase", header.GetCoinbase().ToHex())
		}
		if poswAdjusted != nil && poswAdjusted.Cmp(rHeader.Difficulty) == -1 {
			log.Debug("PoSW applied", "from", rHeader.Difficulty, "to", poswAdjusted, "coinbase", header.GetCoinbase().ToHex())
			return header.GetDifficulty(), bc.Config().Root.PoSWConfig.DiffDivider, nil
		}
		log.Debug("PoSW not satisfied", "coinbase", header.GetCoinbase().ToHex())
	}
	return rHeader.GetDifficulty(), 1, nil
}

func (bc *RootBlockChain) getPoSWAdjustedDiff(header types.IHeader) (*big.Int, error) {
	stakes, err := bc.getSignedPoSWStakes(header)
	if err != nil {
		return nil, err
	}
	return bc.posw.PoSWDiffAdjust(header, stakes, *bc.chainConfig.Root.PoSWConfig.TotalStakePerBlock)
}

func (bc *RootBlockChain) getSignedPoSWStakes(header types.IHeader) (*big.Int, error) {

	// get chain 0 shard 0's last confirmed block header
	lastConfirmedMinorBlockHeader := bc.GetLastConfirmedMinorBlockHeader(header.GetParentHash(), uint32(1))
	if lastConfirmedMinorBlockHeader == nil {
		return nil, errors.New("no shard block has been confirmed")
	}
	getStakes := bc.GetRootChainStakesFunc()
	stakes, signer, err := getStakes(header.GetCoinbase(), lastConfirmedMinorBlockHeader.Hash())
	if err != nil {
		return nil, err
	}
	if signer == nil || *signer == (common.Address{}) { //could be unlocked if stakes is 0 too
		return nil, errors.New("stakes signer not found")
	}
	rHeader := header.(*types.RootBlockHeader)
	recovered, err := sigToAddr(header.SealHash().Bytes(), rHeader.Signature)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(recovered.Bytes(), signer.Bytes()) {
		return nil, errors.New("stakes signer not match")
	}
	return stakes, nil
}

func sigToAddr(sighash []byte, sig [65]byte) (*account.Recipient, error) {
	pub, err := crypto.Ecrecover(sighash, sig[:])
	if err != nil {
		return nil, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return nil, errors.New("invalid public key")
	}
	var addr account.Recipient
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return &addr, nil
}

func (bc *RootBlockChain) GetLastConfirmedMinorBlockHeader(prevBlock common.Hash, fullShardId uint32) *types.MinorBlockHeader {
	headers := bc.GetLatestMinorBlockHeaders(prevBlock)
	for id, header := range headers {
		if id == fullShardId {
			return header
		}
	}
	return nil
}

// SubscribeRemovedLogsEvent registers a subscription of RemovedLogsEvent.
func (bc *RootBlockChain) SubscribeRemovedLogsEvent(ch chan<- RemovedLogsEvent) event.Subscription {
	return bc.scope.Track(bc.rmLogsFeed.Subscribe(ch))
}

// SubscribeChainEvent registers a subscription of ChainEvent.
func (bc *RootBlockChain) SubscribeChainEvent(ch chan<- RootChainEvent) event.Subscription {
	return bc.scope.Track(bc.chainFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (bc *RootBlockChain) SubscribeChainHeadEvent(ch chan<- RootChainHeadEvent) event.Subscription {
	return bc.scope.Track(bc.chainHeadFeed.Subscribe(ch))
}

// SubscribeChainSideEvent registers a subscription of ChainSideEvent.
func (bc *RootBlockChain) SubscribeChainSideEvent(ch chan<- RootChainSideEvent) event.Subscription {
	return bc.scope.Track(bc.chainSideFeed.Subscribe(ch))
}

func (bc *RootBlockChain) CreateBlockToMine(mHeaderList []*types.MinorBlockHeader, address *account.Address, createTime *uint64) (*types.RootBlock, error) {
	if address == nil {
		a := account.CreatEmptyAddress(0)
		address = &a
	}
	if createTime == nil {
		ts := uint64(time.Now().Unix())
		if bc.CurrentBlock().Time()+1 > ts {
			ts = bc.CurrentBlock().Time() + 1
		}
		createTime = &ts
	}
	difficulty, err := bc.engine.CalcDifficulty(bc, *createTime, bc.CurrentBlock())
	if err != nil {
		return nil, err
	}
	block := bc.CurrentBlock().Header().CreateBlockToAppend(createTime, difficulty, address, nil, nil)
	block.ExtendMinorBlockHeaderList(mHeaderList, *createTime)
	coinbaseToken, err := bc.CalculateRootBlockCoinBase(block)
	if err != nil {
		return nil, err
	}
	block.Finalize(coinbaseToken, address, common.Hash{})
	if len(bc.chainConfig.RootSignerPrivateKey) > 0 {
		prvKey, err := crypto.ToECDSA(bc.chainConfig.RootSignerPrivateKey)
		if err != nil {
			return nil, err
		}
		err = block.SignWithPrivateKey(prvKey)
		if err != nil {
			return nil, err
		}
	}
	return block, nil
}

func (bc *RootBlockChain) CalculateRootBlockCoinBase(rootBlock *types.RootBlock) (*types.TokenBalances, error) {
	for _, header := range rootBlock.MinorBlockHeaders() {
		if !bc.ContainMinorBlockByHash(header.Hash()) {
			log.Error("not contain minorBlock", "number", header.Number, "branch", header.Branch.Value, "hash", header.Hash().String())
			return nil, fmt.Errorf("rootBlockChain not contain minorBlock hash:%v", header.Hash().String())
		}
	}

	rewardTokenMap := types.NewEmptyTokenBalances()
	for _, mheader := range rootBlock.MinorBlockHeaders() {
		mToken := bc.GetMinorBlockCoinbaseTokens(mheader.Hash())
		rewardTokenMap.Add(mToken.GetBalanceMap())
	}

	ratio := bc.Config().RewardCalculateRate
	tempToken := rewardTokenMap.GetBalanceMap()
	for token, value := range tempToken {
		newValue := qkccom.BigIntMulBigRat(value, ratio)
		rewardTokenMap.SetValue(newValue, token)
	}
	genesisToken := bc.Config().GetDefaultChainTokenID()
	genesisTokenBalance := rewardTokenMap.GetTokenBalance(genesisToken)
	genesisTokenBalance.Add(genesisTokenBalance, bc.getCoinbaseAmount(rootBlock.NumberU64()))
	rewardTokenMap.SetValue(genesisTokenBalance, genesisToken)
	return rewardTokenMap, nil

}

func (bc *RootBlockChain) getCoinbaseAmount(height uint64) *big.Int {
	epoch := height / bc.Config().Root.EpochInterval
	coinbaseAmount, ok := bc.coinbaseAmountCache[epoch]
	if !ok {
		numerator := powerBigInt(bc.Config().BlockRewardDecayFactor.Num(), epoch)
		denominator := powerBigInt(new(big.Rat).Set(bc.Config().BlockRewardDecayFactor).Denom(), epoch)
		coinbaseAmount = new(big.Int).Mul(bc.Config().Root.CoinbaseAmount, numerator)
		coinbaseAmount = coinbaseAmount.Div(coinbaseAmount, denominator)
		bc.mu.Lock()
		bc.coinbaseAmountCache[epoch] = coinbaseAmount
		bc.mu.Unlock()
	}
	return coinbaseAmount
}

func (bc *RootBlockChain) IsMinorBlockValidated(mHash common.Hash) bool {
	return bc.ContainMinorBlockByHash(mHash)
}

func (bc *RootBlockChain) GetNextDifficulty(create *uint64) (*big.Int, error) {
	if create == nil {
		ts := uint64(time.Now().Unix())
		if ts < bc.CurrentBlock().Time()+1 {
			ts = bc.CurrentBlock().Time() + 1
		}
		create = &ts
	}
	return bc.engine.CalcDifficulty(bc, *create, bc.CurrentBlock())
}

func (bc *RootBlockChain) WriteCommittingHash(hash common.Hash) {
	rawdb.WriteRootBlockCommittingHash(bc.db, hash)
}

func (bc *RootBlockChain) ClearCommittingHash() {
	rawdb.DeleteRbCommittingHash(bc.db)
}

func (bc *RootBlockChain) GetCommittingBlockHash() common.Hash {
	return rawdb.ReadRbCommittingHash(bc.db)
}

func (bc *RootBlockChain) SetEnableCountMinorBlocks(flag bool) {
	bc.countMinorBlocks = flag
}

func (bc *RootBlockChain) SetBroadcastRootBlockFunc(f func(block *types.RootBlock) error) {
	bc.addBlockAndBroad = f
}

func (bc *RootBlockChain) AddBlock(block types.IBlock) error {
	rootBlock, ok := block.(*types.RootBlock)
	if !ok {
		return errors.New("block is not rootBlock")
	}
	return bc.addBlockAndBroad(rootBlock)
}

func (bc *RootBlockChain) PutRootBlockIndex(block *types.RootBlock) error {
	rawdb.WriteCanonicalHash(bc.db, rawdb.ChainTypeRoot, block.Hash(), block.NumberU64())

	if !bc.countMinorBlocks {
		return nil
	}
	var (
		shardRecipientCnt = make(map[uint32]map[account.Recipient]uint32)
		err               error
	)
	if block.NumberU64() > 0 {
		if shardRecipientCnt, err = bc.GetBlockCount(block.Number() - 1); err != nil {
			return err
		}
	}

	for _, header := range block.MinorBlockHeaders() {
		fullShardID := header.Branch.GetFullShardID()
		recipient := header.Coinbase.Recipient
		oldCount := shardRecipientCnt[fullShardID][recipient]
		newCount := oldCount + 1
		if _, ok := shardRecipientCnt[fullShardID]; ok == false {
			shardRecipientCnt[fullShardID] = make(map[account.Recipient]uint32)
		}
		shardRecipientCnt[fullShardID][recipient] = newCount
		blockID := encoder.IDEncoder(header.Hash().Bytes(), fullShardID)
		bc.PutRootBlockConfirmingMinorBlock(blockID, block.Hash())
	}
	for fullShardID, infoList := range shardRecipientCnt {
		dataToDb := new(account.CoinbaseStatses)
		for addr, count := range infoList {
			dataToDb.CoinbaseStatsList = append(dataToDb.CoinbaseStatsList, account.CoinbaseStats{
				Addr: addr,
				Cnt:  count,
			})
		}
		data, err := serialize.SerializeToBytes(dataToDb)
		if err != nil {
			return err
		}
		rawdb.WriteMinorBlockCnt(bc.db, fullShardID, block.Number(), data)
	}
	return nil
}

func (bc *RootBlockChain) GetBlockCount(rootHeight uint32) (map[uint32]map[account.Recipient]uint32, error) {
	// Returns a dict(full_shard_id, dict(miner_recipient, block_count))
	shardRecipientCnt := make(map[uint32]map[account.Recipient]uint32)
	if !bc.countMinorBlocks {
		return shardRecipientCnt, nil
	}
	fullShardIds := bc.chainConfig.GetInitializedShardIdsBeforeRootHeight(rootHeight)
	for _, fullShardId := range fullShardIds {
		data := rawdb.GetMinorBlockCnt(bc.db, fullShardId, rootHeight)
		if len(data) == 0 {
			continue
		}
		infoList := new(account.CoinbaseStatses)
		if err := serialize.DeserializeFromBytes(data, infoList); err != nil {
			panic(err) //TODO delete later unexpected err
		}
		if _, ok := shardRecipientCnt[fullShardId]; !ok {
			shardRecipientCnt[fullShardId] = make(map[account.Recipient]uint32)
		}
		for _, info := range infoList.CoinbaseStatsList {
			shardRecipientCnt[fullShardId][info.Addr] = info.Cnt
		}
	}
	return shardRecipientCnt, nil
}

func (bc *RootBlockChain) ContainMinorBlockByHash(mHash common.Hash) bool {
	return rawdb.ContainMinorBlockByHash(bc.db, mHash)
}

func (bc *RootBlockChain) PutMinorBlockCoinbase(mHash common.Hash, coinBaseTokens *types.TokenBalances) {
	rawdb.WriteMinorBlockCoinbase(bc.db, mHash, coinBaseTokens)
}

func (bc *RootBlockChain) GetMinorBlockCoinbaseTokens(mHash common.Hash) *types.TokenBalances {
	return rawdb.GetMinorBlockCoinbaseToken(bc.db, mHash)
}

func (bc *RootBlockChain) PutRootBlockConfirmingMinorBlock(blockID []byte, rHash common.Hash) {
	rawdb.PutRootBlockConfirmingMinorBlock(bc.db, blockID, rHash)
}

func (bc *RootBlockChain) GetRootBlockConfirmingMinorBlock(blockID []byte) common.Hash {
	//For json Rpc
	return rawdb.GetRootBlockConfirmingMinorBlock(bc.db, blockID)
}

func (bc *RootBlockChain) SetRootChainStakesFunc(getRootChainStakes func(address account.Address,
	lastMinor common.Hash) (*big.Int, *account.Recipient, error)) {
	bc.rootChainStakesFunc = getRootChainStakes
}

func (bc *RootBlockChain) GetRootChainStakesFunc() func(address account.Address,
	lastMinor common.Hash) (*big.Int, *account.Recipient, error) {
	return bc.rootChainStakesFunc
}

func (bc *RootBlockChain) PoSWInfo(header *types.RootBlockHeader) (*rpc.PoSWInfo, error) {
	if header.Number == 0 {
		return nil, nil
	}
	if !bc.posw.IsPoSWEnabled(header.Time, header.NumberU64()) {
		return nil, nil
	}
	stakes, _ := bc.getSignedPoSWStakes(header)
	diff, mineable, mined, _ := bc.posw.GetPoSWInfo(header, stakes, header.Coinbase.Recipient, *bc.chainConfig.Root.PoSWConfig.TotalStakePerBlock)
	return &rpc.PoSWInfo{
		EffectiveDifficulty: diff,
		PoswMinedBlocks:     mined + 1,
		PoswMineableBlocks:  mineable,
	}, nil
}
