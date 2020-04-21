package params

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethParams "github.com/ethereum/go-ethereum/params"
)

var (
	DenomsValue = Denoms{
		Wei:   new(big.Int).SetUint64(1),
		GWei:  new(big.Int).SetUint64(1000000000), //10^9
		Ether: new(big.Int).Mul(new(big.Int).SetUint64(1000000000), new(big.Int).SetUint64(1000000000)),
	}
	GCallValueTransfer = new(big.Int).SetUint64(9000)
	GtxxShardCost      = GCallValueTransfer // x-shard tx deposit gas

	DefaultStateDBGasLimit = new(big.Int).SetUint64(3141592)
	DefaultBlockGasLimit   = new(big.Int).SetUint64(30000 * 400)

	DefaultStartGas = new(big.Int).SetUint64(100 * 1000)
	DefaultGasPrice = new(big.Int).Mul(new(big.Int).SetUint64(10), DenomsValue.GWei)

	DefaultInShardTxGasLimit    = new(big.Int).SetUint64(21000)
	DefaultCrossShardTxGasLimit = new(big.Int).SetUint64(30000)
)

type Denoms struct {
	Wei   *big.Int
	GWei  *big.Int
	Ether *big.Int
}

var (
	DefaultConstantinople = ethParams.ChainConfig{
		ChainID:             big.NewInt(1),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		DAOForkBlock:        big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
	}
)

var (
	PrecompiledContractsAfterEvmEnabled = []common.Address{
		common.HexToAddress("000000000000000000000000000000514b430001"),
		common.HexToAddress("000000000000000000000000000000514b430002"),
		common.HexToAddress("000000000000000000000000000000514b430003"),
	}
)

var (
	PrecompiledContractsMnt = []common.Address{
		common.HexToAddress("000000000000000000000000000000514b430004"),
		common.HexToAddress("000000000000000000000000000000514b430005"),
	}
)

var (
	MAINNET_ENABLE_NON_RESERVED_NATIVE_TOKEN_CONTRACT_TIMESTAMP = uint64(1588291200)
	MAINNET_ENABLE_GENERAL_NATIVE_TOKEN_CONTRACT_TIMESTAMP      = uint64(1588291200)
)
