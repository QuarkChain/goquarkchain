package test

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/master"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/cluster/slave"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
	"time"
)

var (
	clientIdentifier = "master"
)

type clusterNode struct {
	master    *master.QKCMasterBackend
	slavelist []*slave.SlaveBackend
	clstrCfg  *config.ClusterConfig
	services  map[string]*service.Node
}

type Clusterlist []*clusterNode

func (cl Clusterlist) Start() {
	var started []int
	for idx, clstr := range cl {
		if err := clstr.Start(); err != nil {
			for _, idx := range started {
				cl[idx].Stop()
			}
			utils.Fatalf("failed to start cluster, index: %d: %v", idx, err)
		}
		started = append(started, idx)
	}
}

func (cl Clusterlist) Stop() {
	for _, clstr := range cl {
		clstr.Stop()
	}
}

func getClusterConfig(index uint16, geneAcc *account.Account, chainSize,
shardSize, slaveSize uint32, geneRHeights map[uint32]uint32) *config.ClusterConfig {
	cfg := config.NewClusterConfig()
	cfg.Clean = true
	cfg.GenesisDir = ""
	cfg.DbPathRoot = ""
	cfg.Clean = true
	cfg.P2PPort += index
	cfg.JSONRPCPort += index
	cfg.PrivateJSONRPCPort += index
	cfg.Quarkchain.ChainSize = chainSize
	cfg.Quarkchain.Update(chainSize, shardSize, 10, 5)
	cfg.Quarkchain.Root.GRPCPort += index
	cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime = 10
	cfg.Quarkchain.Root.ConsensusType = config.PoWSimulate
	cfg.Quarkchain.Root.DifficultyAdjustmentCutoffTime = 40
	cfg.Quarkchain.Root.MaxStaleRootBlockHeightDiff = 1024

	fullShardIds := cfg.Quarkchain.GetGenesisShardIds()
	for _, fullShardId := range fullShardIds {
		shardCfg := cfg.Quarkchain.GetShardConfigByFullShardID(fullShardId)
		addr := geneAcc.QKCAddress.AddressInShard(fullShardId)
		shardCfg.Genesis.Alloc[addr] = big.NewInt(10000000000)
		shardCfg.DifficultyAdjustmentCutoffTime = 7
		shardCfg.DifficultyAdjustmentFactor = 512
		shardCfg.ConsensusType = config.PoWSimulate
		shardCfg.Genesis.Difficulty = 10
		shardCfg.PoswConfig.WindowSize = 2
	}

	// make different genesis root height
	if geneRHeights != nil && len(geneRHeights) == int(chainSize*shardSize) {
		for chainId := 0; chainId < int(chainSize); chainId++ {
			for shardId := 0; shardId < int(shardSize); shardId++ {
				fullShardId := chainId<<16 | int(shardSize) | shardId
				shardCfg := cfg.Quarkchain.GetShardConfigByFullShardID(uint32(fullShardId))
				shardCfg.Genesis.RootHeight = geneRHeights[uint32(fullShardId)]
			}
		}
	}

	cfg.SlaveList = make([]*config.SlaveConfig, 0, 1)
	for i := 0; i < int(slaveSize); i++ {
		slave := config.NewDefaultSlaveConfig()
		slave.Port = 38000 + uint16(i) + index
		slave.ID = fmt.Sprintf("S%d", i)
		slave.ChainMaskList = append(slave.ChainMaskList, types.NewChainMask(uint32(i|int(slaveSize))))
		cfg.SlaveList = append(cfg.SlaveList, slave)
	}

	for _, slaveCfg := range cfg.SlaveList {
		slaveCfg.Port += index
	}

	cfg.Quarkchain.SkipMinorDifficultyCheck = true
	cfg.Quarkchain.SkipRootDifficultyCheck = true
	cfg.EnableTransactionHistory = true
	cfg.DbPathRoot = ""

	// TODO think about how to use mem db
	return cfg
}

func makeConfigNode(index uint16, geneAcc *account.Account, chainSize, shardSize, slaveSize uint32, geneRHeights map[uint32]uint32) (*config.ClusterConfig, map[string]*service.Node) {
	var (
		nodeList = make(map[string]*service.Node)
		clstrCfg = getClusterConfig(index, geneAcc, chainSize, shardSize, slaveSize, geneRHeights)
	)

	// slave nodes
	for idx, slaveCfg := range clstrCfg.SlaveList {
		svrCfg := defaultNodeConfig()
		svrCfg.Name = slaveCfg.ID
		svrCfg.SvrHost = slaveCfg.IP
		svrCfg.SvrPort = slaveCfg.Port
		svrCfg.IPCPath = ""
		svrCfg.DataDir = fmt.Sprintf("./data/%s_%d_%d", slaveCfg.ID, index, idx)
		svrCfg.P2P.ListenAddr = fmt.Sprintf(":%d", clstrCfg.P2PPort)
		svrCfg.P2P.MaxPeers = int(clstrCfg.P2P.MaxPeers)
		node, err := service.New(svrCfg)
		if err != nil {
			utils.Fatalf("Failed to create the slave_%s: %v", svrCfg.Name, err)
		}
		node.SetIsMaster(false)
		utils.RegisterSlaveService(node, clstrCfg, slaveCfg)
		nodeList[svrCfg.Name] = node
	}

	// master node
	svrCfg := defaultNodeConfig()
	svrCfg.Name = clientIdentifier
	svrCfg.SvrPort = clstrCfg.Quarkchain.Root.GRPCPort
	svrCfg.IPCPath = ""
	svrCfg.DataDir = fmt.Sprintf("./data/%s_%d", clientIdentifier, index)
	node, err := service.New(svrCfg)
	if err != nil {
		utils.Fatalf("Failed to create the master: %v", err)
	}
	node.SetIsMaster(true)
	utils.RegisterMasterService(node, clstrCfg)
	nodeList[clientIdentifier] = node
	return clstrCfg, nodeList
}

func CreateClusterList(numCluster int, chainSize, shardSize, slaveSize uint32, geneRHeights map[uint32]uint32) (*account.Account, Clusterlist) {
	clusterList := make([]*clusterNode, 0, numCluster)
	geneAcc, err := createAcc()
	if err != nil {
		utils.Fatalf("failed to create cluster list: %v", err)
	}
	for i := 0; i < numCluster; i++ {
		clstrCfg, nodeList := makeConfigNode(uint16(i), &geneAcc, chainSize, shardSize, slaveSize, geneRHeights)
		clusterList = append(clusterList, &clusterNode{clstrCfg: clstrCfg, services: nodeList})
	}
	return &geneAcc, clusterList
}

func (c *clusterNode) Stop() {
	if err := c.services[clientIdentifier].Stop(); err != nil {
		utils.Fatalf("Failed to stop %s: %v", clientIdentifier, err)
	}
	for key, node := range c.services {
		if key != clientIdentifier {
			if err := node.Stop(); err != nil {
				utils.Fatalf("Failed to stop %s: %v", key, err)
			}
		}
	}
}

func (c *clusterNode) Start() (err error) {
	var started []string
	for key, node := range c.services {
		if key == clientIdentifier {
			continue
		}
		if err = node.Start(); err != nil {
			utils.Fatalf("failed to start %s: %v", key, err)
			for _, sk := range started {
				if er := c.services[sk].Stop(); er != nil {
					utils.Fatalf("failed to stop %s, when can't start %s: %v", sk, key, er)
				}
			}
		}
		started = append(started, key)
	}
	if err = c.services[clientIdentifier].Start(); err != nil {
		for _, sk := range started {
			if err := c.services[sk].Stop(); err != nil {
				utils.Fatalf("failed to stop %s, when can't start %s: %v", sk, clientIdentifier, err)
			}
		}
	}
	mstr := c.GetMaster()
	return mstr.InitCluster()
}

func (c *clusterNode) GetMaster() *master.QKCMasterBackend {
	if c.master != nil {
		return c.master
	}
	if err := c.services[clientIdentifier].Service(&c.master); err != nil {
		utils.Fatalf("master service not running %v", err)
	}
	return c.master
}

func (c *clusterNode) GetSlavelist() []*slave.SlaveBackend {
	if c.slavelist != nil {
		return c.slavelist
	}
	for _, slv := range c.clstrCfg.SlaveList {
		var sv *slave.SlaveBackend
		if err := c.services[slv.ID].Service(&sv); err != nil {
			c.slavelist = nil
			utils.Fatalf("slave service not running %v", err)
		}
		c.slavelist = append(c.slavelist, sv)
	}
	return c.slavelist
}

func (c *clusterNode) GetSlave(name string) *slave.SlaveBackend {
	var slv *slave.SlaveBackend
	if _, err := c.clstrCfg.GetSlaveConfig(name); err != nil {
		utils.Fatalf("service type is error: %v", err)
	}
	if err := c.services[name].Service(&slv); err != nil {
		utils.Fatalf("slave service not running %v", err)
	}
	return slv
}

func (c *clusterNode) GetShard(fullShardId uint32) (shrd *shard.ShardBackend) {
	c.GetSlavelist()
	for _, slv := range c.slavelist {
		shrd = slv.GetShard(fullShardId)
		if shrd != nil {
			return
		}
	}
	return
}

func (c *clusterNode) GetShardState(fullShardId uint32) *core.MinorBlockChain {
	shrd := c.GetShard(fullShardId)
	if shrd != nil {
		return shrd.MinorBlockChain
	}
	utils.Fatalf("shard not exist, fullShardId, time: %d: %d", time.Now().Unix(), fullShardId)
	return nil
}

func (c *clusterNode) CreateAndInsertBlocks(fullShardId uint32) (rBlock *types.RootBlock) {
	var (
		mstr = c.GetMaster()
		// slaveList = c.GetSlavelist()
	)
	// insert root block
	iBlock, err := mstr.CreateBlockToMine()
	if err != nil {
		goto FAILED
	}
	rBlock = iBlock.(*types.RootBlock)
	fmt.Println("----------------", rBlock.Number(), rBlock.Time())
	for _, hd := range rBlock.MinorBlockHeaders() {
		fmt.Println("--------", hd.Hash().Hex(), hd.Number, hd.Time)
	}

	if err = mstr.AddRootBlock(rBlock); err != nil {
		goto FAILED
	}

	// insert minor block
	/*for _, slv := range slaveList {
		shrd := slv.GetShard(fullShardId)
		if shrd == nil {
			continue
		}
		if iBlock, err = shrd.CreateBlockToMine(); err != nil {
			goto FAILED
		}
		mBlock := iBlock.(*types.MinorBlock)
		fmt.Println("----------- minor: ", mBlock.Header().Time)
		if err = shrd.AddMinorBlock(mBlock); err != nil {
			goto FAILED
		}
	}*/
	return
FAILED:
	utils.Fatalf("failed to create and add root/minor block", "err", err)
	return
}
