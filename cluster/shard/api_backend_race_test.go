//go:build race
// +build race

package shard

import (
	"sync"
	"testing"

	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
)

const raceIterations = 100

func TestAddBlockListForSyncMarkerBodyConsistencyUnderRace(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		sb, db, stop := newTestShardBackend(t)

		genesis := sb.MinorBlockChain.CurrentBlock()
		block := makeTestMinorBlocks(t, db, genesis, 1)[0]

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = sb.AddBlockListForSync([]*types.MinorBlock{block})
		}()
		go func() {
			defer wg.Done()
			_ = sb.MinorBlockChain.SetHead(genesis.NumberU64())
		}()
		wg.Wait()

		hasBody := rawdb.HasBlock(db, block.Hash())
		hasMarker := rawdb.HasCommitMinorBlock(db, block.Hash())
		stop()

		if hasMarker && !hasBody {
			t.Fatalf("iter %d: commit marker present but block body absent", i)
		}
	}
}

func TestAddMinorBlockMarkerBodyConsistencyUnderRace(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		sb, db, stop := newTestShardBackend(t)

		genesis := sb.MinorBlockChain.CurrentBlock()
		blockA := makeTestMinorBlocks(t, db, genesis, 1)[0]
		if _, err := sb.MinorBlockChain.InsertChain([]types.IBlock{blockA}, false); err != nil {
			stop()
			t.Fatalf("iter %d: pre-insert blockA: %v", i, err)
		}
		if !sb.MinorBlockChain.CommitMinorBlockByHash(blockA.Hash()) {
			stop()
			t.Fatalf("iter %d: pre-commit blockA failed", i)
		}

		blockAFork := makeTestForkBlock(t, db, genesis)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = sb.AddMinorBlock(blockAFork)
		}()
		go func() {
			defer wg.Done()
			_ = sb.MinorBlockChain.SetHead(genesis.NumberU64())
		}()
		wg.Wait()

		hasBody := rawdb.HasBlock(db, blockAFork.Hash())
		hasMarker := rawdb.HasCommitMinorBlock(db, blockAFork.Hash())
		stop()

		if hasMarker && !hasBody {
			t.Fatalf("iter %d: commit marker present but block body absent", i)
		}
	}
}
