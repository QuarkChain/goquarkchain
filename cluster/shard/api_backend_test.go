package shard

import (
	"errors"
	"math/big"
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

func (s *stubConnManager) BroadcastMinorBlock(_ string, _ *types.MinorBlock) error {
	return nil
}

func (s *stubConnManager) GetMinorBlocks(_ []common.Hash, _ string, _ uint32) ([]*types.MinorBlock, error) {
	return nil, nil
}

func (s *stubConnManager) GetMinorBlockHeaderList(_ *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	return nil, nil
}

type stubMinerAPI struct{}

func (s *stubMinerAPI) GetDefaultCoinbaseAddress() account.Address {
	return account.CreatEmptyAddress(0)
}

func (s *stubMinerAPI) CreateBlockToMine(_ *account.Address) (types.IBlock, *big.Int, uint64, error) {
	return nil, nil, 0, errors.New("stub miner does not create blocks")
}

func (s *stubMinerAPI) InsertMinedBlock(types.IBlock) error {
	return nil
}

func (s *stubMinerAPI) IsSyncing() bool {
	return true
}

func (s *stubMinerAPI) GetTip() uint64 {
	return 0
}

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

func minorBlocksToIBlocks(blocks []*types.MinorBlock) []types.IBlock {
	result := make([]types.IBlock, len(blocks))
	for i, block := range blocks {
		result[i] = block
	}
	return result
}

func makeTestMinorBlocks(t *testing.T, db ethdb.Database, parent *types.MinorBlock, n int) []*types.MinorBlock {
	t.Helper()
	blocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), parent, new(consensus.FakeEngine), db, n, nil)
	return blocks
}

func makeTestForkBlock(t *testing.T, db ethdb.Database, parent *types.MinorBlock) *types.MinorBlock {
	t.Helper()
	blocks, _ := core.GenerateMinorBlockChain(
		params.TestChainConfig,
		config.NewQuarkChainConfig(),
		parent,
		new(consensus.FakeEngine),
		db,
		1,
		func(_ *config.QuarkChainConfig, _ int, b *core.MinorBlockGen) {
			b.SetCoinbase(account.Address{Recipient: account.Recipient{0: 0xAB}, FullShardKey: 0})
		},
	)
	return blocks[0]
}

func TestAddBlockListForSyncSkipsCommittedBlockAndCommitsRest(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	blocks := makeTestMinorBlocks(t, db, sb.MinorBlockChain.CurrentBlock(), 2)
	blockA, blockB := blocks[0], blocks[1]
	if _, err := sb.MinorBlockChain.InsertChain(minorBlocksToIBlocks(blocks[:1]), false); err != nil {
		t.Fatalf("pre-insert blockA: %v", err)
	}
	if !sb.MinorBlockChain.CommitMinorBlockByHash(blockA.Hash()) {
		t.Fatal("blockA should commit after its body is inserted")
	}

	if err := sb.AddBlockListForSync([]*types.MinorBlock{blockA, blockB}); err != nil {
		t.Fatalf("AddBlockListForSync: %v", err)
	}
	if !sb.MinorBlockChain.HasCommittedBlock(blockA.Hash()) {
		t.Fatal("blockA should remain committed")
	}
	if !sb.MinorBlockChain.HasCommittedBlock(blockB.Hash()) {
		t.Fatal("blockB should be processed and committed after committed blockA is skipped")
	}
	if sb.MinorBlockChain.CurrentBlock().Hash() != blockB.Hash() {
		t.Fatalf("chain tip should be blockB, got %s", sb.MinorBlockChain.CurrentBlock().Hash().Hex())
	}
}

func TestAddMinorBlockWritesCommitMarker(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	genesis := sb.MinorBlockChain.CurrentBlock()
	blockA := makeTestMinorBlocks(t, db, genesis, 1)[0]
	if _, err := sb.MinorBlockChain.InsertChain([]types.IBlock{blockA}, false); err != nil {
		t.Fatalf("pre-insert blockA: %v", err)
	}

	blockAFork := makeTestForkBlock(t, db, genesis)
	if err := sb.AddMinorBlock(blockAFork); err != nil {
		t.Fatalf("AddMinorBlock: %v", err)
	}
	if !rawdb.HasBlock(db, blockAFork.Hash()) {
		t.Fatal("fork block body should be present after AddMinorBlock")
	}
	if !rawdb.HasCommitMinorBlock(db, blockAFork.Hash()) {
		t.Fatal("fork block commit marker should be written by AddMinorBlock")
	}
}

func TestAddBlockListForSyncRecoversUncommittedBody(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	block := makeTestMinorBlocks(t, db, sb.MinorBlockChain.CurrentBlock(), 1)[0]
	if _, err := sb.MinorBlockChain.InsertChain([]types.IBlock{block}, false); err != nil {
		t.Fatalf("pre-insert body: %v", err)
	}
	if !rawdb.HasBlock(db, block.Hash()) {
		t.Fatal("block body should be present after InsertChain")
	}
	if rawdb.HasCommitMinorBlock(db, block.Hash()) {
		t.Fatal("commit marker should be absent after core InsertChain")
	}

	if err := sb.AddBlockListForSync([]*types.MinorBlock{block}); err != nil {
		t.Fatalf("AddBlockListForSync retry: %v", err)
	}
	if !sb.MinorBlockChain.HasCommittedBlock(block.Hash()) {
		t.Fatal("retry should commit existing body")
	}
}

func TestNewMinorBlockRecoversUncommittedBody(t *testing.T) {
	sb, db, stop := newTestShardBackend(t)
	defer stop()

	block := makeTestMinorBlocks(t, db, sb.MinorBlockChain.CurrentBlock(), 1)[0]
	if _, err := sb.MinorBlockChain.InsertChain([]types.IBlock{block}, false); err != nil {
		t.Fatalf("pre-insert body: %v", err)
	}
	if rawdb.HasCommitMinorBlock(db, block.Hash()) {
		t.Fatal("commit marker should be absent after core InsertChain")
	}

	if err := sb.NewMinorBlock("peer-1", block); err != nil {
		t.Fatalf("NewMinorBlock retry: %v", err)
	}
	if !sb.MinorBlockChain.HasCommittedBlock(block.Hash()) {
		t.Fatal("NewMinorBlock should route existing uncommitted body through AddMinorBlock")
	}
}
