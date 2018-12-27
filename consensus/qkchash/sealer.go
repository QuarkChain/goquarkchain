package qkchash

import (
	"bytes"
	"math/big"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/ethereum/go-ethereum/common"
	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

// SealHash returns the hash of a block prior to it being sealed.
func (q *QKCHash) SealHash(header *types.Header) common.Hash {
	return q.commonEngine.SealHash(header)
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (q *QKCHash) Seal(
	chain ethconsensus.ChainReader,
	block *types.Block,
	results chan<- *types.Block,
	stop <-chan struct{}) error {

	return q.commonEngine.Seal(chain, block, results, stop)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (q *QKCHash) VerifySeal(chain ethconsensus.ChainReader, header *types.Header) error {
	if header.Difficulty.Sign() <= 0 {
		return consensus.ErrInvalidDifficulty
	}

	digest, result := q.hashAlgo(q.SealHash(header).Bytes(), header.Nonce.Uint64())

	if !bytes.Equal(header.MixDigest[:], digest) {
		return consensus.ErrInvalidMixDigest
	}
	target := new(big.Int).Div(two256, header.Difficulty)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}
