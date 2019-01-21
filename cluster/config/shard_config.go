package config

import (
	"math"
)

var (
	DefaultShardGenesis = ShardGenesis{
		RootHeight:         0,
		Version:            0,
		Height:             0,
		HashPrevMinorBlock: "",
		HashMerkleRoot:     "",
		ExtraData:          []byte("It was the best of times, it was the worst of times, ... - Charles Dickens"),
		Timestamp:          DefaultRootGenesis.Timestamp,
		Difficulty:         10000,
		GasLimit:           30000 * 400,
		Nonce:              0,
		Alloc:              make(map[string]float64),
	}
)

type ShardGenesis struct {
	RootHeight         int                `json:"ROOT_HEIGHT"`
	Version            int                `json:"VERSION"`
	Height             int                `json:"HEIGHT"`
	HashPrevMinorBlock string             `json:"HASH_PREV_MINOR_BLOCK"`
	HashMerkleRoot     string             `json:"HASH_MERKLE_ROOT"`
	ExtraData          []byte             `json:"EXTRA_DATA"`
	Timestamp          uint64             `json:"TIMESTAMP"`
	Difficulty         int                `json:"DIFFICULTY"`
	GasLimit           int                `json:"GAS_LIMIT"`
	Nonce              int                `json:"NONCE"`
	Alloc              map[string]float64 `json:"ALLOC"`
}

type ShardConfig struct {
	// Only set when CONSENSUS_TYPE is not NONE
	ConsensusType   string        `json:"CONSENSUS_TYPE"`
	ConsensusConfig *POWConfig    `json:"CONSENSUS_CONFIG"` // POWconfig
	Genesis         *ShardGenesis `json:"GENESIS"`          // ShardGenesis
	// TODO coinbase address shuild to be redesigned.
	CoinbaseAddress                    string  `json:"COINBASE_ADDRESS"`
	CoinbaseAmount                     float64 `json:"COINBASE_AMOUNT"` // default 5 * 10^18
	GasLimitEmaDenominator             int     `json:"GAS_LIMIT_EMA_DENOMINATOR"`
	GasLimitAdjustmentFactor           int     `json:"GAS_LIMIT_ADJUSTMENT_FACTOR"`
	GasLimitMinimum                    int     `json:"GAS_LIMIT_MINIMUM"`
	GasLimitMaximum                    uint64  `json:"GAS_LIMIT_MAXIMUM"`
	GasLimitUsageAdjustmentNumerator   int     `json:"GAS_LIMIT_USAGE_ADJUSTMENT_NUMERATOR"`
	GasLimitUsageAdjustmentDenominator int     `json:"GAS_LIMIT_USAGE_ADJUSTMENT_DENOMINATOR"`
	DifficultyAdjustmentCutoffTime     int     `json:"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME"`
	DifficultyAdjustmentFactor         int     `json:"DIFFICULTY_ADJUSTMENT_FACTOR"`
	ExtraShardBlocksInRootBlock        int     `json:"EXTRA_SHARD_BLOCKS_IN_ROOT_BLOCK"`
	rootConfig                         *RootConfig
}

func NewShardConfig() *ShardConfig {
	sharding := &ShardConfig{
		ConsensusType:                      DefaultConsensusJSONType.NONE,
		ConsensusConfig:                    nil,
		CoinbaseAddress:                    "",
		CoinbaseAmount:                     5 * math.Pow10(18),
		GasLimitEmaDenominator:             1024,
		GasLimitAdjustmentFactor:           1024,
		GasLimitMinimum:                    5000,
		GasLimitMaximum:                    (1 << 63) - 1,
		GasLimitUsageAdjustmentNumerator:   3,
		GasLimitUsageAdjustmentDenominator: 2,
		DifficultyAdjustmentCutoffTime:     7,
		DifficultyAdjustmentFactor:         512,
		ExtraShardBlocksInRootBlock:        3,
		Genesis:                            &DefaultShardGenesis,
	}
	// TODO should to be deleted, just for test
	sharding.Genesis.Alloc["0x0000000000000000000000000000000000000000000000000000000000000000"] = math.Pow10(24)
	return sharding
}

func (s *ShardConfig) SetRootConfig(value *RootConfig) {
	s.rootConfig = value
}

func (s *ShardConfig) GetRootConfig() *RootConfig {
	return s.rootConfig
}

func (s *ShardConfig) MaxBlocksPerShardInOneRootBlock() int {
	return int(s.rootConfig.ConsensusConfig.TargetBlockTime/s.ExtraShardBlocksInRootBlock) + s.ExtraShardBlocksInRootBlock
}

//Max_stale_minor_block_height_diff
func (s *ShardConfig) MaxStaleMinorBlockHeightDiff() int {
	return int(s.rootConfig.MaxStaleRootBlockHeightDiff *
		s.rootConfig.ConsensusConfig.TargetBlockTime /
		s.ConsensusConfig.TargetBlockTime)
}

func (s *ShardConfig) MaxMinorBlocksInMemory() int {
	return s.MaxStaleMinorBlockHeightDiff() * 2
}
