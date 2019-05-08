package qkchash

import (
	"encoding/binary"

	"github.com/QuarkChain/goquarkchain/core/state"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
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

// Author returns coinbase address.
func (q *QKCHash) Author(header types.IHeader) (account.Address, error) {
	return header.GetCoinbase(), nil
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

func (q *QKCHash) hashAlgo(hash []byte, nonce uint64) (res consensus.MiningResult, err error) {
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, nonce)

	var digest, result []byte
	if q.useNative {
		digest, result, err = qkcHashNative(hash, nonceBytes, q.cache)
	} else {
		digest, result, err = qkcHashGo(hash, nonceBytes, q.cache)
	}
	if err != nil {
		return res, err
	}
	res = consensus.MiningResult{Digest: common.BytesToHash(digest), Result: result, Nonce: nonce}
	return res, nil
}

// New returns a QKCHash scheme.
func New(useNative bool, diffCalculator consensus.DifficultyCalculator, remote bool) *QKCHash {
	q := &QKCHash{
		useNative: useNative,
		// TODO: cache may depend on block, so a LRU-stype cache could be helpful
		cache: generateCache(cacheEntryCnt, cacheSeed, useNative),
	}
	spec := consensus.MiningSpec{
		Name:       "QKCHash",
		HashAlgo:   q.hashAlgo,
		VerifySeal: q.verifySeal,
	}
	q.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote)
	return q
}
