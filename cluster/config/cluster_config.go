package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/core/types"
)

var (
	GRPCPort             = 38591
	SlavePort            = 38000
	DefaultClusterConfig = ClusterConfig{
		P2PPort:                  38291,
		JSONRPCPort:              38391,
		PrivateJSONRPCPort:       38491,
		EnableTransactionHistory: false,
		DbPathRoot:               "./db",
		LogLevel:                 "info",
		StartSimulatedMining:     false,
		Clean:                    false,
		GenesisDir:               "/dev/null",
	}
	DefaultQuarkChainConfig = QuarkChainConfig{
		ShardSize:                         8,
		MaxNeighbors:                      32,
		NetworkId:                         3,
		TransactionQueueSizeLimitPerShard: 10000,
		BlockExtraDataSizeLimit:           1024,
		GuardianPublicKey:                 "ab856abd0983a82972021e454fcf66ed5940ed595b0898bcd75cbe2d0a51a00f5358b566df22395a2a8bf6c022c1d51a2c3defe654e91a8d244947783029694d",
		GuardianPrivateKey:                nil,
		P2PProtocolVersion:                0,
		P2PCommandSizeLimit:               (1 << 32) - 1,
		SkipRootDifficultyCheck:           false,
		SkipMinorDifficultyCheck:          false,
		RewardTaxRate:                     new(big.Rat).SetFloat64(0.5),
	}
)

func (r *RootConfig) MaxRootBlocksInMemory() uint64 {
	return r.MaxStaleRootBlockHeightDiff * 2
}

type ClusterConfig struct {
	P2PPort                  uint64            `json:"P2P_PORT"`
	JSONRPCPort              uint64            `json:"JSON_RPC_PORT"`
	PrivateJSONRPCPort       uint64            `json:"PRIVATE_JSON_RPC_PORT"`
	EnableTransactionHistory bool              `json:"ENABLE_TRANSACTION_HISTORY"`
	DbPathRoot               string            `json:"DB_PATH_ROOT"`
	LogLevel                 string            `json:"LOG_LEVEL"`
	StartSimulatedMining     bool              `json:"START_SIMULATED_MINING"`
	Clean                    bool              `json:"CLEAN"`
	GenesisDir               string            `json:"GENESIS_DIR"`
	Quarkchain               *QuarkChainConfig `json:"QUARKCHAIN"`
	Master                   *MasterConfig     `json:"MASTER"`
	SlaveList                []*SlaveConfig    `json:"SLAVE_LIST"`
	SimpleNetwork            *SimpleNetwork    `json:"SIMPLE_NETWORK"`
	P2P                      *P2PConfig        `json:"P2P"`
	Monitoring               *MonitoringConfig `json:"MONITORING"`
	// TODO KafkaSampleLogger
}

func NewClusterConfig() ClusterConfig {
	cluster := ClusterConfig{
		P2PPort:                  38291,
		JSONRPCPort:              38291,
		PrivateJSONRPCPort:       38291,
		EnableTransactionHistory: false,
		DbPathRoot:               "./data",
		LogLevel:                 "info",
		StartSimulatedMining:     false,
		Clean:                    false,
		GenesisDir:               "/dev/null",
		Quarkchain:               NewQuarkChainConfig(),
		Master:                   &DefaultMasterConfig,
		SlaveList:                make([]*SlaveConfig, 0),
		SimpleNetwork:            &DefaultSimpleNetwork,
		P2P:                      &DefaultP2PConfig,
		Monitoring:               &DefaultMonitoring,
	}
	for i := 0; i < DefaultNumSlaves; i++ {
		slave := NewDefaultSlaveConfig()
		slave.Port = uint64(SlavePort + i)
		slave.ID = fmt.Sprintf("S%d", i)
		slave.ShardMaskList = append(slave.ShardMaskList, types.NewChainMask(uint32(i|DefaultNumSlaves)))
		cluster.SlaveList = append(cluster.SlaveList, slave)
	}
	return cluster
}

func (c *ClusterConfig) GetSlaveConfig(id string) (*SlaveConfig, error) {
	if c.SlaveList == nil {
		return nil, errors.New("slave config is empty")
	}
	for _, slave := range c.SlaveList {
		if slave != nil && slave.ID == id {
			return slave, nil
		}
	}
	return nil, fmt.Errorf("slave %s is not in cluster config", id)
}

type QuarkChainConfig struct {
	ShardSize                         uint64         `json:"SHARD_SIZE"`
	MaxNeighbors                      uint32         `json:"MAX_NEIGHBORS"`
	NetworkId                         uint64         `json:"NETWORK_ID"`
	TransactionQueueSizeLimitPerShard uint64         `json:"TRANSACTION_QUEUE_SIZE_LIMIT_PER_SHARD"`
	BlockExtraDataSizeLimit           uint32         `json:"BLOCK_EXTRA_DATA_SIZE_LIMIT"`
	GuardianPublicKey                 string         `json:"GUARDIAN_PUBLIC_KEY"`
	GuardianPrivateKey                []byte         `json:"GUARDIAN_PRIVATE_KEY"`
	P2PProtocolVersion                uint32         `json:"P2P_PROTOCOL_VERSION"`
	P2PCommandSizeLimit               uint32         `json:"P2P_COMMAND_SIZE_LIMIT"`
	SkipRootDifficultyCheck           bool           `json:"SKIP_ROOT_DIFFICULTY_CHECK"`
	SkipMinorDifficultyCheck          bool           `json:"SKIP_MINOR_DIFFICULTY_CHECK"`
	GenesisToken                      string         `json:"GENESIS_TOKEN"`
	Root                              *RootConfig    `json:"ROOT"`
	ShardList                         []*ShardConfig `json:"SHARD_LIST"`
	RewardTaxRate                     *big.Rat       `json:"-"`
}

type QuarkChainConfigAlias QuarkChainConfig

func (q *QuarkChainConfig) MarshalJSON() ([]byte, error) {
	rewardTaxRate, _ := q.RewardTaxRate.Float64()
	jsonConfig := struct {
		QuarkChainConfigAlias
		RewardTaxRate float64 `json:"REWARD_TAX_RATE"`
	}{QuarkChainConfigAlias(*q), rewardTaxRate}
	return json.Marshal(jsonConfig)
}

func (q *QuarkChainConfig) UnmarshalJSON(input []byte) error {
	var jsonConfig struct {
		QuarkChainConfigAlias
		RewardTaxRate float64 `json:"REWARD_TAX_RATE"`
	}
	if err := json.Unmarshal(input, &jsonConfig); err != nil {
		return err
	}
	*q = QuarkChainConfig(jsonConfig.QuarkChainConfigAlias)
	q.RewardTaxRate = new(big.Rat).SetFloat64(jsonConfig.RewardTaxRate)
	return nil
}

// GetGenesisRootHeight returns the root block height at which the shard shall be created.
func (q *QuarkChainConfig) GetGenesisRootHeight(shardID uint32) uint32 {
	if q.ShardSize <= uint64(shardID) {
		return 0
	}
	return q.ShardList[shardID].Genesis.Height
}

// GetGenesisShardIds returns a list of ids for shards that have GENESIS.
func (q *QuarkChainConfig) GetGenesisShardIds() []int {
	var result []int
	for shardID := range q.ShardList {
		result = append(result, shardID)
	}
	return result
}

// GetInitializedShardIdsBeforeRootHeight returns a list of ids of the shards that have been
// initialized before a certain root height.
func (q *QuarkChainConfig) GetInitializedShardIdsBeforeRootHeight(rootHeight uint32) []uint32 {
	var result []uint32
	for GetShardConfigById, config := range q.ShardList {
		if config.Genesis != nil && config.Genesis.RootHeight < rootHeight {
			result = append(result, uint32(GetShardConfigById))
		}
	}
	return result
}

func (q *QuarkChainConfig) GetShardConfigById(shardId uint32) *ShardConfig {
	if q.ShardSize <= uint64(shardId) {
		return nil
	}
	return q.ShardList[shardId]
}

func (q *QuarkChainConfig) update(shardSize, rootBlockTime, minorBlockTime uint64) {
	q.ShardSize = shardSize
	if q.Root == nil {
		q.Root = NewRootConfig()
	}
	q.Root.ConsensusType = PoWSimulate
	if q.Root.ConsensusConfig == nil {
		q.Root.ConsensusConfig = NewPOWConfig()
	}
	q.Root.ConsensusConfig.TargetBlockTime = rootBlockTime
	q.Root.Genesis.ShardSize = shardSize

	q.ShardList = make([]*ShardConfig, 0)
	for i := 0; i < int(q.ShardSize); i++ {
		s := NewShardConfig()
		s.SetRootConfig(q.Root)
		s.ConsensusType = PoWSimulate
		s.ConsensusConfig = NewPOWConfig()
		s.ConsensusConfig.TargetBlockTime = minorBlockTime
		// TODO address serialization type shuld to be replaced
		s.CoinbaseAddress = ""
		q.ShardList = append(q.ShardList, s)
	}
}

func NewQuarkChainConfig() *QuarkChainConfig {
	c := &QuarkChainConfig{
		ShardSize:                         8,
		MaxNeighbors:                      32,
		NetworkId:                         3, // testnet_porsche 3
		TransactionQueueSizeLimitPerShard: 10000,
		BlockExtraDataSizeLimit:           1024,
		GuardianPublicKey:                 "ab856abd0983a82972021e454fcf66ed5940ed595b0898bcd75cbe2d0a51a00f5358b566df22395a2a8bf6c022c1d51a2c3defe654e91a8d244947783029694d",
		GuardianPrivateKey:                nil,
		P2PProtocolVersion:                0,
		P2PCommandSizeLimit:               (1 << 32) - 1,
		SkipRootDifficultyCheck:           false,
		SkipMinorDifficultyCheck:          false,
		Root:                              NewRootConfig(),
		ShardList:                         make([]*ShardConfig, 0),
		RewardTaxRate:                     new(big.Rat).SetFloat64(0.5),
	}
	c.Root.ConsensusType = PoWSimulate
	c.Root.ConsensusConfig = NewPOWConfig()
	c.Root.ConsensusConfig.TargetBlockTime = 10
	c.Root.Genesis.ShardSize = DefaultQuarkChainConfig.ShardSize
	for i := 0; i < int(DefaultQuarkChainConfig.ShardSize); i++ {
		s := NewShardConfig()
		s.SetRootConfig(c.Root)
		s.ConsensusType = PoWSimulate
		s.ConsensusConfig = NewPOWConfig()
		s.ConsensusConfig.TargetBlockTime = 3
		c.ShardList = append(c.ShardList, s)
	}
	return c
}
