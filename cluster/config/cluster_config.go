package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/core/types"
)

var (
	HOST                        = "127.0.0.1"
	PORT                 uint64 = 38291
	GRPC_PORT                   = PORT + 200
	DefaultClusterConfig        = ClusterConfig{
		P2Port:                   38291,
		JsonRPCPort:              38291,
		PrivateJsonRPCPort:       38291,
		EnableTransactionHistory: false,
		DbPathRoot:               "./db",
		LogLevel:                 "info",
		StartSimulatedMining:     false,
		Clean:                    false,
		GenesisDir:               "/dev/null",
		Quarkchain:               nil,
		Master:                   nil,
		SlaveList:                nil,
		SimpleNetwork:            nil,
		P2P:                      nil,
		Monitoring:               nil,
		jsonFilepath:             "",
	}
	DefaultQuatrain = QuarkChainConfig{
		ShardSize:                         8,
		MaxNeighbors:                      32,
		NetworkId:                         3,
		TransactionQueueSizeLimitPerShard: 10000,
		BlockExtraDataSizeLimit:           1024,
		GuardianPublicKey:                 "ab856abd0983a82972021e454fcf66ed5940ed595b0898bcd75cbe2d0a51a00f5358b566df22395a2a8bf6c022c1d51a2c3defe654e91a8d244947783029694d",
		GuardianPrivateKey:                nil,
		P2ProtocolVersion:                 0,
		P2PCommandSizeLimit:               (1 << 32) - 1,
		SkipRootDifficultyCheck:           false,
		SkipMinorDifficultyCheck:          false,
		RewardTaxRate:                     Score{500, 1000},
	}
)

func (r *RootConfig) MaxRootBlocksInMemory() uint64 {
	return r.MaxStaleRootBlockHeightDiff * 2
}

type ClusterConfig struct {
	P2Port                   uint64            `json:"P2P_PORT"`
	JsonRPCPort              uint64            `json:"JSON_RPC_PORT"`
	PrivateJsonRPCPort       uint64            `json:"PRIVATE_JSON_RPC_PORT"`
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
	//kafka_logger string

	// configuration path
	jsonFilepath string
}

func NewClusterConfig() ClusterConfig {
	cluster := ClusterConfig{
		P2Port:                   38291,
		JsonRPCPort:              38291,
		PrivateJsonRPCPort:       38291,
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
		jsonFilepath:             "",
		Monitoring:               &DfaultMonitoring,
	}
	for i := 0; i < DefaultNumSlaves; i++ {
		slave := NewSlaveConfig()
		slave.Port = SLAVE_PORT
		slave.Id = fmt.Sprintf("S%d", i)
		slave.ShardMaskList = append(slave.ShardMaskList, types.NewChainMask(uint32(i|DefaultNumSlaves)))
		// slave.ShardMaskList = []
		cluster.SlaveList = append(cluster.SlaveList, slave)
	}
	return cluster
}

func (c *ClusterConfig) GetP2P() *P2PConfig {
	if c.P2P != nil {
		return c.P2P
	}
	return nil
}

func (c *ClusterConfig) GetDbPathRoot() string {
	return c.DbPathRoot
}

func (c *ClusterConfig) GetSlaveConfig(id string) (*SlaveConfig, error) {
	if c.SlaveList == nil {
		return nil, errors.New("slave config is empty")
	}
	for _, slave := range c.SlaveList {
		if slave != nil && slave.Id == id {
			return slave, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("slave %s is not in cluster config", id))
}

type QuarkChainConfig struct {
	ShardSize                         uint64         `json:"SHARD_SIZE"`
	MaxNeighbors                      uint32         `json:"MAX_NEIGHBORS"`
	NetworkId                         uint64         `json:"NETWORK_ID"`
	TransactionQueueSizeLimitPerShard uint64         `json:"TRANSACTION_QUEUE_SIZE_LIMIT_PER_SHARD"`
	BlockExtraDataSizeLimit           uint32         `json:"BLOCK_EXTRA_DATA_SIZE_LIMIT"`
	GuardianPublicKey                 string         `json:"GUARDIAN_PUBLIC_KEY"`
	GuardianPrivateKey                []byte         `json:"GUARDIAN_PRIVATE_KEY"`
	P2ProtocolVersion                 uint32         `json:"P2P_PROTOCOL_VERSION"`
	P2PCommandSizeLimit               uint32         `json:"P2P_COMMAND_SIZE_LIMIT"`
	SkipRootDifficultyCheck           bool           `json:"SKIP_ROOT_DIFFICULTY_CHECK"`
	SkipMinorDifficultyCheck          bool           `json:"SKIP_MINOR_DIFFICULTY_CHECK"`
	GenesisToken                      string         `json:"GENESIS_TOKEN"`
	Root                              *RootConfig    `json:"ROOT"`
	ShardList                         []*ShardConfig `json:"SHARD_LIST"`
	RewardTaxRate                     Score          `json:"REWARD_TAX_RATE"`
	// local_accounts []
}

func NewQuarkChainConfig() *QuarkChainConfig {
	quark := &QuarkChainConfig{
		ShardSize:                         8,
		MaxNeighbors:                      32,
		NetworkId:                         3, // testnet_porsche 3
		TransactionQueueSizeLimitPerShard: 10000,
		BlockExtraDataSizeLimit:           1024,
		GuardianPublicKey:                 "ab856abd0983a82972021e454fcf66ed5940ed595b0898bcd75cbe2d0a51a00f5358b566df22395a2a8bf6c022c1d51a2c3defe654e91a8d244947783029694d",
		GuardianPrivateKey:                nil,
		P2ProtocolVersion:                 0,
		P2PCommandSizeLimit:               (1 << 32) - 1,
		SkipRootDifficultyCheck:           false,
		SkipMinorDifficultyCheck:          false,
		Root:                              NewRootConfig(),
		ShardList:                         make([]*ShardConfig, 0),
		RewardTaxRate:                     Score{500, 1000},
	}
	quark.Root.ConsensusType = PoWSimulate
	quark.Root.ConsensusConfig = NewPOWConfig()
	quark.Root.ConsensusConfig.TargetBlockTime = 10
	quark.Root.Genesis.ShardSize = DefaultQuatrain.ShardSize
	for i := 0; i < int(DefaultQuatrain.ShardSize); i++ {
		s := NewShardConfig()
		s.SetRootConfig(quark.Root)
		s.ConsensusType = PoWSimulate
		s.ConsensusConfig = NewPOWConfig()
		s.ConsensusConfig.TargetBlockTime = 3
		quark.ShardList = append(quark.ShardList, s)
	}
	return quark
}

func (q QuarkChainConfig) MarshalJSON() ([]byte, error) {
	type QuarkChainConfig struct {
		ShardSize                         uint64         `json:"SHARD_SIZE"`
		MaxNeighbors                      uint32         `json:"MAX_NEIGHBORS"`
		NetworkId                         uint64         `json:"NETWORK_ID"`
		TransactionQueueSizeLimitPerShard uint64         `json:"TRANSACTION_QUEUE_SIZE_LIMIT_PER_SHARD"`
		BlockExtraDataSizeLimit           uint32         `json:"BLOCK_EXTRA_DATA_SIZE_LIMIT"`
		GuardianPublicKey                 string         `json:"GUARDIAN_PUBLIC_KEY"`
		GuardianPrivateKey                []byte         `json:"GUARDIAN_PRIVATE_KEY"`
		P2ProtocolVersion                 uint32         `json:"P2P_PROTOCOL_VERSION"`
		P2PCommandSizeLimit               uint32         `json:"P2P_COMMAND_SIZE_LIMIT"`
		SkipRootDifficultyCheck           bool           `json:"SKIP_ROOT_DIFFICULTY_CHECK"`
		SkipMinorDifficultyCheck          bool           `json:"SKIP_MINOR_DIFFICULTY_CHECK"`
		GenesisToken                      string         `json:"GENESIS_TOKEN"`
		Root                              *RootConfig    `json:"ROOT"`
		ShardList                         []*ShardConfig `json:"SHARD_LIST"`
		RewardTaxRate                     float64        `json:"REWARD_TAX_RATE"`
	}
	var enc = QuarkChainConfig{
		ShardSize:                         q.ShardSize,
		MaxNeighbors:                      q.MaxNeighbors,
		NetworkId:                         q.NetworkId,
		TransactionQueueSizeLimitPerShard: q.TransactionQueueSizeLimitPerShard,
		BlockExtraDataSizeLimit:           q.BlockExtraDataSizeLimit,
		GuardianPublicKey:                 q.GuardianPublicKey,
		GuardianPrivateKey:                q.GuardianPrivateKey,
		P2ProtocolVersion:                 q.P2ProtocolVersion,
		P2PCommandSizeLimit:               q.P2PCommandSizeLimit,
		SkipRootDifficultyCheck:           q.SkipRootDifficultyCheck,
		SkipMinorDifficultyCheck:          q.SkipMinorDifficultyCheck,
		GenesisToken:                      q.GenesisToken,
		Root:                              q.Root,
		ShardList:                         q.ShardList,
	}

	enc.RewardTaxRate = float64(q.RewardTaxRate.Numerator) / float64(q.RewardTaxRate.Denominator)
	return json.Marshal(&enc)
}

func (q *QuarkChainConfig) UnmarshalJSON(input []byte) error {
	type QuarkChainConfig struct {
		ShardSize                         uint64         `json:"SHARD_SIZE"`
		MaxNeighbors                      uint32         `json:"MAX_NEIGHBORS"`
		NetworkId                         uint64         `json:"NETWORK_ID"`
		TransactionQueueSizeLimitPerShard uint64         `json:"TRANSACTION_QUEUE_SIZE_LIMIT_PER_SHARD"`
		BlockExtraDataSizeLimit           uint32         `json:"BLOCK_EXTRA_DATA_SIZE_LIMIT"`
		GuardianPublicKey                 string         `json:"GUARDIAN_PUBLIC_KEY"`
		GuardianPrivateKey                []byte         `json:"GUARDIAN_PRIVATE_KEY"`
		P2ProtocolVersion                 uint32         `json:"P2P_PROTOCOL_VERSION"`
		P2PCommandSizeLimit               uint32         `json:"P2P_COMMAND_SIZE_LIMIT"`
		SkipRootDifficultyCheck           bool           `json:"SKIP_ROOT_DIFFICULTY_CHECK"`
		SkipMinorDifficultyCheck          bool           `json:"SKIP_MINOR_DIFFICULTY_CHECK"`
		GenesisToken                      string         `json:"GENESIS_TOKEN"`
		Root                              *RootConfig    `json:"ROOT"`
		ShardList                         []*ShardConfig `json:"SHARD_LIST"`
		RewardTaxRate                     float64        `json:"REWARD_TAX_RATE"`
	}
	var dec QuarkChainConfig
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	q.ShardSize = dec.ShardSize
	q.MaxNeighbors = dec.MaxNeighbors
	q.NetworkId = dec.NetworkId
	q.TransactionQueueSizeLimitPerShard = dec.TransactionQueueSizeLimitPerShard
	q.BlockExtraDataSizeLimit = dec.BlockExtraDataSizeLimit
	q.GuardianPublicKey = dec.GuardianPublicKey
	q.GuardianPrivateKey = dec.GuardianPrivateKey
	q.P2ProtocolVersion = dec.P2ProtocolVersion
	q.P2PCommandSizeLimit = dec.P2PCommandSizeLimit
	q.SkipRootDifficultyCheck = dec.SkipRootDifficultyCheck
	q.SkipMinorDifficultyCheck = dec.SkipMinorDifficultyCheck
	q.GenesisToken = dec.GenesisToken
	q.Root = dec.Root
	q.ShardList = dec.ShardList
	q.RewardTaxRate = Score{int64(dec.RewardTaxRate * 1000), 1000}
	return nil
}

// TODO need to add reward_tax_rate function
/*func (q *QuarkChainConfig) rewardTaxRate() uint {}*/

// Return the root block height at which the shard shall be created
func (q *QuarkChainConfig) GetGenesisRootHeight(shardId int) uint32 {
	if q.ShardSize <= uint64(shardId) {
		return 0
	}
	return q.ShardList[shardId].Genesis.Height
}

// Return a list of ids for shards that have GENESIS
func (q *QuarkChainConfig) GetGenesisShardIds() []int {
	var result []int
	for shardId := range q.ShardList {
		result = append(result, shardId)
	}
	return result
}

// Return a list of ids of the shards that have been initialized before a certain root height
func (q *QuarkChainConfig) GetInitializedShardIdsBeforeRootHeight(rootHeight uint32) []uint32 {
	var result []uint32
	for shardId, config := range q.ShardList {
		if config.Genesis != nil && config.Genesis.RootHeight < rootHeight {
			result = append(result, uint32(shardId))
		}
	}
	return result
}

func (q *QuarkChainConfig) GetShardConfigById(shardId uint64) *ShardConfig {
	if q.ShardSize <= shardId {
		return nil
	}
	return q.ShardList[shardId]
}

// TODO need to add guardian_public_key function
/*func (q *QuarkChainConfig) GuardianPublicKey() {}*/

// TODO need to add guardian_private_key funcion
/*func (q *QuarkChainConfig) GuardianPrivateKey() {}*/

func (q *QuarkChainConfig) Update(shardSize, rootBlockTime, minorBlockTime uint64) {
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
