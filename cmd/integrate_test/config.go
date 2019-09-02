package test

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

func defaultNodeConfig() *service.Config {
	serviceConfig := &service.DefaultConfig
	serviceConfig.Name = ""
	serviceConfig.Version = ""
	serviceConfig.IPCPath = "qkc.ipc"
	serviceConfig.GRPCModules = []string{"rpc"}
	serviceConfig.GRPCEndpoint = fmt.Sprintf("%s:%d", "127.0.0.1", config.GrpcPort)
	return serviceConfig
}

func GetClusterConfig(numClstrs int, chainSize, shardSize, slaveSize uint32, geneRHeights map[uint32]uint32,
	bootnode string, consensusType string, remote bool) []*config.ClusterConfig {
	clstrCfglist := make([]*config.ClusterConfig, numClstrs+1, numClstrs+1)
	for i := 0; i <= numClstrs; i++ {
		if i == numClstrs {
			bootnode = fakeBootNode
		}
		cfg := defaultClusterConfig(chainSize, shardSize, slaveSize, geneRHeights, bootnode, consensusType, remote)
		clstrCfglist[i] = getClusterConfig(uint16(i), cfg)
	}
	return clstrCfglist
}

func getClusterConfig(index uint16, cfg *config.ClusterConfig) *config.ClusterConfig {
	addrList := make([]*account.Address, len(privStrs), len(privStrs))
	for i := 0; i < len(privStrs); i++ {
		addrList[i] = &getAccByIndex(i).QKCAddress
	}
	cfg.P2PPort += index
	cfg.JSONRPCPort += index
	cfg.PrivateJSONRPCPort += index
	cfg.Quarkchain.GRPCPort += index
	if int(index) < len(privStrs) {
		cfg.P2P.PrivKey = privStrs[index]
	}

	for _, slaveCfg := range cfg.SlaveList {
		slaveCfg.Port = slaveCfg.Port + 10*index
	}
	return cfg
}

func defaultClusterConfig(chainSize, shardSize, slaveSize uint32, geneRHeights map[uint32]uint32,
	bootnode string, consensusType string, remote bool) *config.ClusterConfig {
	addrList := make([]*account.Address, len(privStrs), len(privStrs))
	for i := 0; i < len(privStrs); i++ {
		addrList[i] = &getAccByIndex(i).QKCAddress
	}
	cfg := config.NewClusterConfig()
	cfg.Clean = true
	cfg.GenesisDir = ""
	cfg.DbPathRoot = ""
	cfg.Quarkchain.ChainSize = chainSize
	cfg.Quarkchain.Update(chainSize, shardSize, 10, 5)
	cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime = 10
	cfg.Quarkchain.Root.ConsensusConfig.RemoteMine = remote
	cfg.Quarkchain.Root.ConsensusType = consensusType
	cfg.Quarkchain.Root.DifficultyAdjustmentCutoffTime = 40
	cfg.Quarkchain.Root.MaxStaleRootBlockHeightDiff = 1024

	cfg.P2P.BootNodes = bootnode

	fullShardIds := cfg.Quarkchain.GetGenesisShardIds()
	for _, id := range fullShardIds {
		sig := true
		for geneId, val := range geneRHeights {
			if id == geneId && val > 0 {
				sig = false
				break
			}
		}
		if sig {
			cfg.Quarkchain.Root.CoinbaseAddress = addrList[1].AddressInShard(id)
			break
		}
	}

	for _, fullShardId := range fullShardIds {
		shardCfg := cfg.Quarkchain.GetShardConfigByFullShardID(fullShardId)
		shardCfg.CoinbaseAddress = addrList[1].AddressInShard(fullShardId)
		for _, addr := range addrList {
			addr := addr.AddressInShard(fullShardId)
			alloc := config.Allocation{Balances: map[string]*big.Int{
				"QKC": big.NewInt(int64(genesisBalance)),
			}}
			shardCfg.Genesis.Alloc[addr] = alloc
		}
		// shardCfg.Genesis.Alloc[account.CreatEmptyAddress(fullShardId)] = big.NewInt(int64(genesisBalance))
		shardCfg.Genesis.Difficulty = 10
		shardCfg.DifficultyAdjustmentCutoffTime = 7
		shardCfg.DifficultyAdjustmentFactor = 512
		shardCfg.ConsensusConfig.RemoteMine = remote
		shardCfg.ConsensusType = consensusType
		shardCfg.Genesis.Difficulty = 10
		shardCfg.PoswConfig.WindowSize = 2
		// extra minor block headers in root block.
		shardCfg.ExtraShardBlocksInRootBlock = 10
		if _, ok := geneRHeights[fullShardId]; ok {
			shardCfg.Genesis.RootHeight = geneRHeights[fullShardId]
		}
	}

	cfg.SlaveList = make([]*config.SlaveConfig, 0, slaveSize)
	for i := 0; i < int(slaveSize); i++ {
		slave := config.NewDefaultSlaveConfig()
		slave.Port = 38000 + uint16(i)
		slave.ID = fmt.Sprintf("S%d", i)
		slave.ChainMaskList = append(slave.ChainMaskList, types.NewChainMask(uint32(i|int(slaveSize))))
		cfg.SlaveList = append(cfg.SlaveList, slave)
	}

	cfg.Quarkchain.SkipMinorDifficultyCheck = true
	cfg.Quarkchain.SkipRootDifficultyCheck = true
	cfg.EnableTransactionHistory = true

	// TODO think about how to use mem db
	return cfg
}
