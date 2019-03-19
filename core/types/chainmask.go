package types

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/common"
)

type ChainMask struct {
	value uint32
}

/*
	Represent a mask of chains, basically matches all the bits from the right until the leftmost bit is hit.
	E.g.,
	mask = 1, matches *
	mask = 0b10, matches *0
	mask = 0b101, matches *01
*/
func NewChainMask(value uint32) *ChainMask {
	if value == 0 {
		return nil
	}
	return &ChainMask{
		value: value,
	}
}

func (c *ChainMask) GetMask() uint32 {

	return c.value

}

func (c *ChainMask) ContainFullShardId(fullShardId uint32) bool {
	chainId := fullShardId >> 16
	bitMask := uint32((1 << (account.IntLeftMostBit(c.value) - 1)) - 1)
	return (bitMask & chainId) == (c.value & bitMask)
}

func (c *ChainMask) ContainBranch(branch *account.Branch) bool {
	return c.ContainFullShardId(branch.GetFullShardID())
}

func (c *ChainMask) HasOverlap(value uint32) bool {
	return common.MasksHaveOverlap(c.value, value)
}
