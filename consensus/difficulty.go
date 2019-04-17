package consensus

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

type DifficultyCalculator interface {
	CalculateDifficulty(parent types.IHeader, time uint64) (*big.Int, error)
	IsNil() bool
}

type EthDifficultyCalculator struct {
	AdjustmentCutoff  uint32
	AdjustmentFactor  uint32
	MinimumDifficulty *big.Int
}

func (c *EthDifficultyCalculator) CalculateDifficulty(parent types.IHeader, time uint64) (*big.Int, error) {
	parentTime := parent.GetTime()
	if parentTime > time {
		return nil, ErrPreTime
	}

	sign := new(big.Int).Sub(big.NewInt(1), new(big.Int).SetUint64((time-parent.GetTime())/uint64(c.AdjustmentCutoff)))
	if sign.Cmp(big.NewInt(-99)) < 0 {
		sign = big.NewInt(-99)
	}
	offset := new(big.Int).Div(parent.GetDifficulty(), new(big.Int).SetUint64(uint64(c.AdjustmentFactor)))
	diff := new(big.Int).Add(parent.GetDifficulty(), offset.Mul(offset, sign))
	if diff.Cmp(c.MinimumDifficulty) < 0 {
		return c.MinimumDifficulty, nil
	}
	return diff, nil

}

func (c *EthDifficultyCalculator) IsNil() bool {
	return c == nil
}
