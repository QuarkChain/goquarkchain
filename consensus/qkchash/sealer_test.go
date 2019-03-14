package qkchash

import (
	"github.com/QuarkChain/goquarkchain/consensus"
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/stretchr/testify/assert"
)

func TestSealAndVerifySeal(t *testing.T) {
	assert := assert.New(t)
	diffCalculator := consensus.EthDifficultyCalculator{7, 512, big.NewInt(100000)}

	header := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(10)}
	for _, qkcHashNativeFlag := range []bool{true, false} {
		q := New(qkcHashNativeFlag, &diffCalculator)
		rootBlock := types.NewRootBlockWithHeader(header)
		resultsCh := make(chan types.IBlock)
		err := q.Seal(nil, rootBlock, resultsCh, nil)
		assert.NoError(err, "should have no problem sealing the block")
		block := <-resultsCh

		// Correct
		header.Nonce = block.IHeader().GetNonce()
		header.MixDigest = block.IHeader().GetMixDigest()
		err = q.VerifySeal(nil, header)
		assert.NoError(err, "should have correct nonce / mix digest")

		// Wrong
		header.Nonce = block.IHeader().GetNonce() - 1
		err = q.VerifySeal(nil, header)
		assert.Error(err, "should have error because of the wrong nonce")
	}
}
