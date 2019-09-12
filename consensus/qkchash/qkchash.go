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
	// TODO: in the future cache may depend on block height
	cache qkcCache
	// A flag indicating which impl (c++ native or go) to use
	useNative bool
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
	copy(cache.Seed, cache.Hash)
	binary.LittleEndian.PutUint64(cache.Seed[32:], cache.Nonce)

	if q.useNative {
		cache.Digest, cache.Result, err = qkcHashNative(cache.Seed, q.cache)
	} else {
		cache.Digest, cache.Result, err = qkcHashGo(cache.Seed, q.cache)
	}
	return
}
func (q *QKCHash) RefreshWork(tip uint64) {
	q.CommonEngine.RefreshWork(tip)
}

// New returns a QKCHash scheme.
func New(useNative bool, diffCalculator consensus.DifficultyCalculator, remote bool, pubKey []byte) *QKCHash {
	q := &QKCHash{
		useNative: false,
		// TODO: cache may depend on block, so a LRU-stype cache could be helpful
		cache: generateCache(cacheEntryCnt, cacheSeed.Bytes(), false),
	}
	spec := consensus.MiningSpec{
		Name:       config.PoWQkchash,
		HashAlgo:   q.hashAlgo,
		VerifySeal: q.verifySeal,
	}
	//fmt.Println("NNNNNNNNNNNNN", q.cache.ls)
	q.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey)
	return q
}
