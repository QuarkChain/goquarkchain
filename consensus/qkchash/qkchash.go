package qkchash

import (
	"errors"
	"math/big"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/ethereum/go-ethereum/common"
	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rpc"
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
)

// QKCHash is a consensus engine implementing PoW with qkchash algo.
type QKCHash struct {
	hashrate metrics.Meter
	// For reusing existing functions
	ethash *ethash.Ethash
}

// Author returns coinbase address.
func (q *QKCHash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (q *QKCHash) VerifyHeader(chain ethconsensus.ChainReader, header *types.Header, seal bool) error {
	// Short-circuit if the header is known, or parent not
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return ethconsensus.ErrUnknownAncestor
	}
	return consensus.VerifyHeader(chain, header, parent, q)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (q *QKCHash) VerifyHeaders(chain ethconsensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	// TODO: verify concurrently, and support aborting
	errorsOut := make(chan error, len(headers))
	go func() {
		for _, h := range headers {
			err := q.VerifyHeader(chain, h /*seal flag not used*/, true)
			errorsOut <- err
		}
	}()
	return nil, errorsOut
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
	return q.hashrate.Rate1()
}

// Close terminates any background threads maintained by the consensus engine.
func (q *QKCHash) Close() error {
	return nil
}

// New returns a QKCHash scheme.
func New() *QKCHash {
	return &QKCHash{
		hashrate: metrics.NewMeter(),
		ethash:   &ethash.Ethash{},
	}
}
