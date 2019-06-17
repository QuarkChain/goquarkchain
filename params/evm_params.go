package params

import (
	ethParams "github.com/ethereum/go-ethereum/params"
	"math/big"
)

var (
	DenomsValue = Denoms{
		Wei:  new(big.Int).SetUint64(1),
		GWei: new(big.Int).SetUint64(1000000000), //10^9
	}
	GCallValueTransfer = new(big.Int).SetUint64(9000)
	GtxxShardCost      = GCallValueTransfer // x-shard tx deposit gas

	DefaultStateDBGasLimit = new(big.Int).SetUint64(3141592)

	DefaultStartGas = new(big.Int).SetUint64(100 * 1000)
	DefaultGasPrice = new(big.Int).Mul(new(big.Int).SetUint64(10), DenomsValue.GWei)
)

type Denoms struct {
	Wei  *big.Int
	GWei *big.Int
}

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
