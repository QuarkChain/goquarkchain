package config

import (
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math"
)

const (
	NONE = iota
	POW_ETHASH
	POW_SHA3SHA3
	POW_SIMULATE
	POW_QKCHASH
)

var (
	HOST                  = "127.0.0.1"
	PORT             uint = 38291
	DefaultPowConfig      = POWConfig{
		TargetBlockTime: 10,
		RemoteMine:      false,
	}
	DefaultRootGenesis = RootGenesis{
		Version:        0,
		Height:         0,
		ShardSize:      32,
		HashPrevBlock:  common.BytesToHash([]byte{0}),
		HashMerkleRoot: common.BytesToHash([]byte{0}),
		Timestamp:      1519147489,
		Difficulty:     1000000,
		Nonce:          0,
	}
	DefaultRootConfig = RootConfig{
		MaxStaleRootBlockHeightDiff: 60,
		ConsensusType:               NONE,
		ConsensusConfig:             nil,
		Genesis:                     nil,
		// TODO shuld replace with the real address realization.
		CoinbaseAddress:                qcom.QAddress{},
		CoinbaseAmount:                 120 * math.Pow10(18),
		DifficultyAdjustmentCutoffTime: 40,
		DifficultyAdjustmentFactor:     1024,
	}
	DefaultClusterConfig = ClusterConfig{
		P2pPort:                  38291,
		JsonRpcPort:              38291,
		PrivateJsonRpcPort:       38291,
		EnableTransactionHistory: false,
		DbPathRoot:               "./db",
		LogLevel:                 "info",
		StartSimulatedMining:     false,
		Clean:                    false,
		GenesisDir:               nil,
		Quarkchain:               nil,
		Master:                   nil,
		SlaveList:                nil,
		SimpleNetwork:            nil,
		P2p:                      nil,
		Monitoring:               nil,
		jsonFilepath:             nil,
	}
	DfaultMonitoring = MonitoringConfig{
		NetworkName:      "",
		ClusterId:        HOST,
		KafkaRestAddress: "",
		MinerTopic:       "qkc_miner",
		PropagationTopic: "block_propagation",
		Errors:           "error",
	}
	DefaultQuatrain = QuarkChainConfig{
		ShardSize:                         8,
		MaxNeighbors:                      32,
		NetworkId:                         3,
		TransactionQueueSizeLimitPerShard: 10000,
		BlockExtraDataSizeLimit:           1024,
		GuardianPublicKey:                 "ab856abd0983a82972021e454fcf66ed5940ed595b0898bcd75cbe2d0a51a00f5358b566df22395a2a8bf6c022c1d51a2c3defe654e91a8d244947783029694d",
		GuardianPrivateKey:                nil,
		P2pProtocolVersion:                0,
		P2pCommandSizeLimit:               (1 << 32) - 1,
		SkipRootDifficultyCheck:           false,
		SkipMinorDifficultyCheck:          false,
		RewardTaxRate:                     0.5,
	}
	DefaultP2PConfig = P2PConfig{
		BootNodes:        "",
		PrivKey:          "",
		MaxPeers:         25,
		Upnp:             false,
		AllowDialInRatio: 1.0,
		PreferredNodes:   "",
	}
	DefaultSimpleNetwork = SimpleNetwork{
		BootstrapHost: HOST,
		BootstrapPort: PORT,
	}
	DefaultMasterConfig = MasterConfig{
		MasterToSlaveConnectRetryDelay: 1.0,
	}
)

type POWConfig struct {
	TargetBlockTime int  `json:"TARGET_BLOCK_TIME"`
	RemoteMine      bool `json:"REMOTE_MINE"`
}

type SimpleNetwork struct {
	BootstrapHost string `json:"BOOT_STRAP_HOST"`
	BootstrapPort uint   `json:"BOOT_STRAP_PORT"`
}

type RootGenesis struct {
	Version        int         `json:"VERSION"`
	Height         int         `json:"HEIGHT"`
	ShardSize      uint        `json:"SHARD_SIZE"`
	HashPrevBlock  common.Hash `json:"HASH_PREV_BLOCK"`
	HashMerkleRoot common.Hash `json:"HASH_MERKLE_ROOT"`
	Timestamp      uint64      `json:"TIMESTAMP"`
	Difficulty     uint64      `json:"DIFFICULTY"`
	Nonce          int         `json:"NONCE"`
}

type RootConfig struct {
	// To ignore super old blocks from peers
	// This means the network will fork permanently after a long partitio
	MaxStaleRootBlockHeightDiff    int           `json:"MAX_STALE_ROOT_BLOCK_HEIGHT_DIFF"`
	ConsensusType                  int           `json:"CONSENSUS_TYPE"`
	ConsensusConfig                *POWConfig    `json:"CONSENSUS_CONFIG"`
	Genesis                        *RootGenesis  `json:"GENESIS"`
	CoinbaseAddress                qcom.QAddress `json:"COINBASE_ADDRESS"`
	CoinbaseAmount                 float64       `json:"COINBASE_AMOUNT"`
	DifficultyAdjustmentCutoffTime int           `json:"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME"`
	DifficultyAdjustmentFactor     int           `json:"DIFFICULTY_ADJUSTMENT_FACTOR"`
}

func NewRootConfig() *RootConfig {
	return &RootConfig{
		MaxStaleRootBlockHeightDiff: 60,
		ConsensusType:               NONE,
		ConsensusConfig:             nil,
		Genesis:                     &DefaultRootGenesis,
		// TODO address serialization type shuld to be replaced
		CoinbaseAddress:                qcom.BytesToQAddress([]byte{0}),
		CoinbaseAmount:                 120 * math.Pow10(18),
		DifficultyAdjustmentCutoffTime: 40,
		DifficultyAdjustmentFactor:     1024,
	}
}

func (r *RootConfig) MaxRootBlocksInMemory() int {
	return r.MaxStaleRootBlockHeightDiff * 2
}

type ClusterConfig struct {
	P2pPort                  uint              `json:"P2P_PORT"`
	JsonRpcPort              uint              `json:"JSON_RPC_PORT"`
	PrivateJsonRpcPort       uint              `json:"PRIVATE_JSON_RPC_PORT"`
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
	P2p                      *P2PConfig        `json:"P2P"`
	Monitoring               *MonitoringConfig `json:"MONITORING"`
	// TODO KafkaSampleLogger
	//kafka_logger string

	// configuration path
	jsonFilepath string
}

func NewClusterConfig() ClusterConfig {
	cluster := ClusterConfig{
		P2pPort:                  38291,
		JsonRpcPort:              38291,
		PrivateJsonRpcPort:       38291,
		EnableTransactionHistory: false,
		DbPathRoot:               "./db",
		LogLevel:                 "info",
		StartSimulatedMining:     false,
		Clean:                    false,
		GenesisDir:               nil,
		Quarkchain:               NewQuarkChainConfig(),
		Master:                   &DefaultMasterConfig,
		SimpleNetwork:            &DefaultSimpleNetwork,
		P2p:                      nil,
		jsonFilepath:             nil,
		Monitoring:               &DfaultMonitoring,
	}
	slave := &DefaultSlaveonfig
	slave.Port = 38000
	slave.Id = "S0"
	// slave.ShardMaskList = []
	cluster.SlaveList = append(cluster.SlaveList[:], slave)
	return cluster
}

func (c *ClusterConfig) GetP2p() *P2PConfig {
	if c.P2p != nil {
		return c.P2p
	}
	return nil
}

func (c *ClusterConfig) GetDbPathRoot() string {
	return c.DbPathRoot
}

func (c *ClusterConfig) GetSlaveConfig(id string) *SlaveConfig {
	for i := 0; i < len(c.SlaveList); i++ {
		if c.SlaveList[i].Id == id {
			return c.SlaveList[i]
		}
	}
	log.Error("slave config has such slave id", "slave id", id)
	return nil
}

// TODO need to wait SharInfo be realized.
/*func (c *ClusterConfig) GetSlaveInfoList() []*SlaveConfig {}*/

type NetWorkId struct {
	Mainnet        int `json:"MAINNET"`
	TestnetPorsche int `json:"TESTNET_PORSCHE"` // TESTNET_FORD = 2
}

type MasterConfig struct {
	// default 1.0
	MasterToSlaveConnectRetryDelay float32 `json:"MASTER_TO_SLAVE_CONNECT_RETRY_DELAY"`
}

// TODO move to P2P
type P2PConfig struct {
	// *new p2p module*
	BootNodes        string  `json:"BOOT_NODES"` // comma separated encodes format: encode://PUBKEY@IP:PORT
	PrivKey          string  `json:"PRIV_KEY"`
	MaxPeers         uint    `json:"MAX_PEERS"`
	Upnp             bool    `json:"UPNP"`
	AllowDialInRatio float32 `json:"ALLOW_DIAL_IN_RATIO"`
	PreferredNodes   string  `json:"PREFERRED_NODES"`
}

type MonitoringConfig struct {
	NetworkName      string `json:"NETWORK_NAME"`
	ClusterId        string `json:"CLUSTER_ID"`
	KafkaRestAddress string `json:"KAFKA_REST_ADDRESS"` // REST API endpoint for logging to Kafka, IP[:PORT] format
	MinerTopic       string `json:"MINER_TOPIC"`        // "qkc_miner"
	PropagationTopic string `json:"PROPAGATION_TOPIC"`  // "block_propagation"
	Errors           string `json:"ERRORS"`             // "error"
}

type QuarkChainConfig struct {
	ShardSize                         uint                  `json:"SHARD_SIZE"`
	MaxNeighbors                      int                   `json:"MAX_NEIGHBORS"`
	NetworkId                         uint                  `json:"NETWORK_ID"`
	TransactionQueueSizeLimitPerShard int                   `json:"TRANSACTION_QUEUE_SIZE_LIMIT_PER_SHARD"`
	BlockExtraDataSizeLimit           int                   `json:"BLOCK_EXTRA_DATA_SIZE_LIMIT"`
	GuardianPublicKey                 string                `json:"GUARDIAN_PUBLIC_KEY"`
	GuardianPrivateKey                []byte                `json:"GUARDIAN_PRIVATE_KEY"`
	P2pProtocolVersion                int                   `json:"P2P_PROTOCOL_VERSION"`
	P2pCommandSizeLimit               int                   `json:"P2P_COMMAND_SIZE_LIMIT"`
	SkipRootDifficultyCheck           bool                  `json:"SKIP_ROOT_DIFFICULTY_CHECK"`
	SkipMinorDifficultyCheck          bool                  `json:"SKIP_MINOR_DIFFICULTY_CHECK"`
	Root                              *RootConfig           `json:"ROOT"`
	ShardList                         map[uint]*ShardConfig `json:"SHARD_LIST"`
	RewardTaxRate                     float32               `json:"REWARD_TAX_RATE"`
	cachedGuardianPrivateKey          []byte
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
		P2pProtocolVersion:                0,
		P2pCommandSizeLimit:               (1 << 32) - 1,
		SkipRootDifficultyCheck:           false,
		SkipMinorDifficultyCheck:          false,
		Root:          NewRootConfig(),
		ShardList:     make(map[uint]*ShardConfig),
		RewardTaxRate: 0.5,
	}
	quark.Root.ConsensusType = POW_SIMULATE
	quark.Root.ConsensusConfig = &DefaultPowConfig
	quark.Root.ConsensusConfig.TargetBlockTime = 10
	quark.Root.Genesis.ShardSize = DefaultQuatrain.ShardSize
	for i := DefaultQuatrain.ShardSize - 1; i >= 0; i-- {
		s := NewShardConfig()
		s.SetRootConfig(quark.Root)
		s.ConsensusType = POW_SIMULATE
		s.ConsensusConfig = &DefaultPowConfig
		s.ConsensusConfig.TargetBlockTime = 3
		quark.ShardList[i] = s
	}
	return quark
}

// TODO need to add reward_tax_rate function
/*func (q *QuarkChainConfig) rewardTaxRate() uint {}*/

// Return the root block height at which the shard shall be created
func (q *QuarkChainConfig) GetGenesisRootHeight(shard_id uint) int {
	if shard, ok := q.ShardList[shard_id]; ok {
		return shard.Genesis.Height
	}
	return -1
}

// Return a list of ids for shards that have GENESIS
func (q *QuarkChainConfig) GetGenesisShardIds() []uint {
	var result []uint
	for shardId := range q.ShardList {
		result = append(result[:], shardId)
	}
	return result
}

// Return a list of ids of the shards that have been initialized before a certain root height
func (q *QuarkChainConfig) GetInitializedShardIdsBeforeRootHeight(rootHeight uint) []uint {
	var result []uint
	for shardId, config := range q.ShardList {
		if config.Genesis != nil && config.Genesis.RootHeight < rootHeight {
			result = append(result[:], shardId)
		}
	}
	return result
}

// TODO need to add guardian_public_key function
/*func (q *QuarkChainConfig) GuardianPublicKey() {}*/

// TODO need to add guardian_private_key funcion
/*func (q *QuarkChainConfig) GuardianPrivateKey() {}*/

func (q *QuarkChainConfig) Update(shardSize, rootBlockTime, minorBlockTime uint) {
	q.ShardSize = shardSize
	if q.Root == nil {
		q.Root = NewRootConfig()
	}
	q.Root.ConsensusType = POW_SIMULATE
	if q.Root.ConsensusConfig == nil {
		q.Root.ConsensusConfig = &DefaultPowConfig
	}
	q.Root.ConsensusConfig.TargetBlockTime = rootBlockTime
	q.Root.Genesis.ShardSize = shardSize

	q.ShardList = make([]*ShardConfig, q.ShardSize)
	for i := q.ShardSize - 1; i >= 0; i-- {
		s := NewShardConfig()
		s.SetRootConfig(q.Root)
		s.ConsensusType = POW_SIMULATE
		s.ConsensusConfig = &DefaultPowConfig
		s.ConsensusConfig.TargetBlockTime = minorBlockTime
		// TODO address serialization type shuld to be replaced
		s.CoinbaseAddress = qcom.BytesToQAddress([]byte{byte(i)})
		q.ShardList[i] = s
	}
}
