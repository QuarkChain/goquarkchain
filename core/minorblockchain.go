// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// Package core implements the Ethereum consensus protocol.
package core

import (
	"bytes"
	"encoding/hex"
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
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	qkcParams "github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/mclock"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/hashicorp/golang-lru"
)

const (
	maxCrossShardLimit    = 256
	maxRootBlockLimit     = 128
	maxLastConfirmLimit   = 256
	maxGasPriceCacheLimit = 128
)

type gasPriceKey struct {
	currHead common.Hash
	tokenID  uint64
}
type gasPriceSuggestionOracle struct {
	cache       *lru.Cache
	CheckBlocks uint64
	Percentile  uint64
}

// MinorBlockChain represents the canonical chain given a database with a genesis
// block. The Blockchain manages chain imports, reverts, chain reorganisations.
//
// Importing blocks in to the block chain happens according to the set of rules
// defined by the two stage Validator. Processing of blocks is done using the
// Processor which processes the included transaction. The validation of the state
// is done in the second part of the Validator. Failing results in aborting of
// the import.
//
// The MinorBlockChain also helps in returning blocks from **any** chain included
// in the database as well as blocks that represents the canonical chain. It's
// important to note that GetBlock can return any block and does not need to be
// included in the canonical one where as GetBlockByNumber always represents the
// canonical chain.
type MinorBlockChain struct {
	ethChainConfig *params.ChainConfig
	clusterConfig  *config.ClusterConfig // Chain & network configuration
	cacheConfig    *CacheConfig          // Cache configuration for pruning

	db     ethdb.Database // Low level persistent database to store final content in
	triegc *prque.Prque   // Priority queue mapping block numbers to tries to gc
	gcproc time.Duration  // Accumulates canonical block processing for trie dumping

	hc            *HeaderChain
	rmLogsFeed    event.Feed
	chainFeed     event.Feed
	chainSideFeed event.Feed
	chainHeadFeed event.Feed
	logsFeed      event.Feed
	subLogsFeed   event.Feed
	scope         event.SubscriptionScope
	genesisBlock  *types.MinorBlock

	mu      sync.RWMutex // global mutex for locking chain operations
	chainmu sync.RWMutex // blockchain insertion lock
	procmu  sync.RWMutex // block processor lock

	checkpoint   int          // checkpoint counts towards the new checkpoint
	currentBlock atomic.Value // Current head of the block chain

	stateCache            state.Database // State database to reuse between imports (contains state cache)
	receiptsCache         *lru.Cache     // Cache for the most recent receipts per block
	blockCache            *lru.Cache     // Cache for the most recent entire blocks
	futureBlocks          *lru.Cache     // future blocks are blocks added for later processing
	crossShardTxListCache *lru.Cache
	rootBlockCache        *lru.Cache
	lastConfirmCache      *lru.Cache
	coinbaseAmountCache   map[uint64]*types.TokenBalances

	quit    chan struct{} // blockchain quit channel
	running int32         // running must be called atomically
	// procInterrupt must be atomically called
	procInterrupt int32          // interrupt signaler for block processing
	wg            sync.WaitGroup // chain processing wait group for shutting down

	engine    consensus.Engine
	processor Processor // block processor interface
	validator Validator // block and state validator interface
	vmConfig  vm.Config

	shouldPreserve func(*types.MinorBlock) bool // Function used to determine whether should preserve the given block.

	txPool                   *TxPool
	branch                   account.Branch
	shardConfig              *config.ShardConfig
	rootTip                  *types.RootBlockHeader
	confirmedHeaderTip       *types.MinorBlockHeader
	initialized              bool
	rewardCalc               *qkcCommon.ConstMinorBlockRewardCalculator
	gasPriceSuggestionOracle *gasPriceSuggestionOracle
	heightToMinorBlockHashes map[uint64]map[common.Hash]struct{}
	rootHeightToHashes       map[uint64]map[common.Hash]common.Hash // [rootBlockHeight][rootBlockHash][confirmedMinorHash]
	currentEvmState          *state.StateDB
	logInfo                  string
	addMinorBlockAndBroad    func(block *types.MinorBlock) error
	posw                     consensus.PoSWCalculator
	gasLimit                 *big.Int
	xShardGasLimit           *big.Int
}

// NewMinorBlockChain returns a fully initialised block chain using information
// available in the database. It initialises the default Ethereum Validator and
// Processor.
func NewMinorBlockChain(
	db ethdb.Database,
	cacheConfig *CacheConfig,
	chainConfig *params.ChainConfig,
	clusterConfig *config.ClusterConfig,
	engine consensus.Engine,
	vmConfig vm.Config,
	shouldPreserve func(block *types.MinorBlock) bool,
	fullShardID uint32,
) (*MinorBlockChain, error) {
	chainConfig = &qkcParams.DefaultConstantinople //TODO default is constantinople
	if clusterConfig == nil || chainConfig == nil {
		return nil, errors.New("can not new minorBlock: config is nil")
	}
	if cacheConfig == nil {
		cacheConfig = &CacheConfig{
			TrieCleanLimit: 128,
			TrieDirtyLimit: 128,
			TrieTimeLimit:  5 * time.Minute,
			Disabled:       true,
		}
	}
	receiptsCache, _ := lru.New(receiptsCacheLimit)
	blockCache, _ := lru.New(blockCacheLimit)
	futureBlocks, _ := lru.New(maxFutureBlocks)
	crossShardCache, _ := lru.New(maxCrossShardLimit)
	rootBlockCache, _ := lru.New(maxRootBlockLimit)
	lastConfimCache, _ := lru.New(maxLastConfirmLimit)
	gasPriceCache, _ := lru.New(maxGasPriceCacheLimit)
	bc := &MinorBlockChain{
		ethChainConfig:           chainConfig,
		clusterConfig:            clusterConfig,
		cacheConfig:              cacheConfig,
		db:                       db,
		triegc:                   prque.New(nil),
		stateCache:               state.NewDatabaseWithCache(db, cacheConfig.TrieCleanLimit),
		quit:                     make(chan struct{}),
		shouldPreserve:           shouldPreserve,
		receiptsCache:            receiptsCache,
		blockCache:               blockCache,
		futureBlocks:             futureBlocks,
		crossShardTxListCache:    crossShardCache,
		rootBlockCache:           rootBlockCache,
		lastConfirmCache:         lastConfimCache,
		coinbaseAmountCache:      make(map[uint64]*types.TokenBalances),
		engine:                   engine,
		vmConfig:                 vmConfig,
		heightToMinorBlockHashes: make(map[uint64]map[common.Hash]struct{}),
		rootHeightToHashes:       make(map[uint64]map[common.Hash]common.Hash),
		currentEvmState:          new(state.StateDB),
		branch:                   account.Branch{Value: fullShardID},
		shardConfig:              clusterConfig.Quarkchain.GetShardConfigByFullShardID(fullShardID),
		rewardCalc:               &qkcCommon.ConstMinorBlockRewardCalculator{},
		gasPriceSuggestionOracle: &gasPriceSuggestionOracle{
			cache:       gasPriceCache,
			CheckBlocks: 5,
			Percentile:  50,
		},
		logInfo: fmt.Sprintf("shard:%d", fullShardID),
	}
	var err error
	bc.gasLimit, err = bc.clusterConfig.Quarkchain.GasLimit(bc.branch.Value)
	if err != nil {
		return nil, err
	}
	bc.xShardGasLimit = new(big.Int).Set(bc.gasLimit)
	bc.xShardGasLimit = bc.xShardGasLimit.Div(bc.xShardGasLimit, new(big.Int).SetUint64(2))
	bc.SetValidator(NewBlockValidator(clusterConfig.Quarkchain, bc, engine, bc.branch))
	bc.SetProcessor(NewStateProcessor(bc.ethChainConfig, bc, engine))

	bc.hc, err = NewMinorHeaderChain(db, bc.clusterConfig.Quarkchain, engine, bc.getProcInterrupt)
	if err != nil {
		return nil, err
	}

	genesisBlock := bc.GetBlockByNumber(0)
	if qkcCommon.IsNil(genesisBlock) {
		return nil, ErrNoGenesis
	}
	bc.genesisBlock = genesisBlock.(*types.MinorBlock)
	if bc.genesisBlock == nil {
		return nil, ErrNoGenesis
	}
	if err := bc.loadLastState(); err != nil {
		return nil, err
	}
	DefaultTxPoolConfig.NetWorkID = bc.clusterConfig.Quarkchain.NetworkID
	bc.posw = consensus.CreatePoSWCalculator(bc, bc.shardConfig.PoswConfig)
	bc.txPool = NewTxPool(DefaultTxPoolConfig, bc)
	// Take ownership of this particular state
	go bc.update()
	return bc, nil
}

func (m *MinorBlockChain) SetBroadcastMinorBlockFunc(f func(block *types.MinorBlock) error) {
	m.addMinorBlockAndBroad = f
}

func (m *MinorBlockChain) AddBlock(block types.IBlock) error {
	minorBlock, ok := block.(*types.MinorBlock)
	if !ok {
		return errors.New("block is not minorBlock")
	}
	return m.addMinorBlockAndBroad(minorBlock)
}

func (m *MinorBlockChain) getProcInterrupt() bool {
	return atomic.LoadInt32(&m.procInterrupt) == 1
}

// GetVMConfig returns the block chain VM config.
func (m *MinorBlockChain) GetVMConfig() *vm.Config {
	return &m.vmConfig
}

// loadLastState loads the last known chain state from the database. This method
// assumes that the chain manager mutex is held.
func (m *MinorBlockChain) loadLastState() error {
	// Restore the last known head block
	head := rawdb.ReadHeadBlockHash(m.db)
	if head == (common.Hash{}) {
		// Corrupt or empty database, init from scratch
		log.Warn("Empty database, resetting chain")
		return m.Reset()
	}
	// Make sure the entire head block is available
	currentBlock := m.GetMinorBlock(head)
	if currentBlock == nil {
		// Corrupt or empty database, init from scratch
		log.Warn("Head block missing, resetting chain", "hash", head)
		return m.Reset()
	}

	// Make sure the state associated with the block is available
	if _, err := m.StateAt(currentBlock.GetMetaData().Root); err != nil {
		// Dangling block without a state associated, init from scratch
		log.Warn("Head state missing, repairing chain", "number", currentBlock.NumberU64(), "hash", currentBlock.Hash())
		if err := m.repair(&currentBlock); err != nil {
			return err
		}
	}
	// Everything seems to be fine, set as the head block
	m.currentBlock.Store(currentBlock)

	// Restore the last known head header
	currentHeader := currentBlock.Header()
	if head := rawdb.ReadHeadHeaderHash(m.db); head != (common.Hash{}) {
		if header := m.GetHeaderByHash(head); !qkcCommon.IsNil(header) {
			currentHeader = header.(*types.MinorBlockHeader)
		}
	}
	m.hc.SetCurrentHeader(currentHeader)
	return nil
}

// SetHead rewinds the local chain to a new head. In the case of Headers, everything
// above the new head will be deleted and the new one set. In the case of blocks
// though, the head may be further rewound if block bodies are missing (non-archive
// nodes after a fast sync).
// already have locked
func (m *MinorBlockChain) SetHead(head uint64) error {
	m.chainmu.Lock()
	defer m.chainmu.Unlock()
	return m.setHead(head)
}

func (m *MinorBlockChain) setHead(head uint64) error {
	log.Warn("Rewinding blockchain", "target", head)
	// Rewind the header chain, deleting all block bodies until then
	delFn := func(db rawdb.DatabaseDeleter, hash common.Hash) {
		rawdb.DeleteMinorBlock(db, hash)
	}
	m.hc.SetHead(head, delFn)
	currentHeader := m.hc.CurrentHeader()

	// Rewind the block chain, ensuring we don't end up with a stateless head block
	if currentBlock := m.CurrentBlock(); currentBlock != nil && currentHeader.NumberU64() < currentBlock.NumberU64() {
		m.currentBlock.Store(m.GetBlock(currentHeader.Hash()))
	}
	if currentBlock := m.CurrentBlock(); currentBlock != nil {
		if _, err := m.StateAt(currentBlock.GetMetaData().Root); err != nil {
			// Rewound state missing, rolled back to before pivot, reset to genesis
			m.currentBlock.Store(m.genesisBlock)
		}
	}

	// If either blocks reached nil, reset to the genesis state
	if currentBlock := m.CurrentBlock(); currentBlock == nil {
		m.currentBlock.Store(m.genesisBlock)
	}

	currentBlock := m.CurrentBlock()
	rawdb.WriteHeadBlockHash(m.db, currentBlock.Hash())

	// Clear out any stale content from the caches
	m.receiptsCache.Purge()
	m.blockCache.Purge()
	m.futureBlocks.Purge()
	m.crossShardTxListCache.Purge()
	m.rootBlockCache.Purge()
	m.lastConfirmCache.Purge()

	return m.loadLastState()
}

// GasLimit returns the gas limit of the current HEAD block.
func (m *MinorBlockChain) GasLimit() uint64 {
	return m.currentBlock.Load().(*types.MinorBlock).GasLimit().Uint64()
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (m *MinorBlockChain) CurrentBlock() *types.MinorBlock {
	loaded := m.currentBlock.Load()
	if loaded == nil {
		return nil
	}
	return loaded.(*types.MinorBlock)
}

// SetProcessor sets the processor required for making state modifications.
func (m *MinorBlockChain) SetProcessor(processor Processor) {
	m.procmu.Lock()
	defer m.procmu.Unlock()
	m.processor = processor
}

// SetValidator sets the validator which is used to validate incoming blocks.
func (m *MinorBlockChain) SetValidator(validator Validator) {
	m.procmu.Lock()
	defer m.procmu.Unlock()
	m.validator = validator
}

// Validator returns the current validator.
func (m *MinorBlockChain) Validator() Validator {
	m.procmu.RLock()
	defer m.procmu.RUnlock()
	return m.validator
}

// Processor returns the current processor.
func (m *MinorBlockChain) Processor() Processor {
	m.procmu.RLock()
	defer m.procmu.RUnlock()
	return m.processor
}

// State returns a new mutable state based on the current HEAD block.
func (m *MinorBlockChain) State() (*state.StateDB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentEvmState, nil
}

// StateAt returns a new mutable state based on a particular point in time.
func (m *MinorBlockChain) StateAt(root common.Hash) (*state.StateDB, error) {
	evmState, err := state.New(root, m.stateCache)
	if err != nil {
		return nil, err
	}
	evmState.SetShardConfig(m.shardConfig)
	evmState.SetTimeStamp(uint64(time.Now().Unix()))
	return evmState, nil
}

func (m *MinorBlockChain) stateAtWithSenderDisallowMap(minorBlock *types.MinorBlock, coinbase *account.Recipient) (*state.StateDB, error) {
	evmState, err := m.StateAt(minorBlock.Root())
	if err != nil {
		return nil, err
	}
	senderDisallowMap, err := m.posw.BuildSenderDisallowMap(minorBlock.Hash(), coinbase)
	if err != nil {
		return nil, err
	}
	evmState.SetSenderDisallowMap(senderDisallowMap)
	m.setEvmStateWithHeader(evmState, minorBlock.Header())
	return evmState, nil
}

func (m *MinorBlockChain) SkipDifficultyCheck() bool {
	return m.Config().SkipMinorDifficultyCheck
}

func (m *MinorBlockChain) GetAdjustedDifficulty(header types.IHeader) (*big.Int, uint64, error) {
	diff := header.GetDifficulty()
	if m.posw.IsPoSWEnabled(header) {
		preHash := header.GetParentHash()
		balance, err := m.GetBalance(header.GetCoinbase().Recipient, &preHash)
		if err != nil {
			log.Error(m.logInfo, "PoSW: failed to get coinbase balance", err)
			return nil, 0, err
		}
		poswAdjusted, err := m.posw.PoSWDiffAdjust(header, balance.GetTokenBalance(m.clusterConfig.Quarkchain.GetDefaultChainTokenID()))
		if err != nil {
			log.Error(m.logInfo, "PoSW: err", err)
			return nil, 0, err
		}
		if poswAdjusted != nil && poswAdjusted.Cmp(diff) == -1 {
			log.Debug(m.logInfo, "PoSW: from", diff, "to", poswAdjusted)
			diff = poswAdjusted
		}
	}
	return diff, 1, nil
}

// StateCache returns the caching database underpinning the blockchain instance.
func (m *MinorBlockChain) StateCache() state.Database {
	return m.stateCache
}

// reset purges the entire blockchain, restoring it to its genesis state.
func (m *MinorBlockChain) Reset() error {
	return m.ResetWithGenesisBlock(m.genesisBlock)
}

// ResetWithGenesisBlock purges the entire blockchain, restoring it to the
// specified genesis state.
func (m *MinorBlockChain) ResetWithGenesisBlock(genesis *types.MinorBlock) error {
	// Dump the entire block chain and purge the caches
	if err := m.SetHead(0); err != nil {
		return err
	}

	rawdb.WriteMinorBlock(m.db, genesis)

	m.genesisBlock = genesis
	m.insert(m.genesisBlock)
	m.currentBlock.Store(m.genesisBlock)
	m.hc.SetGenesis(m.genesisBlock.Header())
	m.hc.SetCurrentHeader(m.genesisBlock.Header())

	return nil
}

// repair tries to repair the current blockchain by rolling back the current block
// until one with associated state is found. This is needed to fix incomplete db
// writes caused either by crashes/power outages, or simply non-committed tries.
//
// This method only rolls back the current block. The current header and current
// fast block are left intact.
func (m *MinorBlockChain) repair(head **types.MinorBlock) error {
	for {
		// Abort if we've rewound to a head block that does have associated state
		if _, err := m.StateAt((*head).Root()); err == nil {
			log.Info("Rewound blockchain to past state", "number", (*head).Number(), "hash", (*head).Hash())
			return nil
		}
		// Otherwise rewind one block and recheck state availability there
		block := m.GetBlock((*head).ParentHash())
		if qkcCommon.IsNil(block) {
			return fmt.Errorf("missing block %d [%x]", (*head).NumberU64()-1, (*head).ParentHash())
		}
		(*head) = block.(*types.MinorBlock)
	}
}

// Export writes the active chain to the given writer.
func (m *MinorBlockChain) Export(w io.Writer) error {
	return m.ExportN(w, uint64(0), m.CurrentBlock().NumberU64())
}

// ExportN writes a subset of the active chain to the given writer.
func (m *MinorBlockChain) ExportN(w io.Writer, first uint64, last uint64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if first > last {
		return fmt.Errorf("export failed: first (%d) is greater than last (%d)", first, last)
	}
	log.Info("Exporting batch of blocks", "count", last-first+1)

	start, reported := time.Now(), time.Now()
	for nr := first; nr <= last; nr++ {
		block := m.GetBlockByNumber(nr)
		if block == nil {
			return fmt.Errorf("export failed on #%d: not found", nr)
		}
		data, err := serialize.SerializeToBytes(block)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		if err != nil {
			return err
		}

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
func (m *MinorBlockChain) insert(block *types.MinorBlock) {
	// If the block is on a side chain or an unknown one, force other heads onto it too
	updateHeads := rawdb.ReadCanonicalHash(m.db, rawdb.ChainTypeMinor, block.NumberU64()) != block.Hash()

	// Add the block to the canonical chain number scheme and mark as the head
	rawdb.WriteCanonicalHash(m.db, rawdb.ChainTypeMinor, block.Hash(), block.NumberU64())
	rawdb.WriteHeadBlockHash(m.db, block.Hash())

	m.currentBlock.Store(block)

	// If the block is better than our head or is on a different chain, force update heads
	if updateHeads {
		m.hc.SetCurrentHeader(block.Header())
	}

}

// Genesis retrieves the chain's genesis block.
func (m *MinorBlockChain) Genesis() *types.MinorBlock {
	return m.genesisBlock
}

// HasBlock checks if a block is fully present in the database or not.
func (m *MinorBlockChain) HasBlock(hash common.Hash) bool {
	return m.IsMinorBlockCommittedByHash(hash)
}

// HasState checks if state trie is fully present in the database or not.
func (m *MinorBlockChain) HasState(hash common.Hash) bool {
	_, err := m.stateCache.OpenTrie(hash)
	return err == nil
}

// HasBlockAndState checks if a block and associated state trie is fully present
// in the database or not, caching it if present.
func (m *MinorBlockChain) HasBlockAndState(hash common.Hash) bool {
	// Check first that the block itself is known
	flag := m.HasBlock(hash)
	if !flag {
		return false
	}
	block := m.GetMinorBlock(hash)
	if block == nil {
		panic("bug fix block can not be nil")
	}
	return m.HasState(block.GetMetaData().Root)
}

// GetBlock retrieves a block from the database by hash and number,
// caching it if found.
func (m *MinorBlockChain) GetBlock(hash common.Hash) types.IBlock {
	return m.GetMinorBlock(hash)
}

// GetMinorBlock retrieves a block from the database by hash, caching it if found.
func (m *MinorBlockChain) GetMinorBlock(hash common.Hash) *types.MinorBlock {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := m.blockCache.Get(hash); ok {
		return block.(*types.MinorBlock)
	}
	block := rawdb.ReadMinorBlock(m.db, hash)
	if block == nil {
		return nil
	}
	// Cache the found block for next time and return
	m.blockCache.Add(block.Hash(), block)
	return block
}

// GetBlockByNumber retrieves a block from the database by number, caching it
// (associated with its hash) if found.
func (m *MinorBlockChain) GetBlockByNumber(number uint64) types.IBlock {
	hash := rawdb.ReadCanonicalHash(m.db, rawdb.ChainTypeMinor, number)
	if hash == (common.Hash{}) {
		return nil
	}
	return m.GetBlock(hash)
}

func (m *MinorBlockChain) GetHashByHeight(height *uint64) (common.Hash, error) {
	if height != nil {
		hash := rawdb.ReadCanonicalHash(m.db, rawdb.ChainTypeMinor, *height)
		if bytes.Equal(common.Hash{}.Bytes(), hash.Bytes()) {
			return hash, fmt.Errorf("shard %v do no have this  height  %v", m.branch.Value, *height)
		}
	}
	return m.CurrentBlock().Hash(), nil
}

// GetReceiptsByHash retrieves the receipts for all transactions in a given block.
func (m *MinorBlockChain) GetReceiptsByHash(hash common.Hash) types.Receipts {
	if receipts, ok := m.receiptsCache.Get(hash); ok {
		return receipts.(types.Receipts)
	}
	number := rawdb.ReadHeaderNumber(m.db, hash)
	if number == nil {
		return nil
	}
	receipts := rawdb.ReadReceipts(m.db, hash)
	m.receiptsCache.Add(hash, receipts)
	return receipts
}

func (m *MinorBlockChain) GetLogs(hash common.Hash) [][]*types.Log {
	receipts := m.GetReceiptsByHash(hash)
	logs := make([][]*types.Log, len(receipts))
	for index, receipt := range receipts {
		logs[index] = receipt.Logs
	}
	return logs
}

// GetBlocksFromHash returns the block corresponding to hash and up to n-1 ancestors.
// [deprecated by eth/62]
func (m *MinorBlockChain) GetBlocksFromHash(hash common.Hash, n int) (blocks []types.IBlock) {
	number := m.hc.GetBlockNumber(hash)
	if number == nil {
		return nil
	}
	for i := 0; i < n; i++ {
		block := m.GetBlock(hash)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		hash = block.ParentHash()
		*number--
	}
	return
}

// TrieNode retrieves a blob of data associated with a trie node (or code hash)
// either from ephemeral in-memory cache, or from persistent storage.
func (m *MinorBlockChain) TrieNode(hash common.Hash) ([]byte, error) {
	return m.stateCache.TrieDB().Node(hash)
}

func (m *MinorBlockChain) getNeedStoreHeight(rootHash common.Hash, heightDiff []uint64) []uint64 {
	var (
		currNumber = m.CurrentBlock().NumberU64()
	)
	headerTip := m.getLastConfirmedMinorBlockHeaderAtRootBlock(rootHash)
	if headerTip != nil && headerTip.Number < currNumber {
		log.Info("trie", "tip", headerTip.Number, "rootHash", rootHash.String())
		heightDiff = append(heightDiff, currNumber-headerTip.Number)
		if headerTip.Number >= 1 {
			heightDiff = append(heightDiff, currNumber-(headerTip.Number-1))
		}
		if headerTip.Number >= triesInMemory {
			heightDiff = append(heightDiff, currNumber-(headerTip.Number-triesInMemory))
		}
		currBlockNumber := m.CurrentBlock().Number()
		for index := headerTip.Number; index <= currBlockNumber; index++ {
			heightDiff = append(heightDiff, currBlockNumber-index)
		}

	}
	return heightDiff
}

// Stop stops the blockchain service. If any imports are currently in progress
// it will abort them using the procInterrupt.
func (m *MinorBlockChain) Stop() {
	m.txPool.Stop()
	if !atomic.CompareAndSwapInt32(&m.running, 0, 1) {
		return
	}
	// Unsubscribe all subscriptions registered from blockchain
	m.scope.Close()
	close(m.quit)
	atomic.StoreInt32(&m.procInterrupt, 1)

	m.wg.Wait()

	// Ensure the state of a recent block is also stored to disk before exiting.
	// We're writing three different states to catch different restart scenarios:
	//  - HEAD:     So we don't need to reprocess any blocks in the general case
	//  - HEAD-1:   So we don't do large reorgs if our HEAD becomes an uncle
	//  - HEAD-127: So we have a hard limit on the number of blocks reexecuted
	if !m.cacheConfig.Disabled {
		triedb := m.stateCache.TrieDB()
		var (
			currNumber = m.CurrentBlock().NumberU64()
			heightDiff = []uint64{0, 1, triesInMemory - 1}
		)
		if m.rootTip != nil {
			log.Info("need stored tire", "number", m.rootTip.Number)

			for hash, _ := range m.rootHeightToHashes[m.rootTip.NumberU64()] {
				heightDiff = m.getNeedStoreHeight(hash, heightDiff)
			}
			heightDiff = qkcCommon.RemoveDuplicate(heightDiff)
		}
		for _, offset := range heightDiff {
			if currNumber > offset {
				recentBlockInterface := m.GetBlockByNumber(currNumber - offset)
				if qkcCommon.IsNil(recentBlockInterface) {
					log.Error("block is nil", "err", errInsufficientBalanceForGas)
					continue
				}
				recent := recentBlockInterface.(*types.MinorBlock)

				log.Info("Writing cached state to disk", "block", recent.NumberU64(), "hash", recent.Hash(), "root", recent.GetMetaData().Root)
				if err := triedb.Commit(recent.GetMetaData().Root, true); err != nil {
					log.Error("Failed to commit recent state trie", "err", err)
				}
			}
		}
		for !m.triegc.Empty() {
			triedb.Dereference(m.triegc.PopItem().(common.Hash))
		}
		if size, _ := triedb.Size(); size != 0 {
			log.Error("Dangling trie nodes after full cleanup")
		}
	}
	log.Info("Blockchain manager stopped")
}

func (m *MinorBlockChain) procFutureBlocks() {
	blocks := make([]types.IBlock, 0, m.futureBlocks.Len())
	for _, hash := range m.futureBlocks.Keys() {
		if block, exist := m.futureBlocks.Peek(hash); exist {
			blocks = append(blocks, block.(*types.MinorBlock))
		}
	}
	if len(blocks) > 0 {
		sort.Slice(blocks, func(i, j int) bool { return blocks[i].NumberU64() < blocks[j].NumberU64() })
		// Insert one by one as chain insertion needs contiguous ancestry between blocks
		for i := range blocks {
			m.InsertChain(blocks[i:i+1], false)
		}
	}
}

// Rollback is designed to remove a chain of links from the database that aren't
// certain enough to be valid.
func (m *MinorBlockChain) Rollback(chain []common.Hash) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := len(chain) - 1; i >= 0; i-- {
		hash := chain[i]

		currentHeader := m.hc.CurrentHeader()
		if currentHeader.Hash() == hash {
			m.hc.SetCurrentHeader(m.GetHeader(currentHeader.GetParentHash()).(*types.MinorBlockHeader))
		}

		if currentBlock := m.CurrentBlock(); currentBlock.Hash() == hash {
			newBlock := m.GetBlock(currentBlock.ParentHash())
			m.currentBlock.Store(newBlock)
			rawdb.WriteHeadBlockHash(m.db, newBlock.Hash())
		}
	}
}

// SetReceiptsData computes all the non-consensus fields of the receipts
func SetReceiptsData(config *config.QuarkChainConfig, mBlock types.IBlock, receipts types.Receipts) error {
	if qkcCommon.IsNil(mBlock) {
		return ErrMinorBlockIsNil
	}
	block := mBlock.(*types.MinorBlock)
	signer := types.MakeSigner(uint32(config.NetworkID))

	transactions, logIndex := block.GetTransactions(), uint32(0)
	if len(transactions) != len(receipts) {
		return errors.New("transaction and receipt count mismatch")
	}

	for j := 0; j < len(receipts); j++ {
		// The transaction hash can be retrieved from the transaction itself
		receipts[j].TxHash = transactions[j].Hash()

		// The contract address can be derived from the transaction itself
		if transactions[j].EvmTx.To() == nil {
			// Deriving the signer is expensive, only do if it's actually needed
			from, _ := types.Sender(signer, transactions[j].EvmTx)
			toFullShardKey := transactions[j].EvmTx.ToFullShardKey()
			receipts[j].ContractAddress = account.BytesToIdentityRecipient(vm.CreateAddress(from, &toFullShardKey, transactions[j].EvmTx.Nonce()).Bytes())
		}
		// The used gas can be calculated based on previous receipts
		if j == 0 {
			receipts[j].GasUsed = receipts[j].CumulativeGasUsed
		} else {
			receipts[j].GasUsed = receipts[j].CumulativeGasUsed - receipts[j-1].CumulativeGasUsed
		}
		// The derived log fields can simply be set from the block and transaction
		for k := 0; k < len(receipts[j].Logs); k++ {
			receipts[j].Logs[k].BlockNumber = block.NumberU64()
			receipts[j].Logs[k].BlockHash = block.Hash()
			receipts[j].Logs[k].TxHash = receipts[j].TxHash
			receipts[j].Logs[k].TxIndex = uint32(j)
			receipts[j].Logs[k].Index = logIndex
			logIndex++
		}
	}
	return nil
}

// InsertReceiptChain attempts to complete an already existing header chain with
// transaction and receipt data.
func (m *MinorBlockChain) InsertReceiptChain(blockChain []types.IBlock, receiptChain []types.Receipts) (int, error) {
	m.wg.Add(1)
	defer m.wg.Done()

	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(blockChain); i++ {
		if blockChain[i].NumberU64() != blockChain[i-1].NumberU64()+1 || blockChain[i].ParentHash() != blockChain[i-1].Hash() {
			log.Error("Non contiguous receipt insert", "number", blockChain[i].NumberU64(), "hash", blockChain[i].Hash(), "parent", blockChain[i].ParentHash(),
				"prevnumber", blockChain[i-1].NumberU64(), "prevhash", blockChain[i-1].Hash())
			return 0, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, blockChain[i-1].NumberU64(),
				blockChain[i-1].Hash().Bytes()[:4], i, blockChain[i].NumberU64(), blockChain[i].Hash().Bytes()[:4], blockChain[i].ParentHash().Bytes()[:4])
		}
	}

	var (
		stats = struct{ processed, ignored int32 }{}
		start = time.Now()
		bytes = 0
		batch = m.db.NewBatch()
	)
	for i, block := range blockChain {
		receipts := receiptChain[i]
		// Short circuit insertion if shutting down or processing failed
		if atomic.LoadInt32(&m.procInterrupt) == 1 {
			return 0, nil
		}
		// Short circuit if the owner header is unknown
		if !m.HasHeader(block.Hash(), block.NumberU64()) {
			return i, fmt.Errorf("containing header #%d [%x…] unknown", block.NumberU64(), block.Hash().Bytes()[:4])
		}
		// Skip if the entire data is already known
		if m.HasBlock(block.Hash()) {
			stats.ignored++
			continue
		}
		// Compute all the non-consensus fields of the receipts
		if err := SetReceiptsData(m.clusterConfig.Quarkchain, block, receipts); err != nil {
			return i, fmt.Errorf("failed to set receipts data: %v", err)
		}
		// Write all the data out into the database
		if qkcCommon.IsNil(block) {
			return 0, ErrMinorBlockIsNil
		}
		rawdb.WriteMinorBlock(batch, block.(*types.MinorBlock))
		rawdb.WriteReceipts(batch, block.Hash(), receipts)
		rawdb.WriteBlockContentLookupEntriesWithCrossShardHashList(batch, block, nil)

		stats.processed++

		if batch.ValueSize() >= ethdb.IdealBatchSize {
			if err := batch.Write(); err != nil {
				return 0, err
			}
			bytes += batch.ValueSize()
			batch.Reset()
		}
	}
	if batch.ValueSize() > 0 {
		bytes += batch.ValueSize()
		if err := batch.Write(); err != nil {
			return 0, err
		}
	}

	// Update the head fast sync block if better
	head := blockChain[len(blockChain)-1]

	context := []interface{}{
		"count", stats.processed, "elapsed", common.PrettyDuration(time.Since(start)),
		"number", head.NumberU64(), "hash", head.Hash(), "age", common.PrettyAge(time.Unix(int64(head.Time()), 0)),
		"size", common.StorageSize(bytes),
	}
	if stats.ignored > 0 {
		context = append(context, []interface{}{"ignored", stats.ignored}...)
	}
	log.Info("Imported new block receipts", context...)

	return 0, nil
}

// WriteBlockWithoutState writes only the block and its metadata to the database,
// but does not write any state. This is used to construct competing side forks
// up to the point where they exceed the canonical total difficulty.
func (m *MinorBlockChain) WriteBlockWithoutState(block types.IBlock) (err error) {
	m.wg.Add(1)
	defer m.wg.Done()

	rawdb.WriteMinorBlock(m.db, block.(*types.MinorBlock))
	m.blockCache.Add(block.Hash(), block.(*types.MinorBlock))

	return nil
}

// WriteBlockWithState writes the block and all associated state to the database.
func (m *MinorBlockChain) WriteBlockWithState(block *types.MinorBlock, receipts []*types.Receipt, state *state.StateDB, xShardList []*types.CrossShardTransactionDeposit, updateTip bool) (status WriteStatus, err error) {
	m.wg.Add(1)
	defer m.wg.Done()

	// Make sure no inconsistent state is leaked during insertion
	m.mu.Lock()
	defer m.mu.Unlock()

	currentBlock := m.CurrentBlock()

	if err := m.putMinorBlock(block, xShardList); err != nil {
		return NonStatTy, err
	}

	root, err := state.Commit(true)
	if err != nil {
		return NonStatTy, err
	}
	triedb := m.stateCache.TrieDB()

	// If we're running an archive node, always flush
	if m.cacheConfig.Disabled {
		if err := triedb.Commit(root, false); err != nil {
			return NonStatTy, err
		}
	} else {
		// Full but not archive node, do proper garbage collection
		triedb.Reference(root, common.Hash{}) // metadata reference to keep trie alive
		m.triegc.Push(root, -int64(block.NumberU64()))

		if current := block.NumberU64(); current > triesInMemory {
			// If we exceeded our memory allowance, flush matured singleton nodes to disk
			var (
				nodes, imgs = triedb.Size()
				limit       = common.StorageSize(m.cacheConfig.TrieDirtyLimit) * 1024 * 1024
			)
			if nodes > limit || imgs > 4*1024*1024 {
				triedb.Cap(limit - ethdb.IdealBatchSize)
			}
			// Find the next state trie we need to commit
			header := m.GetHeaderByNumber(current - triesInMemory)
			preBlockInterface := m.GetBlockByNumber(current - triesInMemory)
			if qkcCommon.IsNil(preBlockInterface) {
				log.Error("minorBlock not found", "height", current-triesInMemory)
			}
			preBlock := preBlockInterface.(*types.MinorBlock)
			chosen := header.NumberU64()

			// If we exceeded out time allowance, flush an entire trie to disk
			if m.gcproc > m.cacheConfig.TrieTimeLimit {
				// If we're exceeding limits but haven't reached a large enough memory gap,
				// warn the user that the system is becoming unstable.
				if chosen < lastWrite+triesInMemory && m.gcproc >= 2*m.cacheConfig.TrieTimeLimit {
					log.Info("State in memory for too long, committing", "time", m.gcproc, "allowance", m.cacheConfig.TrieTimeLimit, "optimum", float64(chosen-lastWrite)/triesInMemory)
				}
				// Flush an entire trie and restart the counters
				triedb.Commit(preBlock.GetMetaData().Root, true)
				lastWrite = chosen
				m.gcproc = 0
			}
			// Garbage collect anything below our required write retention
			for !m.triegc.Empty() {
				root, number := m.triegc.Pop()
				if uint64(-number) > chosen {
					m.triegc.Push(root, number)
					break
				}
				triedb.Dereference(root.(common.Hash))
			}
		}
	}

	// Write other block data using a batch.
	batch := m.db.NewBatch()
	rawdb.WriteReceipts(batch, block.Hash(), receipts)

	if updateTip {
		// Reorganise the chain if the parent is not the head block
		if block.ParentHash() != currentBlock.Hash() {
			if err := m.reorg(currentBlock, block); err != nil {
				return NonStatTy, err
			}
		}
		// Write the positional metadata for transaction/receipt lookups and preimages
		if err := m.putTxIndexFromBlock(batch, block); err != nil {
			panic(err)
		}
		rawdb.WritePreimages(batch, state.Preimages())
		status = CanonStatTy

	} else {
		status = SideStatTy
	}

	if err := batch.Write(); err != nil {
		return NonStatTy, err
	}

	// Set new head.
	if status == CanonStatTy {
		m.insert(block)
	}
	m.CommitMinorBlockByHash(block.Hash())
	m.futureBlocks.Remove(block.Hash())
	return status, nil
}

// addFutureBlock checks if the block is within the max allowed window to get
// accepted for future processing, and returns an error if the block is too far
// ahead and was not added.
func (m *MinorBlockChain) addFutureBlock(block types.IBlock) error {
	max := big.NewInt(time.Now().Unix() + maxTimeFutureBlocks)
	if block.Time() > max.Uint64() {
		return fmt.Errorf("future block timestamp %v > allowed %v", block.Time(), max)
	}
	m.futureBlocks.Add(block.Hash(), block)
	return nil
}

// InsertChain attempts to insert the given batch of blocks in to the canonical
// chain or, otherwise, create a fork. If an error is returned it will return
// the index number of the failing block as well an error describing what went
// wrong.
//
// After insertion is done, all accumulated events will be fired.
func (m *MinorBlockChain) InsertChain(chain []types.IBlock, isCheckDB bool) (int, error) {
	n, _, err := m.InsertChainForDeposits(chain, isCheckDB)
	return n, err
}

// InsertChainForDeposits also return cross-shard transaction deposits in addition
// to content returned from `InsertChain`.
func (m *MinorBlockChain) InsertChainForDeposits(chain []types.IBlock, isCheckDB bool) (int, [][]*types.CrossShardTransactionDeposit, error) {
	// Sanity check that we have something meaningful to import
	if len(chain) == 0 {
		return 0, nil, nil
	}
	// Do a sanity check that the provided chain is actually ordered and linked
	for i := 1; i < len(chain); i++ {
		if chain[i].NumberU64() != chain[i-1].NumberU64()+1 || chain[i].ParentHash() != chain[i-1].Hash() {
			// Chain broke ancestry, log a message (programming error) and skip insertion
			log.Error("Non contiguous block insert", "number", chain[i].NumberU64(), "hash", chain[i].Hash(),
				"parent", chain[i].ParentHash(), "prevnumber", chain[i-1].NumberU64(), "prevhash", chain[i-1].Hash())

			return 0, nil, fmt.Errorf("non contiguous insert: item %d is #%d [%x…], item %d is #%d [%x…] (parent [%x…])", i-1, chain[i-1].NumberU64(),
				chain[i-1].Hash().Bytes()[:4], i, chain[i].NumberU64(), chain[i].Hash().Bytes()[:4], chain[i].ParentHash().Bytes()[:4])
		}
	}
	// Pre-checks passed, start the full block imports
	m.wg.Add(1)
	m.chainmu.Lock()
	n, events, logs, xShardList, err := m.insertChain(chain, true, isCheckDB)
	m.chainmu.Unlock()
	m.wg.Done()

	m.PostChainEvents(events, logs)
	confirmed := m.confirmedHeaderTip
	if confirmed == nil {
		log.Warn("confirmed is nil")
	} else {
		log.Debug(m.logInfo, "tip", m.CurrentBlock().NumberU64(), "tipHash", m.CurrentBlock().Hash().String(), "to add", chain[0].NumberU64(), "hash", chain[0].NumberU64(), "confirmed", confirmed.Number)
	}

	return n, xShardList, err
}

// insertChain is the internal implementation of insertChain, which assumes that
// 1) chains are contiguous, and 2) The chain mutex is held.
//
// This method is split out so that import batches that require re-injecting
// historical blocks can do so without releasing the lock, which could lead to
// racey behaviour. If a sidechain import is in progress, and the historic state
// is imported, but then new canon-head is added before the actual sidechain
// completes, then the historic state could be pruned again
func (m *MinorBlockChain) insertChain(chain []types.IBlock, verifySeals bool, isCheckDB bool) (int, []interface{}, []*types.Log, [][]*types.CrossShardTransactionDeposit, error) {
	xShardList := make([][]*types.CrossShardTransactionDeposit, 0)
	// If the chain is terminating, don't even bother starting u
	if atomic.LoadInt32(&m.procInterrupt) == 1 {
		return 0, nil, nil, xShardList, nil
	}

	headersToRecover := make([]*types.MinorBlock, 0)
	for _, v := range chain {
		headersToRecover = append(headersToRecover, v.(*types.MinorBlock))
	}
	// Start a parallel signature recovery (signer will fluke on fork transition, minimal perf loss)
	senderCacher.recoverFromBlocks(types.MakeSigner(uint32(m.Config().NetworkID)), headersToRecover)

	// A queued approach to delivering events. This is generally
	// faster than direct delivery and requires much less mutex
	// acquiring.
	var (
		stats         = insertStats{startTime: mclock.Now()}
		events        = make([]interface{}, 0, len(chain))
		lastCanon     types.IBlock
		coalescedLogs []*types.Log
	)
	// Start the parallel header verifier
	headers := make([]types.IHeader, len(chain))
	seals := make([]bool, len(chain))

	for i, block := range chain {
		headers[i] = block.IHeader()
		seals[i] = verifySeals
	}
	abort, results := m.engine.VerifyHeaders(m, headers, seals)
	defer close(abort)

	// Peek the error for the first block to decide the directing import logic
	it := newInsertIterator(chain, results, m.Validator(), isCheckDB)
	block, err := it.next()
	switch {
	// First block is pruned, insert as sidechain and reorg only if TD grows enough
	case err == ErrPrunedAncestor:
		return m.insertSidechain(it, isCheckDB)

	// First block is future, shove it (and all children) to the future queue (unknown ancestor)
	case err == ErrFutureBlock || (err == ErrUnknownAncestor && m.futureBlocks.Contains(it.first().ParentHash())):
		for block != nil && (it.index == 0 || err == ErrUnknownAncestor) {
			if err := m.addFutureBlock(block); err != nil {
				return it.index, events, coalescedLogs, xShardList, err
			}
			block, err = it.next()
		}
		stats.queued += it.processed()
		stats.ignored += it.remaining()

		// If there are any still remaining, mark as ignored
		return it.index, events, coalescedLogs, xShardList, err

	// First block (and state) is known
	//   1. We did a roll-back, and should now do a re-import
	//   2. The block is stored as a sidechain, and is lying about it's stateroot, and passes a stateroot
	// 	    from the canonical chain, which has not been verified.
	case err == ErrKnownBlock:
		// Skip all known blocks that behind us
		current := m.CurrentBlock().NumberU64()

		for block != nil && err == ErrKnownBlock && current >= block.NumberU64() {
			stats.ignored++
			block, err = it.next()
		}
		// Falls through to the block import
		xShardList = append(xShardList, make([]*types.CrossShardTransactionDeposit, 0))
	// Some other error occurred, abort
	case err != nil:
		stats.ignored += len(it.chain)
		m.reportBlock(block, nil, err)
		return it.index, events, coalescedLogs, xShardList, err
	}

	// No validation errors for the first block (or chain prefix skipped)
	for ; !qkcCommon.IsNil(block) && err == nil; block, err = it.next() {
		mBlock := block.(*types.MinorBlock)
		// If the chain is terminating, stop processing blocks
		if atomic.LoadInt32(&m.procInterrupt) == 1 {
			log.Debug("Premature abort during blocks processing")
			break
		}
		// Retrieve the parent block and it's state to execute on top
		start := time.Now()
		parent := it.previous()
		if parent == nil {
			parent = m.GetBlock(mBlock.ParentHash())
		}
		if qkcCommon.IsNil(parent) {
			return it.index, events, coalescedLogs, xShardList, err
		}
		// Process block using the parent state as reference point.

		state, receipts, logs, usedGas, xShardReceiveTxList, err := m.runBlock(mBlock)
		if err != nil {
			m.reportBlock(block, receipts, err)
			return it.index, events, coalescedLogs, xShardList, err
		}
		// Validate the state using the default validator
		if err := m.Validator().ValidateState(block, parent, state, receipts, usedGas); err != nil {
			m.reportBlock(block, receipts, err)
			return it.index, events, coalescedLogs, xShardList, err
		}
		proctime := time.Since(start)

		if isCheckDB {
			xShardList = append(xShardList, state.GetXShardList())
			return 0, events, coalescedLogs, xShardList, nil
		}
		updateTip, err := m.updateTip(state, mBlock)
		if err != nil {
			return it.index, events, coalescedLogs, xShardList, err
		}
		// Write the block to the chain and get the status.
		status, err := m.WriteBlockWithState(mBlock, receipts, state, xShardReceiveTxList, updateTip)
		if err != nil {
			return it.index, events, coalescedLogs, xShardList, err
		}
		switch status {
		case CanonStatTy:
			log.Debug("Inserted new block", "number", mBlock.NumberU64(), "hash", mBlock.Hash(),
				"txs", len(mBlock.GetTransactions()), "gas", mBlock.GetMetaData().GasUsed.Value.Uint64(),
				"elapsed", common.PrettyDuration(time.Since(start)),
				"root", mBlock.GetMetaData().Root)

			coalescedLogs = append(coalescedLogs, logs...)
			events = append(events, MinorChainEvent{mBlock, mBlock.Hash(), logs})
			lastCanon = block

			// Only count canonical blocks for GC processing time
			m.gcproc += proctime

		case SideStatTy:
			log.Debug("Inserted forked block", "number", mBlock.NumberU64(), "hash", mBlock.Hash(),
				"diff", mBlock.Difficulty(), "elapsed", common.PrettyDuration(time.Since(start)),
				"txs", len(mBlock.GetTransactions()), "gas", mBlock.GetMetaData().GasUsed,
				"root", mBlock.GetMetaData().Root)
			events = append(events, MinorChainSideEvent{mBlock})
		}
		stats.processed++
		stats.usedGas += usedGas

		//	stats.report(chain, it.index)
		xShardList = append(xShardList, state.GetXShardList())
	}
	// Any blocks remaining here? The only ones we care about are the future ones
	if !qkcCommon.IsNil(block) && err == ErrFutureBlock {
		if err := m.addFutureBlock(block); err != nil {
			return it.index, events, coalescedLogs, xShardList, err
		}
		block, err = it.next()

		for ; block != nil && err == ErrUnknownAncestor; block, err = it.next() {
			if err := m.addFutureBlock(block); err != nil {
				return it.index, events, coalescedLogs, xShardList, err
			}
			stats.queued++
		}
	}
	stats.ignored += it.remaining()

	// Append a single chain head event if we've progressed the chain
	if lastCanon != nil && m.CurrentBlock().Hash() == lastCanon.Hash() {
		events = append(events, MinorChainHeadEvent{lastCanon.(*types.MinorBlock)})
	}
	return it.index, events, coalescedLogs, xShardList, err
}

// insertSidechain is called when an import batch hits upon a pruned ancestor
// error, which happens when a sidechain with a sufficiently old fork-block is
// found.
//
// The method writes all (header-and-body-valid) blocks to disk, then tries to
// switch over to the new chain if the TD exceeded the current chain.
func (m *MinorBlockChain) insertSidechain(it *insertIterator, isCheckDB bool) (int, []interface{}, []*types.Log, [][]*types.CrossShardTransactionDeposit, error) {
	var (
		current      = m.CurrentBlock().NumberU64()
		externHeight = uint64(0)
	)
	// The first sidechain block error is already verified to be ErrPrunedAncestor.
	// Since we don't import them here, we expect ErrUnknownAncestor for the remaining
	// ones. Any other errors means that the block is invalid, and should not be written
	// to disk.
	block, err := it.current(), ErrPrunedAncestor
	for ; !qkcCommon.IsNil(block) && (err == ErrPrunedAncestor); block, err = it.next() {
		// Check the canonical state root for that number
		if number := block.NumberU64(); current >= number {
			if canonical := m.GetBlockByNumber(number); !qkcCommon.IsNil(canonical) && canonical.(*types.MinorBlock).GetMetaData().Root == block.(*types.MinorBlock).GetMetaData().Root {
				// This is most likely a shadow-state attack. When a fork is imported into the
				// database, and it eventually reaches a block height which is not pruned, we
				// just found that the state already exist! This means that the sidechain block
				// refers to a state which already exists in our canon chain.
				//
				// If left unchecked, we would now proceed importing the blocks, without actually
				// having verified the state of the previous blocks.
				log.Warn("Sidechain ghost-state attack detected", "number", block.NumberU64(), "sideroot", block.(*types.MinorBlock).GetMetaData().Root, "canonroot", canonical.(*types.MinorBlock).GetMetaData().Root)

				// If someone legitimately side-mines blocks, they would still be imported as usual. However,
				// we cannot risk writing unverified blocks to disk when they obviously target the pruning
				// mechanism.
				return it.index, nil, nil, nil, errors.New("sidechain ghost-state attack")
			}
		}
		externHeight = block.NumberU64()
		if !m.HasBlock(block.Hash()) {
			start := time.Now()
			if err := m.WriteBlockWithoutState(block); err != nil {
				return it.index, nil, nil, nil, err
			}
			m.CommitMinorBlockByHash(block.Hash())
			log.Debug("Inserted sidechain block", "number", block.NumberU64(), "hash", block.Hash(),
				"diff", block.IHeader().GetDifficulty(), "elapsed", common.PrettyDuration(time.Since(start)),
				"txs", len(block.(*types.MinorBlock).GetTransactions()), "gas", block.(*types.MinorBlock).GetMetaData().GasUsed,
				"root", block.(*types.MinorBlock).GetMetaData().Root)
		}
	}
	// At this point, we've written all sidechain blocks to database. Loop ended
	// either on some other error or all were processed. If there was some other
	// error, we can ignore the rest of those blocks.
	//

	if current > externHeight {
		log.Info("Sidechain written to disk", "start", it.first().NumberU64(), "end", it.previous().NumberU64(), "sidetd", externHeight, "localtd", current)
		return it.index, nil, nil, nil, err
	}
	// Gather all the sidechain hashes (full blocks may be memory heavy)
	var (
		hashes  []common.Hash
		numbers []uint64
	)
	parent := m.GetBlock(it.previous().Hash())
	for !qkcCommon.IsNil(parent) && !m.HasState(parent.(*types.MinorBlock).GetMetaData().Root) {
		hashes = append(hashes, parent.Hash())
		numbers = append(numbers, parent.NumberU64())

		parent = m.GetBlock(parent.ParentHash())
	}
	if parent == nil {
		return it.index, nil, nil, nil, errors.New("missing parent")
	}
	// Import all the pruned blocks to make the state available
	var (
		blocks []types.IBlock
		memory common.StorageSize
	)
	for i := len(hashes) - 1; i >= 0; i-- {
		// Append the next block to our batch
		block := m.GetBlock(hashes[i])
		blocks = append(blocks, block)
		memory += block.GetSize()

		// If memory use grew too large, import and continue. Sadly we need to discard
		// all raised events and logs from notifications since we're too heavy on the
		// memory here.
		if len(blocks) >= 2048 || memory > 64*1024*1024 {
			log.Info("Importing heavy sidechain segment", "blocks", len(blocks), "start", blocks[0].NumberU64(), "end", block.NumberU64())
			if _, _, _, _, err := m.insertChain(blocks, false, isCheckDB); err != nil {
				return 0, nil, nil, nil, err
			}
			blocks, memory = blocks[:0], 0

			// If the chain is terminating, stop processing blocks
			if atomic.LoadInt32(&m.procInterrupt) == 1 {
				log.Debug("Premature abort during blocks processing")
				return 0, nil, nil, nil, nil
			}
		}
	}
	if len(blocks) > 0 {
		log.Info("Importing sidechain segment", "start", blocks[0].NumberU64(), "end", blocks[len(blocks)-1].NumberU64())
		return m.insertChain(blocks, false, isCheckDB)
	}
	return 0, nil, nil, nil, nil
}

// reorgs takes two blocks, an old chain and a new chain and will reconstruct the blocks and inserts them
// to be part of the new canonical chain and accumulates potential missing transactions and post an
// event about them
func (m *MinorBlockChain) reorg(oldBlock, newBlock types.IBlock) error {
	if qkcCommon.IsNil(oldBlock) || qkcCommon.IsNil(newBlock) {
		return errors.New("reorg err:block is nil")
	}
	var (
		newChain    []types.IBlock
		oldChain    []types.IBlock
		commonBlock types.IBlock
		deletedLogs []*types.Log
		// collectLogs collects the logs that were generated during the
		// processing of the block that corresponds with the given hash.
		// These logs are later announced as deleted.
		collectLogs = func(hash common.Hash) {
			// Coalesce logs and set 'Removed'.
			number := m.hc.GetBlockNumber(hash)
			if number == nil {
				return
			}
			receipts := rawdb.ReadReceipts(m.db, hash)
			for _, receipt := range receipts {
				for _, log := range receipt.Logs {
					del := *log
					del.Removed = true
					deletedLogs = append(deletedLogs, &del)
				}
			}
		}
	)

	// first reduce whoever is higher bound
	if oldBlock.NumberU64() > newBlock.NumberU64() {
		// reduce old chain
		for ; oldBlock != nil && oldBlock.NumberU64() != newBlock.NumberU64(); oldBlock = m.GetBlock(oldBlock.ParentHash()) {
			oldChain = append(oldChain, oldBlock)
			collectLogs(oldBlock.Hash())
		}
	} else {
		// reduce new chain and append new chain blocks for inserting later on
		for ; newBlock != nil && newBlock.NumberU64() != oldBlock.NumberU64(); newBlock = m.GetBlock(newBlock.ParentHash()) {
			newChain = append(newChain, newBlock)
		}
	}
	if qkcCommon.IsNil(oldBlock) {
		return fmt.Errorf("Invalid old chain")
	}
	if qkcCommon.IsNil(newBlock) {
		return fmt.Errorf("Invalid new chain")
	}

	for {
		if oldBlock.Hash() == newBlock.Hash() {
			commonBlock = oldBlock
			break
		}

		oldChain = append(oldChain, oldBlock)
		newChain = append(newChain, newBlock)
		collectLogs(oldBlock.Hash())
		if oldBlock.NumberU64() == 0 || newBlock.NumberU64() == 0 { //revert genesisBlock: no commonBlock
			log.Warn("reorg", "ready to revert genesis? oldBlock", oldBlock.Hash().String(),
				"newBlock", newBlock.Hash().String(), "currBlock", m.CurrentBlock().Hash().String())
			break
		}

		oldBlock, newBlock = m.GetBlock(oldBlock.ParentHash()), m.GetBlock(newBlock.ParentHash())
		if qkcCommon.IsNil(oldBlock) {
			return fmt.Errorf("Invalid old chain")
		}
		if qkcCommon.IsNil(newBlock) {
			return fmt.Errorf("Invalid new chain")
		}
	}
	// Ensure the user sees large reorgs
	if len(oldChain) > 0 && len(newChain) > 0 {
		logFn := log.Debug
		if len(oldChain) > 63 {
			logFn = log.Warn
		}
		if commonBlock != nil {
			logFn("Chain split detected", "number", commonBlock.NumberU64(), "hash", commonBlock.Hash(),
				"drop", len(oldChain), "dropfrom", oldChain[0].Hash(), "add", len(newChain), "addfrom", newChain[0].Hash())
		} else {
			log.Warn("ChainRevert genesis", "drop", len(oldChain), "dropfrom", oldChain[0].Hash(), "add", len(newChain), "addfrom", newChain[0].Hash())
		}

	} else {
		// we support reorg block from same chain,because we should delete and add tx index
		log.Warn("reorg", "same chain oldBlock", oldBlock.NumberU64(), "oldBlock.Hash", oldBlock.Hash().String(),
			"newBlock", newBlock.NumberU64(), "newBlock's hash", newBlock.Hash().String())
		if err := m.setHead(newBlock.NumberU64()); err != nil {
			return err
		}
	}

	// When transactions get deleted from the database that means the
	// receipts that were created in the fork must also be deleted
	batch := m.db.NewBatch()
	for i := len(oldChain) - 1; i >= 0; i-- {
		if err := m.removeTxIndexFromBlock(batch, oldChain[i].(*types.MinorBlock)); err != nil {
			return err
		}
	}

	batch.Write()

	// Insert the new chain, taking care of the proper incremental order
	for i := len(newChain) - 1; i >= 0; i-- {
		// insert the block in the canonical way, re-writing history
		m.insert(newChain[i].(*types.MinorBlock))
		// write lookup entries for hash based transaction/receipt searches
		if err := m.putTxIndexFromBlock(m.db, newChain[i]); err != nil {
			return err
		}
	}

	if len(deletedLogs) > 0 {
		var logs [][]*types.Log
		logs = append(logs, deletedLogs)
		m.subLogsFeed.Send(LoglistEvent{Logs: logs, IsRemoved: true})
	}

	if len(oldChain) > 0 {
		for _, iB := range oldChain {
			m.subLogsFeed.Send(LoglistEvent{Logs: m.GetLogs(iB.Hash()), IsRemoved: true})
		}
		go func() {
			for _, block := range oldChain {
				m.chainSideFeed.Send(MinorChainSideEvent{Block: block.(*types.MinorBlock)})
			}
		}()
	}
	for _, iB := range newChain {
		m.subLogsFeed.Send(LoglistEvent{Logs: m.GetLogs(iB.Hash()), IsRemoved: false})
	}

	return nil
}

// PostChainEvents iterates over the events generated by a chain insertion and
// posts them into the event feed.
// TODO: Should not expose PostChainEvents. The chain events should be posted in WriteBlock.
func (m *MinorBlockChain) PostChainEvents(events []interface{}, logs []*types.Log) {
	// post event logs for further processing
	if logs != nil {
		m.logsFeed.Send(logs)
	}
	for _, event := range events {
		switch ev := event.(type) {
		case MinorChainEvent:
			m.chainFeed.Send(ev)

		case MinorChainHeadEvent:
			m.chainHeadFeed.Send(ev)

		case MinorChainSideEvent:
			m.chainSideFeed.Send(ev)
		}
	}
}

func (m *MinorBlockChain) update() {
	futureTimer := time.NewTicker(5 * time.Second)
	defer futureTimer.Stop()
	for {
		select {
		case <-futureTimer.C:
			m.procFutureBlocks()
		case <-m.quit:
			return
		}
	}
}

// reportBlock logs a bad block error.
func (m *MinorBlockChain) reportBlock(block types.IBlock, receipts types.Receipts, err error) {

	var receiptString string
	for i, receipt := range receipts {
		receiptString += fmt.Sprintf("\t %d: cumulative: %v gas: %v contract: %v status: %v tx: %v logs: %v bloom: %x state: %x\n",
			i, receipt.CumulativeGasUsed, receipt.GasUsed, hex.EncodeToString(receipt.ContractAddress.Bytes()),
			receipt.Status, receipt.TxHash.Hex(), receipt.Logs, receipt.Bloom, receipt.PostState)
	}
	log.Error(fmt.Sprintf(`
########## BAD BLOCK #########
Chain config: %v

Number: %v
Hash: 0x%x
%v

Error: %v
##############################
`, m.ethChainConfig, block.NumberU64(), block.Hash(), receiptString, err))
}

// InsertHeaderChain attempts to insert the given header chain in to the local
// chain, possibly creating a reorg. If an error is returned, it will return the
// index number of the failing header as well an error describing what went wrong.
//
// The verify parameter can be used to fine tune whether nonce verification
// should be done or not. The reason behind the optional check is because some
// of the header retrieval mechanisms already need to verify nonces, as well as
// because nonces can be verified sparsely, not needing to check each.
func (m *MinorBlockChain) InsertHeaderChain(chain []types.IHeader, checkFreq int) (int, error) {
	start := time.Now()

	headers := make([]*types.MinorBlockHeader, 0)
	for k, v := range chain {
		if qkcCommon.IsNil(v) {
			return k, errors.New("InsertHeaderChain err:header is nil")
		}
		headers = append(headers, v.(*types.MinorBlockHeader))
	}
	if i, err := m.hc.ValidateHeaderChain(headers, checkFreq); err != nil {
		return i, err
	}

	// Make sure only one thread manipulates the chain at once
	m.chainmu.Lock()
	defer m.chainmu.Unlock()

	m.wg.Add(1)
	defer m.wg.Done()

	whFunc := func(header *types.MinorBlockHeader) error {
		m.mu.Lock()
		defer m.mu.Unlock()

		_, err := m.hc.WriteHeader(header)
		return err
	}

	return m.hc.InsertHeaderChain(headers, whFunc, start)
}

// writeHeader writes a header into the local chain, given that its parent is
// already known. If the total difficulty of the newly inserted header becomes
// greater than the current known TD, the canonical chain is re-routed.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing Headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (m *MinorBlockChain) writeHeader(header *types.MinorBlockHeader) error {
	m.wg.Add(1)
	defer m.wg.Done()

	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.hc.WriteHeader(header)
	return err
}

// CurrentHeader retrieves the current head header of the canonical chain. The
// header is retrieved from the HeaderChain's internal cache.
func (m *MinorBlockChain) CurrentHeader() types.IHeader {
	return m.CurrentBlock().Header()
}

// GetHeader retrieves a block header from the database by hash and number,
// caching it if found.
func (m *MinorBlockChain) GetHeader(hash common.Hash) types.IHeader {
	return m.hc.GetHeader(hash)
}

// GetHeaderByHash retrieves a block header from the database by hash, caching it if
// found.
func (m *MinorBlockChain) GetHeaderByHash(hash common.Hash) types.IHeader {
	return m.hc.GetHeaderByHash(hash)
}

// HasHeader checks if a block header is present in the database or not, caching
// it if present.
func (m *MinorBlockChain) HasHeader(hash common.Hash, number uint64) bool {
	return m.hc.HasHeader(hash, number)
}

// GetBlockHashesFromHash retrieves a number of block hashes starting at a given
// hash, fetching towards the genesis block.
func (m *MinorBlockChain) GetBlockHashesFromHash(hash common.Hash, max uint64) []common.Hash {
	return m.hc.GetBlockHashesFromHash(hash, max)
}

// GetAncestor retrieves the Nth ancestor of a given block. It assumes that either the given block or
// a close ancestor of it is canonical. maxNonCanonical points to a downwards counter limiting the
// number of blocks to be individually checked before we reach the canonical chain.
//
// Note: ancestor == 0 returns the same block, 1 returns its parent and so on.
func (m *MinorBlockChain) GetAncestor(hash common.Hash, number, ancestor uint64, maxNonCanonical *uint64) (common.Hash, uint64) {
	m.chainmu.Lock()
	defer m.chainmu.Unlock()

	return m.hc.GetAncestor(hash, number, ancestor, maxNonCanonical)
}

// GetHeaderByNumber retrieves a block header from the database by number,
// caching it (associated with its hash) if found.
func (m *MinorBlockChain) GetHeaderByNumber(number uint64) types.IHeader {
	return m.hc.GetHeaderByNumber(number)
}

// Config retrieves the blockchain's chain configuration.
func (m *MinorBlockChain) Config() *config.QuarkChainConfig { return m.clusterConfig.Quarkchain }

// Engine retrieves the blockchain's consensus engine.
func (m *MinorBlockChain) Engine() consensus.Engine { return m.engine }

// SubscribeChainEvent registers a subscription of ChainEvent.
func (m *MinorBlockChain) SubscribeChainEvent(ch chan<- MinorChainEvent) event.Subscription {
	return m.scope.Track(m.chainFeed.Subscribe(ch))
}

// SubscribeChainHeadEvent registers a subscription of ChainHeadEvent.
func (m *MinorBlockChain) SubscribeChainHeadEvent(ch chan<- MinorChainHeadEvent) event.Subscription {
	return m.scope.Track(m.chainHeadFeed.Subscribe(ch))
}

// SubscribeChainSideEvent registers a subscription of ChainSideEvent.
func (m *MinorBlockChain) SubscribeChainSideEvent(ch chan<- MinorChainSideEvent) event.Subscription {
	return m.scope.Track(m.chainSideFeed.Subscribe(ch))
}

// SubscribeLogsEvent registers a subscription of []*types.Log.
func (m *MinorBlockChain) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return m.scope.Track(m.logsFeed.Subscribe(ch))
}

// SubscribeLogsEvent registers a subscription of LoglistEvent
func (m *MinorBlockChain) SubReorgLogsEvent(ch chan<- LoglistEvent) event.Subscription {
	return m.scope.Track(m.subLogsFeed.Subscribe(ch))
}

func (m *MinorBlockChain) SubscribeNewTxsEvent(ch chan<- NewTxsEvent) event.Subscription {
	return m.txPool.SubscribeNewTxsEvent(ch)
}

func (m *MinorBlockChain) getRootBlockHeaderByHash(hash common.Hash) *types.RootBlockHeader {
	if data, ok := m.rootBlockCache.Get(hash); ok {
		return data.(*types.RootBlock).Header()
	}
	data := rawdb.ReadRootBlock(m.db, hash)
	if data != nil {
		m.rootBlockCache.Add(hash, data)
		return data.Header()
	}

	return nil
}

// GetRootBlockByHash get rootBlock by hash in minorBlockChain
func (m *MinorBlockChain) GetRootBlockByHash(hash common.Hash) *types.RootBlock {
	if data, ok := m.rootBlockCache.Get(hash); ok {
		return data.(*types.RootBlock)
	}
	data := rawdb.ReadRootBlock(m.db, hash)
	if data != nil {
		m.rootBlockCache.Add(hash, data)
		return data
	}
	return nil
}

func (m *MinorBlockChain) GetRootBlockHeaderByHeight(h common.Hash, height uint64) *types.RootBlockHeader {
	rHeader := m.getRootBlockHeaderByHash(h)
	if rHeader == nil || height > rHeader.NumberU64() {
		return nil
	}
	for height != rHeader.NumberU64() {
		if rHeader = m.getRootBlockHeaderByHash(rHeader.ParentHash); rHeader == nil {
			log.Crit("bug should fix", "GetRootBlockHeaderByHeight rootBlock is nil hash", rHeader.ParentHash, "currNumber", rHeader.NumberU64(), "currHash", rHeader.Hash().String())
		}
	}
	return rHeader
}

func (m *MinorBlockChain) ContainRootBlockByHash(h common.Hash) bool {
	if m.getRootBlockHeaderByHash(h) == nil {
		return false
	}
	return true
}

func (m *MinorBlockChain) GetGenesisToken() uint64 {
	return m.clusterConfig.Quarkchain.GetDefaultChainTokenID()
}

func (m *MinorBlockChain) GetGenesisRootHeight() uint32 {
	return m.clusterConfig.Quarkchain.GetGenesisRootHeight(m.branch.Value)
}

func (m *MinorBlockChain) getEvmStateByBlock(block *types.MinorBlock) (*state.StateDB, error) {
	if bytes.Equal(block.Hash().Bytes(), m.CurrentBlock().Hash().Bytes()) {
		m.mu.Lock()
		stateDB := m.currentEvmState.Copy()
		m.mu.Unlock()
		return stateDB, nil
	}
	return m.StateAt(block.GetMetaData().Root)
}

func (m *MinorBlockChain) GetRootChainStakes(coinbase account.Recipient, lastMinor common.Hash) (*big.Int,
	*account.Recipient, error) {

	if m.branch.GetChainID() != 0 || m.branch.GetShardID() != 0 {
		return nil, nil, errors.New("not chain 0 shard 0")
	}

	last := m.GetMinorBlock(lastMinor)
	if last == nil {
		panic(fmt.Sprintf("block not found: %x", lastMinor))
	}
	evmState, err := m.getEvmStateByBlock(last)
	if err != nil {
		return nil, nil, err
	}
	evmState.SetGasUsed(big.NewInt(0))
	contractAddress := vm.SystemContracts[vm.ROOT_CHAIN_POSW].Address()
	code := evmState.GetCode(contractAddress)
	if code == nil {
		return nil, nil, ErrPoswOnRootChainIsNotFound
	}
	codeHash := crypto.Keccak256Hash(code)
	//have to make sure the code is expected
	if bytes.Compare(codeHash[:], m.clusterConfig.Quarkchain.RootChainPoSWContractBytecodeHash[:]) != 0 {
		return nil, nil, errors.New("PoSW-on-root-chain contract is invalid")
	}
	//call the contract's 'getLockedStakes' function
	mockSender := account.Recipient{}
	data := common.Hex2Bytes("fd8c4646000000000000000000000000")
	data = append(data[:], coinbase[:]...)
	nonce := evmState.GetNonce(mockSender)
	toFullShardKey := uint32(0)
	msg := types.NewMessage(mockSender, &contractAddress, nonce, new(big.Int), 1000000, new(big.Int), data,
		false, 0, &toFullShardKey, m.GetGenesisToken(), m.GetGenesisToken())
	context := NewEVMContext(msg, last.Header(), m)
	evmState.SetQuarkChainConfig(m.clusterConfig.Quarkchain)
	vmenv := vm.NewEVM(context, evmState, m.ethChainConfig, *m.GetVMConfig())
	gp := new(GasPool).AddGas(evmState.GetGasLimit().Uint64())
	output, _, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil || output == nil || failed {
		return nil, nil, err
	}
	stake := new(big.Int).SetBytes(output[:32])
	signer := account.BytesToIdentityRecipient(output[32+12:])
	return stake, &signer, nil
}
