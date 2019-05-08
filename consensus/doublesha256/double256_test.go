package doublesha256

import (
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/stretchr/testify/assert"
)

func TestVerifySeal(t *testing.T) {
	assert := assert.New(t)
	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 7, AdjustmentFactor: 512, MinimumDifficulty: big.NewInt(100000)}

	header := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(10)}
	rootBlock := types.NewRootBlockWithHeader(header)
	d := New(&diffCalculator, false)

	resultsCh := make(chan types.IBlock)
	err := d.Seal(nil, rootBlock, resultsCh, nil)
	assert.NoError(err, "should have no problem sealing the block")
	block := <-resultsCh

	// Correct
	header.Nonce = block.IHeader().GetNonce()
	header.MixDigest = block.IHeader().GetMixDigest()
	err = d.VerifySeal(nil, header, big.NewInt(0))
	assert.NoError(err, "should have correct nonce")

	// Wrong
	header.Nonce = 0
	err = d.VerifySeal(nil, header, big.NewInt(0))
	assert.Error(err, "should have error because of the wrong nonce")
}
