package qkchash

import (
	"bytes"
	"errors"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))

	errNoMiningWork = errors.New("no mining work available yet")
)

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (q *QKCHash) Seal(
	chain consensus.ChainReader,
	block types.IBlock,
	results chan<- types.IBlock,
	stop <-chan struct{}) error {
	if q.commonEngine.IsRemoteMining() {
		q.commonEngine.SetWork(block, results)
		return nil
	}
	return q.commonEngine.Seal(block, results, stop)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (q *QKCHash) VerifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	if header.GetDifficulty().Sign() <= 0 {
		return consensus.ErrInvalidDifficulty
	}
	if adjustedDiff.Cmp(new(big.Int).SetUint64(0)) == 0 {
		adjustedDiff = header.GetDifficulty()
	}

	miningRes, err := q.hashAlgo(header.SealHash().Bytes(), header.GetNonce())
	if err != nil {
		return err
	}
	if !bytes.Equal(header.GetMixDigest().Bytes(), miningRes.Digest.Bytes()) {
		return consensus.ErrInvalidMixDigest
	}
	target := new(big.Int).Div(two256, adjustedDiff)
	if new(big.Int).SetBytes(miningRes.Result).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}
