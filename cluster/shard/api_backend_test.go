package shard

import (
	"errors"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/miner"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

// raceIterations is how many times the *UnderRace tests repeat the interleaving.
// A single run rarely lands in the marker-deletion window, so the tests loop and
// MUST be run with -race to be meaningful.
const raceIterations = 100

// ── stubs ──────────────────────────────────────────────────────────────────

// stubConnManager is a no-op ConnManager for tests.
// AddBlockListForSync uses BatchBroadcastXshardTxList and SendMinorBlockHeaderListToMaster;
// AddMinorBlock uses BroadcastXshardTxList and SendMinorBlockHeaderToMaster.
// All return nil.
type stubConnManager struct{}

func (s *stubConnManager) BroadcastXshardTxList(_ *types.MinorBlock, _ []*types.CrossShardTransactionDeposit, _ uint32) error {
	return nil
}
func (s *stubConnManager) SendMinorBlockHeaderToMaster(_ *rpc.AddMinorBlockHeaderRequest) error {
	return nil
}
func (s *stubConnManager) SendMinorBlockHeaderListToMaster(_ *rpc.AddMinorBlockHeaderListRequest) error {
	return nil
}
func (s *stubConnManager) BatchBroadcastXshardTxList(_ map[common.Hash]*XshardListTuple, _ account.Branch) error {
	return nil
}
func (s *stubConnManager) BroadcastNewTip(_ []*types.MinorBlockHeader, _ *types.RootBlockHeader, _ uint32) error {
	return nil
}
func (s *stubConnManager) BroadcastTransactions(_ string, _ uint32, _ []*types.Transaction) error {
	return nil
}
func (s *stubConnManager) BroadcastMinorBlock(_ string, _ *types.MinorBlock) error { return nil }
func (s *stubConnManager) GetMinorBlocks(_ []common.Hash, _ string, _ uint32) ([]*types.MinorBlock, error) {
	return nil, nil
}
func (s *stubConnManager) GetMinorBlockHeaderList(_ *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	return nil, nil
}

// stubMinerAPI satisfies miner.MinerAPI. IsSyncing returns true so that
// HandleNewTip never sends to startCh, keeping the miner goroutines idle and
// preventing any attempt to create blocks during tests.
type stubMinerAPI struct{}

func (s *stubMinerAPI) GetDefaultCoinbaseAddress() account.Address {
	return account.CreatEmptyAddress(0)
}
func (s *stubMinerAPI) CreateBlockToMine(_ *account.Address) (types.IBlock, *big.Int, uint64, error) {
	return nil, nil, 0, errors.New("stub: no mining in tests")
}
func (s *stubMinerAPI) InsertMinedBlock(_ types.IBlock) error { return nil }
func (s *stubMinerAPI) IsSyncing() bool                       { return true }
func (s *stubMinerAPI) GetTip() uint64                        { return 0 }

// ── helpers ────────────────────────────────────────────────────────────────

// newTestShardBackend returns a minimal ShardBackend backed by a real
// MinorBlockChain (in-memory DB, FakeEngine). Only fields accessed by
// AddBlockListForSync and AddMinorBlock are populated.
// The returned cleanup function stops both the miner and the blockchain.
func newTestShardBackend(t *testing.T) (*ShardBackend, ethdb.Database, func()) {
	t.Helper()

	clusterCfg := config.NewClusterConfig()
	clusterCfg.Quarkchain.SkipMinorDifficultyCheck = true
	clusterCfg.Quarkchain.SkipRootDifficultyCheck = true
	clusterCfg.Quarkchain.SkipRootCoinbaseCheck = true
	fullShardID := clusterCfg.Quarkchain.Chains[0].ShardSize | 0

	db := ethdb.NewMemDatabase()
	gspec := core.NewGenesis(clusterCfg.Quarkchain)
	rootBlock := gspec.CreateRootBlock()
	gspec.MustCommitMinorBlock(db, rootBlock, fullShardID)

	blockchain, err := core.NewMinorBlockChain(db, nil, params.TestChainConfig, clusterCfg, new(consensus.FakeEngine), vm.Config{}, nil, fullShardID)
	if err != nil {
		t.Fatalf("NewMinorBlockChain: %v", err)
	}
	if _, err := blockchain.InitGenesisState(rootBlock); err != nil {
		t.Fatalf("InitGenesisState: %v", err)
	}

	// The stub miner is required because AddMinorBlock calls
	// go s.miner.HandleNewTip() when the chain head changes.
	// IsSyncing=true keeps the miner goroutines idle.
	testMiner := miner.New(nil, &stubMinerAPI{}, new(consensus.FakeEngine))

	sb := &ShardBackend{
		branch:          account.Branch{Value: fullShardID},
		MinorBlockChain: blockchain,
		conn:            &stubConnManager{},
		mBPool:          newBlockPool{BlockPool: make(map[common.Hash]bool)},
		logInfo:         "test-shard",
		miner:           testMiner,
	}
	return sb, db, func() {
		testMiner.Stop()
		blockchain.Stop()
	}
}

func toIBlocks(blocks []*types.MinorBlock) []types.IBlock {
	result := make([]types.IBlock, len(blocks))
	for i, b := range blocks {
		result[i] = b
	}
	return result
}

// forkBlock returns a block at height 1 whose parent is the given genesis block
// but whose coinbase differs from blocks generated with a nil gen function.
// Inserting it after a canonical height-1 block produces a sidechain: the head
// does not change, so AddMinorBlock never calls s.miner.HandleNewTip.
func forkBlock(t *testing.T, genesis *types.MinorBlock, engine consensus.Engine, db ethdb.Database) *types.MinorBlock {
	t.Helper()
	blocks, _ := core.GenerateMinorBlockChain(
		params.TestChainConfig,
		config.NewQuarkChainConfig(),
		genesis, engine, db, 1,
		func(_ *config.QuarkChainConfig, _ int, b *core.MinorBlockGen) {
			b.SetCoinbase(account.Address{Recipient: account.Recipient{0: 0xAB}, FullShardKey: 0})
		},
	)
	return blocks[0]
}

// ── Bug 3 test ─────────────────────────────────────────────────────────────

// TestAddBlockListForSync_ContinuesPastKnownBlock verifies that
// AddBlockListForSync processes every block in the batch even when an earlier
// block is already committed.
//
// SCOPE: this test exercises the PRE-EXISTING BLOCK_COMMITTED → continue
// early-exit at the top of the loop (api_backend.go ~line 441), which was
// already present upstream before the Bug 3 fix. It therefore passes on both the
// old and the fixed code and does NOT, by itself, cover the changed lines.
//
// The actual Bug 3 change (the non-committed guard, and replacing 'return nil'
// with 'continue' when InsertChainForDeposits returns an empty xshard for a
// block that was NOT yet committed) is driven directly by the seam-based tests:
//   - TestAddBlockListForSync_ContinuesAfterCommittedEmptyXshard (continue path)
//   - TestAddBlockListForSync_ErrorsOnNonCommittedEmptyXshard    (error path)
func TestAddBlockListForSync_ContinuesPastKnownBlock(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()

	// Generate two sequential blocks on top of genesis.
	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 2, nil)
	blockA, blockB := blocks[0], blocks[1]

	// Insert blockA canonically. writeBlockWithState commits it, so
	// getBlockCommitStatusByHash will return BLOCK_COMMITTED for blockA.
	if _, err := sb.MinorBlockChain.InsertChain(toIBlocks(blocks[:1]), false); err != nil {
		t.Fatalf("pre-insert blockA: %v", err)
	}
	if !sb.MinorBlockChain.IsMinorBlockCommittedByHash(blockA.Hash()) {
		t.Fatal("blockA must be committed after InsertChain")
	}

	// Call AddBlockListForSync with [blockA (committed), blockB (new)].
	// The old code would hit 'return nil' on blockA and never reach blockB.
	// The fixed code skips blockA via 'continue' and processes blockB.
	if err := sb.AddBlockListForSync([]*types.MinorBlock{blockA, blockB}); err != nil {
		t.Fatalf("AddBlockListForSync: %v", err)
	}

	if !sb.MinorBlockChain.IsMinorBlockCommittedByHash(blockB.Hash()) {
		t.Error("blockB must be committed: AddBlockListForSync abandoned the batch early")
	}
	if sb.MinorBlockChain.CurrentBlock().Hash() != blockB.Hash() {
		t.Errorf("chain tip must be blockB (%s), got %s",
			blockB.Hash().Hex(), sb.MinorBlockChain.CurrentBlock().Hash().Hex())
	}
}

// ── Bug 1 tests: AddBlockListForSync ───────────────────────────────────────

// TestAddBlockListForSync_CommitMarkerPresentAfterSync verifies that after
// AddBlockListForSync the commit marker (written by InsertChainForDeposits
// inside m.chainmu) is present for every synced block.
//
// Bug 1 removed a redundant CommitMinorBlockByHash call from the shard layer.
// This test confirms the internal write is sufficient.
func TestAddBlockListForSync_CommitMarkerPresentAfterSync(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()

	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 2, nil)

	if err := sb.AddBlockListForSync([]*types.MinorBlock{blocks[0], blocks[1]}); err != nil {
		t.Fatalf("AddBlockListForSync: %v", err)
	}

	for _, b := range blocks {
		if !rawdb.HasBlock(db, b.Hash()) {
			t.Errorf("block %d: body absent after sync", b.NumberU64())
		}
		if !rawdb.HasCommitMinorBlock(db, b.Hash()) {
			t.Errorf("block %d: commit marker absent after sync", b.NumberU64())
		}
	}
}

// TestAddBlockListForSync_MarkerBodyConsistencyUnderRace documents and detects
// the Bug 1 race condition for AddBlockListForSync.
//
// The old code called CommitMinorBlockByHash from the shard layer OUTSIDE
// m.chainmu. Concurrently, AddRootBlock → setHead (inside m.chainmu) could
// delete the same marker. The racy re-write left the block with
// marker=present, body=absent — causing HasBlock() to return true for a block
// whose body was gone. The next block would fail with "unknown ancestor".
//
// The fix removes the redundant call; the marker is written only once, inside
// InsertChainForDeposits under m.chainmu, so setHead serializes cleanly.
//
// Run with: go test -race ./cluster/shard/... -run TestAddBlockListForSync_MarkerBodyConsistencyUnderRace
func TestAddBlockListForSync_MarkerBodyConsistencyUnderRace(t *testing.T) {
	// A single interleaving rarely lands in the marker-deletion window, so repeat
	// it raceIterations times with a fresh backend each round.
	// Run with: go test -race ./cluster/shard/... -run TestAddBlockListForSync_MarkerBodyConsistencyUnderRace
	for i := 0; i < raceIterations; i++ {
		sb, db, stop := newTestShardBackend(t)

		engine := new(consensus.FakeEngine)
		genesis := sb.MinorBlockChain.CurrentBlock()
		genesisNum := genesis.NumberU64()

		blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 1, nil)
		block := blocks[0]

		var wg sync.WaitGroup
		wg.Add(2)

		// goroutine A: sync the block via AddBlockListForSync
		go func() {
			defer wg.Done()
			_ = sb.AddBlockListForSync([]*types.MinorBlock{block})
		}()

		// goroutine B: roll back past the block, simulating AddRootBlock → setHead
		go func() {
			defer wg.Done()
			_ = sb.MinorBlockChain.SetHead(genesisNum)
		}()

		wg.Wait()

		hasBody := rawdb.HasBlock(db, block.Hash())
		hasMarker := rawdb.HasCommitMinorBlock(db, block.Hash())
		if !hasBody && hasMarker {
			stop()
			t.Fatalf("iter %d: Bug 1 race detected: commit marker present but block body absent — "+
				"CommitMinorBlockByHash was called outside m.chainmu after setHead deleted the marker", i)
		}
		stop()
	}
}

// ── Bug 1 tests: AddMinorBlock ─────────────────────────────────────────────

// TestAddMinorBlock_CommitMarkerPresentAfterBlock verifies that after
// AddMinorBlock the commit marker is present for the added block.
//
// The test uses a sidechain block (blockAFork) so the chain head does not
// change, meaning s.miner.HandleNewTip is never called. This lets us test the
// full AddMinorBlock path without a production miner setup.
//
// Bug 1 removed a redundant CommitMinorBlockByHash call from AddMinorBlock.
// This test confirms the internal write (inside InsertChainForDeposits) is
// sufficient.
func TestAddMinorBlock_CommitMarkerPresentAfterBlock(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()

	// blockA: canonical block at height 1
	blocksA, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 1, nil)
	blockA := blocksA[0]

	if _, err := sb.MinorBlockChain.InsertChain(toIBlocks([]*types.MinorBlock{blockA}), false); err != nil {
		t.Fatalf("pre-insert blockA: %v", err)
	}

	// blockAFork: different block at height 1 (same parent, different coinbase).
	// With equal total difficulty, blockAFork stays a sidechain — the canonical
	// head remains blockA, so HandleNewTip is never called.
	blockAFork := forkBlock(t, genesis, engine, db)

	if err := sb.AddMinorBlock(blockAFork); err != nil {
		t.Fatalf("AddMinorBlock(blockAFork): %v", err)
	}

	if !rawdb.HasBlock(db, blockAFork.Hash()) {
		t.Error("blockAFork: body absent after AddMinorBlock")
	}
	if !rawdb.HasCommitMinorBlock(db, blockAFork.Hash()) {
		t.Error("blockAFork: commit marker absent after AddMinorBlock — " +
			"marker must be written inside InsertChainForDeposits")
	}
}

// TestAddMinorBlock_MarkerBodyConsistencyUnderRace documents and detects the
// Bug 1 race condition for AddMinorBlock.
//
// Same race as for AddBlockListForSync: the old code re-wrote the commit marker
// outside m.chainmu after InsertChainForDeposits returned. Concurrently,
// setHead (under m.chainmu) deleted the marker, allowing the redundant write to
// produce marker=present, body=absent.
//
// Run with: go test -race ./cluster/shard/... -run TestAddMinorBlock_MarkerBodyConsistencyUnderRace
func TestAddMinorBlock_MarkerBodyConsistencyUnderRace(t *testing.T) {
	// Repeat raceIterations times with a fresh backend each round.
	// Run with: go test -race ./cluster/shard/... -run TestAddMinorBlock_MarkerBodyConsistencyUnderRace
	for i := 0; i < raceIterations; i++ {
		sb, db, stop := newTestShardBackend(t)

		engine := new(consensus.FakeEngine)
		genesis := sb.MinorBlockChain.CurrentBlock()
		genesisNum := genesis.NumberU64()

		// Pre-insert blockA so blockAFork enters AddMinorBlock as a sidechain.
		blocksA, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 1, nil)
		if _, err := sb.MinorBlockChain.InsertChain(toIBlocks([]*types.MinorBlock{blocksA[0]}), false); err != nil {
			stop()
			t.Fatalf("iter %d: pre-insert blockA: %v", i, err)
		}

		blockAFork := forkBlock(t, genesis, engine, db)

		var wg sync.WaitGroup
		wg.Add(2)

		// goroutine A: add the fork block via AddMinorBlock
		go func() {
			defer wg.Done()
			_ = sb.AddMinorBlock(blockAFork)
		}()

		// goroutine B: roll back to genesis, simulating AddRootBlock → setHead
		go func() {
			defer wg.Done()
			_ = sb.MinorBlockChain.SetHead(genesisNum)
		}()

		wg.Wait()

		hasBody := rawdb.HasBlock(db, blockAFork.Hash())
		hasMarker := rawdb.HasCommitMinorBlock(db, blockAFork.Hash())
		if !hasBody && hasMarker {
			stop()
			t.Fatalf("iter %d: Bug 1 race detected: commit marker present but block body absent — "+
				"CommitMinorBlockByHash was called outside m.chainmu after setHead deleted the marker", i)
		}
		stop()
	}
}

// ── Bug 3 tests: the actual changed lines, via the insertChainForDeposits seam ──
//
// AddBlockListForSync's Bug 3 branch only triggers when InsertChainForDeposits
// returns an empty xshard list (len != 1) for a block that was NOT committed at
// the top-of-loop check. With a real chain that requires either a 128-block
// pruned-state setup (ErrPrunedAncestor → insertSidechain) or a shutting-down
// chain (procInterrupt). These tests instead override the insertChainForDeposits
// seam to reproduce both outcomes deterministically, so the guard and the
// 'continue' are exercised directly. The branching logic under test (commit-status
// check, guard, continue, post-loop RPCs) is the real production code.

// TestAddBlockListForSync_ContinuesAfterCommittedEmptyXshard covers the
// ErrPrunedAncestor → insertSidechain case: InsertChainForDeposits commits the
// block internally but returns an empty xshard. The fixed code must 'continue'
// and keep processing the rest of the batch; the old code did 'return nil' and
// abandoned everything after the first such block.
func TestAddBlockListForSync_ContinuesAfterCommittedEmptyXshard(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()
	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 2, nil)

	// Simulate insertSidechain: commit the block, return an empty xshard, no error.
	var seen []common.Hash
	orig := insertChainForDeposits
	defer func() { insertChainForDeposits = orig }()
	insertChainForDeposits = func(mbc *core.MinorBlockChain, chain []types.IBlock, _ bool) (int, [][]*types.CrossShardTransactionDeposit, error) {
		h := chain[0].Hash()
		seen = append(seen, h)
		mbc.CommitMinorBlockByHash(h)
		return 0, [][]*types.CrossShardTransactionDeposit{}, nil
	}

	if err := sb.AddBlockListForSync(blocks); err != nil {
		t.Fatalf("AddBlockListForSync: unexpected error: %v", err)
	}

	// Both blocks must have been handed to InsertChainForDeposits. With the old
	// 'return nil' this would be 1 (batch abandoned after the first block).
	if len(seen) != 2 {
		t.Errorf("continue did not process the whole batch: insert called %d time(s), want 2", len(seen))
	}
	for _, b := range blocks {
		if !sb.MinorBlockChain.IsMinorBlockCommittedByHash(b.Hash()) {
			t.Errorf("block %d should be committed by the simulated sidechain insert", b.NumberU64())
		}
	}
}

// TestAddBlockListForSync_ErrorsOnNonCommittedEmptyXshard covers the procInterrupt
// case: InsertChainForDeposits returns an empty xshard for a block it did NOT
// commit (node shutting down, nothing written). The fixed code must return an
// error so the caller retries; the old code silently did 'return nil'.
func TestAddBlockListForSync_ErrorsOnNonCommittedEmptyXshard(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()
	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 1, nil)

	// Simulate procInterrupt: nothing inserted, block left uncommitted.
	orig := insertChainForDeposits
	defer func() { insertChainForDeposits = orig }()
	insertChainForDeposits = func(_ *core.MinorBlockChain, _ []types.IBlock, _ bool) (int, [][]*types.CrossShardTransactionDeposit, error) {
		return 0, nil, nil
	}

	err := sb.AddBlockListForSync(blocks)
	if err == nil {
		t.Fatal("expected an error for a non-committed empty-xshard block (procInterrupt), got nil")
	}
	if !strings.Contains(err.Error(), "non-committed") {
		t.Errorf("error should mention the non-committed block, got: %v", err)
	}
	if sb.MinorBlockChain.IsMinorBlockCommittedByHash(blocks[0].Hash()) {
		t.Error("block must not be committed when InsertChainForDeposits wrote nothing")
	}
}
