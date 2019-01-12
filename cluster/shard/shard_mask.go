package shard

import "github.com/QuarkChain/goquarkchain/cluster"

type ShardMask struct {
	value uint64
}

func NewShardMask(value uint64) *ShardMask {
	return &ShardMask{
		value: value,
	}
}

func (s *ShardMask) ContainShardId(shardId uint64) bool {
	bitMask := uint64((1 << (cluster.IntLeftMostBit(s.value) - 1)) - 1)
	return (bitMask & shardId) == (s.value & bitMask)
}

// TODO Supplement the implementation of this function
/*func (s *ShardMask) Contain_branch()
*/

func (s *ShardMask) HasOverlap(value uint64) bool {
	return cluster.MasksHaveOverlap(s.value, value)
}

// TODO Supplement the Iterate
/*func (s *ShardMask) Iterate(shard_size uint64) {}*/
