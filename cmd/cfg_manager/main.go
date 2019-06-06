package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"io/ioutil"
	"math/big"
	"regexp"
	"strings"
)

const (
	defaultIp   = "127.0.0.1"
	defaultPriv = "0xca0143c9aa51c3013f08e83f3b6368a4f3ba5b52c4841c6e0c22c300f7ee6827"
)

var (
	cfgFile           = flag.String("config", "", "config file")
	difficulty        = flag.Int("diff", 1000000, "difficulty of shard chain")
	chainSize         = flag.Int("num_chains", 1, "total chains")
	shardSizePerChain = flag.Int("num_shards_per_chain", 1, "shard num in pre chain")
	rootBlockTime     = flag.Int("root_block_time", 30, "root block time")
	minorBlockTime    = flag.Int("minor_block_time", 10, "minor block time")
	coinBaseAddress   = flag.String("coinbase", "0xb067ac9ebeeecb10bbcd1088317959d58d1e38f6b0ee10d5", "coinbase address in root chain and shard chain")
	numSlaves         = flag.Int("num_slaves", config.DefaultNumSlaves, "sum of slaves")
	slaveIpList       = flag.String("ip_list", defaultIp, "etc: ip,ip,ip")
	privatekey        = flag.String("private_key", defaultPriv, "private key in p2p config")
)

func loadConfig(file string, cfg *config.ClusterConfig) error {
	var (
		content []byte
		err     error
	)
	if content, err = ioutil.ReadFile(file); err != nil {
		return errors.New(file + ", " + err.Error())
	}
	return json.Unmarshal(content, cfg)
}

func updateShardConfig(cfg *config.ClusterConfig) {
	address := common.HexToAddress(string(regexp.MustCompile("0x[0-9a-fA-F]{40}").FindString(*coinBaseAddress)))
	fullShardIds := cfg.Quarkchain.GetGenesisShardIds()
	for _, fullShardId := range fullShardIds {
		shard := cfg.Quarkchain.GetShardConfigByFullShardID(fullShardId)
		shard.ConsensusType = config.PoWDoubleSha256
		shard.Genesis.Difficulty = uint64(*difficulty)
		addr := account.NewAddress(address, fullShardId)
		shard.Genesis.Alloc[addr] = new(big.Int).Mul(big.NewInt(1000000), config.QuarkashToJiaozi)
		shard.CoinbaseAddress = addr
	}
}

func updateChains(cfg *config.ClusterConfig, chainSize, shardSizePerChain, rootBlockTime, minorBlockTime uint32) {
	if chainSize == 0 && shardSizePerChain == 0 && rootBlockTime == 0 && minorBlockTime == 0 {
		return
	}
	if chainSize == 0 {
		chainSize = cfg.Quarkchain.ChainSize
	}
	if shardSizePerChain == 0 {
		shardSizePerChain = cfg.Quarkchain.Chains[0].ShardSize
	}
	if rootBlockTime == 0 {
		rootBlockTime = cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime
	}
	if minorBlockTime == 0 {
		minorBlockTime = cfg.Quarkchain.Chains[0].ConsensusConfig.TargetBlockTime
	}
	cfg.Quarkchain.Update(chainSize, shardSizePerChain, rootBlockTime, minorBlockTime)
	updateShardConfig(cfg)
}

func updateSlaves(cfg *config.ClusterConfig, numSlaves int, slaveIpList string) {
	ipList := strings.Split(slaveIpList, ",")
	if numSlaves == 0 {
		numSlaves = len(cfg.SlaveList)
	}
	if len(ipList) == 0 {
		ipList = append(ipList, defaultIp)
	}
	batchSize := numSlaves / len(ipList)
	cfg.SlaveList = make([]*config.SlaveConfig, 0, numSlaves)
	for i := 0; i < numSlaves; i++ {
		slaveCfg := config.NewDefaultSlaveConfig()
		slaveCfg.Port = uint16(38000 + i)
		slaveCfg.ID = fmt.Sprintf("S%d", i)
		slaveCfg.IP = ipList[i/batchSize]
		slaveCfg.ChainMaskList = append(slaveCfg.ChainMaskList, types.NewChainMask(uint32(i|numSlaves)))
		cfg.SlaveList = append(cfg.SlaveList, slaveCfg)
	}
}

func updateP2P(cfg *config.ClusterConfig) {
	if *privatekey != "" {
		cfg.P2P.PrivKey = *privatekey
	}
}

func main() {
	flag.Parse()
	cfg := config.NewClusterConfig()
	err := loadConfig(*cfgFile, cfg)
	if err != nil {
		utils.Fatalf("Failed to load config file %v", err)
	}
	updateChains(cfg, uint32(*chainSize), uint32(*shardSizePerChain), uint32(*rootBlockTime), uint32(*minorBlockTime))
	updateSlaves(cfg, *numSlaves, *slaveIpList)
	updateP2P(cfg)
	bytes, err := json.MarshalIndent(cfg, "", "	")
	if err != nil {
		utils.Fatalf("Failed to marshal json file content, %v", err)
	}
	err = ioutil.WriteFile(*cfgFile, bytes, 0644)
	if err != nil {
		utils.Fatalf("Failed to write file, %v", err)
	}
}
