package config

import (
	"math/big"
	"strings"
)

const (
	// PoWNone is the default empty consensus type specifying no shard.
	PoWNone = "NONE"
	// PoWEthash is the consensus type running ethash algorithm.
	PoWEthash = "POW_ETHASH"
	// PoWDoubleSha256 is the consensus type running double-sha256 algorithm.
	PowDoubleSha256 = "POW_DOUBLESHA256"
	// PoWSimulate is the simulated consensus type by simply sleeping.
	PoWSimulate = "POW_SIMULATE"
	// PoSQkchash is the consensus type running qkchash algorithm.
	PoWQkchash = "POW_QKCHASH"

	SLAVE_PORT = 38000
)

var (
	QUARKSH_TO_JIAOZI = big.NewInt(1000000000000000000)
	DefaultNumSlaves  = 4
	DefaultPOSWConfig = POSWConfig{
		Enabled:            false,
		DiffDivider:        20,
		WindowSize:         256,
		TotalStakePerBlock: new(big.Int).Mul(big.NewInt(9), QUARKSH_TO_JIAOZI),
	}
	DefaultRootGenesis = RootGenesis{
		Version:        0,
		Height:         0,
		ShardSize:      32,
		HashPrevBlock:  "",
		HashMerkleRoot: "",
		Timestamp:      1519147489,
		Difficulty:     1000000,
		Nonce:          0,
	}
	DfaultMonitoring = MonitoringConfig{
		NetworkName:      "",
		ClusterId:        HOST,
		KafkaRestAddress: "",
		MinerTopic:       "qkc_miner",
		PropagationTopic: "block_propagation",
		Errors:           "error",
	}
	DefaultP2PConfig = P2PConfig{
		BootNodes:        "",
		PrivKey:          "",
		MaxPeers:         25,
		UPnP:             false,
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
	TargetBlockTime uint64 `json:"TARGET_BLOCK_TIME"`
	RemoteMine      bool   `json:"REMOTE_MINE"`
}

func NewPOWConfig() *POWConfig {
	return &POWConfig{
		TargetBlockTime: 10,
		RemoteMine:      false,
	}
}

type POSWConfig struct {
	Enabled            bool
	DiffDivider        uint32
	WindowSize         uint32
	TotalStakePerBlock *big.Int
}

type SimpleNetwork struct {
	BootstrapHost string `json:"BOOT_STRAP_HOST"`
	BootstrapPort uint64 `json:"BOOT_STRAP_PORT"`
}

type RootGenesis struct {
	Version        uint32 `json:"VERSION"`
	Height         uint32 `json:"HEIGHT"`
	ShardSize      uint64 `json:"SHARD_SIZE"`
	HashPrevBlock  string `json:"HASH_PREV_BLOCK"`
	HashMerkleRoot string `json:"HASH_MERKLE_ROOT"`
	Timestamp      uint64 `json:"TIMESTAMP"`
	Difficulty     uint64 `json:"DIFFICULTY"`
	Nonce          uint32 `json:"NONCE"`
}

type RootConfig struct {
	// To ignore super old blocks from peers
	// This means the network will fork permanently after a long partitio
	MaxStaleRootBlockHeightDiff    uint64       `json:"MAX_STALE_ROOT_BLOCK_HEIGHT_DIFF"`
	ConsensusType                  string       `json:"CONSENSUS_TYPE"`
	ConsensusConfig                *POWConfig   `json:"CONSENSUS_CONFIG"`
	Genesis                        *RootGenesis `json:"GENESIS"`
	CoinbaseAddress                string       `json:"COINBASE_ADDRESS"`
	CoinbaseAmount                 *big.Int     `json:"COINBASE_AMOUNT"`
	DifficultyAdjustmentCutoffTime uint32       `json:"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME"`
	DifficultyAdjustmentFactor     uint32       `json:"DIFFICULTY_ADJUSTMENT_FACTOR"`
}

func NewRootConfig() *RootConfig {
	return &RootConfig{
		MaxStaleRootBlockHeightDiff: 60,
		ConsensusType:               PoWNone,
		ConsensusConfig:             nil,
		Genesis:                     &DefaultRootGenesis,
		// TODO address serialization type shuld to be replaced
		CoinbaseAddress:                "",
		CoinbaseAmount:                 new(big.Int).Mul(big.NewInt(120), QUARKSH_TO_JIAOZI),
		DifficultyAdjustmentCutoffTime: 40,
		DifficultyAdjustmentFactor:     1024,
	}
}

// TODO need to wait SharInfo be realized.
/*func (c *ClusterConfig) GetSlaveInfoList() []*SlaveConfig {}*/

type NetWorkId struct {
	Mainnet        uint32 `json:"MAINNET"`
	TestnetPorsche uint32 `json:"TESTNET_PORSCHE"` // TESTNET_FORD = 2
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
	MaxPeers         uint64  `json:"MAX_PEERS"`
	UPnP             bool    `json:"UPNP"`
	AllowDialInRatio float32 `json:"ALLOW_DIAL_IN_RATIO"`
	PreferredNodes   string  `json:"PREFERRED_NODES"`
}

func (s *P2PConfig) GetBootNodes() []string {
	return strings.Split(s.BootNodes, ",")
}

type MonitoringConfig struct {
	NetworkName      string `json:"NETWORK_NAME"`
	ClusterId        string `json:"CLUSTER_ID"`
	KafkaRestAddress string `json:"KAFKA_REST_ADDRESS"` // REST API endpoint for logging to Kafka, IP[:PORT] format
	MinerTopic       string `json:"MINER_TOPIC"`        // "qkc_miner"
	PropagationTopic string `json:"PROPAGATION_TOPIC"`  // "block_propagation"
	Errors           string `json:"ERRORS"`             // "error"
}
