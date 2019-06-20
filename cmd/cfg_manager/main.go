package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core/types"
	"io/ioutil"
	"strings"
)

const (
	defaultIp         = "127.0.0.1"
	defaultConfigPath = "./cluster_config_template.json"
)

var (
	initConf      = flag.String("init_params", "", "init conf for gen full conf")
	createDefault = flag.Int("create", 0, "to create default config ")
)

func loadConfig(file string, cfg *genConfigParams) error {
	var (
		content []byte
		err     error
	)
	if content, err = ioutil.ReadFile(file); err != nil {
		return errors.New(file + ", " + err.Error())
	}
	return json.Unmarshal(content, cfg)
}
func loadClusterConfig(file string, cfg *config.ClusterConfig) error {
	var (
		content []byte
		err     error
	)
	if content, err = ioutil.ReadFile(file); err != nil {
		return errors.New(file + ", " + err.Error())
	}
	return json.Unmarshal(content, cfg)
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

func updateChains(cfg *config.ClusterConfig, initParams *genConfigParams, defaultChainConfig config.ChainConfig) {
	if uint32(*initParams.ChainSize) == 0 && uint32(*initParams.ShardSizePerChain) == 0 {
		return
	}
	Update(cfg.Quarkchain, uint32(*initParams.ChainSize), uint32(*initParams.ShardSizePerChain), defaultChainConfig)
	//	updateShardConfig(cfg, initParams)
	updateSlaves(cfg, int(*initParams.NumSlaves), initParams.SlaveIpList)
}

func updateSlaves(cfg *config.ClusterConfig, numSlaves int, slaveIpList string) {
	ipList := strings.Split(slaveIpList, ",")
	if numSlaves == 0 {
		numSlaves = len(cfg.SlaveList)
	}
	if len(ipList) == 0 {
		ipList = append(ipList, defaultIp)
	}

	cfg.SlaveList = make([]*config.SlaveConfig, 0, numSlaves)
	for i := 0; i < numSlaves; i++ {
		slaveCfg := config.NewDefaultSlaveConfig()
		slaveCfg.Port = uint16(38000 + i)
		slaveCfg.ID = fmt.Sprintf("S%d", i)
		slaveCfg.IP = ipList[i%len(ipList)]
		slaveCfg.ChainMaskList = append(slaveCfg.ChainMaskList, types.NewChainMask(uint32(i|numSlaves)))
		cfg.SlaveList = append(cfg.SlaveList, slaveCfg)
	}
}

func GenConfigDependInitConfig() {
	if initConf == nil {
		utils.Fatalf("please set init config")
	}
	initParams := new(genConfigParams)
	if err := loadConfig(*initConf, initParams); err != nil {
		utils.Fatalf("%v", err)
	}
	initParams.SetDefault()
	cfg := config.NewClusterConfig()
	if err := loadClusterConfig(initParams.CfgFile, cfg); err != nil {
		utils.Fatalf("load cluster config err", err)
	}
	if len(cfg.Quarkchain.Chains) != 1 {
		utils.Fatalf("init config 's chain must be 1")
	}
	defaultChainConfig := *cfg.Quarkchain.Chains[0]
	updateChains(cfg, initParams, defaultChainConfig)
	WriteConfigToFile(cfg, initParams.CfgFile)
}
func GenDefaultConfig() {
	cfg := config.NewClusterConfig()
	cfg.Quarkchain.Update(1, 1, 10, 30)
	cfg.Quarkchain.Root.ConsensusType = config.PoWDoubleSha256
	for _, v := range cfg.Quarkchain.Chains {
		v.ConsensusType = config.PoWDoubleSha256
	}
	WriteConfigToFile(cfg, defaultConfigPath)
	fmt.Printf("init config will save in %v\n", defaultConfigPath)
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
func main() {
	flag.Parse()
	switch *createDefault {
	case 0:
		GenConfigDependInitConfig()
	case 1:
		GenDefaultConfig()
	default:
		utils.Fatalf("only support\n--create=0:gen default config\n--create=1:gen real config depend default config")
	}
}
