package qkchash

import (
	"encoding/binary"
	"github.com/QuarkChain/goquarkchain/core/state"
	"math/big"

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
	commonEngine   *consensus.CommonEngine
	diffCalculator consensus.DifficultyCalculator
	// TODO: in the future cache may depend on block height
	cache qkcCache
	// A flag indicating which impl (c++ native or go) to use
	useNative bool
}

// Author returns coinbase address.
func (q *QKCHash) Author(header types.IHeader) (account.Address, error) {
	return header.GetCoinbase(), nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (q *QKCHash) VerifyHeader(chain consensus.ChainReader, header types.IHeader, seal bool) error {
	return q.commonEngine.VerifyHeader(chain, header, seal, q)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (q *QKCHash) VerifyHeaders(chain consensus.ChainReader, headers []types.IHeader, seals []bool) (chan<- struct{}, <-chan error) {
	return q.commonEngine.VerifyHeaders(chain, headers, seals, q)
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (q *QKCHash) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (q *QKCHash) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction,
	uncles []types.IHeader, receipts []*types.Receipt) (types.IBlock, error) {
	panic("not implemented")
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (q *QKCHash) CalcDifficulty(chain consensus.ChainReader, time uint64, parent types.IHeader) *big.Int {
	if q.diffCalculator == nil {
		panic("diffCalculator is not existed")
	}

	return q.diffCalculator.CalculateDifficulty(parent, time)
}

// Hashrate returns the current mining hashrate of a PoW consensus engine.
func (q *QKCHash) Hashrate() float64 {
	return q.commonEngine.Hashrate()
}

// Close terminates any background threads maintained by the consensus engine.
func (q *QKCHash) Close() error {
	return nil
}

// FindNonce finds the desired nonce and mixhash for a given block header.
func (q *QKCHash) FindNonce(
	work consensus.MiningWork,
	results chan<- consensus.MiningResult,
	stop <-chan struct{},
) error {
	return q.commonEngine.FindNonce(work, results, stop)
}

// Name returns the consensus engine's name.
func (q *QKCHash) Name() string {
	return q.commonEngine.Name()
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
func New(useNative bool, diffCalculator consensus.DifficultyCalculator) *QKCHash {
	q := &QKCHash{
		diffCalculator: diffCalculator,
		useNative:      useNative,
		// TOOD: cache may depend on block, so a LRU-stype cache could be helpful
		cache: generateCache(cacheEntryCnt, cacheSeed, useNative),
	}
	spec := consensus.MiningSpec{
		Name:     "QKCHash",
		HashAlgo: q.hashAlgo,
	}
	q.commonEngine = consensus.NewCommonEngine(spec)
	return q
}
