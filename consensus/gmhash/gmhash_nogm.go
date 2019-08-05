//+build !gm

package gmhash

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
)

type GmSm3Hash struct {
	*consensus.CommonEngine
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (g *GmSm3Hash) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (g *GmSm3Hash) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) (types.IBlock, error) {
	panic(errors.New("not finalize"))
}

// New returns a GmSm3Hash scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool) *GmSm3Hash {
	panic("do not support gm consensus for !gm build")
}
