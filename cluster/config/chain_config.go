package config

import (
	"encoding/json"
	"github.com/QuarkChain/goquarkchain/account"
	ethcom "github.com/ethereum/go-ethereum/common"
	"math/big"
)

type ChainConfig struct {
	ChainID           uint32 `json:"CHAIN_ID"`
	ShardSize         uint32 `json:"SHARD_SIZE"`
	DefaultChainToken string `json:"-"`
	ConsensusType     string `json:"CONSENSUS_TYPE"`

	// Only set when CONSENSUS_TYPE is not NONE
	ConsensusConfig *POWConfig    `json:"CONSENSUS_CONFIG"`
	Genesis         *ShardGenesis `json:"GENESIS"`

	CoinbaseAddress account.Address `json:"-"`
	CoinbaseAmount  *big.Int        `json:"COINBASE_AMOUNT"`

	EpochInterval *big.Int

	// Gas Limit
	GasLimitEmaDenominator             uint32      `json:"GAS_LIMIT_EMA_DENOMINATOR"`
	GasLimitAdjustmentFactor           uint32      `json:"GAS_LIMIT_ADJUSTMENT_FACTOR"`
	GasLimitMinimum                    uint64      `json:"GAS_LIMIT_MINIMUM"`
	GasLimitMaximum                    uint64      `json:"GAS_LIMIT_MAXIMUM"`
	GasLimitUsageAdjustmentNumerator   uint32      `json:"GAS_LIMIT_USAGE_ADJUSTMENT_NUMERATOR"`
	GasLimitUsageAdjustmentDenominator uint32      `json:"GAS_LIMIT_USAGE_ADJUSTMENT_DENOMINATOR"`
	DifficultyAdjustmentCutoffTime     uint32      `json:"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME"`
	DifficultyAdjustmentFactor         uint32      `json:"DIFFICULTY_ADJUSTMENT_FACTOR"`
	ExtraShardBlocksInRootBlock        uint32      `json:"EXTRA_SHARD_BLOCKS_IN_ROOT_BLOCK"`
	PoswConfig                         *POSWConfig `json:"POSW_CONFIG"`
}

func NewChainConfig() *ChainConfig {
	return &ChainConfig{
		ChainID:                            0,
		ShardSize:                          2,
		DefaultChainToken:                  DefaultToken,
		ConsensusType:                      PoWNone,
		ConsensusConfig:                    nil,
		Genesis:                            NewShardGenesis(),
		CoinbaseAmount:                     new(big.Int).Mul(big.NewInt(5), QuarkashToJiaozi),
		GasLimitEmaDenominator:             1024,
		GasLimitAdjustmentFactor:           1024,
		GasLimitMinimum:                    5000,
		GasLimitMaximum:                    1<<63 - 1,
		GasLimitUsageAdjustmentNumerator:   3,
		GasLimitUsageAdjustmentDenominator: 2,
		DifficultyAdjustmentCutoffTime:     7,
		DifficultyAdjustmentFactor:         512,
		ExtraShardBlocksInRootBlock:        3,
		PoswConfig:                         NewPOSWConfig(),
		EpochInterval:                      new(big.Int).SetUint64(210000 * 60),
	}
}

type ChainConfigAlias ChainConfig

func (c *ChainConfig) MarshalJSON() ([]byte, error) {
	addr := c.CoinbaseAddress.ToHex()
	jsonConfig := struct {
		ChainConfigAlias
		CoinbaseAddress string `json:"COINBASE_ADDRESS"`
	}{ChainConfigAlias: ChainConfigAlias(*c), CoinbaseAddress: addr}
	return json.Marshal(jsonConfig)
}

func (c *ChainConfig) UnmarshalJSON(input []byte) error {
	var jsonConfig struct {
		ChainConfigAlias
		CoinbaseAddress string `json:"COINBASE_ADDRESS"`
	}
	if err := json.Unmarshal(input, &jsonConfig); err != nil {
		return err
	}
	*c = ChainConfig(jsonConfig.ChainConfigAlias)
	address, err := account.CreatAddressFromBytes(ethcom.FromHex(jsonConfig.CoinbaseAddress))
	if err != nil {
		return err
	}
	c.CoinbaseAddress = address
	return nil
}
