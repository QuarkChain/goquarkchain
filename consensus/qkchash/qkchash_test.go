package qkchash

import (
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestVerifyHeaderAndHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	assert := assert.New(t)

	for _, qkcHashNativeFlag := range []bool{true, false} {
		q := New(qkcHashNativeFlag)

		parent := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(10), Time: 42}
		header := &types.RootBlockHeader{
			Number:     2,
			Difficulty: big.NewInt(3), // mock diff
			Time:       43,            // greater than parent
			ParentHash: parent.Hash(),
		}
		sealBlock(t, q, header)

		cr := consensus.NewMockChainReader(ctrl)
		// No short-circuit
		cr.EXPECT().GetHeader(header.Hash(), uint64(2)).Return(nil).AnyTimes()
		cr.EXPECT().GetHeader(parent.Hash(), uint64(1)).Return(parent).AnyTimes()
		err := q.VerifyHeader(cr, header, true)
		assert.NoError(err)

		// Reuse headers to test verifying a list of them
		var headers []types.IHeader
		for i := 1; i <= 5; i++ {
			// Add one bad block
			h := *header
			if i == 5 {
				h.Nonce = 123123
				cr.EXPECT().GetHeader(h.Hash(), uint64(2)).Return(nil)
			}
			headers = append(headers, &h)
		}

		abort, errorCh := q.VerifyHeaders(cr, headers, nil)
		assert.Nil(abort)

		errCnt, noErrCnt := 0, 0
		for i := 1; i <= 5; i++ {
			err := <-errorCh
			if err != nil {
				errCnt++
			} else {
				noErrCnt++
			}
		}
		assert.Equal(4, noErrCnt)
		assert.Equal(1, errCnt)
	}
}

func sealBlock(t *testing.T, q *QKCHash, h *types.RootBlockHeader) {
	resultsCh := make(chan types.IBlock)
	rootBlock := types.NewRootBlockWithHeader(h)
	err := q.Seal(nil, rootBlock, resultsCh, nil)
	assert.NoError(t, err, "should have no problem sealing the block")
	block := <-resultsCh
	h.Nonce = block.IHeader().GetNonce()
	h.MixDigest = block.IHeader().GetMixDigest()
}
