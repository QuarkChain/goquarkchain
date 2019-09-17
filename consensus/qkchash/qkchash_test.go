package qkchash

import (
	"encoding/binary"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash/native"
	"github.com/QuarkChain/goquarkchain/mocks/mock_consensus"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestSealWithQKCX(t *testing.T) {
	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 1, AdjustmentFactor: 1, MinimumDifficulty: big.NewInt(3)}
	q := New(true, &diffCalculator, false, []byte{}, 0)
	q.SetThreads(1)

	parent := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(3), Time: 42, ToTalDifficulty: big.NewInt(3)}
	header := &types.RootBlockHeader{
		Number:          2,
		Difficulty:      big.NewInt(3), // mock diff
		Time:            43,            // greater than parent
		ParentHash:      parent.Hash(),
		ToTalDifficulty: big.NewInt(6),
	}

	minerRes := consensus.ShareCache{
		Height: header.NumberU64(),
		Hash:   header.SealHash().Bytes(),
		Seed:   make([]byte, 40),
		Nonce:  header.GetNonce(),
	}
	q.hashAlgo(&minerRes)

	//use hashX directly
	seedBlockNumber := getSeedFromBlockNumber(header.NumberU64())
	cacheS := make([]byte, 40)
	copy(cacheS, seedBlockNumber)
	binary.LittleEndian.PutUint64(cacheS[32:], header.Nonce)
	seed := crypto.Keccak512(cacheS)
	var seedArray [8]uint64
	for i := 0; i < 8; i++ {
		seedArray[i] = binary.LittleEndian.Uint64(seed[i*8:])
	}
	hashRes, err := native.HashWithRotationStats(q.cache.nativeCache, seedArray)
	assert.NoError(t, err)

	digest := make([]byte, common.HashLength)
	for i, val := range hashRes {
		binary.LittleEndian.PutUint64(digest[i*8:], val)
	}
	assert.Equal(t, digest, minerRes.Digest)
}
func TestVerifyHeaderAndHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	assert := assert.New(t)
	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 1, AdjustmentFactor: 1, MinimumDifficulty: big.NewInt(3)}

	for _, qkcHashNativeFlag := range []bool{true, false} {
		q := New(qkcHashNativeFlag, &diffCalculator, false, []byte{}, 100)
		parent := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(3), Time: 42, ToTalDifficulty: big.NewInt(3)}
		header := &types.RootBlockHeader{
			Number:          2,
			Difficulty:      big.NewInt(3), // mock diff
			Time:            43,            // greater than parent
			ParentHash:      parent.Hash(),
			ToTalDifficulty: big.NewInt(6),
		}
		sealBlock(t, q, header)

		cr := mock_consensus.NewMockChainReader(ctrl)
		// No short-circuit
		cr.EXPECT().Config().Return(config.NewQuarkChainConfig()).AnyTimes()
		cr.EXPECT().GetHeader(header.Hash()).Return(nil).AnyTimes()
		cr.EXPECT().GetHeader(parent.Hash()).Return(parent).AnyTimes()
		cr.EXPECT().SkipDifficultyCheck().Return(true).AnyTimes()
		cr.EXPECT().GetAdjustedDifficulty(gomock.Any()).Return(header.Difficulty, nil).AnyTimes()
		err := q.VerifyHeader(cr, header, true)
		assert.NoError(err)

		// Reuse headers to test verifying a list of them
		var headers []types.IHeader
		for i := 1; i <= 5; i++ {
			// Add one bad block
			h := *header
			if i == 5 {
				h.Nonce = 123123
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
		assert.Equal(4, noErrCnt)
		assert.Equal(1, errCnt)
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
