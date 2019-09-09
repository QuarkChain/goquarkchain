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

package core

import (
	"encoding/hex"
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
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

// So we can deterministically seed different blockchains
var (
	FakeQkcConfig = config.NewQuarkChainConfig()
	engine        = new(consensus.FakeEngine)
)
var (
	errInvalidChainID = errors.New("invalid chain id for signer")
)

func ToHeaders(minorBlockHeader []*types.MinorBlockHeader) []types.IHeader {
	blocks := make([]types.IHeader, len(minorBlockHeader))
	for i, block := range minorBlockHeader {
		blocks[i] = block
	}
	return blocks
}

// newMinorCanonical creates a chain database, and injects a deterministic canonical
// chain. Depending on the full flag, if creates either a full block chain or a
// header only chain.
func newMinorCanonical(cacheConfig *CacheConfig, engine consensus.Engine, n int, full bool) (ethdb.Database, *MinorBlockChain, error) {
	var (
		fakeClusterConfig = config.NewClusterConfig()
		fakeFullShardID   = fakeClusterConfig.Quarkchain.Chains[0].ShardSize | 0
		db                = ethdb.NewMemDatabase()
		gspec             = &Genesis{qkcConfig: config.NewQuarkChainConfig()}
		rootBlock         = gspec.CreateRootBlock()
		genesis           = gspec.MustCommitMinorBlock(db, rootBlock, fakeFullShardID)
	)
	fakeClusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	fakeClusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	fakeClusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	// Initialize a fresh chain with only a genesis block
	chainConfig := params.TestChainConfig
	blockchain, _ := NewMinorBlockChain(db, cacheConfig, chainConfig, fakeClusterConfig, engine, vm.Config{}, nil, fakeFullShardID)
	genesis, err := blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	// Create and inject the requested chain
	if n == 0 {
		return db, blockchain, nil
	}
	if full {
		// Full block-chain requested
		blocks := makeBlockChain(genesis, n, engine, db, canonicalSeed)
		_, err := blockchain.InsertChain(toMinorBlocks(blocks), false)
		return db, blockchain, err
	}
	// Header-only chain requested
	headers := makeHeaderChain(genesis.Header(), genesis.Meta(), n, engine, db, canonicalSeed)
	_, err = blockchain.InsertHeaderChain(ToHeaders(headers), 1)
	return db, blockchain, err
}

// TestMinor fork of length N starting from block i
func testMinorFork(t *testing.T, blockchain *MinorBlockChain, i, n int, full bool, comparator func(td1, td2 *big.Int)) {
	// Copy old chain up to #i into a new db
	engine := &consensus.FakeEngine{}
	db, blockchain2, err := newMinorCanonical(nil, engine, i, full)
	if err != nil {
		t.Fatal("could not make new canonical in testMinorFork", err)
	}
	defer blockchain2.Stop()

	// Assert the chains have the same header/block at #i
	var hash1, hash2 common.Hash
	if full {
		hash1 = blockchain.GetBlockByNumber(uint64(i)).Hash()
		hash2 = blockchain2.GetBlockByNumber(uint64(i)).Hash()
	} else {
		hash1 = blockchain.GetHeaderByNumber(uint64(i)).Hash()
		hash2 = blockchain2.GetHeaderByNumber(uint64(i)).Hash()
	}
	if hash1 != hash2 {
		t.Errorf("chain content mismatch at %d: have hash %v, want hash %v", i, hash2, hash1)
	}
	// Extend the newly created chain
	var (
		blockChainB  []*types.MinorBlock
		headerChainB []*types.MinorBlockHeader
	)
	if full {
		blockChainB = makeBlockChain(blockchain2.CurrentBlock(), n, engine, db, forkSeed)
		if _, err := blockchain2.InsertChain(toMinorBlocks(blockChainB), false); err != nil {
			t.Fatalf("failed to insert forking chain: %v", err)
		}
	} else {
		headerChainB = makeHeaderChain(blockchain2.CurrentHeader().(*types.MinorBlockHeader), blockchain2.CurrentBlock().Meta(), n, engine, db, forkSeed)
		if _, err := blockchain2.InsertHeaderChain(ToHeaders(headerChainB), 1); err != nil {
			t.Fatalf("failed to insert forking chain: %v", err)
		}
	}
	// Sanity check that the forked chain can be imported into the original
	var tdPre, tdPost *big.Int

	if full {
		tdPre = blockchain.GetTdByHash(blockchain.CurrentBlock().Hash())
		if err := testMinorBlockChainImport(toMinorBlocks(blockChainB), blockchain); err != nil {
			t.Fatalf("failed to import forked block chain: %v", err)
		}
		tdPost = blockchain.GetTdByHash(blockChainB[len(blockChainB)-1].Hash())
	} else {
		tdPre = blockchain.GetTdByHash(blockchain.CurrentHeader().Hash())
		if err := testMinorHeaderChainImport(headerChainB, blockchain); err != nil {
			t.Fatalf("failed to import forked header chain: %v", err)
		}
		tdPost = blockchain.GetTdByHash(headerChainB[len(headerChainB)-1].Hash())
	}
	// Compare the total difficulties of the chains
	comparator(tdPre, tdPost)
}

func printMinorChain(bc *MinorBlockChain) {
	for i := bc.CurrentBlock().NumberU64(); i > 0; i-- {
		b := bc.GetBlockByNumber(uint64(i))
		fmt.Printf("\t%x %v\n", b.Hash(), b.IHeader().GetDifficulty())
	}
}

// testMinorBlockChainImport tries to process a chain of blocks, writing them into
// the database if successful.
func testMinorBlockChainImport(chain []types.IBlock, blockchain *MinorBlockChain) error {
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
		statedb, err := state.New(blockchain.GetMinorBlock(block.IHeader().GetParentHash()).GetMetaData().Root, blockchain.stateCache)
		if err != nil {
			return err
		}
		statedb.SetTxCursorInfo(block.(*types.MinorBlock).Meta().XShardTxCursorInfo)
		receipts, _, usedGas, err := blockchain.Processor().Process(block.(*types.MinorBlock), statedb, vm.Config{})
		if err != nil {
			blockchain.reportBlock(block, receipts, err)
			return err
		}
		err = blockchain.validator.ValidateState(block, blockchain.GetMinorBlock(block.IHeader().GetParentHash()), statedb, receipts, usedGas)
		if err != nil {
			blockchain.reportBlock(block, receipts, err)
			return err
		}
		blockchain.mu.Lock()
		rawdb.WriteTd(blockchain.db, block.Hash(), new(big.Int).Add(block.IHeader().GetDifficulty(), blockchain.GetTdByHash(block.IHeader().GetParentHash())))
		rawdb.WriteMinorBlock(blockchain.db, block.(*types.MinorBlock))
		statedb.Commit(true)
		blockchain.mu.Unlock()
	}
	return nil
}

// testMinorHeaderChainImport tries to process a chain of header, writing them into
// the database if successful.
func testMinorHeaderChainImport(chain []*types.MinorBlockHeader, blockchain *MinorBlockChain) error {
	for _, header := range chain {
		// Try and validate the header
		if err := blockchain.engine.VerifyHeader(blockchain, header, false); err != nil {
			return err
		}
		// Manually insert the header into the database, but don't reorganise (allows subsequent testMinoring)
		blockchain.mu.Lock()
		rawdb.WriteTd(blockchain.db, header.Hash(), new(big.Int).Add(header.Difficulty, blockchain.GetTdByHash(header.ParentHash)))
		rawdb.WriteMinorBlockHeader(blockchain.db, header)
		blockchain.mu.Unlock()
	}
	return nil
}

func insertChain(done chan bool, blockchain *MinorBlockChain, chain []types.IBlock, t *testing.T) {
	_, err := blockchain.InsertChain(chain, false)
	if err != nil {
		fmt.Println(err)
		t.FailNow()
	}
	done <- true
}

func TestMinorLastBlock(t *testing.T) {
	engine := &consensus.FakeEngine{}
	_, blockchain, err := newMinorCanonical(nil, engine, 0, true)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer blockchain.Stop()

	blocks := makeBlockChain(blockchain.CurrentBlock(), 1, engine, blockchain.db, 0)
	if _, err := blockchain.InsertChain(toMinorBlocks(blocks), false); err != nil {
		t.Fatalf("Failed to insert block: %v", err)
	}
	if blocks[len(blocks)-1].Hash() != rawdb.ReadHeadBlockHash(blockchain.db) {
		t.Fatalf("Write/Get HeadBlockHash failed")
	}
}

//TestMinors that given a starting canonical chain of a given size, it can be extended
//with various length chains.
func TestMinorExtendCanonicalHeaders(t *testing.T) { testMinorExtendCanonical(t, false) }
func TestMinorExtendCanonicalBlocks(t *testing.T)  { testMinorExtendCanonical(t, true) }

func testMinorExtendCanonical(t *testing.T, full bool) {
	length := 5
	// Make first chain starting from genesis
	engine := &consensus.FakeEngine{}
	_, processor, err := newMinorCanonical(nil, engine, length, full)
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
	testMinorFork(t, processor, length, 1, full, better)
	testMinorFork(t, processor, length, 2, full, better)
	testMinorFork(t, processor, length, 5, full, better)
	testMinorFork(t, processor, length, 10, full, better)
}

//TestMinors that given a starting canonical chain of a given size, creating shorter
//forks do not take canonical ownership.
func TestMinorShorterForkHeaders(t *testing.T) { testMinorShorterFork(t, false) }
func TestMinorShorterForkBlocks(t *testing.T)  { testMinorShorterFork(t, true) }

func testMinorShorterFork(t *testing.T, full bool) {
	length := 10
	// Make first chain starting from genesis
	engine := &consensus.FakeEngine{}
	_, processor, err := newMinorCanonical(nil, engine, length, full)
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
	testMinorFork(t, processor, 0, 3, full, worse)
	testMinorFork(t, processor, 0, 7, full, worse)
	testMinorFork(t, processor, 1, 1, full, worse)
	testMinorFork(t, processor, 1, 7, full, worse)
	testMinorFork(t, processor, 5, 3, full, worse)
	testMinorFork(t, processor, 5, 4, full, worse)
}

//TestMinors that given a starting canonical chain of a given size, creating longer
//forks do take canonical ownership.
func TestMinorLongerForkHeaders(t *testing.T) { testMinorLongerFork(t, false) }
func TestMinorLongerForkBlocks(t *testing.T)  { testMinorLongerFork(t, true) }

func testMinorLongerFork(t *testing.T, full bool) {
	length := 10
	// Make first chain starting from genesis
	engine := &consensus.FakeEngine{}
	_, processor, err := newMinorCanonical(nil, engine, length, full)
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
	testMinorFork(t, processor, 0, 11, full, better)
	testMinorFork(t, processor, 0, 15, full, better)
	testMinorFork(t, processor, 1, 10, full, better)
	testMinorFork(t, processor, 1, 12, full, better)
	testMinorFork(t, processor, 5, 6, full, better)
	testMinorFork(t, processor, 5, 8, full, better)
}

//
//TestMinors that given a starting canonical chain of a given size, creating equal
//forks do take canonical ownership.
func TestMinorEqualForkHeaders(t *testing.T) { testMinorEqualFork(t, false) }
func TestMinorEqualForkBlocks(t *testing.T)  { testMinorEqualFork(t, true) }

func testMinorEqualFork(t *testing.T, full bool) {
	length := 10

	// Make first chain starting from genesis
	engine := &consensus.FakeEngine{}
	_, processor, err := newMinorCanonical(nil, engine, length, full)
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
	testMinorFork(t, processor, 0, 10, full, equal)
	testMinorFork(t, processor, 1, 9, full, equal)
	testMinorFork(t, processor, 2, 8, full, equal)
	testMinorFork(t, processor, 5, 5, full, equal)
	testMinorFork(t, processor, 6, 4, full, equal)
	testMinorFork(t, processor, 9, 1, full, equal)
}

// TestMinors that chains missing links do not get accepted by the processor.
func TestMinorBrokenHeaderChain(t *testing.T) { testMinorBrokenChain(t, false) }
func TestMinorBrokenBlockChain(t *testing.T)  { testMinorBrokenChain(t, true) }

func testMinorBrokenChain(t *testing.T, full bool) {
	// Make chain starting from genesis
	engine := &consensus.FakeEngine{}
	db, blockchain, err := newMinorCanonical(nil, engine, 10, full)
	if err != nil {
		t.Fatalf("failed to make new canonical chain: %v", err)
	}
	defer blockchain.Stop()

	engine.Err = consensus.ErrUnknownAncestor
	engine.NumberToFail = 12
	// Create a forked chain, and try to insert with a missing link
	if full {
		chain := makeBlockChain(blockchain.CurrentBlock(), 5, engine, db, forkSeed)[1:]
		if err := testMinorBlockChainImport(toMinorBlocks(chain), blockchain); err == nil {
			t.Errorf("broken block chain not reported")
		}
	} else {
		chain := makeHeaderChain(blockchain.CurrentHeader().(*types.MinorBlockHeader), blockchain.CurrentBlock().Meta(), 5, engine, db, forkSeed)[1:]
		if err := testMinorHeaderChainImport(chain, blockchain); err == nil {
			t.Errorf("broken header chain not reported")
		}
	}
}

//
//TestMinors that reorganising a long difficult chain after a short easy one
//overwrites the canonical numbers and links in the database.
func TestMinorReorgLongHeaders(t *testing.T) { testMinorReorgLong(t, false) }
func TestMinorReorgLongBlocks(t *testing.T)  { testMinorReorgLong(t, true) }

func testMinorReorgLong(t *testing.T, full bool) {
	testMinorReorg(t, []uint64{10000, 10000, 10000}, []uint64{10000, 10000, 10000, 10000}, 40000, full)
}

//TestMinors that reorganising a short difficult chain after a long easy one
//overwrites the canonical numbers and links in the database.
func TestMinorReorgShortHeaders(t *testing.T) { testMinorReorgShort(t, false) }
func TestMinorReorgShortBlocks(t *testing.T)  { testMinorReorgShort(t, true) }

func testMinorReorgShort(t *testing.T, full bool) {
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
	testMinorReorg(t, easy, diff, 960000, full)
}

func testMinorReorg(t *testing.T, first, second []uint64, td int64, full bool) {
	engine := &consensus.FakeEngine{}
	db, blockchain, err := newMinorCanonical(nil, engine, 0, full)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer blockchain.Stop()

	// Insert an easy and a difficult chain afterwards
	easyBlocks, _ := GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), blockchain.CurrentBlock(), engine, db, len(first), func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetDifficulty(first[i])
	})
	diffBlocks, _ := GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), blockchain.CurrentBlock(), engine, db, len(second), func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetDifficulty(second[i])
	})
	if full {
		if _, err := blockchain.InsertChain(toMinorBlocks(easyBlocks), false); err != nil {
			t.Fatalf("failed to insert easy chain: %v", err)
		}
		if _, err := blockchain.InsertChain(toMinorBlocks(diffBlocks), false); err != nil {
			t.Fatalf("failed to insert difficult chain: %v", err)
		}
	} else {
		easyHeaders := make([]*types.MinorBlockHeader, len(easyBlocks))
		for i, block := range easyBlocks {
			easyHeaders[i] = block.Header()
		}
		diffHeaders := make([]*types.MinorBlockHeader, len(diffBlocks))
		for i, block := range diffBlocks {
			diffHeaders[i] = block.Header()
		}
		if _, err := blockchain.InsertHeaderChain(ToHeaders(easyHeaders), 1); err != nil {
			t.Fatalf("failed to insert easy chain: %v", err)
		}

		if _, err := blockchain.InsertHeaderChain(ToHeaders(diffHeaders), 1); err != nil {
			t.Fatalf("failed to insert difficult chain: %v", err)
		}
	}
	// Check that the chain is valid number and link wise
	if full {
		prev := blockchain.CurrentBlock()
		for block := blockchain.GetBlockByNumber(blockchain.CurrentBlock().NumberU64() - 1); block.NumberU64() != 0; prev, block = block.(*types.MinorBlock), blockchain.GetBlockByNumber(block.NumberU64()-1) {
			if prev.ParentHash() != block.Hash() {
				t.Errorf("parent block hash mismatch: have %x, want %x", prev.ParentHash(), block.Hash())
			}
		}
	} else {
		prev := blockchain.CurrentHeader()
		for header := blockchain.GetHeaderByNumber(blockchain.CurrentHeader().NumberU64() - 1); header.NumberU64() != 0; prev, header = header, blockchain.GetHeaderByNumber(header.NumberU64()-1) {
			if prev.GetParentHash() != header.Hash() {
				t.Errorf("parent header hash mismatch: have %x, want %x", prev.GetParentHash(), header.Hash())
			}
		}
	}

	// Make sure the chain total difficulty is the correct one
	want := new(big.Int).Add(blockchain.genesisBlock.Difficulty(), big.NewInt(td))
	if full {
		if have := blockchain.GetTdByHash(blockchain.CurrentBlock().Hash()); have.Cmp(want) != 0 {
			t.Errorf("total difficulty mismatch: have %v, want %v", have, want)
		}
	} else {
		if have := blockchain.GetTdByHash(blockchain.CurrentHeader().Hash()); have.Cmp(want) != 0 {
			t.Errorf("total difficulty mismatch: have %v, want %v", have, want)
		}
	}
}

// TestMinors chain insertions in the face of one entity containing an invalid nonce.
func TestMinorHeadersInsertNonceError(t *testing.T) { testMinorInsertNonceError(t, false) }
func TestMinorBlocksInsertNonceError(t *testing.T)  { testMinorInsertNonceError(t, true) }

func testMinorInsertNonceError(t *testing.T, full bool) {
	for i := 1; i < 10 && !t.Failed(); i++ {
		// Create a pristine chain and database
		engine := &consensus.FakeEngine{}
		cacheConfig := &CacheConfig{
			TrieCleanLimit: 32,
			TrieDirtyLimit: 32,
			TrieTimeLimit:  5 * time.Minute,
		}

		db, blockchain, err := newMinorCanonical(cacheConfig, engine, 0, full)
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
		if full {
			blocks := makeBlockChain(blockchain.CurrentBlock(), i, engine, db, 0)

			failAt = rand.Int() % len(blocks)
			failNum = blocks[failAt].NumberU64()

			engine.NumberToFail = failNum
			engine.Err = errors.New("fack engine expected fail")
			failRes, err = blockchain.InsertChain(toMinorBlocks(blocks), false)
		} else {
			headers := makeHeaderChain(blockchain.CurrentHeader().(*types.MinorBlockHeader), blockchain.CurrentBlock().Meta(), i, engine, db, 0)

			failAt = rand.Int() % len(headers)
			failNum = headers[failAt].Number
			engine.NumberToFail = failNum
			engine.Err = errors.New("fack engine expected fail")
			blockchain.hc.engine = blockchain.engine
			failRes, err = blockchain.InsertHeaderChain(ToHeaders(headers), 1)
		}
		// Check that the returned error indicates the failure
		if failRes != failAt {
			t.Errorf("test %d: failure (%v) index mismatch: have %d, want %d", i, err, failRes, failAt)
		}
		// Check that all blocks after the failing block have been inserted
		for j := 0; j < i-failAt; j++ {
			if full {
				if block := blockchain.GetBlockByNumber(failNum + uint64(j)); block != nil {
					t.Errorf("test %d: invalid block in chain: %v", i, block)
				}
			} else {
				if header := blockchain.GetHeaderByNumber(failNum + uint64(j)); header != nil {
					t.Errorf("test %d: invalid header in chain: %v", i, header)
				}
			}
		}
	}
}

// TestMinors that fast importing a block chain produces the same chain data as the
// classical full block processing.
func TestMinorFastVsFullChains(t *testing.T) {
	// Configure and generate a sample block chain
	var (
		id1, _ = account.CreatRandomIdentity()

		addr1         = account.CreatAddressFromIdentity(id1, 0)
		gendb         = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
	)

	prvKey1, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	ids := clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		addr := addr1.AddressInShard(v)
		shardConfig := clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		temp := make(map[string]*big.Int)
		temp["QKC"] = big.NewInt(1000000)
		alloc := config.Allocation{Balances: temp}
		shardConfig.Genesis.Alloc[addr] = alloc
	}

	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	genesis := gspec.MustCommitMinorBlock(gendb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	engine := &consensus.FakeEngine{}
	chainConfig := params.TestChainConfig
	blocks, receipts := GenerateMinorBlockChain(chainConfig, clusterConfig.Quarkchain, genesis, engine, gendb, 1024, func(config *config.QuarkChainConfig, i int, block *MinorBlockGen) {
		account := account.NewAddress(account.BytesToIdentityRecipient([]byte{}), 0)
		block.SetCoinbase(account)
		// If the block number is multiple of 3, send a few bonus transactions to the miner
		if i%3 == 2 {
			for j := 0; j < i%4+1; j++ {
				tx, err := types.SignTx(types.NewEvmTransaction(block.TxNonce(addr1.Recipient), account.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), types.MakeSigner(0), prvKey1)
				if err != nil {
					panic(err)
				}
				block.AddTx(config, transEvmTxToTx(tx))
			}
		}
		// If the block number is a multiple of 5, add a few bonus uncles to the block
		if i%5 == 5 {
			//	block.AddUncle(&types.Header{ParentHash: block.PrevBlock(i - 1).Hash(), Number: big.NewInt(int64(i - 1))})
		}
	})
	// Import the chain as an archive node for the comparison baseline
	archiveDb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(archiveDb, nil, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	archive, _ := NewMinorBlockChain(archiveDb, nil, chainConfig, clusterConfig, engine, vm.Config{}, nil, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	genesis, err = archive.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	defer archive.Stop()

	if n, err := archive.InsertChain(toMinorBlocks(blocks), false); err != nil {
		t.Fatalf("failed to process block %d: %v", n, err)
	}
	// Fast import the chain as a non-archive node to testMinor
	fastDb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(fastDb, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	fast, _ := NewMinorBlockChain(fastDb, nil, chainConfig, config.NewClusterConfig(), engine, vm.Config{}, nil, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	defer fast.Stop()

	headers := make([]*types.MinorBlockHeader, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	if n, err := fast.InsertHeaderChain(ToHeaders(headers), 1); err != nil {
		t.Fatalf("failed to insert header %d: %v", n, err)
	}
	if n, err := fast.InsertReceiptChain(toMinorBlocks(blocks), receipts); err != nil {
		t.Fatalf("failed to insert receipt %d: %v", n, err)
	}
	// Iterate over all chain data components, and cross reference
	for i := 0; i < len(blocks); i++ {
		num, hash := blocks[i].NumberU64(), blocks[i].Hash()

		if ftd, atd := fast.GetTdByHash(hash), archive.GetTdByHash(hash); ftd.Cmp(atd) != 0 {
			t.Errorf("block #%d [%x]: td mismatch: have %v, want %v", num, hash, ftd, atd)
		}
		if fheader, aheader := fast.GetHeaderByHash(hash), archive.GetHeaderByHash(hash); fheader.Hash() != aheader.Hash() {
			t.Errorf("block #%d [%x]: header mismatch: have %v, want %v", num, hash, fheader, aheader)
		}
		if fblock, ablock := fast.GetMinorBlock(hash), archive.GetMinorBlock(hash); fblock.Hash() != ablock.Hash() {
			t.Errorf("block #%d [%x]: block mismatch: have %v, want %v", num, hash, fblock, ablock)
		} else if types.DeriveSha(fblock.GetTransactions()) != types.DeriveSha(ablock.GetTransactions()) {
			t.Errorf("block #%d [%x]: transactions mismatch: have %v, want %v", num, hash, fblock.GetTransactions(), ablock.GetTransactions())
		}

		if freceipts, areceipts := rawdb.ReadReceipts(fastDb, hash), rawdb.ReadReceipts(archiveDb, hash); types.DeriveSha(freceipts) != types.DeriveSha(areceipts) {
			t.Errorf("block #%d [%x]: receipts mismatch: have %v, want %v", num, hash, freceipts, areceipts)
		}
	}
	// Check that the canonical chains are the same between the databases
	for i := 0; i < len(blocks)+1; i++ {
		if fhash, ahash := rawdb.ReadCanonicalHash(fastDb, rawdb.ChainTypeMinor, uint64(i)), rawdb.ReadCanonicalHash(archiveDb, rawdb.ChainTypeMinor, uint64(i)); fhash != ahash {
			t.Errorf("block #%d: canonical hash mismatch: have %v, want %v", i, fhash, ahash)
		}
	}
}

// TestMinors that various import methods move the chain head pointers to the correct
// positions.
func TestMinorLightVsFastVsFullChainHeads(t *testing.T) {
	// Configure and generate a sample block chain
	var (
		gendb         = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
		genesis   = gspec.MustCommitMinorBlock(gendb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	)
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	height := uint64(1024)
	engine := &consensus.FakeEngine{}
	blocks, receipts := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, gendb, int(height), nil)

	// Configure a subchain to roll back
	remove := []common.Hash{}
	for _, block := range blocks[height/2:] {
		remove = append(remove, block.Hash())
	}
	// Create a small assertion method to check the three heads
	assert := func(t *testing.T, kind string, chain *MinorBlockChain, header uint64, fast uint64, block uint64) {
		if num := chain.CurrentBlock().NumberU64(); num != block {
			t.Errorf("%s head block mismatch: have #%v, want #%v", kind, num, block)
		}
		//if num := chain.CurrentFastBlock().(); num != fast {
		//	t.Errorf("%s head fast-block mismatch: have #%v, want #%v", kind, num, fast)
		//}
		if num := chain.CurrentHeader().NumberU64(); num != header {
			t.Errorf("%s head header mismatch: have #%v, want #%v", kind, num, header)
		}
	}
	// Import the chain as an archive node and ensure all pointers are updated
	archiveDb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(archiveDb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)

	archive, _ := NewMinorBlockChain(archiveDb, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	genesis, err := archive.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	if n, err := archive.InsertChain(toMinorBlocks(blocks), false); err != nil {
		t.Fatalf("failed to process block %d: %v", n, err)
	}
	defer archive.Stop()

	assert(t, "archive", archive, height, height, height)
	archive.Rollback(remove)
	assert(t, "archive", archive, height/2, height/2, height/2)

	// Import the chain as a non-archive node and ensure all pointers are updated
	fastDb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(fastDb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	fast, _ := NewMinorBlockChain(fastDb, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	_, err = fast.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	defer fast.Stop()

	headers := make([]*types.MinorBlockHeader, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
	if n, err := fast.InsertHeaderChain(ToHeaders(headers), 1); err != nil {
		t.Fatalf("failed to insert header %d: %v", n, err)
	}
	if n, err := fast.InsertReceiptChain(toMinorBlocks(blocks), receipts); err != nil {
		t.Fatalf("failed to insert receipt %d: %v", n, err)
	}
	assert(t, "fast", fast, height, height, 0)
	fast.Rollback(remove)
	assert(t, "fast", fast, height/2, height/2, 0)

	// Import the chain as a light node and ensure all pointers are updated
	lightDb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(lightDb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)

	light, _ := NewMinorBlockChain(lightDb, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	_, err = light.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	if n, err := light.InsertHeaderChain(ToHeaders(headers), 1); err != nil {
		t.Fatalf("failed to insert header %d: %v", n, err)
	}
	defer light.Stop()

	assert(t, "light", light, height, 0, 0)
	light.Rollback(remove)
	assert(t, "light", light, height/2, 0, 0)
}

// TestMinors that chain reorganisations handle transaction removals and reinsertions.
func TestMinorChainTxReorgs(t *testing.T) {
	var (
		id1, _        = account.CreatRandomIdentity()
		id2, _        = account.CreatRandomIdentity()
		id3, _        = account.CreatRandomIdentity()
		addr1         = account.CreatAddressFromIdentity(id1, 0)
		addr2         = account.CreatAddressFromIdentity(id2, 0)
		addr3         = account.CreatAddressFromIdentity(id3, 0)
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
		signer    = types.NewEIP155Signer(uint32(gspec.qkcConfig.NetworkID))
	)

	prvKey1, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	prvKey2, err := crypto.HexToECDSA(hex.EncodeToString(id2.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}

	prvKey3, err := crypto.HexToECDSA(hex.EncodeToString(id3.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}

	ids := clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		temp := make(map[string]*big.Int)
		temp["QKC"] = big.NewInt(1000000)
		alloc := config.Allocation{Balances: temp}
		addr := addr1.AddressInShard(v)
		shardConfig := clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		shardConfig.Genesis.Alloc[addr] = alloc

		addr = addr2.AddressInShard(v)
		shardConfig = clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		shardConfig.Genesis.Alloc[addr] = alloc

		addr = addr3.AddressInShard(v)
		shardConfig = clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		shardConfig.Genesis.Alloc[addr] = alloc
	}
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	// Create two transactions shared between the chains:
	//  - postponed: transaction included at a later block in the forked chain
	//  - swapped: transaction included at the same block number in the forked chain
	postponed, _ := types.SignTx(types.NewEvmTransaction(0, addr1.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey1)
	swapped, _ := types.SignTx(types.NewEvmTransaction(1, addr1.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey1)

	// Create two transactions that will be dropped by the forked chain:
	//  - pastDrop: transaction dropped retroactively from a past block
	//  - freshDrop: transaction dropped exactly at the block where the reorg is detected
	var pastDrop, freshDrop *types.EvmTransaction

	// Create three transactions that will be added in the forked chain:
	//  - pastAdd:   transaction added before the reorganization is detected
	//  - freshAdd:  transaction added at the exact block the reorg is detected
	//  - futureAdd: transaction added after the reorg has already finished
	var pastAdd, freshAdd, futureAdd *types.EvmTransaction
	engine := &consensus.FakeEngine{}
	chain, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 3, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {
		switch i {
		case 0:
			pastDrop, _ = types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr2.Recipient), addr2.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey2)

			gen.AddTx(config, transEvmTxToTx(pastDrop))  // This transaction will be dropped in the fork from below the split point
			gen.AddTx(config, transEvmTxToTx(postponed)) // This transaction will be postponed till block #3 in the fork

		case 2:
			freshDrop, _ = types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr2.Recipient), addr2.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey2)

			gen.AddTx(config, transEvmTxToTx(freshDrop)) // This transaction will be dropped in the fork from exactly at the split point
			gen.AddTx(config, transEvmTxToTx(swapped))   // This transaction will be swapped out at the exact height

			gen.SetDifficulty(9) // Lower the block difficulty to simulate a weaker chain
		}
	})
	// Import the chain. This runs all block validation rules.
	blockchain, _ := NewMinorBlockChain(db, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	genesis, err = blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}

	if i, err := blockchain.InsertChain(toMinorBlocks(chain), false); err != nil {
		t.Fatalf("failed to insert original chain[%d]: %v", i, err)
	}
	defer blockchain.Stop()

	// overwrite the old chain
	chain, _ = GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 5, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {
		switch i {
		case 0:
			pastAdd, _ = types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr3.Recipient), addr3.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey3)
			gen.AddTx(config, transEvmTxToTx(pastAdd)) // This transaction needs to be injected during reorg

		case 2:
			gen.AddTx(config, transEvmTxToTx(postponed)) // This transaction was postponed from block #1 in the original chain
			gen.AddTx(config, transEvmTxToTx(swapped))   // This transaction was swapped from the exact current spot in the original chain

			freshAdd, _ = types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr3.Recipient), addr3.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey3)
			gen.AddTx(config, transEvmTxToTx(freshAdd)) // This transaction will be added exactly at reorg time

		case 3:
			futureAdd, _ = types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr3.Recipient), addr3.Recipient, big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey3)
			gen.AddTx(config, transEvmTxToTx(futureAdd)) // This transaction will be added after a full reorg
		}
	})

	if _, err := blockchain.InsertChain(toMinorBlocks(chain), false); err != nil {
		t.Fatalf("failed to insert forked chain: %v", err)
	}

	// removed tx
	for i, tx := range (types.Transactions{transEvmTxToTx(pastDrop), transEvmTxToTx(freshDrop)}) {
		if txn, _, _ := rawdb.ReadTransaction(db, tx.Hash()); txn != nil {
			t.Errorf("drop %d: tx %v found while shouldn't have been", i, txn)
		}
		if rcpt, _, _ := rawdb.ReadReceipt(db, tx.Hash()); rcpt != nil {
			t.Errorf("drop %d: receipt %v found while shouldn't have been", i, rcpt)
		}
	}
	// added tx
	for i, tx := range (types.Transactions{transEvmTxToTx(pastAdd), transEvmTxToTx(freshAdd), transEvmTxToTx(futureAdd)}) {
		if txn, _, _ := rawdb.ReadTransaction(db, tx.Hash()); txn == nil {
			t.Errorf("add %d: expected tx to be found", i)
		}
		if rcpt, _, _ := rawdb.ReadReceipt(db, tx.Hash()); rcpt == nil {
			t.Errorf("add %d: expected receipt to be found", i)
		}
	}
	// shared tx
	for i, tx := range (types.Transactions{transEvmTxToTx(postponed), transEvmTxToTx(swapped)}) {
		if txn, _, _ := rawdb.ReadTransaction(db, tx.Hash()); txn == nil {
			t.Errorf("share %d: expected tx to be found", i)
		}
		if rcpt, _, _ := rawdb.ReadReceipt(db, tx.Hash()); rcpt == nil {
			t.Errorf("share %d: expected receipt to be found", i)
		}
	}
}

func TestMinorLogReorgs(t *testing.T) {

	var (
		id1, _ = account.CreatRandomIdentity()

		addr1 = account.CreatAddressFromIdentity(id1, 0)
		db    = ethdb.NewMemDatabase()
		// this code generates a log
		//code          = common.Hex2Bytes("60606040525b7f24ec1d3ff24c2f6ff210738839dbc339cd45a5294d85c79361016243157aae7b60405180905060405180910390a15b600a8060416000396000f360606040526008565b00")
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
		signer    = types.NewEIP155Signer(uint32(gspec.qkcConfig.NetworkID))
	)
	prvKey1, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	ids := clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		addr := addr1.AddressInShard(v)
		shardConfig := clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		temp := make(map[string]*big.Int)
		temp["QKC"] = big.NewInt(1000000)
		alloc := config.Allocation{Balances: temp}
		shardConfig.Genesis.Alloc[addr] = alloc

	}
	engine := &consensus.FakeEngine{}
	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	blockchain, _ := NewMinorBlockChain(db, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	genesis, err = blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	defer blockchain.Stop()

	rmLogsCh := make(chan RemovedLogsEvent)
	blockchain.SubscribeRemovedLogsEvent(rmLogsCh)
	chain, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 2, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {
		if i == 1 {
			tx, err := types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr1.Recipient), addr1.Recipient, new(big.Int), 1000000, new(big.Int), 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey1)
			if err != nil {
				t.Fatalf("failed to create tx: %v", err)
			}
			gen.AddTx(config, transEvmTxToTx(tx))
		}
	})
	if _, err := blockchain.InsertChain(toMinorBlocks(chain), false); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}

	chain, _ = GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 3, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {})

	if _, err := blockchain.InsertChain(toMinorBlocks(chain), false); err != nil {
		t.Fatalf("failed to insert forked chain: %v", err)
	}

	//TODO later to fix
}

func TestMinorReorgSideEvent(t *testing.T) {
	var (
		id1, _ = account.CreatRandomIdentity()

		addr1         = account.CreatAddressFromIdentity(id1, 0)
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
		signer    = types.NewEIP155Signer(uint32(gspec.qkcConfig.NetworkID))
	)

	prvKey1, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	ids := clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		addr := addr1.AddressInShard(v)
		shardConfig := clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		temp := make(map[string]*big.Int)
		temp["QKC"] = big.NewInt(1000000)
		alloc := config.Allocation{Balances: temp}
		shardConfig.Genesis.Alloc[addr] = alloc

	}
	engine := &consensus.FakeEngine{}
	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	blockchain, _ := NewMinorBlockChain(db, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	genesis, err = blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	defer blockchain.Stop()

	chain, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 3, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {})
	if _, err := blockchain.InsertChain(toMinorBlocks(chain), false); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}

	replacementBlocks, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 4, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {
		tx, err := types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr1.Recipient), addr1.Recipient, new(big.Int), 1000000, new(big.Int), 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey1)
		if i == 2 {
			gen.SetDifficulty(100000000)
		}
		if err != nil {
			t.Fatalf("failed to create tx: %v", err)
		}
		gen.AddTx(config, transEvmTxToTx(tx))
	})
	chainSideCh := make(chan MinorChainSideEvent, 64)
	blockchain.SubscribeChainSideEvent(chainSideCh)
	if _, err := blockchain.InsertChain(toMinorBlocks(replacementBlocks), false); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}

	// first two block of the secondary chain are for a brief moment considered
	// side chains because up to that point the first one is considered the
	// heavier chain.
	expectedSideHashes := map[common.Hash]bool{
		replacementBlocks[0].Hash(): true,
		replacementBlocks[1].Hash(): true,
		replacementBlocks[2].Hash(): true,
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

// TestMinors if the canonical block can be fetched from the database during chain insertion.
func TestMinorCanonicalBlockRetrieval(t *testing.T) {
	engine := &consensus.FakeEngine{}
	_, blockchain, err := newMinorCanonical(nil, engine, 0, true)
	if err != nil {
		t.Fatalf("failed to create pristine chain: %v", err)
	}
	defer blockchain.Stop()

	chain, _ := GenerateMinorBlockChain(blockchain.ethChainConfig, blockchain.clusterConfig.Quarkchain, blockchain.genesisBlock, engine, blockchain.db, 10, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {})

	var pend sync.WaitGroup
	pend.Add(len(chain))

	for i := range chain {
		go func(block *types.MinorBlock) {
			defer pend.Done()

			// try to retrieve a block by its canonical hash and see if the block data can be retrieved.
			for {
				ch := rawdb.ReadCanonicalHash(blockchain.db, rawdb.ChainTypeMinor, block.NumberU64())
				if ch == (common.Hash{}) {
					continue // busy wait for canonical hash to be written
				}
				if ch != block.Hash() {
					t.Fatalf("unknown canonical hash, want %s, got %s", block.Hash().Hex(), ch.Hex())
				}
				fb := rawdb.ReadMinorBlock(blockchain.db, ch)
				if fb == nil {
					t.Fatalf("unable to retrieve block %d for canonical hash: %s", block.NumberU64(), ch.Hex())
				}
				if fb.Hash() != block.Hash() {
					t.Fatalf("invalid block hash for block %d, want %s, got %s", block.NumberU64(), block.Hash().Hex(), fb.Hash().Hex())
				}
				return
			}
		}(chain[i])

		if _, err := blockchain.InsertChain([]types.IBlock{chain[i]}, false); err != nil {
			t.Fatalf("failed to insert block %d: %v", i, err)
		}
	}
	pend.Wait()
}

func TestMinorEIP161AccountRemoval(t *testing.T) {
	//qkc delete empty addr in all height
	//eth delete empty addr depend height
	// Configure and generate a sample block chain
	var (
		id1, _        = account.CreatRandomIdentity()
		id2, _        = account.CreatRandomIdentity()
		addr1         = account.CreatAddressFromIdentity(id1, 0)
		addr2         = account.CreatAddressFromIdentity(id2, 0)
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
	)

	prvKey1, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	ids := clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		addr := addr1.AddressInShard(v)
		shardConfig := clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		temp := make(map[string]*big.Int)
		temp["QKC"] = big.NewInt(1000000)
		alloc := config.Allocation{Balances: temp}
		shardConfig.Genesis.Alloc[addr] = alloc

	}
	chainConfig := &params.ChainConfig{
		ChainID:        big.NewInt(1),
		HomesteadBlock: new(big.Int),
		EIP155Block:    new(big.Int),
		EIP158Block:    big.NewInt(2),
	}
	engine := &consensus.FakeEngine{}
	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	blockchain, _ := NewMinorBlockChain(db, nil, chainConfig, clusterConfig, engine, vm.Config{}, nil, clusterConfig.Quarkchain.Chains[0].ShardSize)
	genesis, err = blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	defer blockchain.Stop()

	blocks, _ := GenerateMinorBlockChain(chainConfig, config.NewQuarkChainConfig(), genesis, engine, db, 3, func(config *config.QuarkChainConfig, i int, block *MinorBlockGen) {
		var (
			tx     *types.EvmTransaction
			err    error
			signer = types.NewEIP155Signer(uint32(gspec.qkcConfig.NetworkID))
		)
		switch i {
		case 0:
			tx, err = types.SignTx(types.NewEvmTransaction(block.TxNonce(addr1.Recipient), addr2.Recipient, new(big.Int), 21000, new(big.Int), 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey1)
		case 1:
			tx, err = types.SignTx(types.NewEvmTransaction(block.TxNonce(addr1.Recipient), addr2.Recipient, new(big.Int), 21000, new(big.Int), 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey1)
		case 2:
			tx, err = types.SignTx(types.NewEvmTransaction(block.TxNonce(addr1.Recipient), addr2.Recipient, new(big.Int), 21000, new(big.Int), 0, 0, 3, 0, nil, genesisTokenID, genesisTokenID), signer, prvKey1)
		}
		if err != nil {
			t.Fatal(err)
		}
		block.AddTx(config, transEvmTxToTx(tx))
	})
	// account must exist pre eip 161
	if _, err := blockchain.InsertChain([]types.IBlock{blocks[0]}, false); err != nil {
		t.Fatal(err)
	}
	if st := blockchain.currentEvmState; st.Exist(addr2.Recipient) {
		t.Error("expected account to exist")
	}

	// account needs to be deleted post eip 161
	if _, err := blockchain.InsertChain([]types.IBlock{blocks[1]}, false); err != nil {
		t.Fatal(err)
	}
	if st, _ := blockchain.State(); st.Exist(addr2.Recipient) {
		t.Error("account should  exist")
	}

	// account musn't be created post eip 161 --do not care
	if _, err := blockchain.InsertChain([]types.IBlock{blocks[2]}, false); err != nil {
		t.Fatal(err)
	}
	if st, _ := blockchain.State(); st.Exist(addr2.Recipient) {
		t.Error("account should  exist")
	}
}

// This is a regression testMinor (i.e. as weird as it is, don't delete it ever), which
// testMinors that under weird reorg conditions the blockchain and its internal header-
// chain return the same latest block/header.
//
// https://github.com/ethereum/go-ethereum/pull/15941
func TestMinorBlockchainHeaderchainReorgConsistency(t *testing.T) {
	// Generate a canonical chain to act as the main dataset
	var (
		engine        = &consensus.FakeEngine{}
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
	)
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true

	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	blocks, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 64, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(common.Address{1}.Bytes()), 0))
	})

	// Generate a bunch of fork blocks, each side forking from the canonical chain
	forks := make([]*types.MinorBlock, len(blocks))
	for i := 0; i < len(forks); i++ {
		parent := genesis
		if i > 0 {
			parent = blocks[i-1]
		}
		fork, _ := GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), parent, engine, db, 1, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
			b.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(common.Address{2}.Bytes()), 0))
		})
		forks[i] = fork[0]
	}
	// Import the canonical and fork chain side by side, verifying the current block
	// and current header consistency
	diskdb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(diskdb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)

	chain, err := NewMinorBlockChain(diskdb, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	genesis, err = chain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	if err != nil {
		t.Fatalf("failed to create testMinorer chain: %v", err)
	}
	for i := 0; i < len(blocks); i++ {
		if _, err := chain.InsertChain(toMinorBlocks(blocks[i:i+1]), false); err != nil {
			t.Fatalf("block %d: failed to insert into chain: %v", i, err)
		}
		if chain.CurrentBlock().Hash() != chain.CurrentHeader().Hash() {
			t.Errorf("block %d: current block/header mismatch: block #%d [%x…], header #%d [%x…]", i, chain.CurrentBlock().Number(), chain.CurrentBlock().Hash().Bytes()[:4], chain.CurrentHeader().NumberU64(), chain.CurrentHeader().Hash().Bytes()[:4])
		}
		if _, err := chain.InsertChain(toMinorBlocks(forks[i:i+1]), false); err != nil {
			t.Fatalf(" fork %d: failed to insert into chain: %v", i, err)
		}
		if chain.CurrentBlock().Hash() != chain.CurrentHeader().Hash() {
			t.Errorf(" fork %d: current block/header mismatch: block #%d [%x…], header #%d [%x…]", i, chain.CurrentBlock().Number(), chain.CurrentBlock().Hash().Bytes()[:4], chain.CurrentHeader().NumberU64(), chain.CurrentHeader().Hash().Bytes()[:4])
		}
	}
}

// TestMinors that importing small side forks doesn't leave junk in the trie database
// cache (which would eventually cause memory issues).
func TestMinorTrieForkGC(t *testing.T) {
	// Generate a canonical chain to act as the main dataset
	var (
		engine        = &consensus.FakeEngine{}
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
	)
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true

	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	blocks, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 2*triesInMemory, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(common.Address{1}.Bytes()), 0))
	})

	// Generate a bunch of fork blocks, each side forking from the canonical chain
	forks := make([]*types.MinorBlock, len(blocks))
	for i := 0; i < len(forks); i++ {
		parent := genesis
		if i > 0 {
			parent = blocks[i-1]
		}
		fork, _ := GenerateMinorBlockChain(params.TestChainConfig, config.NewQuarkChainConfig(), parent, engine, db, 1, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
			b.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(common.Address{2}.Bytes()), 0))
		})
		forks[i] = fork[0]
	}
	// Import the canonical and fork chain side by side, forcing the trie cache to cache both
	diskdb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(diskdb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)

	chain, err := NewMinorBlockChain(diskdb, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	if err != nil {
		t.Fatalf("failed to create testMinorer chain: %v", err)
	}
	genesis, err = chain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	for i := 0; i < len(blocks); i++ {
		if _, err := chain.InsertChain(toMinorBlocks(blocks[i:i+1]), false); err != nil {
			t.Fatalf("block %d: failed to insert into chain: %v", i, err)
		}
		if _, err := chain.InsertChain(toMinorBlocks(forks[i:i+1]), false); err != nil {
			t.Fatalf("fork %d: failed to insert into chain: %v", i, err)
		}
	}
	// Dereference all the recent tries and ensure no past trie is left in
	for i := 0; i < triesInMemory; i++ {
		chain.stateCache.TrieDB().Dereference(blocks[len(blocks)-1-i].Root())
		chain.stateCache.TrieDB().Dereference(forks[len(blocks)-1-i].Root())
	}
	if len(chain.stateCache.TrieDB().Nodes()) > 0 {
		t.Fatalf("stale tries still alive after garbase collection")
	}
}

// TestMinors that doing large reorgs works even if the state associated with the
// forking point is not available any more.
func TestMinorLargeReorgTrieGC(t *testing.T) {
	// Generate the original common chain segment and the two competing forks
	var (
		engine        = &consensus.FakeEngine{}
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
	)
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	shared, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 64, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(common.Address{1}.Bytes()), 0))
	})
	original, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, shared[len(shared)-1], engine, db, 2*triesInMemory, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(common.Address{2}.Bytes()), 0))
	})
	competitor, _ := GenerateMinorBlockChain(params.TestChainConfig, clusterConfig.Quarkchain, shared[len(shared)-1], engine, db, 2*triesInMemory+1, func(config *config.QuarkChainConfig, i int, b *MinorBlockGen) {
		b.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(common.Address{3}.Bytes()), 0))
	})

	// Import the shared chain and the original canonical one
	diskdb := ethdb.NewMemDatabase()
	gspec.MustCommitMinorBlock(diskdb, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)

	cacheConfig := &CacheConfig{
		TrieCleanLimit: 256,
		TrieDirtyLimit: 256,
		TrieTimeLimit:  5 * time.Minute,
		Disabled:       false,
	}

	chain, err := NewMinorBlockChain(diskdb, cacheConfig, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	if err != nil {
		t.Fatalf("failed to create testMinorer chain: %v", err)
	}
	genesis, err = chain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	if _, err := chain.InsertChain(toMinorBlocks(shared), false); err != nil {
		t.Fatalf("failed to insert shared chain: %v", err)
	}

	if _, err := chain.InsertChain(toMinorBlocks(original), false); err != nil {
		t.Fatalf("failed to insert original chain: %v", err)
	}

	// Ensure that the state associated with the forking point is pruned away
	if node, _ := chain.stateCache.TrieDB().Node(shared[len(shared)-1].Root()); node != nil {
		t.Fatalf("common-but-old ancestor still cache")
	}
	// Import the competitor chain without exceeding the canonical's TD and ensure
	// we have not processed any of the blocks (protection against malicious blocks)

	if _, err := chain.InsertChain(toMinorBlocks(competitor[:len(competitor)-2]), false); err != nil {
		t.Fatalf("failed to insert competitor chain: %v", err)
	}
	for i, block := range competitor[:len(competitor)-2] {
		if node, _ := chain.stateCache.TrieDB().Node(block.Root()); node != nil {
			t.Fatalf("competitor %d: low TD chain became processed", i)
		}
	}
	// Import the head of the competitor chain, triggering the reorg and ensure we
	// successfully reprocess all the stashed away blocks.
	if _, err := chain.InsertChain(toMinorBlocks(competitor[len(competitor)-2:]), false); err != nil {
		t.Fatalf("failed to finalize competitor chain: %v", err)
	}
	for i, block := range competitor[:len(competitor)-triesInMemory] {
		if node, _ := chain.stateCache.TrieDB().Node(block.Root()); node != nil {
			t.Fatalf("competitor %d: competing chain state missing", i)
		}
	}
}

//TODO
//Bench test: qkc genesis not support code set
