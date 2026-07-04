package shard

import (
	"errors"
	"math/big"
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
// Bug 3 fix: replaced 'return nil' with 'continue' when a block in the batch
// is already committed or InsertChainForDeposits returns an empty xshard list.
// This ensures the rest of the batch is processed instead of being abandoned.
func TestAddBlockListForSync_ContinuesPastKnownBlock(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()

	// Generate two sequential blocks on top of genesis.
	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 2, nil)
	blockA, blockB := blocks[0], blocks[1]

	// Insert blockA canonically, then commit it via the shard layer (simulating
	// what AddBlockListForSync / AddMinorBlock would do after broadcast).
	// CommitMinorBlockByHash is no longer called inside WriteBlockWithState;
	// it is only called by the shard layer after distributed coordination.
	if _, err := sb.MinorBlockChain.InsertChain(toIBlocks(blocks[:1]), false); err != nil {
		t.Fatalf("pre-insert blockA: %v", err)
	}
	sb.MinorBlockChain.CommitMinorBlockByHash(blockA.Hash())
	if !sb.MinorBlockChain.HasCommittedBlock(blockA.Hash()) {
		t.Fatal("blockA must be committed after InsertChain + CommitMinorBlockByHash")
	}

	// Call AddBlockListForSync with [blockA (committed), blockB (new)].
	// The old code would hit 'return nil' on blockA and never reach blockB.
	// The fixed code skips blockA via 'continue' and processes blockB.
	if err := sb.AddBlockListForSync([]*types.MinorBlock{blockA, blockB}); err != nil {
		t.Fatalf("AddBlockListForSync: %v", err)
	}

	if !sb.MinorBlockChain.HasCommittedBlock(blockB.Hash()) {
		t.Error("blockB must be committed: AddBlockListForSync abandoned the batch early")
	}
	if sb.MinorBlockChain.CurrentBlock().Hash() != blockB.Hash() {
		t.Errorf("chain tip must be blockB (%s), got %s",
			blockB.Hash().Hex(), sb.MinorBlockChain.CurrentBlock().Hash().Hex())
	}
}

// ── Bug 1 tests: AddBlockListForSync ───────────────────────────────────────

// TestAddBlockListForSync_CommitMarkerPresentAfterSync verifies that after
// AddBlockListForSync the commit marker (written by the shard layer after
// broadcast and report-to-master) is present for every synced block.
//
// Bug 1 fix: CommitMinorBlockByHash was removed from WriteBlockWithState so
// that the marker is written ONLY by the shard layer after distributed
// coordination completes (broadcast + SendMinorBlockHeaderListToMaster).
// This test confirms the shard-layer write is the sole writer and is sufficient.
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

// TestAddBlockListForSync_MarkerBodyConsistencyUnderRace asserts the body/marker
// consistency invariant under a concurrent AddBlockListForSync vs SetHead: the
// commit marker is never present while the body is absent.
//
// Two mechanisms together guarantee this, and this test guards both against
// regression:
//  1. CommitMinorBlockByHash checks HasBlock (body present) under m.mu before
//     writing the marker, and SetHead also holds m.mu, so a marker is never
//     written for a body that a concurrent SetHead already deleted.
//  2. rawdb.DeleteMinorBlock deletes the body AND the commit marker together,
//     so SetHead can never strip the body while leaving the marker behind.
//
// If either regresses (e.g. the body-check is dropped from CommitMinorBlockByHash,
// or DeleteMinorBlock stops deleting the marker), the (marker && !body) state
// becomes observable and this test fails. Must run with -race to exercise the
// interleaving; the loop repeats it many times to hit the window.
//
// Run with: go test -race ./cluster/shard/... -run TestAddBlockListForSync_MarkerBodyConsistencyUnderRace
func TestAddBlockListForSync_MarkerBodyConsistencyUnderRace(t *testing.T) {
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
		// Invariant: a commit marker is never present without its body. If it is,
		// either CommitMinorBlockByHash's body-check or DeleteMinorBlock's
		// marker-deletion has regressed, and HasCommittedBlock would report a
		// block whose body is gone — the "unknown ancestor" bug.
		if hasMarker && !hasBody {
			stop()
			t.Fatalf("iter %d: body/marker inconsistency: commit marker present but body absent — "+
				"CommitMinorBlockByHash must not write the marker for a deleted body, and "+
				"DeleteMinorBlock must delete body and marker together", i)
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
// Bug 1 fix: CommitMinorBlockByHash was removed from WriteBlockWithState.
// The shard layer is now the sole writer, calling it after BroadcastXshardTxList
// and SendMinorBlockHeaderToMaster complete. This test confirms that path works.
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
			"marker must be written by the shard layer after BroadcastXshardTxList + SendMinorBlockHeaderToMaster")
	}
}

// TestAddMinorBlock_MarkerBodyConsistencyUnderRace asserts the body/marker
// consistency invariant under a concurrent AddMinorBlock vs SetHead: the commit
// marker is never present while the body is absent.
//
// Same invariant and rationale as
// TestAddBlockListForSync_MarkerBodyConsistencyUnderRace: CommitMinorBlockByHash
// checks body-presence under m.mu before writing the marker, and
// rawdb.DeleteMinorBlock deletes body and marker together. This test guards both
// against regression.
//
// Run with: go test -race ./cluster/shard/... -run TestAddMinorBlock_MarkerBodyConsistencyUnderRace
func TestAddMinorBlock_MarkerBodyConsistencyUnderRace(t *testing.T) {
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
		// Invariant: a commit marker is never present without its body.
		if hasMarker && !hasBody {
			stop()
			t.Fatalf("iter %d: body/marker inconsistency: commit marker present but body absent — "+
				"CommitMinorBlockByHash must not write the marker for a deleted body, and "+
				"DeleteMinorBlock must delete body and marker together", i)
		}
		stop()
	}
}

// TestAddBlockListForSync_RecoversXShardListOnRetry verifies Issue 3 fix:
// when a block body already exists in the DB but the commit marker is absent
// (retry after partial failure), InsertChainForDeposits with force=true
// re-executes the block to recover the outgoing xshard list without writing state.
func TestAddBlockListForSync_RecoversXShardListOnRetry(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()
	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 1, nil)
	block := blocks[0]

	// Simulate partial failure: write the block body directly to DB without
	// going through the shard layer, so the commit marker is absent.
	if _, err := sb.MinorBlockChain.InsertChain([]types.IBlock{block}, false); err != nil {
		t.Fatalf("pre-insert body: %v", err)
	}
	if !rawdb.HasBlock(db, block.Hash()) {
		t.Fatal("block body must be present after InsertChain")
	}
	if rawdb.HasCommitMinorBlock(db, block.Hash()) {
		t.Fatal("commit marker must be absent — simulating partial failure")
	}

	// Now retry via AddBlockListForSync. With force=true, InsertChainForDeposits
	// should re-execute the block, recover the xshard list, and commit successfully.
	if err := sb.AddBlockListForSync([]*types.MinorBlock{block}); err != nil {
		t.Fatalf("AddBlockListForSync on retry: %v", err)
	}

	if !rawdb.HasBlock(db, block.Hash()) {
		t.Error("block body must still be present after retry")
	}
	if !rawdb.HasCommitMinorBlock(db, block.Hash()) {
		t.Error("commit marker must be written after successful retry — Issue 3 fix")
	}
	if sb.MinorBlockChain.CurrentBlock().Hash() != block.Hash() {
		t.Errorf("chain tip must advance to block %s after retry, got %s",
			block.Hash().Hex(), sb.MinorBlockChain.CurrentBlock().Hash().Hex())
	}
}

// TestNewMinorBlock_RecoversUncommittedBodyOnRetry verifies that NewMinorBlock
// does not strand a block whose body is present but whose commit marker is
// absent (a prior AddMinorBlock inserted the body, then failed before commit).
//
// The block passes NewMinorBlock's HasCommittedBlock guard (marker absent), so
// it reaches the pre-validation ValidateBlock call. If that call used force=false,
// HasBlockAndState would return ErrKnownBlock and NewMinorBlock would return nil
// before delegating to AddMinorBlock — the uncommitted body would never be
// committed. NewMinorBlock must validate with force=true so AddMinorBlock's
// force=true recovery path runs and writes the marker.
func TestNewMinorBlock_RecoversUncommittedBodyOnRetry(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	engine := new(consensus.FakeEngine)
	genesis := sb.MinorBlockChain.CurrentBlock()
	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 1, nil)
	block := blocks[0]

	// Simulate partial failure: body written to DB, commit marker absent.
	if _, err := sb.MinorBlockChain.InsertChain([]types.IBlock{block}, false); err != nil {
		t.Fatalf("pre-insert body: %v", err)
	}
	if !rawdb.HasBlock(db, block.Hash()) {
		t.Fatal("block body must be present after InsertChain")
	}
	if rawdb.HasCommitMinorBlock(db, block.Hash()) {
		t.Fatal("commit marker must be absent — simulating partial failure")
	}

	// Re-deliver the block over P2P. Must not be dropped as ErrKnownBlock; must
	// reach AddMinorBlock and commit.
	if err := sb.NewMinorBlock("peer-1", block); err != nil {
		t.Fatalf("NewMinorBlock on retry: %v", err)
	}

	if !rawdb.HasCommitMinorBlock(db, block.Hash()) {
		t.Error("commit marker must be written after NewMinorBlock retry — " +
			"uncommitted body must be validated with force=true and reach AddMinorBlock recovery")
	}
}
