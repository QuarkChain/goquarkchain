package simulate

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
	"time"
)

type PowSimulate struct {
	*consensus.CommonEngine
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (d *PowSimulate) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (d *PowSimulate) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) (types.IBlock, error) {
	panic(errors.New("not finalize"))
}

func (q *PowSimulate) RefreshWork(tip uint64) {
	q.CommonEngine.RefreshWork(tip)
}

func hashAlgo(cache *consensus.ShareCache) error {
	time.Sleep(0.1)

	return nil
}

func verifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	return nil
}

// New returns a DoubleSHA256 scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool, pubKey []byte) *PowSimulate {
	spec := consensus.MiningSpec{
		Name:       config.PoWSimulate,
		HashAlgo:   hashAlgo,
		VerifySeal: verifySeal,
	}
	return &PowSimulate{
		CommonEngine: consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey),
	}
}

