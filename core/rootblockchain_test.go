// Modified from go-ethereum under GNU Lesser General Public License

package core

import (
	"errors"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/stretchr/testify/assert"
)

var (
	canonicalSeed = 1
	forkSeed      = 2
	qkcconfig     = config.NewQuarkChainConfig()
)

// newCanonical creates a chain database, and injects a deterministic canonical
// chain.
func newCanonical(engine consensus.Engine, n int) (ethdb.Database, *RootBlockChain, error) {
	var (
		db           = ethdb.NewMemDatabase()
		genesis      = NewGenesis(qkcconfig)
		genesisBlock = genesis.MustCommitRootBlock(db)
	)

	qkcconfig.SkipRootCoinbaseCheck = true
	// Initialize a fresh chain with only a genesis block
	blockchain, _ := NewRootBlockChain(db, qkcconfig, engine)
	// Create and inject the requested chain
	if n == 0 {
		return db, blockchain, nil
	}
	blocks := makeRootBlockChain(genesisBlock, n, engine, canonicalSeed)
	_, err := blockchain.InsertChain(ToBlocks(blocks))
	return db, blockchain, err
}

// Test fork of length N starting from block i
func testFork(t *testing.T, blockchain *RootBlockChain, i, n int, comparator func(td1, td2 *big.Int)) {
	// Copy old chain up to #i into a new db
	engine := new(consensus.FakeEngine)
	_, blockchain2, err := newCanonical(engine, i)
	if err != nil {
		t.Fatal("could not make new canonical in testFork", err)
	}
	defer blockchain2.Stop()

	// Assert the chains have the same header/block at #i
	hash1 := blockchain.GetBlockByNumber(uint64(i)).Hash()
	hash2 := blockchain2.GetBlockByNumber(uint64(i)).Hash()
	if hash1 != hash2 {
		t.Errorf("chain content mismatch at %d: have hash %v, want hash %v", i, hash2, hash1)
	}
	// Extend the newly created chain
	var (
		blockChainB []*types.RootBlock
	)
	blockChainB = makeRootBlockChain(blockchain2.CurrentBlock(), n, engine, forkSeed)
	if _, err := blockchain2.InsertChain(ToBlocks(blockChainB)); err != nil {
		t.Fatalf("failed to insert forking chain: %v", err)
	}
	// Sanity check that the forked chain can be imported into the original
	var tdPre, tdPost *big.Int

	tdPre = blockchain.CurrentBlock().TotalDifficulty()
	if err := testBlockChainImport(blockChainB, blockchain); err != nil {
		t.Fatalf("failed to import forked block chain: %v", err)
	}
	tdPost = blockChainB[len(blockChainB)-1].TotalDifficulty()
	// Compare the total difficulties of the chains
	comparator(tdPre, tdPost)
}

func printChain(bc *RootBlockChain) {
	for i := bc.CurrentBlock().NumberU64(); i > 0; i-- {
		b := bc.GetBlockByNumber(uint64(i))
		fmt.Printf("\t%x %v\n", b.Hash(), b.IHeader().GetDifficulty())
	}
}

// testBlockChainImport tries to process a chain of blocks, writing them into
// the database if successful.
func testBlockChainImport(chain []*types.RootBlock, blockchain *RootBlockChain) error {
	for _, block := range chain {
		// Try and process the block
		err := blockchain.engine.VerifyHeader(blockchain, block.IHeader(), true)
		if err == nil {
			err = blockchain.validator.ValidateBlock(block, false)
		}
		if err != nil {
			if err == ErrKnownBlock {
				continue
			}
			return err
		}
		blockchain.mu.Lock()
		rawdb.WriteRootBlock(blockchain.db, block)
		blockchain.mu.Unlock()
	}
	return nil
}

func TestLastBlock(t *testing.T) {
	engine := new(consensus.FakeEngine)
	_, blockchain, err := newCanonical(engine, 0)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer blockchain.Stop()

	blocks := makeRootBlockChain(blockchain.CurrentBlock(), 1, engine, 0)
	if _, err := blockchain.InsertChain(ToBlocks(blocks)); err != nil {
		t.Fatalf("Failed to insert block: %v", err)
	}
	if blocks[len(blocks)-1].Hash() != rawdb.ReadHeadBlockHash(blockchain.db) {
		t.Fatalf("Write/Get HeadBlockHash failed")
	}
}

// Tests that given a starting canonical chain of a given size, it can be extended
// with various length chains.

func TestExtendCanonicalBlocks(t *testing.T) {
	length := 5
	engine := new(consensus.FakeEngine)
	// Make first chain starting from genesis
	_, processor, err := newCanonical(engine, length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
	defer processor.Stop()

	// Define the difficulty comparator
	better := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) <= 0 {
			t.Errorf("total difficulty mismatch: have %v, expected more than %v", td2, td1)
		}
	}
	// Start fork from current height
	testFork(t, processor, length, 1, better)
	testFork(t, processor, length, 2, better)
	testFork(t, processor, length, 5, better)
	testFork(t, processor, length, 10, better)
}

// Tests that given a starting canonical chain of a given size, creating shorter
// forks do not take canonical ownership.
func TestShorterForkBlocks(t *testing.T) {
	length := 10
	engine := new(consensus.FakeEngine)
	// Make first chain starting from genesis
	_, processor, err := newCanonical(engine, length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
	defer processor.Stop()

	// Define the difficulty comparator
	worse := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) >= 0 {
			t.Errorf("total difficulty mismatch: have %v, expected less than %v", td2, td1)
		}
	}
	// Sum of numbers must be less than `length` for this to be a shorter fork
	testFork(t, processor, 0, 3, worse)
	testFork(t, processor, 0, 7, worse)
	testFork(t, processor, 1, 1, worse)
	testFork(t, processor, 1, 7, worse)
	testFork(t, processor, 5, 3, worse)
	testFork(t, processor, 5, 4, worse)
}

// Tests that given a starting canonical chain of a given size, creating longer
// forks do take canonical ownership.
func TestLongerForkBlocks(t *testing.T) {
	length := 10
	engine := new(consensus.FakeEngine)
	// Make first chain starting from genesis
	_, processor, err := newCanonical(engine, length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
	defer processor.Stop()

	// Define the difficulty comparator
	better := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) <= 0 {
			t.Errorf("total difficulty mismatch: have %v, expected more than %v", td2, td1)
		}
	}
	// Sum of numbers must be greater than `length` for this to be a longer fork
	testFork(t, processor, 0, 11, better)
	testFork(t, processor, 0, 15, better)
	testFork(t, processor, 1, 10, better)
	testFork(t, processor, 1, 12, better)
	testFork(t, processor, 5, 6, better)
	testFork(t, processor, 5, 8, better)
}

// Tests that given a starting canonical chain of a given size, creating equal
// forks do take canonical ownership.
func TestEqualForkBlocks(t *testing.T) {
	length := 10
	engine := new(consensus.FakeEngine)
	// Make first chain starting from genesis
	_, processor, err := newCanonical(engine, length)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
	defer processor.Stop()

	// Define the difficulty comparator
	equal := func(td1, td2 *big.Int) {
		if td2.Cmp(td1) != 0 {
			t.Errorf("total difficulty mismatch: have %v, want %v", td2, td1)
		}
	}
	// Sum of numbers must be equal to `length` for this to be an equal fork
	testFork(t, processor, 0, 10, equal)
	testFork(t, processor, 1, 9, equal)
	testFork(t, processor, 2, 8, equal)
	testFork(t, processor, 5, 5, equal)
	testFork(t, processor, 6, 4, equal)
	testFork(t, processor, 9, 1, equal)
}

// Tests that chains missing links do not get accepted by the processor.
func TestBrokenBlockChain(t *testing.T) {
	engine := new(consensus.FakeEngine)
	// Make chain starting from genesis
	_, blockchain, err := newCanonical(engine, 10)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
	defer blockchain.Stop()

	engine.Err = consensus.ErrUnknownAncestor
	engine.NumberToFail = 12
	// Create a forked chain, and try to insert with a missing link
	chain := makeRootBlockChain(blockchain.CurrentBlock(), 5, engine, forkSeed)[1:]
	if err := testBlockChainImport(chain, blockchain); err == nil {
		t.Errorf("broken block chain not reported")
	}
}

// Tests that reorganising a long difficult chain after a short easy one
// overwrites the canonical numbers and links in the database.
func TestReorgLongBlocks(t *testing.T) {
	testReorg(t, []uint64{10000, 10000, 10000}, []uint64{10000, 10000, 10000, 10000}, 40000)
}

// Tests that reorganising a short difficult chain after a long easy one
// overwrites the canonical numbers and links in the database.
func TestReorgShortBlocks(t *testing.T) {
	// Create a long easy chain vs. a short heavy one. Due to difficulty adjustment
	// we need a fairly long chain of blocks with different difficulties for a short
	// one to become heavyer than a long one. The 96 is an empirical value.
	easy := make([]uint64, 96)
	for i := 0; i < len(easy); i++ {
		easy[i] = 10000
	}
	diff := make([]uint64, len(easy)-1)
	for i := 0; i < len(diff); i++ {
		diff[i] = 100000
	}
	testReorg(t, easy, diff, 9500000)
}

func testReorg(t *testing.T, first, second []uint64, td int64) {
	engine := new(consensus.FakeEngine)
	// Create a pristine chain and database
	_, blockchain, err := newCanonical(engine, 0)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer blockchain.Stop()

	// Insert an easy and a difficult chain afterwards
	easyBlocks := GenerateRootBlockChain(blockchain.CurrentBlock(), engine, len(first), func(i int, b *RootBlockGen) {
		b.SetDifficulty(first[i])
	})
	diffBlocks := GenerateRootBlockChain(blockchain.CurrentBlock(), engine, len(second), func(i int, b *RootBlockGen) {
		b.SetDifficulty(second[i])
	})
	if _, err := blockchain.InsertChain(ToBlocks(easyBlocks)); err != nil {
		t.Fatalf("failed to insert easy chain: %v", err)
	}
	if _, err := blockchain.InsertChain(ToBlocks(diffBlocks)); err != nil {
		t.Fatalf("failed to insert difficult chain: %v", err)
	}
	// Check that the chain is valid number and link wise
	prev := blockchain.CurrentBlock()
	for block := blockchain.GetBlockByNumber(blockchain.CurrentBlock().NumberU64() - 1); block.NumberU64() != 0; prev, block = block.(*types.RootBlock), blockchain.GetBlockByNumber(block.NumberU64()-1) {
		if prev.ParentHash() != block.Hash() {
			t.Errorf("parent block hash mismatch: have %x, want %x", prev.ParentHash(), block.Hash())
		}
	}
	// Make sure the chain total difficulty is the correct one
	want := new(big.Int).Add(blockchain.genesisBlock.Difficulty(), big.NewInt(td))
	if have := blockchain.CurrentBlock().TotalDifficulty(); have.Cmp(want) != 0 {
		t.Errorf("total difficulty mismatch for block %d: have %v, want %v", blockchain.CurrentBlock().NumberU64(), have, want)
	}
}

func TestIsSameChain(t *testing.T) {
	engine := new(consensus.FakeEngine)
	// Create a pristine chain and database
	_, blockchain, err := newCanonical(engine, 0)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer blockchain.Stop()

	// Insert an easy and a difficult chain afterwards
	firstBlocks := GenerateRootBlockChain(blockchain.CurrentBlock(), engine, 10, func(i int, b *RootBlockGen) {
		b.SetDifficulty(1000)
	})
	secondBlocks := GenerateRootBlockChain(blockchain.CurrentBlock(), engine, 10, func(i int, b *RootBlockGen) {
		b.SetDifficulty(1100)
	})

	blockchain.InsertChain(ToBlocks(firstBlocks))
	blockchain.InsertChain(ToBlocks(secondBlocks))
	if !blockchain.isSameChain(firstBlocks[9], firstBlocks[3]) ||
		!blockchain.isSameChain(secondBlocks[9], secondBlocks[3]) {
		t.Fatalf("isSameChain result is false, want true")
	}

	if blockchain.isSameChain(firstBlocks[9], secondBlocks[3]) ||
		blockchain.isSameChain(secondBlocks[9], firstBlocks[3]) {
		t.Fatalf("isSameChain result is true, want false")
	}
}

// Tests chain insertions in the face of one entity containing an invalid nonce.
func TestBlocksInsertNonceError(t *testing.T) {
	engine := new(consensus.FakeEngine)
	for i := 1; i < 25 && !t.Failed(); i++ {
		// Create a pristine chain and database
		_, blockchain, err := newCanonical(engine, 0)
		if err != nil {
			t.Fatalf("failed to create pristine chain: %v", err)
		}
		defer blockchain.Stop()

		// Create and insert a chain with a failing nonce
		var (
			failAt  int
			failRes int
			failNum uint64
		)
		blocks := makeRootBlockChain(blockchain.CurrentBlock(), i, engine, 0)

		failAt = rand.Int() % len(blocks)
		failNum = blocks[failAt].NumberU64()

		engine.NumberToFail = failNum
		engine.Err = errors.New("fack engine expected fail")
		failRes, err = blockchain.InsertChain(ToBlocks(blocks))
		// Check that the returned error indicates the failure
		if failRes != failAt {
			t.Errorf("test %d: failure (%v) index mismatch: have %d, want %d", i, err, failRes, failAt)
		}
		// Check that all blocks after the failing block have been inserted
		for j := 0; j < i-failAt; j++ {
			if block := blockchain.GetBlockByNumber(failNum + uint64(j)); block != nil {
				t.Errorf("test %d: invalid block in chain: %v", i, block)
			}
		}
	}
}

func TestReorgSideEvent(t *testing.T) {
	var (
		db    = ethdb.NewMemDatabase()
		addr1 = account.Address{Recipient: account.Recipient{1}, FullShardKey: 0}
		gspec = &Genesis{
			qkcConfig: config.NewQuarkChainConfig(),
		}
		genesis = gspec.MustCommitRootBlock(db)
		engine  = new(consensus.FakeEngine)
	)

	blockchain, _ := NewRootBlockChain(db, gspec.qkcConfig, engine)
	blockchain.validator = new(fakeRootBlockValidator)
	defer blockchain.Stop()

	chain := GenerateRootBlockChain(genesis, engine, 3, func(i int, gen *RootBlockGen) {})
	if _, err := blockchain.InsertChain(ToBlocks(chain)); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}

	engine.Difficulty = new(big.Int).Add(genesis.Difficulty(), new(big.Int).SetUint64(10000))
	replacementBlocks := GenerateRootBlockChain(genesis, engine, 4, func(i int, gen *RootBlockGen) {
		header := types.MinorBlockHeader{Coinbase: addr1}
		gen.Headers = append(gen.Headers, &header)
	})
	chainSideCh := make(chan RootChainSideEvent, 64)
	blockchain.SubscribeChainSideEvent(chainSideCh)
	if _, err := blockchain.InsertChain(ToBlocks(replacementBlocks)); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}

	// first two block of the secondary chain are for a brief moment considered
	// side chains because up to that point the first one is considered the
	// heavier chain.
	expectedSideHashes := map[common.Hash]bool{
		replacementBlocks[0].Hash(): true,
		replacementBlocks[1].Hash(): true,
		chain[0].Hash():             true,
		chain[1].Hash():             true,
		chain[2].Hash():             true,
	}

	i := 0

	const timeoutDura = 10 * time.Second
	timeout := time.NewTimer(timeoutDura)
done:
	for {
		select {
		case ev := <-chainSideCh:
			block := ev.Block
			if _, ok := expectedSideHashes[block.Hash()]; !ok {
				t.Errorf("%d: didn't expect %x to be in side chain", i, block.Hash())
			}
			i++

			if i == len(expectedSideHashes) {
				timeout.Stop()

				break done
			}
			timeout.Reset(timeoutDura)

		case <-timeout.C:
			t.Fatal("Timeout. Possibly not all blocks were triggered for sideevent")
		}
	}

	// make sure no more events are fired
	select {
	case e := <-chainSideCh:
		t.Errorf("unexpected event fired: %v", e)
	case <-time.After(250 * time.Millisecond):
	}

}

// Tests if the canonical block can be fetched from the database during chain insertion.
func TestCanonicalBlockRetrieval(t *testing.T) {
	engine := new(consensus.FakeEngine)
	_, blockchain, err := newCanonical(engine, 0)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer blockchain.Stop()

	chain := GenerateRootBlockChain(blockchain.genesisBlock, engine, 10, func(i int, gen *RootBlockGen) {})

	var pend sync.WaitGroup
	pend.Add(len(chain))

	for i := range chain {
		go func(block *types.RootBlock) {
			defer pend.Done()

			// try to retrieve a block by its canonical hash and see if the block data can be retrieved.
			for {
				ch := rawdb.ReadCanonicalHash(blockchain.db, rawdb.ChainTypeRoot, block.NumberU64())
				if ch == (common.Hash{}) {
					continue // busy wait for canonical hash to be written
				}
				if ch != block.Hash() {
					t.Fatalf("unknown canonical hash, want %s, got %s", block.Hash().Hex(), ch.Hex())
				}
				fb := rawdb.ReadRootBlock(blockchain.db, ch)
				if fb == nil {
					t.Fatalf("unable to retrieve block %d for canonical hash: %s", block.NumberU64(), ch.Hex())
				}
				if fb.Hash() != block.Hash() {
					t.Fatalf("invalid block hash for block %d, want %s, got %s", block.NumberU64(), block.Hash().Hex(), fb.Hash().Hex())
				}
				return
			}
		}(chain[i])

		if _, err := blockchain.InsertChain([]types.IBlock{chain[i]}); err != nil {
			t.Fatalf("failed to insert block %d: %v", i, err)
		}
	}
	pend.Wait()
}

// This is a regression test (i.e. as weird as it is, don't delete it ever), which
// tests that under weird reorg conditions the blockchain and its internal header-
// chain return the same latest block/header.
//
// https://github.com/ethereum/go-ethereum/pull/15941
func TestBlockchainHeaderchainReorgConsistency(t *testing.T) {
	// Generate a canonical chain to act as the main dataset
	db := ethdb.NewMemDatabase()
	genesis := NewGenesis(qkcconfig)
	genesisBlock := genesis.MustCommitRootBlock(db)
	engine := new(consensus.FakeEngine)
	blocks := GenerateRootBlockChain(genesisBlock, engine, 64, func(i int, b *RootBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{1}, FullShardKey: 1})
	})

	// Generate a bunch of fork blocks, each side forking from the canonical chain
	forks := make([]*types.RootBlock, len(blocks))
	for i := 0; i < len(forks); i++ {
		parent := genesisBlock
		if i > 0 {
			parent = blocks[i-1]
		}
		fork := GenerateRootBlockChain(parent, engine, 1, func(i int, b *RootBlockGen) {
			b.SetCoinbase(account.Address{Recipient: account.Recipient{2}, FullShardKey: 1})
		})
		forks[i] = fork[0]
	}
	// Import the canonical and fork chain side by side, verifying the current block
	// and current header consistency
	diskdb := ethdb.NewMemDatabase()
	genesis.MustCommitRootBlock(diskdb)

	chain, err := NewRootBlockChain(diskdb, qkcconfig, engine)
	if err != nil {
		t.Fatalf("failed to create tester chain: %v", err)
	}
	for i := 0; i < len(blocks); i++ {
		if _, err := chain.InsertChain(ToBlocks(blocks[i : i+1])); err != nil {
			t.Fatalf("block %d: failed to insert into chain: %v", i, err)
		}
		if chain.CurrentBlock().Hash() != chain.CurrentHeader().Hash() {
			t.Errorf("block %d: current block/header mismatch: block #%d [%x…], header #%d [%x…]", i, chain.CurrentBlock().Number(), chain.CurrentBlock().Hash().Bytes()[:4], chain.CurrentHeader().NumberU64(), chain.CurrentHeader().Hash().Bytes()[:4])
		}
		if _, err := chain.InsertChain(ToBlocks(forks[i : i+1])); err != nil {
			t.Fatalf(" fork %d: failed to insert into chain: %v", i, err)
		}
		if chain.CurrentBlock().Hash() != chain.CurrentHeader().Hash() {
			t.Errorf(" fork %d: current block/header mismatch: block #%d [%x…], header #%d [%x…]", i, chain.CurrentBlock().Number(), chain.CurrentBlock().Hash().Bytes()[:4], chain.CurrentHeader().NumberU64(), chain.CurrentHeader().Hash().Bytes()[:4])
		}
	}
}

// Tests that importing small side forks doesn't leave junk in the trie database
// cache (which would eventually cause memory issues).
func TestTrieForkGC(t *testing.T) {
	// Generate a canonical chain to act as the main dataset
	db := ethdb.NewMemDatabase()
	qkcconfig.SkipRootCoinbaseCheck = true
	genesis := NewGenesis(qkcconfig)
	genesisBlock := genesis.MustCommitRootBlock(db)
	engine := new(consensus.FakeEngine)
	blocks := GenerateRootBlockChain(genesisBlock, engine, 2*triesInMemory, func(i int, b *RootBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{1}, FullShardKey: 1})
	})

	// Generate a bunch of fork blocks, each side forking from the canonical chain
	forks := make([]*types.RootBlock, len(blocks))
	for i := 0; i < len(forks); i++ {
		parent := genesisBlock
		if i > 0 {
			parent = blocks[i-1]
		}
		fork := GenerateRootBlockChain(parent, engine, 1, func(i int, b *RootBlockGen) {
			b.SetCoinbase(account.Address{Recipient: account.Recipient{2}, FullShardKey: 1})
		})
		forks[i] = fork[0]
	}
	// Import the canonical and fork chain side by side, forcing the trie cache to cache both
	diskdb := ethdb.NewMemDatabase()
	genesis.MustCommitRootBlock(diskdb)

	chain, err := NewRootBlockChain(diskdb, qkcconfig, engine)
	if err != nil {
		t.Fatalf("failed to create tester chain: %v", err)
	}
	for i := 0; i < len(blocks); i++ {
		if _, err := chain.InsertChain(ToBlocks(blocks[i : i+1])); err != nil {
			t.Fatalf("block %d: failed to insert into chain: %v", i, err)
		}
		if _, err := chain.InsertChain(ToBlocks(forks[i : i+1])); err != nil {
			t.Fatalf("fork %d: failed to insert into chain: %v", i, err)
		}
	}
}

// Tests that doing large reorgs works even if the state associated with the
// forking point is not available any more.
func TestLargeReorgTrieGC(t *testing.T) {
	// Generate the original common chain segment and the two competing forks
	db := ethdb.NewMemDatabase()
	qkcconfig.SkipRootCoinbaseCheck = true
	genesis := NewGenesis(qkcconfig)
	genesisBlock := genesis.MustCommitRootBlock(db)
	engine := new(consensus.FakeEngine)

	shared := GenerateRootBlockChain(genesisBlock, engine, 64, func(i int, b *RootBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{1}, FullShardKey: 1})
	})
	original := GenerateRootBlockChain(shared[len(shared)-1], engine, 2*triesInMemory, func(i int, b *RootBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{2}, FullShardKey: 1})
	})
	competitor := GenerateRootBlockChain(shared[len(shared)-1], engine, 2*triesInMemory+1, func(i int, b *RootBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{3}, FullShardKey: 1})
	})

	// Import the shared chain and the original canonical one
	diskdb := ethdb.NewMemDatabase()
	genesis.MustCommitRootBlock(diskdb)

	chain, err := NewRootBlockChain(diskdb, qkcconfig, engine)
	if err != nil {
		t.Fatalf("failed to create tester chain: %v", err)
	}
	if _, err := chain.InsertChain(ToBlocks(shared)); err != nil {
		t.Fatalf("failed to insert shared chain: %v", err)
	}
	if _, err := chain.InsertChain(ToBlocks(original)); err != nil {
		t.Fatalf("failed to insert original chain: %v", err)
	}
	// Import the competitor chain without exceeding the canonical's TD and ensure
	// we have not processed any of the blocks (protection against malicious blocks)
	if _, err := chain.InsertChain(ToBlocks(competitor[:len(competitor)-2])); err != nil {
		t.Fatalf("failed to insert competitor chain: %v", err)
	}
	// Import the head of the competitor chain, triggering the reorg and ensure we
	// successfully reprocess all the stashed away blocks.
	if _, err := chain.InsertChain(ToBlocks(competitor[len(competitor)-2:])); err != nil {
		t.Fatalf("failed to finalize competitor chain: %v", err)
	}
}

func TestGetBlockCnt(t *testing.T) {
	var (
		addr1        = account.Address{Recipient: account.Recipient{1}, FullShardKey: 0}
		addr2        = account.Address{Recipient: account.Recipient{2}, FullShardKey: 0}
		db           = ethdb.NewMemDatabase()
		qkcconfig    = config.NewQuarkChainConfig()
		genesis      = Genesis{qkcConfig: qkcconfig}
		genesisBlock = genesis.MustCommitRootBlock(db)
		engine       = new(consensus.FakeEngine)
	)

	chain := GenerateRootBlockChain(genesisBlock, engine, 5, func(i int, gen *RootBlockGen) {
		switch i {
		case 0:
			// In block 1, addr1 sends addr2 some ether.
			header := types.MinorBlockHeader{Number: 1, Branch: account.Branch{Value: 2}, Coinbase: addr2, ParentHash: genesisBlock.Hash(), Time: genesisBlock.Time()}
			header1 := types.MinorBlockHeader{Number: 1, Branch: account.Branch{Value: 3}, Coinbase: addr1, ParentHash: genesisBlock.Hash(), Time: genesisBlock.Time()}
			gen.Headers = append(gen.Headers, &header)
			gen.Headers = append(gen.Headers, &header1)
		case 1:
			// In block 2, addr1 sends some more ether to addr2.
			// addr2 passes it on to addr3.
			header1 := types.MinorBlockHeader{Number: 2, Branch: account.Branch{Value: 3}, Coinbase: addr1, ParentHash: genesisBlock.Hash(), Time: genesisBlock.Time()}
			header2 := types.MinorBlockHeader{Number: 2, Branch: account.Branch{Value: 2}, Coinbase: addr2, ParentHash: header1.Hash(), Time: genesisBlock.Time()}
			gen.Headers = append(gen.Headers, &header1)
			gen.Headers = append(gen.Headers, &header2)
		}
	})

	// Import the chain. This runs all block validation rules.
	blockchain, err := NewRootBlockChain(db, qkcconfig, engine)
	if err != nil {
		fmt.Printf("new root block chain error %v\n", err)
		return
	}
	blockchain.SetEnableCountMinorBlocks(true)
	//defer blockchain.Stop()

	blockchain.SetValidator(&fakeRootBlockValidator{nil})
	if i, err := blockchain.InsertChain(ToBlocks(chain)); err != nil {
		fmt.Printf("insert error (block %d): %v\n", chain[i].NumberU64(), err)
		return
	}
	data, err := blockchain.GetBlockCount(blockchain.CurrentBlock().Number())
	assert.NoError(t, err)
	assert.Equal(t, data[2][addr1.Recipient], uint32(0))
	assert.Equal(t, data[2][addr2.Recipient], uint32(2))
	assert.Equal(t, data[3][addr1.Recipient], uint32(2))
	assert.Equal(t, data[3][addr2.Recipient], uint32(0))

}

func BenchmarkBlockChain_1x1000ValueTransferToNonexisting(b *testing.B) {
	var (
		numitems  = 1000
		numBlocks = 1
	)

	benchmarkLargeNumberOfValueToNonexisting(b, numitems, numBlocks)
}

// Benchmarks large blocks with value transfers to non-existing accounts
func benchmarkLargeNumberOfValueToNonexisting(b *testing.B, numItems, numBlocks int) {
	engine := new(consensus.FakeEngine)
	// Generate the original common chain segment and the two competing forks
	gspec := Genesis{qkcconfig}
	db := ethdb.NewMemDatabase()
	genesis := gspec.MustCommitRootBlock(db)

	blockGenerator := func(i int, block *RootBlockGen) {
		block.SetCoinbase(account.Address{Recipient: account.Recipient{1}, FullShardKey: 0})
		headers := make(types.MinorBlockHeaders, numItems, numItems)
		for index := 0; index < numItems; index++ {
			uniq := uint64(i*numItems + index)
			header := types.MinorBlockHeader{Version: 0, Number: uniq}
			headers[index] = &header
		}
		block.Headers = headers
	}

	shared := GenerateRootBlockChain(genesis, engine, numBlocks, blockGenerator)
	b.StopTimer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Import the shared chain and the original canonical one
		diskdb := ethdb.NewMemDatabase()
		gspec.MustCommitRootBlock(diskdb)

		chain, err := NewRootBlockChain(diskdb, qkcconfig, engine)
		chain.validator = new(fakeRootBlockValidator)
		if err != nil {
			b.Fatalf("failed to create tester chain: %v", err)
		}
		b.StartTimer()
		if _, err := chain.InsertChain(ToBlocks(shared)); err != nil {
			b.Fatalf("failed to insert shared chain: %v", err)
		}
		b.StopTimer()
		if got := chain.CurrentBlock().MinorBlockHeaders().Len(); got != numItems*numBlocks {
			b.Fatalf("Minor block header were not included, expected %d, got %d", (numItems * numBlocks), got)
		}
	}
}

func ToBlocks(rootBlocks []*types.RootBlock) []types.IBlock {
	blocks := make([]types.IBlock, len(rootBlocks))
	for i, block := range rootBlocks {
		blocks[i] = block
	}
	return blocks
}
