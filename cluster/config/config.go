package config

import (
	"encoding/json"
	"github.com/QuarkChain/goquarkchain/params"
	"math"
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
	QUARKSH_TO_JIAOZI = math.Pow10(18)
	DefaultNumSlaves  = 4
	DefaultPOSWConfig = POSWConfig{
		Enabled:            false,
		DiffDivider:        20,
		WindowSize:         256,
		TotalStakePerBlock: math.Pow10(9) * QUARKSH_TO_JIAOZI,
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
		BootNodes:        params.MainnetBootnodes,
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
	TotalStakePerBlock float64
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
	CoinbaseAmount                 float64      `json:"COINBASE_AMOUNT"`
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
		CoinbaseAmount:                 120 * QUARKSH_TO_JIAOZI,
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
	BootNodes        []string `json:"BOOT_NODES"` // comma separated encodes format: encode://PUBKEY@IP:PORT
	PrivKey          string   `json:"PRIV_KEY"`
	MaxPeers         uint64   `json:"MAX_PEERS"`
	UPnP             bool     `json:"UPNP"`
	AllowDialInRatio float32  `json:"ALLOW_DIAL_IN_RATIO"`
	PreferredNodes   string   `json:"PREFERRED_NODES"`
}

func (p P2PConfig) MarshalJSON() ([]byte, error) {
	type P2PConfig struct {
		BootNodes        string  `json:"BOOT_NODES"`
		PrivKey          string  `json:"PRIV_KEY"`
		MaxPeers         uint64  `json:"MAX_PEERS"`
		UPnP             bool    `json:"UPNP"`
		AllowDialInRatio float32 `json:"ALLOW_DIAL_IN_RATIO"`
		PreferredNodes   string  `json:"PREFERRED_NODES"`
	}
	var enc = P2PConfig{
		PrivKey:          p.PrivKey,
		MaxPeers:         p.MaxPeers,
		UPnP:             p.UPnP,
		AllowDialInRatio: p.AllowDialInRatio,
		PreferredNodes:   p.PreferredNodes,
	}
	for _, node := range p.BootNodes {
		if enc.BootNodes == "" {
			enc.BootNodes = node
			continue
		}
		enc.BootNodes += "," + node
	}
	return json.Marshal(&enc)
}

func (p *P2PConfig) UnmarshalJSON(input []byte) error {
	type P2PConfig struct {
		BootNodes        string  `json:"BOOT_NODES"`
		PrivKey          string  `json:"PRIV_KEY"`
		MaxPeers         uint64  `json:"MAX_PEERS"`
		UPnP             bool    `json:"UPNP"`
		AllowDialInRatio float32 `json:"ALLOW_DIAL_IN_RATIO"`
		PreferredNodes   string  `json:"PREFERRED_NODES"`
	}
	var dec P2PConfig
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	p.BootNodes = strings.Split(dec.BootNodes, ",")
	p.PrivKey = dec.PrivKey
	p.MaxPeers = dec.MaxPeers
	p.UPnP = dec.UPnP
	p.AllowDialInRatio = dec.AllowDialInRatio
	p.PreferredNodes = dec.PreferredNodes
	return nil
}

type MonitoringConfig struct {
	NetworkName      string `json:"NETWORK_NAME"`
	ClusterId        string `json:"CLUSTER_ID"`
	KafkaRestAddress string `json:"KAFKA_REST_ADDRESS"` // REST API endpoint for logging to Kafka, IP[:PORT] format
	MinerTopic       string `json:"MINER_TOPIC"`        // "qkc_miner"
	PropagationTopic string `json:"PROPAGATION_TOPIC"`  // "block_propagation"
	Errors           string `json:"ERRORS"`             // "error"
}

/*type ChainConfig struct {
	ChainId                            uint32
	ShardSize                          uint32
	DefaultChainToken                  string
	ConsensusType                      uint32
	ConsensusConfig                    *POWConfig
	Genesis                            *ShardGenesis
	CoinbaseAddress                    string
	CoinbaseAmount                     float64
	GasLimitEmaDenominator             uint64
	GasLimitAdjustmentFactor           float64
	GasLimitMinimum                    uint64
	GasLimitMaximum                    uint64
	GasLimitUsageAdjustmentNumerator   uint32
	GasLimitUsageAdjustmentDenominator uint32
	DifficultyAdjustmentCutoffTime     uint32
	DifficultyAdjustmentFactor         uint32
	ExtraShardBlocksInRootBlock        uint32
	PoswConfig                         *POSWConfig
}*/
