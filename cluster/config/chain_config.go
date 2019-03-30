package config

import (
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
)

var (
	DefaultChainConfig = ChainConfig{
		ChainID:                            0,
		ShardSize:                          2,
		DefaultChainToken:                  "",
		ConsensusType:                      PoWNone,
		ConsensusConfig:                    nil,
		Genesis:                            nil,
		CoinbaseAddress:                    "",
		CoinbaseAmount:                     new(big.Int).Mul(big.NewInt(120), QuarkashToJiaozi),
		GasLimitEmaDenominator:             1024,
		GasLimitAdjustmentFactor:           1024,
		GasLimitMinimum:                    5000,
		GasLimitMaximum:                    1<<63 - 1,
		GasLimitUsageAdjustmentNumerator:   3,
		GasLimitUsageAdjustmentDenominator: 2,
		DifficultyAdjustmentCutoffTime:     7,
		DifficultyAdjustmentFactor:         512,
		ExtraShardBlocksInRootBlock:        3,
		PoswConfig:                         nil,
	}
)

type ChainConfig struct {
	ChainID           uint32 `json:"CHAIN_ID"`
	ShardSize         uint32 `json:"SHARD_SIZE"`
	DefaultChainToken string `json:"-"`
	ConsensusType     string `json:"CONSENSUS_TYPE"`

	// Only set when CONSENSUS_TYPE is not NONE
	ConsensusConfig *POWConfig    `json:"CONSENSUS_CONFIG"`
	Genesis         *ShardGenesis `json:"GENESIS"`

	CoinbaseAddress string   `json:"COINBASE_ADDRESS"`
	CoinbaseAmount  *big.Int `json:"COINBASE_AMOUNT"`

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
	chainCfg := new(ChainConfig)
	if err := common.DeepCopy(chainCfg, &DefaultChainConfig); err != nil {
		log.Error("chain config copy from default", "error", err)
	}
	chainCfg.Genesis = NewShardGenesis()
	chainCfg.PoswConfig = NewPOSWConfig()
	return chainCfg
}
