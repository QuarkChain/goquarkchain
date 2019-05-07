package params

import "math/big"

var (
	GCallValueTransfer = new(big.Int).SetUint64(9000)
	GtxxShardCost      = GCallValueTransfer // x-shard tx deposit gas

	DefaultStateDBGasLimit = new(big.Int).SetUint64(3141592)
)
