package consensus

import (
	"math/big"

	"github.com/QuarkChain/goquarkchain/core/types"
)

type DifficultyCalculator interface {
	CalculateDifficulty(parent types.IBlock, time uint64) (*big.Int, error)
}

type EthDifficultyCalculator struct {
	AdjustmentCutoff  uint32
	AdjustmentFactor  uint32
	MinimumDifficulty *big.Int
}

func (c *EthDifficultyCalculator) CalculateDifficulty(parent types.IBlock, time uint64) (*big.Int, error) {
	parentTime := parent.Time()
	if parentTime > time {
		return nil, ErrPreTime
	}

	sign := new(big.Int).Sub(big.NewInt(1), new(big.Int).SetUint64((time-parent.Time())/uint64(c.AdjustmentCutoff)))
	if sign.Cmp(big.NewInt(-99)) < 0 {
		sign = big.NewInt(-99)
	}
	offset := new(big.Int).Div(parent.Difficulty(), new(big.Int).SetUint64(uint64(c.AdjustmentFactor)))
	diff := new(big.Int).Add(parent.Difficulty(), offset.Mul(offset, sign))
	if diff.Cmp(c.MinimumDifficulty) < 0 {
		return c.MinimumDifficulty, nil
	}
	return diff, nil

}
