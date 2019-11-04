// Modified from go-ethereum under GNU Lesser General Public License

package core

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/common"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

// Tests that simple header verification works, for both good and bad blocks.
// and verify block later
func TestValidateBlock(t *testing.T) {
	// Create a simple chain to verify
	var (
		testdb           = ethdb.NewMemDatabase()
		qkcconfig        = config.NewQuarkChainConfig()
		gspec            = NewGenesis(qkcconfig)
		genesisrootBlock = gspec.MustCommitRootBlock(testdb)
		engine           = new(consensus.FakeEngine)
		rootBlocks       = GenerateRootBlockChain(genesisrootBlock, engine, 8, func(i int, b *RootBlockGen) {
			t := types.NewTokenBalancesWithMap(map[uint64]*big.Int{
				common.TokenIDEncode("QKC"): qkcconfig.Root.CoinbaseAmount,
			})
			b.header.CoinbaseAmount = t
		})
	)
	headers := make([]types.IHeader, len(rootBlocks))
	blocks := make([]types.IBlock, len(rootBlocks))
	for i, block := range rootBlocks {
		headers[i] = block.IHeader()
		blocks[i] = block
	}
	// Run the header checker for blocks one-by-one, checking for both valid and invalid nonces
	chain, _ := NewRootBlockChain(testdb, qkcconfig, engine)
	defer chain.Stop()

	for i := 0; i < len(blocks); i++ {
		for j, valid := range []bool{true, false} {
			var results <-chan error
			if valid {
				engine.Err = nil
			} else {
				engine.Err = fmt.Errorf("engine VerifyHeader error for block %d", i)
				engine.NumberToFail = blocks[i].NumberU64()
			}
			_, results = engine.VerifyHeaders(chain, []types.IHeader{headers[i]}, []bool{true})
			// Wait for the verification result
			select {
			case result := <-results:
				if (result == nil) != valid {
					t.Errorf("test %d.%d: validity mismatch: have %v, want %v", i, j, result, valid)
				}
			case <-time.After(time.Second):
				t.Fatalf("test %d.%d: verification timeout", i, j)
			}
			// Make sure no more data is returned
			select {
			case result := <-results:
				t.Fatalf("test %d.%d: unexpected result returned: %v", i, j, result)
			case <-time.After(25 * time.Millisecond):
			}
		}
		chain.InsertChain(blocks[i : i+1])
	}
	if chain.CurrentBlock().NumberU64() != 0 {
		t.Fatalf("verify chain CurrentBlock hight, have %d, want 0", chain.CurrentBlock().NumberU64())
	}

	engine.Err = nil
	for i := 0; i < len(blocks); i++ {
		_, err := chain.InsertChain(blocks[i : i+1])
		if err != nil {
			t.Fatalf("InsertChain failed with error: %v", err.Error())
		}
	}
}

// Tests that concurrent header verification works, for both good and bad blocks.
func TestHeaderConcurrentVerification2(t *testing.T)  { testHeaderConcurrentVerification(t, 2) }
func TestHeaderConcurrentVerification8(t *testing.T)  { testHeaderConcurrentVerification(t, 8) }
func TestHeaderConcurrentVerification32(t *testing.T) { testHeaderConcurrentVerification(t, 32) }

func testHeaderConcurrentVerification(t *testing.T, threads int) {
	// Create a simple chain to verify
	var (
		testdb           = ethdb.NewMemDatabase()
		qkcconfig        = config.NewQuarkChainConfig()
		gspec            = NewGenesis(qkcconfig)
		genesisrootBlock = gspec.MustCommitRootBlock(testdb)
		engine           = new(consensus.FakeEngine)
		rootBlocks       = GenerateRootBlockChain(genesisrootBlock, engine, 8, nil)
	)
	headers := make([]types.IHeader, len(rootBlocks))
	blocks := make([]types.IBlock, len(rootBlocks))
	seals := make([]bool, len(rootBlocks))
	for i, block := range rootBlocks {
		headers[i] = block.IHeader()
		blocks[i] = block
		seals[i] = true
	}

	// Set the number of threads to verify on
	old := runtime.GOMAXPROCS(threads)
	defer runtime.GOMAXPROCS(old)

	// Run the header checker for the entire block chain at once both for a valid and
	// also an invalid chain (enough if one arbitrary block is invalid).
	for i, valid := range []bool{true, false} {
		var results <-chan error

		if valid {
			engine.Err = nil
			chain, _ := NewRootBlockChain(testdb, qkcconfig, engine)
			_, results = chain.engine.VerifyHeaders(chain, headers, seals)
			chain.Stop()
		} else {
			engine.Err = fmt.Errorf("engine VerifyHeader error for block %d", i)
			engine.NumberToFail = blocks[len(headers)-1].NumberU64()
			chain, _ := NewRootBlockChain(testdb, qkcconfig, engine)
			_, results = chain.engine.VerifyHeaders(chain, headers, seals)
			chain.Stop()
		}
		// Wait for all the verification results
		checks := make(map[int]error)
		for j := 0; j < len(blocks); j++ {
			select {
			case result := <-results:
				checks[j] = result

			case <-time.After(time.Second):
				t.Fatalf("test %d.%d: verification timeout", i, j)
			}
		}
		// Check nonce check validity
		for j := 0; j < len(blocks); j++ {
			want := valid || (j < len(blocks)-1) // We chose the last-but-one nonce in the chain to fail
			if (checks[j] == nil) != want {
				t.Errorf("test %d.%d: validity mismatch: have %v, want %v", i, j, checks[j], want)
			}
			if !want {
				// A few blocks after the first error may pass verification due to concurrent
				// workers. We don't care about those in this test, just that the correct block
				// errors out.
				break
			}
		}
		// Make sure no more data is returned
		select {
		case result := <-results:
			t.Fatalf("test %d: unexpected result returned: %v", i, result)
		case <-time.After(25 * time.Millisecond):
		}
	}
}
