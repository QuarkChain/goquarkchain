package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core/types"
	"io/ioutil"
	"os"
)

type NodeIndo struct {
	IP       string `json:"IP"`
	Port     uint64 `json:"Port"`
	User     string `json:"User"`
	Password string `json:"Password"`
	Service  string `json:"Service"`
}

type ExtraClusterConfig struct {
	TargetRootBlockTime  uint32 `json:"TargetRootBlockTime"`
	TargetMinorBlockTime uint32 `json:"TargetMinorBlockTime"`
	GasLimit             uint64 `json:"GasLimit"`
}

type LocalConfig struct {
	IPList             []NodeIndo          `json:"IPList"`
	BootNode           string              `json:"BootNode"`
	ChainNumber        uint32              `json:"ChainNumber"`
	ShardNumber        uint32              `json:"ShardNumber"`
	ExtraClusterConfig *ExtraClusterConfig `json:"ExtraClusterConfig"`
}

func LoadConfig(filePth string) *LocalConfig {
	var config LocalConfig
	f, err := os.Open(filePth)
	Checkerr(err)

	buffer, err := ioutil.ReadAll(f)
	Checkerr(err)
	err = json.Unmarshal(buffer, &config)
	Checkerr(err)
	return &config
}

func LoadClusterConfig(file string, cfg *config.ClusterConfig) error {
	var (
		content []byte
		err     error
	)
	if content, err = ioutil.ReadFile(file); err != nil {
		return errors.New(file + ", " + err.Error())
	}
	return json.Unmarshal(content, cfg)
}

func WriteConfigToFile(cfg *config.ClusterConfig, file string) {
	bytes, err := json.MarshalIndent(cfg, "", "	")
	if err != nil {
		utils.Fatalf("Failed to marshal json file content, %v", err)
	}
	err = ioutil.WriteFile(file, bytes, 0644)
	if err != nil {
		utils.Fatalf("Failed to write file, %v", err)
	}
}

func Update(q *config.QuarkChainConfig, chainSize, shardSizePerChain uint32, defaultChainConfig config.ChainConfig) {
	q.ChainSize = chainSize
	if q.Root == nil {
		q.Root = config.NewRootConfig()
	}
	if q.Root.ConsensusType == "" {
		q.Root.ConsensusType = config.PoWSimulate
	}
	if q.Root.ConsensusConfig == nil {
		q.Root.ConsensusConfig = config.NewPOWConfig()
	}

	q.Chains = make(map[uint32]*config.ChainConfig)
	shards := make(map[uint32]*config.ShardConfig)
	for chainId := uint32(0); chainId < chainSize; chainId++ {
		chainCfg := defaultChainConfig
		chainCfg.ChainID = chainId
		chainCfg.ShardSize = shardSizePerChain
		q.Chains[chainId] = &chainCfg
		for shardId := uint32(0); shardId < shardSizePerChain; shardId++ {
			shardCfg := config.NewShardConfig(&chainCfg)
			shardCfg.SetRootConfig(q.Root)
			shardCfg.ShardID = shardId
			shards[shardCfg.GetFullShardId()] = shardCfg
		}
	}
	q.SetShardsAndValidate(shards)
}

func updateChains(cfg *config.ClusterConfig, ChainSize uint32, shardSizePerChain uint32, ipList []string, defaultChainConfig config.ChainConfig) {
	Update(cfg.Quarkchain, ChainSize, shardSizePerChain, defaultChainConfig)
	updateSlaves(cfg, ipList)
}

func updateSlaves(cfg *config.ClusterConfig, ipList []string) {
	numSlaves := len(ipList)
	cfg.SlaveList = make([]*config.SlaveConfig, 0, numSlaves)
	for i := 0; i < numSlaves; i++ {
		slaveCfg := config.NewDefaultSlaveConfig()
		slaveCfg.IP = ipList[i%len(ipList)]
		slaveCfg.Port = uint16(38000 + i)
		slaveCfg.ID = fmt.Sprintf("S%d", i)
		slaveCfg.WSPort = uint16(39000 + i)
		slaveCfg.ChainMaskList = append(slaveCfg.ChainMaskList, types.NewChainMask(uint32(i|numSlaves)))
		cfg.SlaveList = append(cfg.SlaveList, slaveCfg)
	}
}
func GenConfigDependInitConfig(chainSize uint32, shardSizePerChain uint32, ipList []string, extraClusterConfig *ExtraClusterConfig) *config.ClusterConfig {
	cfg := config.NewClusterConfig()
	defaultChainConfig := *cfg.Quarkchain.Chains[0]

	//TODO @scf to fix
	//update root
	cfg.Quarkchain.Root.ConsensusType = config.PoWDoubleSha256
	cfg.Quarkchain.Root.Genesis.Difficulty = 10000
	cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime = extraClusterConfig.TargetRootBlockTime

	//update minor
	defaultChainConfig.ConsensusType = config.PoWDoubleSha256
	defaultChainConfig.Genesis.Difficulty = 10000
	defaultChainConfig.ConsensusConfig.TargetBlockTime = extraClusterConfig.TargetMinorBlockTime
	defaultChainConfig.Genesis.GasLimit = extraClusterConfig.GasLimit

	updateChains(cfg, chainSize, shardSizePerChain, ipList, defaultChainConfig)
	return cfg
}
