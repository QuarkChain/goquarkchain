package qkchash

import (
	"encoding/binary"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
)

// QKCHash is a consensus engine implementing PoW with qkchash algo.
// See the interface definition:
// https://github.com/ethereum/go-ethereum/blob/9e9fc87e70accf2b81be8772ab2ab0c914e95666/consensus/consensus.go#L111
// Implements consensus.Pow
type QKCHash struct {
	*consensus.CommonEngine
	cache          *cacheSeed
	qkcHashXHeight uint64
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (q *QKCHash) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (q *QKCHash) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction,
	receipts []*types.Receipt) (types.IBlock, error) {
	panic("not implemented")
}

func (q *QKCHash) hashAlgo(cache *consensus.ShareCache) (err error) {
	c := q.cache.getCacheFromHeight(cache.Height)
	copy(cache.Seed, cache.Hash)
	binary.LittleEndian.PutUint64(cache.Seed[32:], cache.Nonce)
	cache.Digest, cache.Result, err = qkcHashX(cache.Seed, c, cache.Height >= q.qkcHashXHeight)
	return
}

func (q *QKCHash) RefreshWork(tip uint64) {
	q.CommonEngine.RefreshWork(tip)
}

// New returns a QKCHash scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool, pubKey []byte, qkcHashXHeight uint64) *QKCHash {
	q := &QKCHash{
		// TODO: cache may depend on block, so a LRU-stype cache could be helpful
		cache:          NewcacheSeed(),
		qkcHashXHeight: qkcHashXHeight,
	}
	spec := consensus.MiningSpec{
		Name:       config.PoWQkchash,
		HashAlgo:   q.hashAlgo,
		VerifySeal: q.verifySeal,
	}
	q.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey)
	return q
}
