package config

import (
	qcommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/common"
	"math"
)

var DefaultShardConfig = ShardConfig{
	ConsensusType: NONE,
}

type ShardGenesis struct {
	RootHeight         int                      `json:"ROOT_HEIGHT"`
	Version            int                      `json:"VERSION"`
	Height             int                      `json:"HEIGHT"`
	HashPrevMinorBlock common.Hash              `json:"HASH_PREV_MINOR_BLOCK"`
	HashMerkleRoot     common.Hash              `json:"HASH_MERKLE_ROOT"`
	ExtraData          []byte                   `json:"EXTRA_DATA"`
	Timestamp          int                      `json:"TIMESTAMP"`
	Difficulty         int                      `json:"DIFFICULTY"`
	GasLimit           int                      `json:"GAS_LIMIT"`
	Nonce              int                      `json:"NONCE"`
	Alloc              map[qcommon.QAddress]int `json:"ALLOC"`
}

type ShardConfig struct {
	// Only set when CONSENSUS_TYPE is not NONE
	ConsensusType                      int              `json:"CONSENSUS_TYPE"`
	ConsensusConfig                    *POWConfig       `json:"CONSENSUS_CONFIG"` // POWconfig
	Genesis                            *ShardGenesis    `json:"GENESIS"`          // ShardGenesis
	CoinbaseAddress                    qcommon.QAddress `json:"COINBASE_ADDRESS"`
	CoinbaseAmount                     float64          `json:"COINBASE_AMOUNT"` // default 5 * 10^18
	GasLimitEmaDenominator             int              `json:"GAS_LIMIT_EMA_DENOMINATOR"`
	GasLimitAdjustmentFactor           int              `json:"GAS_LIMIT_ADJUSTMENT_FACTOR"`
	GasLimitMinimum                    int              `json:"GAS_LIMIT_MINIMUM"`
	GasLimitMaximum                    uint64           `json:"GAS_LIMIT_MAXIMUM"`
	GasLimitUsageAdjustmentNumerator   int              `json:"GAS_LIMIT_USAGE_ADJUSTMENT_NUMERATOR"`
	GasLimitUsageAdjustmentDenominator int              `json:"GAS_LIMIT_USAGE_ADJUSTMENT_DENOMINATOR"`
	DifficultyAdjustmentCutoffTime     int              `json:"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME"`
	DifficultyAdjustmentFactor         int              `json:"DIFFICULTY_ADJUSTMENT_FACTOR"`
	ExtraShardBlocksInRootBlock        int              `json:"EXTRA_SHARD_BLOCKS_IN_ROOT_BLOCK"`
	rootConfig                         *RootConfig
}

func NewShardConfig() *ShardConfig {
	shardconfig := &ShardConfig{
		ConsensusType:                      NONE,
		ConsensusConfig:                    nil,
		CoinbaseAddress:                    qcommon.BytesToQAddress([]byte{0}),
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
		Genesis:                            &ShardGenesis{},
	}
	return shardconfig
}

func (s *ShardConfig) SetRootConfig(value *RootConfig) {
	s.rootConfig = value
}

func (s *ShardConfig) GetRootConfig() *RootConfig {
	return s.rootConfig
}
