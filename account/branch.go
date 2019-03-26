package account

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/common"
)

// Branch branch include it's value
type Branch struct {
	Value uint32
}

// NewBranch new branch with value
func NewBranch(value uint32) Branch {
	return Branch{
		Value: value,
	}
}

// GetChainID get branch's chainID
func (Self *Branch) GetChainID() uint32 {
	return Self.Value >> 16
}

// GetShardSize get branch's shardSize
func (Self *Branch) GetShardSize() uint32 {
	branchValue := Self.Value & ((1 << 16) - 1)
	return 1 << (IntLeftMostBit(branchValue) - 1)
}

// GetFullShardID get branch's fullShardId
func (Self *Branch) GetFullShardID() uint32 {
	return Self.Value
}

// GetShardID get branch branch's shardID
func (Self *Branch) GetShardID() uint32 {
	branchValue := Self.Value & ((1 << 16) - 1)
	return branchValue ^ Self.GetShardSize()
}

// IsInBranch check shardKey is in current branch
func (Self *Branch) IsInBranch(fullShardKey uint32) bool {
	chainIDMatch := (fullShardKey >> 16) == Self.GetChainID()
	if chainIDMatch == false {
		return false
	}
	return (fullShardKey & (Self.GetShardSize() - 1)) == Self.GetShardID()
}

// CreatBranch create branch depend shardSize and shardID
func CreatBranch(shardSize uint32, shardID uint32) (Branch, error) {
	if common.IsP2(shardSize) == false {
		return Branch{}, errors.New("shardSize is not correct")
	}
	return NewBranch(shardSize | shardID), nil
}
