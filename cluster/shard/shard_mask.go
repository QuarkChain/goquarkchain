package shard

type ShardMask struct {
	value uint64
}

func NewShardMask(value uint64) *ShardMask {
	return &ShardMask{
		value: value,
	}
}

// TODO Supplement the Iterate
/*func (s *ShardMask) Iterate(shard_size uint64) {}*/
