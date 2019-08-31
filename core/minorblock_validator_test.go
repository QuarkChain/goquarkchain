// Copyright 2015 The go-ethereum Authors
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
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"runtime"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

func toMinorBlocks(minorBlocks []*types.MinorBlock) []types.IBlock {
	blocks := make([]types.IBlock, len(minorBlocks))
	for i, block := range minorBlocks {
		blocks[i] = block
	}
	return blocks
}

func ToMinorHeaders(minorHeaders []*types.MinorBlockHeader) []types.IHeader {
	blocks := make([]types.IHeader, len(minorHeaders))
	for i, block := range minorHeaders {
		blocks[i] = block
	}
	return blocks
}

// Tests that simple header verification works, for both good and bad blocks.
func TestHeaderVerification(t *testing.T) {
	// Create a simple chain to verify
	var (
		fakeClusterConfig = config.NewClusterConfig()
		engine            = &consensus.FakeEngine{}
		fakeFullShardID   = fakeClusterConfig.Quarkchain.Chains[0].ShardSize | 0
		testdb            = ethdb.NewMemDatabase()
		gspec             = &Genesis{qkcConfig: fakeClusterConfig.Quarkchain}
		rootBlock         = gspec.CreateRootBlock()
		genesisBlock      = gspec.MustCommitMinorBlock(testdb, rootBlock, fakeFullShardID)
	)
	chainConfig := params.TestChainConfig
	// Run the header checker for blocks one-by-one, checking for both valid and invalid nonces
	chain, _ := NewMinorBlockChain(testdb, nil, chainConfig, fakeClusterConfig, engine, vm.Config{}, nil, fakeFullShardID)
	genesisBlock, err := chain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	defer chain.Stop()
	blocks, _ := GenerateMinorBlockChain(params.TestChainConfig, fakeClusterConfig.Quarkchain, genesisBlock, engine, testdb, 1, nil)
	headers := make([]*types.MinorBlockHeader, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}
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
		chain.InsertChain(toMinorBlocks(blocks[i:i+1]), false)
	}
}

// Tests that concurrent header verification works, for both good and bad blocks.
func TestMinorHeaderConcurrentVerification2(t *testing.T) { testMinorHeaderConcurrentVerification(t, 2) }
func TestMinorHeaderConcurrentVerification8(t *testing.T) { testMinorHeaderConcurrentVerification(t, 8) }
func TestMinorHeaderConcurrentVerification32(t *testing.T) {
	testMinorHeaderConcurrentVerification(t, 32)
}

func testMinorHeaderConcurrentVerification(t *testing.T, threads int) {
	// Create a simple chain to verify
	var (
		fakeClusterConfig = config.NewClusterConfig()
		engine            = &consensus.FakeEngine{}
		fakeFullShardID   = fakeClusterConfig.Quarkchain.Chains[0].ShardSize | 0
		testdb            = ethdb.NewMemDatabase()
		gspec             = &Genesis{qkcConfig: config.NewQuarkChainConfig()}
		rootBlock         = gspec.CreateRootBlock()
		genesis           = gspec.MustCommitMinorBlock(testdb, rootBlock, fakeFullShardID)
		blocks, _         = GenerateMinorBlockChain(params.TestChainConfig, fakeClusterConfig.Quarkchain, genesis, engine, testdb, 8, nil)
		err               = *new(error)
	)
	headers := make([]*types.MinorBlockHeader, len(blocks))
	seals := make([]bool, len(blocks))

	for i, block := range blocks {
		headers[i] = block.Header()
		seals[i] = true
	}
	// Set the number of threads to verify on
	old := runtime.GOMAXPROCS(threads)
	defer runtime.GOMAXPROCS(old)

	// Run the header checker for the entire block chain at once both for a valid and
	// also an invalid chain (enough if one arbitrary block is invalid).
	for i, valid := range []bool{true, false} {
		var results <-chan error
		chainConfig := params.TestChainConfig
		if valid {
			chain, _ := NewMinorBlockChain(testdb, nil, chainConfig, fakeClusterConfig, engine, vm.Config{}, nil, fakeFullShardID)
			genesis, err = chain.InitGenesisState(rootBlock)
			if err != nil {
				panic(err)
			}
			_, results = chain.engine.VerifyHeaders(chain, ToMinorHeaders(headers), seals)
			chain.Stop()
		} else {
			engine := new(consensus.FakeEngine)
			engine.Err = errors.New("err ")
			engine.NumberToFail = blocks[len(headers)-1].NumberU64()
			chain, _ := NewMinorBlockChain(testdb, nil, chainConfig, fakeClusterConfig, engine, vm.Config{}, nil, fakeFullShardID)
			genesis, err = chain.InitGenesisState(rootBlock)
			_, results = chain.engine.VerifyHeaders(chain, ToMinorHeaders(headers), seals)
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
