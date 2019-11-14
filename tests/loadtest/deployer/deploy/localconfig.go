package deploy

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core/types"
)

type NodeInfo struct {
	IP          string `json:"IP"`
	Port        uint64 `json:"Port"`
	User        string `json:"User"`
	Password    string `json:"Password"`
	IsMaster    bool   `json:"IsMaster"`
	SlaveNumber uint64 `json:"SlaveNumber"`
	ClusterID   int    `json:"ClusterID"`
}

type ExtraClusterConfig struct {
	TargetRootBlockTime  uint32 `json:"TargetRootBlockTime"`
	TargetMinorBlockTime uint32 `json:"TargetMinorBlockTime"`
	GasLimit             uint64 `json:"GasLimit"`
}

type LocalConfig struct {
	DockerName         string              `json:"DockerName"`
	Hosts              map[int][]NodeInfo  `json:"Hosts"`
	ChainSize          uint32              `json:"CHAIN_SIZE"`
	ShardSize          uint32              `json:"SHARD_SIZE"`
	ExtraClusterConfig *ExtraClusterConfig `json:"ExtraClusterConfig"`
}

func (l *LocalConfig) UnmarshalJSON(input []byte) error {
	type LocalConfig struct {
		DockerName         string              `json:"DockerName"`
		Hosts              []NodeInfo          `json:"Hosts"`
		ChainNumber        uint32              `json:"CHAIN_SIZE"`
		ShardNumber        uint32              `json:"SHARD_SIZE"`
		ExtraClusterConfig *ExtraClusterConfig `json:"ExtraClusterConfig"`
	}
	var dec LocalConfig

	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	l.DockerName = dec.DockerName

	l.Hosts = make(map[int][]NodeInfo, 0)
	for _, v := range dec.Hosts {
		if _, ok := l.Hosts[v.ClusterID]; !ok {
			l.Hosts[v.ClusterID] = make([]NodeInfo, 0)
		}
		l.Hosts[v.ClusterID] = append(l.Hosts[v.ClusterID], v)
	}

	l.ChainSize = dec.ChainNumber
	l.ShardSize = dec.ShardNumber
	l.ExtraClusterConfig = dec.ExtraClusterConfig
	return nil
}

func LoadConfig(filePth string) *LocalConfig {
	var config LocalConfig
	f, err := os.Open(filePth)
	CheckErr(err)

	buffer, err := ioutil.ReadAll(f)
	CheckErr(err)
	err = json.Unmarshal(buffer, &config)
	CheckErr(err)
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
		slaveCfg.Port = uint16(48000 + i)
		slaveCfg.ID = fmt.Sprintf("S%d", i)
		slaveCfg.WSPort = uint16(49000 + i)
		slaveCfg.ChainMaskList = append(slaveCfg.ChainMaskList, types.NewChainMask(uint32(i|numSlaves)))
		cfg.SlaveList = append(cfg.SlaveList, slaveCfg)
	}
}

func GenConfigDependInitConfig(chainSize uint32, shardSizePerChain uint32, ipList []string, extraClusterConfig *ExtraClusterConfig) *config.ClusterConfig {
	cfg := config.NewClusterConfig()
	defaultChainConfig := *cfg.Quarkchain.Chains[0]

	//update account
	cfg.GenesisDir = gensisAccountPath

	cfg.Quarkchain.TransactionQueueSizeLimitPerShard = 100000

	//update root
	cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime = extraClusterConfig.TargetRootBlockTime

	//update minor
	defaultChainConfig.ConsensusConfig.TargetBlockTime = extraClusterConfig.TargetMinorBlockTime
	defaultChainConfig.Genesis.GasLimit = extraClusterConfig.GasLimit

	updateChains(cfg, chainSize, shardSizePerChain, ipList, defaultChainConfig)
	return cfg
}
