// Modified from go-ethereum under GNU Lesser General Public License
package ethash

import (
	"io/ioutil"
	"math/big"
	"math/rand"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	qkconsensus "github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
)

// This test checks that cache lru logic doesn't crash under load.
// It reproduces https://github.com/ethereum/go-ethereum/issues/14943
func TestCacheFileEvict(t *testing.T) {
	tmpdir, err := ioutil.TempDir("", "ethash-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpdir)
	diffCalculator := qkconsensus.EthDifficultyCalculator{AdjustmentCutoff: 7, AdjustmentFactor: 512, MinimumDifficulty: big.NewInt(100000)}
	e := New(Config{CachesInMem: 3, CachesOnDisk: 10, CacheDir: tmpdir, PowMode: ModeTest}, &diffCalculator, false, []byte{})

	workers := 8
	epochs := 100
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go verifyTest(&wg, e, i, epochs)
	}
	wg.Wait()
}

func verifyTest(wg *sync.WaitGroup, e *QEthash, workerIndex, epochs int) {
	defer wg.Done()

	const wiggle = 4 * epochLength
	r := rand.New(rand.NewSource(int64(workerIndex)))
	for epoch := 0; epoch < epochs; epoch++ {
		block := int64(epoch)*epochLength - wiggle/2 + r.Int63n(wiggle)
		if block < 0 {
			block = 0
		}
		header := &types.RootBlockHeader{Number: uint32(block), Difficulty: big.NewInt(100)}
		e.verifySeal(nil, header, nil)
	}
}

func TestVerifySeal(t *testing.T) {
	assert := assert.New(t)
	diffCalculator := qkconsensus.EthDifficultyCalculator{AdjustmentCutoff: 7, AdjustmentFactor: 512, MinimumDifficulty: big.NewInt(100000)}

	header := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(10)}
	rootBlock := types.NewRootBlockWithHeader(header)
	e := New(Config{CachesInMem: 3, CachesOnDisk: 10, PowMode: ModeTest}, &diffCalculator, false, []byte{})

	resultsCh := make(chan types.IBlock)
	err := e.Seal(nil, rootBlock, nil, 1, resultsCh, nil)
	assert.NoError(err, "should have no problem sealing the block")
	block := <-resultsCh

	// Correct
	header.Nonce = block.IHeader().GetNonce()
	header.MixDigest = block.IHeader().GetMixDigest()
	err = e.VerifySeal(nil, header, big.NewInt(0))
	assert.NoError(err, "should have correct nonce")

	// Wrong
	header.Nonce = 0
	err = e.VerifySeal(nil, header, big.NewInt(0))
	assert.Error(err, "should have error because of the wrong nonce")
}
