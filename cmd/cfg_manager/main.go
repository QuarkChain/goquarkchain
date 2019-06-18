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
	initConf = flag.String("init_params", "", "init conf for gen full conf")
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

func updateShardConfig(cfg *config.ClusterConfig, initParams *genConfigParams) {
	address := common.HexToAddress(string(regexp.MustCompile("0x[0-9a-fA-F]{40}").FindString(initParams.CoinBaseAddress)))
	fullShardIds := cfg.Quarkchain.GetGenesisShardIds()
	for _, fullShardId := range fullShardIds {
		shard := cfg.Quarkchain.GetShardConfigByFullShardID(fullShardId)
		shard.ConsensusType = initParams.ConsensusType
		shard.Genesis.Difficulty = uint64(*initParams.Difficulty)
		addr := account.NewAddress(address, fullShardId)
		shard.Genesis.Alloc[addr] = new(big.Int).Mul(big.NewInt(1000000), config.QuarkashToJiaozi)
		shard.CoinbaseAddress = addr
	}
}

func updateChains(cfg *config.ClusterConfig, initParams *genConfigParams) {
	if uint32(*initParams.ChainSize) == 0 && uint32(*initParams.ShardSizePerChain) == 0 && uint32(*initParams.RootBlockTime) == 0 && uint32(*initParams.MinorBlockTime) == 0 {
		return
	}
	cfg.GenesisDir = initParams.GenesisDir
	cfg.Clean = true
	cfg.Quarkchain.NetworkID = uint32(*initParams.NetworkId)

	cfg.Quarkchain.Update(uint32(*initParams.ChainSize), uint32(*initParams.ShardSizePerChain), uint32(*initParams.RootBlockTime), uint32(*initParams.MinorBlockTime))
	cfg.Quarkchain.Root.ConsensusType = initParams.ConsensusType
	receipt := common.HexToAddress(string(regexp.MustCompile("0x[0-9a-fA-F]{40}").FindString(initParams.CoinBaseAddress)))
	qkcAddress := account.NewAddress(receipt, 0)
	cfg.Quarkchain.Root.CoinbaseAddress = qkcAddress
	updateShardConfig(cfg, initParams)
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

func main() {
	flag.Parse()
	initParams := new(genConfigParams)
	if initParams == nil {
		utils.Fatalf("please set init config")
	}
	if err := loadConfig(*initConf, initParams); err != nil {
		utils.Fatalf("%v", err)
	}
	cfg := config.NewClusterConfig()
	updateChains(cfg, initParams)
	updateSlaves(cfg, int(*initParams.NumSlaves), initParams.SlaveIpList)
	cfg.P2P.PrivKey = initParams.Privatekey
	bytes, err := json.MarshalIndent(cfg, "", "	")
	if err != nil {
		utils.Fatalf("Failed to marshal json file content, %v", err)
	}
	err = ioutil.WriteFile(initParams.CfgFile, bytes, 0644)
	if err != nil {
		utils.Fatalf("Failed to write file, %v", err)
	}
}
