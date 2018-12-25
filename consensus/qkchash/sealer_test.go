package qkchash

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func TestSealAndVerifySeal(t *testing.T) {
	assert := assert.New(t)

	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(10)}
	q := New()

	block, err := q.Seal(nil, types.NewBlockWithHeader(header), nil)
	assert.NoError(err, "should have no problem sealing the block")

	// Correct
	header.Nonce = types.EncodeNonce(block.Nonce())
	header.MixDigest = block.MixDigest()
	err = q.VerifySeal(nil, header)
	assert.NoError(err, "should have correct nonce / mix digest")

	// Wrong
	header.Nonce = types.EncodeNonce(block.Nonce() - 1)
	err = q.VerifySeal(nil, header)
	assert.Error(err, "should have error because of the wrong nonce")
}
