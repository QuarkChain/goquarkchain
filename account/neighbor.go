package account

import (
	"github.com/QuarkChain/goquarkchain/common"
)

func absUint32(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// IsNeighbor Check if the two branches are neighbor
func IsNeighbor(b1, b2 Branch, shardSize uint32) bool {
	if shardSize <= 32 {
		return true
	}
	if b1.GetChainID() == b2.GetChainID() {
		return common.IsP2(absUint32(b1.GetShardID(), b2.GetShardID()))
	}
	if b1.GetShardID() == b2.GetShardID() {
		return common.IsP2(absUint32(b1.GetChainID(), b2.GetChainID()))
	}
	return false
}
