package params

import (
	ethParams "github.com/ethereum/go-ethereum/params"
	"math/big"
)

var (
	GCallValueTransfer = new(big.Int).SetUint64(9000)
	GtxxShardCost      = GCallValueTransfer // x-shard tx deposit gas

	DefaultStateDBGasLimit = new(big.Int).SetUint64(3141592)
)

var (
	DefaultByzantium = ethParams.ChainConfig{
		ChainID:        big.NewInt(1),
		HomesteadBlock: big.NewInt(0),
		EIP150Block:    big.NewInt(0),
		EIP155Block:    big.NewInt(0),
		EIP158Block:    big.NewInt(0),
		DAOForkBlock:   big.NewInt(0),
		ByzantiumBlock: big.NewInt(0),
	}
)
