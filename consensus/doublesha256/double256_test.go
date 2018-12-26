package doublesha256

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
)

func TestVerifySeal(t *testing.T) {
	assert := assert.New(t)

	header := &types.Header{Number: big.NewInt(1), Difficulty: big.NewInt(10)}
	d := New()
	nonce := uint64(85) // Hand-crafted nonce since mining is not implemented yet

	// Correct
	header.Nonce = types.EncodeNonce(nonce)
	err := d.VerifySeal(nil, header)
	assert.NoError(err, "should have correct nonce")

	// Wrong
	header.Nonce = types.EncodeNonce(nonce - 1)
	err = d.VerifySeal(nil, header)
	assert.Error(err, "should have error because of the wrong nonce")
}
