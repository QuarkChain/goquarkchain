// Modified from go-ethereum under GNU Lesser General Public License

package core

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	mrand "math/rand"
	"sync/atomic"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
)

const (
	headerCacheLimit = 512
	tdCacheLimit     = 1024
	numberCacheLimit = 2048
)

// RootHeaderChain implements the basic block header chain logic that is shared by
// core.RootBlockChain and light.LightChain. It is not usable in itself, only as
// a part of either structure.
// It is not thread safe either, the encapsulating chain structures should do
// the necessary mutex locking/unlocking.
type RootHeaderChain struct {
	config *config.QuarkChainConfig

	chainDb       ethdb.Database
	genesisHeader *types.RootBlockHeader

	currentHeader     atomic.Value // Current head of the header chain (may be above the block chain!)
	currentHeaderHash common.Hash  // Hash of the current head of the header chain (prevent recomputing all the time)

	headerCache *lru.Cache // Cache for the most recent block Headers
	tdCache     *lru.Cache // Cache for the most recent block total difficulties
	numberCache *lru.Cache // Cache for the most recent block numbers

	procInterrupt func() bool

	rand   *mrand.Rand
	engine consensus.Engine
}

// NewHeaderChain creates a new RootHeaderChain structure.
//  getValidator should return the parent's validator
//  procInterrupt points to the parent's interrupt semaphore
//  wg points to the parent's shutdown wait group
func NewHeaderChain(chainDb ethdb.Database, config *config.QuarkChainConfig, engine consensus.Engine, procInterrupt func() bool) (*RootHeaderChain, error) {
	headerCache, _ := lru.New(headerCacheLimit)
	tdCache, _ := lru.New(tdCacheLimit)
	numberCache, _ := lru.New(numberCacheLimit)

	// Seed a fast but crypto originating random generator
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return nil, err
	}

	hc := &RootHeaderChain{
		config:        config,
		chainDb:       chainDb,
		headerCache:   headerCache,
		tdCache:       tdCache,
		numberCache:   numberCache,
		procInterrupt: procInterrupt,
		rand:          mrand.New(mrand.NewSource(seed.Int64())),
		engine:        engine,
	}

	header := hc.GetHeaderByNumber(0)
	if header == nil {
		return nil, ErrNoGenesis
	}

	hc.genesisHeader = header.(*types.RootBlockHeader) //todo
	hc.currentHeader.Store(hc.genesisHeader)
	if head := rawdb.ReadHeadBlockHash(chainDb); head != (common.Hash{}) {
		if chead := hc.GetHeader(head); chead != nil {
			hc.currentHeader.Store(chead)
		}
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	return hc, nil
}

// GetBlockNumber retrieves the block number belonging to the given hash
// from the cache or database
func (hc *RootHeaderChain) GetBlockNumber(hash common.Hash) *uint64 {
	if cached, ok := hc.numberCache.Get(hash); ok {
		number := cached.(uint64)
		return &number
	}
	number := rawdb.ReadHeaderNumber(hc.chainDb, hash)
	if number != nil {
		hc.numberCache.Add(hash, *number)
	}
	return number
}

// WriteHeader writes a header into the local chain, given that its parent is
// already known. If the total difficulty of the newly inserted header becomes
// greater than the current known TD, the canonical chain is re-routed.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing Headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (hc *RootHeaderChain) WriteHeader(header *types.RootBlockHeader) (status WriteStatus, err error) {
	// Cache some values to prevent constant recalculation
	var (
		hash   = header.Hash()
		number = header.NumberU64()
	)
	// Calculate the total difficulty of the header
	ptd := hc.GetTd(header.ParentHash)
	if ptd == nil {
		return NonStatTy, consensus.ErrInvalidDifficulty //todo
	}
	localTd := hc.GetTd(hc.currentHeaderHash)
	externTd := new(big.Int).Add(header.Difficulty, ptd)

	// Irrelevant of the canonical status, write the td and header to the database
	if err := hc.WriteTd(hash, externTd); err != nil {
		log.Crit("Failed to write header total difficulty", "err", err)
	}
	rawdb.WriteRootBlockHeader(hc.chainDb, header)

	// If the total difficulty is higher than our known, add it to the canonical chain
	// Second clause in the if statement reduces the vulnerability to selfish mining.
	// Please refer to http://www.cs.cornell.edu/~ie53/publications/btcProcFC.pdf
	if externTd.Cmp(localTd) > 0 || (externTd.Cmp(localTd) == 0 && mrand.Float64() < 0.5) {
		// Delete any canonical number assignments above the new head
		batch := hc.chainDb.NewBatch()
		for i := number + 1; ; i++ {
			hash := rawdb.ReadCanonicalHash(hc.chainDb, rawdb.ChainTypeRoot, i)
			if hash == (common.Hash{}) {
				break
			}
			rawdb.DeleteCanonicalHash(batch, rawdb.ChainTypeRoot, i)
		}
		batch.Write()

		// Overwrite any stale canonical number assignments
		var (
			headHash   = header.ParentHash
			headNumber = header.NumberU64() - 1
			headHeader = hc.GetHeader(headHash)
		)
		for rawdb.ReadCanonicalHash(hc.chainDb, rawdb.ChainTypeRoot, headNumber) != headHash {
			rawdb.WriteCanonicalHash(hc.chainDb, rawdb.ChainTypeRoot, headHash, headNumber)

			headHash = headHeader.GetParentHash()
			headNumber = headHeader.NumberU64() - 1
			headHeader = hc.GetHeader(headHash)
		}
		// Extend the canonical chain with the new header
		rawdb.WriteCanonicalHash(hc.chainDb, rawdb.ChainTypeRoot, hash, number)
		rawdb.WriteHeadHeaderHash(hc.chainDb, hash)

		hc.currentHeaderHash = hash
		hc.currentHeader.Store(types.CopyRootBlockHeader(header))

		status = CanonStatTy
	} else {
		status = SideStatTy
	}

	hc.headerCache.Add(hash, header)
	hc.numberCache.Add(hash, number)

	return
}

// WhCallback is a callback function for inserting individual Headers.
// A callback is used for two reasons: first, in a LightChain, status should be
// processed and light chain events sent, while in a RootBlockChain this is not
// necessary since chain events are sent after inserting blocks. Second, the
// header writes should be protected by the parent chain mutex individually.
type WhCallback func(*types.RootBlockHeader) error

func (hc *RootHeaderChain) ValidateHeaderChain(chain []*types.RootBlockHeader, checkFreq int) (int, error) {
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].Number != chain[i-1].Number+1 || chain[i].ParentHash != chain[i-1].Hash() {
			// Chain broke ancestry, log a message (programming error) and skip insertion
			log.Error("Non contiguous header insert", "number", chain[i].Number, "hash", chain[i].Hash(),
				"parent", chain[i].ParentHash, "prevnumber", chain[i-1].Number, "prevhash", chain[i-1].Hash())

			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].Number,
				chain[i-1].Hash().Bytes()[:4], i, chain[i].Number, chain[i].Hash().Bytes()[:4], chain[i].ParentHash[:4])
		}
	}

	// Generate the list of seal verification requests, and start the parallel verifier
	seals := make([]bool, len(chain))
	for i := 0; i < len(seals)/checkFreq; i++ {
		index := i*checkFreq + hc.rand.Intn(checkFreq)
		if index >= len(seals) {
			index = len(seals) - 1
		}
		seals[index] = true
	}
	seals[len(seals)-1] = true // Last should always be verified to avoid junk

	headers := make([]types.IHeader, len(chain), len(chain))
	for i, header := range chain {
		headers[i] = header
	}
	abort, results := hc.engine.VerifyHeaders(hc, headers, seals)
	defer close(abort)

	// Iterate over the Headers and ensure they all check out
	for i, _ := range chain {
		// If the chain is terminating, stop processing blocks
		if hc.procInterrupt() {
			log.Debug("Premature abort during Headers verification")
			return 0, errors.New("aborted")
		}

		// Otherwise wait for Headers checks and ensure they pass
		if err := <-results; err != nil {
			return i, err
		}
	}

	return 0, nil
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verfy nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
func (hc *RootHeaderChain) InsertHeaderChain(chain []*types.RootBlockHeader, writeHeader WhCallback, start time.Time) (int, error) {
	// Collect some import statistics to report on
	stats := struct{ processed, ignored int }{}
	// All Headers passed verification, import them into the database
	for i, header := range chain {
		// Short circuit insertion if shutting down
		if hc.procInterrupt() {
			log.Debug("Premature abort during Headers import")
			return i, errors.New("aborted")
		}
		// If the header's already known, skip it, otherwise store
		if hc.HasHeader(header.Hash()) {
			stats.ignored++
			continue
		}
		if err := writeHeader(header); err != nil {
			return i, err
		}
		stats.processed++
	}
	// Report some public statistics so the user has a clue what's going on
	last := chain[len(chain)-1]

	context := []interface{}{
		"count", stats.processed, "elapsed", common.PrettyDuration(time.Since(start)),
		"number", last.Number, "hash", last.Hash(),
	}
	if timestamp := time.Unix(int64(last.Time), 0); time.Since(timestamp) > time.Minute {
		context = append(context, []interface{}{"age", common.PrettyAge(timestamp)}...)
	}
	if stats.ignored > 0 {
		context = append(context, []interface{}{"ignored", stats.ignored}...)
	}
	log.Info("Imported new block Headers", context...)

	return 0, nil
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (hc *RootHeaderChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	// Get the origin header from which to fetch
	header := hc.GetHeader(hash)
	if header == nil {
		return nil
	}
	// Iterate the Headers until enough is collected or the genesis reached
	chain := make([]common.Hash, 0, max)
	for i := uint64(0); i < max; i++ {
		next := header.GetParentHash()
		if header = hc.GetHeader(next); header == nil {
			break
		}
		chain = append(chain, next)
		if header.NumberU64() == 0 {
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
func (hc *RootHeaderChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	if ancestor > number {
		return common.Hash{}, 0
	}
	if ancestor == 1 {
		// in this case it is cheaper to just read the header
		if header := hc.GetHeader(hash); header != nil {
			return header.GetParentHash(), number - 1
		} else {
			return common.Hash{}, 0
		}
	}
	for ancestor != 0 {
		if rawdb.ReadCanonicalHash(hc.chainDb, rawdb.ChainTypeRoot, number) == hash {
			number -= ancestor
			return rawdb.ReadCanonicalHash(hc.chainDb, rawdb.ChainTypeRoot, number), number
		}
		if *maxNonCanonical == 0 {
			return common.Hash{}, 0
		}
		*maxNonCanonical--
		ancestor--
		header := hc.GetHeader(hash)
		if header == nil {
			return common.Hash{}, 0
		}
		hash = header.GetParentHash()
		number--
	}
	return hash, number
}

func (hc *RootHeaderChain) isSameChain(longerChainHeader, shorterChainHeader *types.RootBlockHeader) bool {
	return isSameChain(hc.chainDb, longerChainHeader, shorterChainHeader)
}

// GetTd retrieves a block's total difficulty in the canonical chain from the
// database by hash and number, caching it if found.
func (hc *RootHeaderChain) GetTd(hash common.Hash) *big.Int {
	// Short circuit if the td's already in the cache, retrieve otherwise
	if cached, ok := hc.tdCache.Get(hash); ok {
		return cached.(*big.Int)
	}
	td := rawdb.ReadTd(hc.chainDb, hash)
	if td == nil {
		return nil
	}
	// Cache the found body for next time and return
	hc.tdCache.Add(hash, td)
	return td
}

func (m *RootHeaderChain) SkipDifficultyCheck() bool {
	return m.Config().SkipRootDifficultyCheck
}

func (m *RootHeaderChain) GetAdjustedDifficulty(header types.IHeader) (*big.Int, uint64, error) {
	// todo add logic or move the header chain later
	return header.GetDifficulty(), 1, nil
}

// WriteTd stores a block's total difficulty into the database, also caching it
// along the way.
func (hc *RootHeaderChain) WriteTd(hash common.Hash, td *big.Int) error {
	rawdb.WriteTd(hc.chainDb, hash, td)
	hc.tdCache.Add(hash, new(big.Int).Set(td))
	return nil
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (hc *RootHeaderChain) GetHeader(hash common.Hash) types.IHeader {
	// Short circuit if the header's already in the cache, retrieve otherwise
	if header, ok := hc.headerCache.Get(hash); ok {
		return header.(*types.RootBlockHeader)
	}
	header := rawdb.ReadRootBlockHeader(hc.chainDb, hash)
	if header == nil {
		return nil
	}
	// Cache the found header for next time and return
	hc.headerCache.Add(hash, header)
	return header
}

// HasHeader checks if a block header is present in the database or not.
func (hc *RootHeaderChain) HasHeader(hash common.Hash) bool {
	if hc.headerCache.Contains(hash) {
		return true
	}
	return rawdb.HasHeader(hc.chainDb, hash)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (hc *RootHeaderChain) GetHeaderByNumber(number uint64) types.IHeader {
	hash := rawdb.ReadCanonicalHash(hc.chainDb, rawdb.ChainTypeRoot, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return hc.GetHeader(hash)
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the RootHeaderChain's internal cache.
func (hc *RootHeaderChain) CurrentHeader() types.IHeader {
	return hc.currentHeader.Load().(*types.RootBlockHeader)
}

// SetCurrentHeader sets the current head header of the canonical chain.
func (hc *RootHeaderChain) SetCurrentHeader(head *types.RootBlockHeader) {
	rawdb.WriteHeadHeaderHash(hc.chainDb, head.Hash())

	hc.currentHeader.Store(head)
	hc.currentHeaderHash = head.Hash()
}

// DeleteCallback is a callback function that is called by SetHead before
// each header is deleted.
type DeleteCallback func(rawdb.DatabaseDeleter, common.Hash)

// SetHead rewinds the local chain to a new head. Everything above the new head
// will be deleted and the new one set.
func (hc *RootHeaderChain) SetHead(head uint64, delFn DeleteCallback) {
	height := uint64(0)

	if hdr := hc.CurrentHeader(); hdr != nil {
		height = uint64(hdr.NumberU64())
	}
	batch := hc.chainDb.NewBatch()
	for hdr := hc.CurrentHeader(); hdr != nil && uint64(hdr.NumberU64()) > head; hdr = hc.CurrentHeader() {
		hash := hdr.Hash()
		if delFn != nil {
			delFn(batch, hash)
		}
		rawdb.DeleteRootBlockHeader(batch, hash)
		rawdb.DeleteTd(batch, hash)

		hc.currentHeader.Store(hc.GetHeader(hdr.GetParentHash()))
	}
	// Roll back the canonical chain numbering
	for i := height; i > head; i-- {
		rawdb.DeleteCanonicalHash(batch, rawdb.ChainTypeRoot, i)
	}
	batch.Write()

	// Clear out any stale content from the caches
	hc.headerCache.Purge()
	hc.tdCache.Purge()
	hc.numberCache.Purge()

	if hc.CurrentHeader() == nil {
		hc.currentHeader.Store(hc.genesisHeader)
	}
	hc.currentHeaderHash = hc.CurrentHeader().Hash()

	rawdb.WriteHeadHeaderHash(hc.chainDb, hc.currentHeaderHash)
}

// SetGenesis sets a new genesis block header for the chain
func (hc *RootHeaderChain) SetGenesis(head *types.RootBlockHeader) {
	hc.genesisHeader = head
}

// Config retrieves the header chain's chain configuration.
func (hc *RootHeaderChain) Config() *config.QuarkChainConfig { return hc.config }

// Engine retrieves the header chain's consensus engine.
func (hc *RootHeaderChain) Engine() consensus.Engine { return hc.engine }

// GetBlock implements consensus.ChainReader, and returns nil for every input as
// a header chain does not have blocks available for retrieval.
func (hc *RootHeaderChain) GetBlock(hash common.Hash) types.IBlock {
	return nil
}
