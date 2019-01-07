package qkchash

import (
	"encoding/binary"
	"math/big"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/ethereum/go-ethereum/common"
	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

// QKCHash is a consensus engine implementing PoW with qkchash algo.
// See the interface definition:
// https://github.com/ethereum/go-ethereum/blob/9e9fc87e70accf2b81be8772ab2ab0c914e95666/consensus/consensus.go#L111
// Implements consensus.Pow
type QKCHash struct {
	commonEngine *consensus.CommonEngine
	// For reusing existing functions
	ethash *ethash.Ethash
	// TODO: in the future cache may depend on block height
	cache qkcCache
}

// Author returns coinbase address.
func (q *QKCHash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (q *QKCHash) VerifyHeader(chain ethconsensus.ChainReader, header *types.Header, seal bool) error {
	return q.commonEngine.VerifyHeader(chain, header, seal, q)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (q *QKCHash) VerifyHeaders(chain ethconsensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	return q.commonEngine.VerifyHeaders(chain, headers, seals, q)
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (q *QKCHash) VerifyUncles(chain ethconsensus.ChainReader, block *types.Block) error {
	// For now QuarkChain won't verify uncles.
	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (q *QKCHash) Prepare(chain ethconsensus.ChainReader, header *types.Header) error {
	panic("not implemented")
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
func (q *QKCHash) Finalize(chain ethconsensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	panic("not implemented")
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (q *QKCHash) CalcDifficulty(chain ethconsensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	return big.NewInt(3)
}

// APIs returns the RPC APIs this consensus engine provides.
func (q *QKCHash) APIs(chain ethconsensus.ChainReader) []rpc.API {
	panic("not implemented")
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

func (q *QKCHash) hashAlgo(hash []byte, nonce uint64) consensus.MiningResult {
	// TOOD: cache may depend on block, so a LRU-stype cache could be helpful
	if len(q.cache.ls) == 0 {
		q.cache = generateCache(cacheEntryCnt, cacheSeed)
	}
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, nonce)
	digest, result := qkcHash(hash, nonceBytes, q.cache)
	return consensus.MiningResult{Digest: common.BytesToHash(digest), Result: result, Nonce: nonce}
}

// New returns a QKCHash scheme.
func New() *QKCHash {
	q := &QKCHash{
		ethash: &ethash.Ethash{},
	}
	spec := consensus.MiningSpec{
		Name:     "QKCHash",
		HashAlgo: q.hashAlgo,
	}
	q.commonEngine = consensus.NewCommonEngine(spec)
	return q
}
