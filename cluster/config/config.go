package config

import (
	"encoding/json"
	"fmt"
	"github.com/QuarkChain/goquarkchain/params"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const (
	// PoWNone is the default empty consensus type specifying no shard.
	PoWNone = "NONE"
	// PoWEthash is the consensus type running ethash algorithm.
	PoWEthash = "POW_ETHASH"
	// PoWDoubleSha256 is the consensus type running double-sha256 algorithm.
	PoWDoubleSha256 = "POW_DOUBLESHA256"
	// PoWSimulate is the simulated consensus type by simply sleeping.
	PoWSimulate = "POW_SIMULATE"
	// PoWQkchash is the consensus type running qkchash algorithm.
	PoWQkchash = "POW_QKCHASH"

	DefaultGrpcPort    uint16 = 38191
	DefaultP2PPort     uint16 = 38291
	DefaultPubRpcPort  uint16 = 38391
	DefaultPrivRpcPort uint16 = 38491
	DefaultWSPort      uint16 = 38590
	DefaultHost               = "localhost"

	HeartbeatInterval = time.Duration(4 * time.Second)
)

var (
	QuarkashToJiaozi               = big.NewInt(1000000000000000000)
	DefaultNumSlaves               = 4
	DefaultToken                   = "QKC"
	DefaultP2PCmddSizeLimit uint32 = 128 * 1024 * 1024
)

var (
	templateFile = "alloc/%d.json"
	testFile     = "loadtest.json"
	tempErrMsg   = "Error importing genesis accounts from %s: %v "
	testErrMsg   = "No loadtest accounts imported into genesis alloc %s: %v "
)

type POWConfig struct {
	TargetBlockTime uint32 `json:"TARGET_BLOCK_TIME"`
	RemoteMine      bool   `json:"REMOTE_MINE"`
}

func NewPOWConfig() *POWConfig {
	return &POWConfig{
		TargetBlockTime: 10,
		RemoteMine:      false,
	}
}

type POSWConfig struct {
	Enabled            bool     `json:"ENABLED"`
	DiffDivider        uint64   `json:"DIFF_DIVIDER"`
	WindowSize         uint64   `json:"WINDOW_SIZE"`
	TotalStakePerBlock *big.Int `json:"TOTAL_STAKE_PER_BLOCK"`
}

func NewPOSWConfig() *POSWConfig {
	return &POSWConfig{
		Enabled:            false,
		DiffDivider:        20,
		WindowSize:         256,
		TotalStakePerBlock: new(big.Int).Mul(big.NewInt(1000000000), QuarkashToJiaozi),
	}
}

type SimpleNetwork struct {
	BootstrapHost string `json:"BOOT_STRAP_HOST"`
	BootstrapPort uint16 `json:"BOOT_STRAP_PORT"`
}

func NewSimpleNetwork() *SimpleNetwork {
	return &SimpleNetwork{
		BootstrapHost: "127.0.0.1",
		BootstrapPort: DefaultP2PPort,
	}
}

type RootGenesis struct {
	Version        uint32 `json:"VERSION"`
	Height         uint32 `json:"HEIGHT"`
	HashPrevBlock  string `json:"HASH_PREV_BLOCK"`
	HashMerkleRoot string `json:"HASH_MERKLE_ROOT"`
	Timestamp      uint64 `json:"TIMESTAMP"`
	Difficulty     uint64 `json:"DIFFICULTY"`
	Nonce          uint32 `json:"NONCE"`
}

func NewRootGenesis() *RootGenesis {
	return &RootGenesis{
		Version:        0,
		Height:         0,
		HashPrevBlock:  "",
		HashMerkleRoot: "",
		Timestamp:      1519147489,
		Difficulty:     1000000,
		Nonce:          0,
	}
}

type RootConfig struct {
	// To ignore super old blocks from peers
	// This means the network will fork permanently after a long partition
	// Use Ethereum's number, which is
	// - 30000 * 3 blocks = 90000 * 15 / 3600 = 375 hours = 375 * 3600 / 60 = 22500
	MaxStaleRootBlockHeightDiff    uint64          `json:"MAX_STALE_ROOT_BLOCK_HEIGHT_DIFF"`
	ConsensusType                  string          `json:"CONSENSUS_TYPE"`
	ConsensusConfig                *POWConfig      `json:"CONSENSUS_CONFIG"`
	Genesis                        *RootGenesis    `json:"GENESIS"`
	CoinbaseAddress                account.Address `json:"-"`
	CoinbaseAmount                 *big.Int        `json:"COINBASE_AMOUNT"`
	EpochInterval                  *big.Int        `json:"EPOCH_INTERVAL"`
	DifficultyAdjustmentCutoffTime uint32          `json:"DIFFICULTY_ADJUSTMENT_CUTOFF_TIME"`
	DifficultyAdjustmentFactor     uint32          `json:"DIFFICULTY_ADJUSTMENT_FACTOR"`
}

func NewRootConfig() *RootConfig {
	return &RootConfig{
		MaxStaleRootBlockHeightDiff:    22500,
		ConsensusType:                  PoWNone,
		ConsensusConfig:                nil,
		Genesis:                        NewRootGenesis(),
		CoinbaseAddress:                account.CreatEmptyAddress(0),
		CoinbaseAmount:                 new(big.Int).Mul(big.NewInt(120), QuarkashToJiaozi),
		EpochInterval:                  new(big.Int).SetUint64(210000 * 10),
		DifficultyAdjustmentCutoffTime: 40,
		DifficultyAdjustmentFactor:     1024,
	}
}

type RootConfigAlias RootConfig

func (r *RootConfig) MarshalJSON() ([]byte, error) {
	addr := r.CoinbaseAddress.ToHex()
	jsonConfig := struct {
		RootConfigAlias
		CoinbaseAddress string `json:"COINBASE_ADDRESS"`
	}{RootConfigAlias: RootConfigAlias(*r), CoinbaseAddress: addr}
	return json.Marshal(jsonConfig)
}

func (r *RootConfig) UnmarshalJSON(input []byte) error {
	var jsonConfig struct {
		RootConfigAlias
		CoinbaseAddress string `json:"COINBASE_ADDRESS"`
	}
	if err := json.Unmarshal(input, &jsonConfig); err != nil {
		return err
	}
	*r = RootConfig(jsonConfig.RootConfigAlias)
	address, err := account.CreatAddressFromBytes(common.FromHex(jsonConfig.CoinbaseAddress))
	if err != nil {
		return err
	}
	r.CoinbaseAddress = address
	return nil
}

func (r *RootConfig) MaxRootBlocksInMemory() uint64 {
	return r.MaxStaleRootBlockHeightDiff * 2
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

func NewMasterConfig() *MasterConfig {
	return &MasterConfig{
		MasterToSlaveConnectRetryDelay: 1.0,
	}
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

func NewP2PConfig() *P2PConfig {
	return &P2PConfig{
		BootNodes:        "",
		PrivKey:          "",
		MaxPeers:         25,
		UPnP:             false,
		AllowDialInRatio: 1.0,
		PreferredNodes:   "",
	}
}

func (s *P2PConfig) GetBootNodes() []string {
	return strings.Split(s.BootNodes, ",")
}

type MonitoringConfig struct {
	NetworkName      string `json:"NETWORK_NAME"`
	ClusterID        string `json:"CLUSTER_ID"`
	KafkaRestAddress string `json:"KAFKA_REST_ADDRESS"` // REST API endpoint for logging to Kafka, IP[:PORT] format
	MinerTopic       string `json:"MINER_TOPIC"`        // "qkc_miner"
	PropagationTopic string `json:"PROPAGATION_TOPIC"`  // "block_propagation"
	Errors           string `json:"ERRORS"`             // "error"
}

func NewMonitoringConfig() *MonitoringConfig {
	return &MonitoringConfig{
		NetworkName:      "",
		ClusterID:        "127.0.0.1",
		KafkaRestAddress: "",
		MinerTopic:       "qkc_miner",
		PropagationTopic: "block_propagation",
		Errors:           "error",
	}
}

type GenesisAddress struct {
	Address string `json:"address"`
	PrivKey string `json:"key"`
}

func loadGenesisAddrs(file string) ([]GenesisAddress, error) {
	var addresses []GenesisAddress
	fp, err := os.Open(file)
	if err != nil {
		log.Warn("loadGenesisAddr", "no such file", file)
		return addresses, nil
	}
	defer fp.Close()
	decoder := json.NewDecoder(fp)
	for decoder.More() {
		err := decoder.Decode(&addresses)
		if err != nil {
			return nil, err
		}
	}
	return addresses, nil
}

// Update ShardConfig.GENESIS.ALLOC
func UpdateGenesisAlloc(cluserConfig *ClusterConfig) error {
	if cluserConfig.GenesisDir == "" {
		return nil
	}
	loadtestFile := filepath.Join(cluserConfig.GenesisDir, testFile)
	qkcConfig := cluserConfig.Quarkchain

	eight := new(big.Int).SetUint64(10000000)
	genesis := new(big.Int).Mul(new(big.Int).SetUint64(1000000), params.DenomsValue.Ether)

	qetc := new(big.Int).Mul(new(big.Int).SetUint64(2), params.DenomsValue.Ether)
	qetc = new(big.Int).Mul(qetc, eight)

	qfb := new(big.Int).Mul(new(big.Int).SetUint64(3), params.DenomsValue.Ether)
	qfb = new(big.Int).Mul(qetc, eight)

	qaapl := new(big.Int).Mul(new(big.Int).SetUint64(4), params.DenomsValue.Ether)
	qaapl = new(big.Int).Mul(qetc, eight)

	qtsla := new(big.Int).Mul(new(big.Int).SetUint64(5), params.DenomsValue.Ether)
	qtsla = new(big.Int).Mul(qetc, eight)

	balances := map[string]*big.Int{
		qkcConfig.GenesisToken: genesis,
		"QETC":                 qetc,
		"QFB":                  qfb,
		"QAAPL":                qaapl,
		"QTSLA":                qtsla,
	}

	for chainId := 0; chainId < int(qkcConfig.ChainSize); chainId++ {
		allocFile := filepath.Join(cluserConfig.GenesisDir, fmt.Sprintf(templateFile, chainId))
		addresses, err := loadGenesisAddrs(allocFile)
		if err != nil {
			return fmt.Errorf(tempErrMsg, allocFile, err)
		}
		for _, addr := range addresses {
			address, err := account.CreatAddressFromBytes(common.FromHex(addr.Address))
			if err != nil {
				return fmt.Errorf(tempErrMsg, allocFile, err)
			}
			fullShardId, err := qkcConfig.GetFullShardIdByFullShardKey(address.FullShardKey)
			if err != nil {
				return err
			}
			shard, ok := qkcConfig.shards[fullShardId]
			if !ok {
				continue
			}
			allocation := Allocation{
				Balances: balances,
			}
			shard.Genesis.Alloc[address] = allocation
		}
		log.Info("Load template genesis accounts", "chain id", chainId, "imported", len(addresses), "config file", allocFile)
	}

	items, err := loadGenesisAddrs(loadtestFile)
	if err != nil {
		return fmt.Errorf(testErrMsg, loadtestFile, err)
	}
	for _, item := range items {
		bytes := common.FromHex(item.Address)
		for fullShardId, shardCfg := range qkcConfig.shards {
			addr := account.NewAddress(common.BytesToAddress(bytes[:20]), fullShardId)
			alloc := shardCfg.Genesis.Alloc[addr]
			alloc.Balances = balances
		}
	}
	log.Info("Loadtest accounts", "loadtest file", loadtestFile, "imported", len(items))

	return nil
}

func LoadtestAccounts(genesisDir string) []*account.Account {
	var (
		accounts = make([]*account.Account, 0)
		err      error
	)
	if genesisDir == "" {
		return nil
	}
	loadtestFile := filepath.Join(genesisDir, testFile)
	items, err := loadGenesisAddrs(loadtestFile)
	if err != nil {
		log.Error("load test file", "err", err)
		return nil
	}
	for _, item := range items {
		key := account.BytesToIdentityKey(common.FromHex(item.PrivKey))
		acc, err := account.NewAccountWithKey(key)
		if err != nil {
			log.Error("create account by key", "err", err)
			return nil
		}
		accounts = append(accounts, &acc)
	}
	return accounts
}
