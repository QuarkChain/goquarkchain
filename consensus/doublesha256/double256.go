package doublesha256

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/rpc"
)

// DoubleSHA256 is a consensus engine implementing PoW with double-sha256 algo.
type DoubleSHA256 struct {
	hashrate metrics.Meter
}

// Author returns coinbase address.
func (d *DoubleSHA256) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (d *DoubleSHA256) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	panic("not implemented")
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (d *DoubleSHA256) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	panic("not implemented")
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (d *DoubleSHA256) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	// For now QuarkChain won't verify uncles.
	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (d *DoubleSHA256) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	panic("not implemented")
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (d *DoubleSHA256) Prepare(chain consensus.ChainReader, header *types.Header) error {
	panic("not implemented")
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
func (d *DoubleSHA256) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	panic("not implemented")
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (d *DoubleSHA256) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	panic("not implemented")
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (d *DoubleSHA256) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	panic("not implemented")
}

// APIs returns the RPC APIs this consensus engine provides.
func (d *DoubleSHA256) APIs(chain consensus.ChainReader) []rpc.API {
	panic("not implemented")
}

// Hashrate returns the current mining hashrate of a PoW consensus engine.
func (d *DoubleSHA256) Hashrate() float64 {
	return d.hashrate.Rate1()
}

// New returns a DoubleSHA256 scheme.
func New() *DoubleSHA256 {
	return nil
}
