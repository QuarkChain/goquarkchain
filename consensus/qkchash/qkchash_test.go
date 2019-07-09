package qkchash

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/mocks/mock_consensus"
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
	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 1, AdjustmentFactor: 1, MinimumDifficulty: big.NewInt(3)}

	for _, qkcHashNativeFlag := range []bool{true, false} {
		q := New(qkcHashNativeFlag, &diffCalculator, false)

		parent := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(3), Time: 42}
		header := &types.RootBlockHeader{
			Number:     2,
			Difficulty: big.NewInt(3), // mock diff
			Time:       43,            // greater than parent
			ParentHash: parent.Hash(),
		}
		sealBlock(t, q, header)

		cr := mock_consensus.NewMockChainReader(ctrl)
		// No short-circuit
		cr.EXPECT().Config().Return(config.NewQuarkChainConfig()).AnyTimes()
		cr.EXPECT().GetHeader(header.Hash()).Return(nil).AnyTimes()
		cr.EXPECT().GetHeader(parent.Hash()).Return(parent).AnyTimes()
		err := q.VerifyHeader(cr, header, true)
		assert.NoError(err)

		// Reuse headers to test verifying a list of them
		var headers []types.IHeader
		for i := 1; i <= 5; i++ {
			// Add one bad block
			h := *header
			if i == 5 {
				h.Nonce = 123123
				cr.EXPECT().GetHeader(h.Hash()).Return(nil)
			}
			headers = append(headers, &h)
		}

		abort, errorCh := q.VerifyHeaders(cr, headers, nil)
		assert.NotNil(abort)

		errCnt, noErrCnt := 0, 0
		for i := 1; i <= 5; i++ {
			err := <-errorCh
			if err != nil {
				errCnt++
			} else {
				noErrCnt++
			}
		}
		assert.Equal(5, noErrCnt)
		assert.Equal(0, errCnt)
	}
}

func sealBlock(t *testing.T, q *QKCHash, h *types.RootBlockHeader) {
	resultsCh := make(chan types.IBlock)
	rootBlock := types.NewRootBlockWithHeader(h)
	stop := make(chan struct{})
	err := q.Seal(nil, rootBlock, nil, resultsCh, stop)
	assert.NoError(t, err, "should have no problem sealing the block")
	block := <-resultsCh
	close(stop)
	h.Nonce = block.IHeader().GetNonce()
	h.MixDigest = block.IHeader().GetMixDigest()
}
