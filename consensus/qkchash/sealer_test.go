package qkchash

import (
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func TestSealAndVerifySeal(t *testing.T) {
	assert := assert.New(t)
	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 7, AdjustmentFactor: 512, MinimumDifficulty: big.NewInt(100000)}

	header := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(10)}
	q := New(&diffCalculator, false, []byte{}, 100)
	rootBlock := types.NewRootBlockWithHeader(header)
	resultsCh := make(chan types.IBlock)
	err := q.Seal(nil, rootBlock, nil, 1, resultsCh, nil)
	assert.NoError(err, "should have no problem sealing the block")
	block := <-resultsCh

	// Correct
	header.Nonce = block.IHeader().GetNonce()
	header.MixDigest = block.IHeader().GetMixDigest()
	err = q.VerifySeal(nil, header, big.NewInt(0))
	assert.NoError(err, "should have correct nonce / mix digest")

	// Wrong
	header.Nonce = block.IHeader().GetNonce() - 1
	err = q.VerifySeal(nil, header, big.NewInt(0))
	assert.Error(err, "should have error because of the wrong nonce")
}

func TestRemoteSealer(t *testing.T) {
	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 7, AdjustmentFactor: 512, MinimumDifficulty: big.NewInt(100000)}
	header := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(100)}
	block := types.NewRootBlockWithHeader(header)

	qkc := New(&diffCalculator, true, []byte{}, 100)
	if _, err := qkc.GetWork(account.Address{}); err.Error() != errNoMiningWork.Error() {
		t.Error("expect to return an error indicate there is no mining work")
	}
	hash := block.Header().SealHash()

	var (
		work *consensus.MiningWork
		err  error
	)
	qkc.Seal(nil, block, nil, 1, nil, nil)
	if work, err = qkc.GetWork(account.Address{}); err != nil || work.HeaderHash != hash {
		t.Error("expect to return a mining work has same hash")
	}

	if res := qkc.SubmitWork(0, hash, common.Hash{}, nil); res {
		t.Error("expect to return false when submit a fake solution")
	}
}

func TestStaleSubmission(t *testing.T) {

	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 7, AdjustmentFactor: 512, MinimumDifficulty: big.NewInt(100000)}

	qkchash := New(&diffCalculator, false, []byte{}, 100)
	testcases := []struct {
		headers     []*types.RootBlockHeader
		submitIndex int
		submitRes   bool
	}{
		// Case1: submit solution for the latest mining package
		{
			[]*types.RootBlockHeader{
				{ParentHash: common.BytesToHash([]byte{0xa}), Number: 1, Difficulty: big.NewInt(100)},
			},
			0,
			true,
		},
		// Case2: submit solution for the previous package but have same parent.
		{
			[]*types.RootBlockHeader{
				{ParentHash: common.BytesToHash([]byte{0xb}), Number: 2, Difficulty: big.NewInt(100)},
				{ParentHash: common.BytesToHash([]byte{0xb}), Number: 2, Difficulty: big.NewInt(100)},
			},
			0,
			true,
		},
		// Case4: submit very old solution
		{
			[]*types.RootBlockHeader{
				{ParentHash: common.BytesToHash([]byte{0xe}), Number: 10, Difficulty: big.NewInt(100)},
				{ParentHash: common.BytesToHash([]byte{0xf}), Number: 17, Difficulty: big.NewInt(100)},
			},
			0,
			false,
		},
	}

	resultsCh := make(chan types.IBlock, 16)
	stop := make(chan struct{})

	for id, c := range testcases {
		for _, h := range c.headers {
			_ = qkchash.Seal(nil, types.NewRootBlockWithHeader(h), nil, 1, resultsCh, stop)
		}

		if !c.submitRes {
			continue
		}
		select {
		case res := <-resultsCh:
			if res.IHeader().GetDifficulty().Uint64() != c.headers[c.submitIndex].Difficulty.Uint64() {
				t.Errorf("case %d block difficulty mismatch, want %d, get %d", id+1, c.headers[c.submitIndex].Difficulty, res.IHeader().GetDifficulty())
			}
			if res.NumberU64() != c.headers[c.submitIndex].NumberU64() {
				t.Errorf("case %d block number mismatch, want %d, get %d", id+1, c.headers[c.submitIndex].NumberU64(), res.NumberU64())
			}
			if res.ParentHash() != c.headers[c.submitIndex].ParentHash {
				t.Errorf("case %d block parent hash mismatch, want %s, get %s", id+1, c.headers[c.submitIndex].ParentHash.Hex(), res.ParentHash().Hex())
			}
		}
	}
}

func TestTestGetWorkWithDifferentAddr(t *testing.T) {
	diffCalculator := consensus.EthDifficultyCalculator{AdjustmentCutoff: 7, AdjustmentFactor: 512, MinimumDifficulty: big.NewInt(100000)}
	header := &types.RootBlockHeader{Number: 1, Difficulty: big.NewInt(100)}
	block := types.NewRootBlockWithHeader(header)
	oldHash := block.Header().SealHash()

	qkc := New(&diffCalculator, true, []byte{}, 100)
	if _, err := qkc.GetWork(account.Address{}); err.Error() != errNoMiningWork.Error() {
		t.Error("expect to return an error indicate there is no mining work")
	}

	var (
		work *consensus.MiningWork
		err  error
	)
	qkc.Seal(nil, block, nil, 1, nil, nil)

	newID, _ := account.CreatRandomIdentity()
	newAddress := account.NewAddress(newID.GetRecipient(), 0)
	newHeader := block.Header()
	newHeader.SetCoinbase(newAddress)
	newBlock := types.NewRootBlockWithHeader(newHeader)
	newHash := newBlock.Header().SealHash()
	qkc.Seal(nil, newBlock, nil, 1, nil, nil)

	if work, err = qkc.GetWork(newAddress); err != nil || work.HeaderHash != newHash { //getWork with newAddress
		t.Error("expect to return a mining work has same hash")
	}

	if res := qkc.SubmitWork(0, newHash, common.Hash{}, nil); res {
		t.Error("expect to return false when submit a fake solution")
	}
	if res := qkc.SubmitWork(0, oldHash, common.Hash{}, nil); res {
		t.Error("expect to return false when submit a fake solution")
	}
}
