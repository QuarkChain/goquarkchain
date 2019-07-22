package test

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/master"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/cluster/slave"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"golang.org/x/sync/errgroup"
	"math/big"
	"runtime/debug"
	"strings"
	"time"
)

var (
	clientIdentifier = "master"
)

type clusterNode struct {
	index     int
	status    bool
	master    *master.QKCMasterBackend
	slavelist []*slave.SlaveBackend
	clstrCfg  *config.ClusterConfig
	services  map[string]*service.Node
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
	cfg.Quarkchain.Root.ConsensusConfig.RemoteMine = true
	cfg.Quarkchain.Root.ConsensusType = config.PoWSimulate
	cfg.Quarkchain.Root.DifficultyAdjustmentCutoffTime = 40
	cfg.Quarkchain.Root.MaxStaleRootBlockHeightDiff = 1024
	if int(index) < len(privStrs) {
		cfg.P2P.PrivKey = privStrs[index]
	}
	cfg.P2P.BootNodes = "" // bootNode
	coinBaseAddr := getAccByIndex(1).QKCAddress

	fullShardIds := cfg.Quarkchain.GetGenesisShardIds()
	for _, fullShardId := range fullShardIds {
		shardCfg := cfg.Quarkchain.GetShardConfigByFullShardID(fullShardId)
		addr := geneAcc.QKCAddress.AddressInShard(fullShardId)
		shardCfg.CoinbaseAddress = coinBaseAddr.AddressInShard(fullShardId)
		shardCfg.Genesis.Alloc[addr] = big.NewInt(10000000000)
		shardCfg.Genesis.Alloc[shardCfg.CoinbaseAddress] = big.NewInt(10000000000)
		shardCfg.Genesis.Difficulty = 10
		shardCfg.DifficultyAdjustmentCutoffTime = 7
		shardCfg.DifficultyAdjustmentFactor = 512
		shardCfg.ConsensusConfig.RemoteMine = true
		shardCfg.ConsensusType = config.PoWSimulate
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

func fakeClusterNode(clstrNum int, geneAcc *account.Account, chainSize, shardSize, slaveSize uint32,
	geneRHeights map[uint32]uint32) (*config.ClusterConfig, map[string]*service.Node) {
	var (
		nodeList  = make(map[string]*service.Node)
		clstrCfg  = getClusterConfig(uint16(clstrNum), geneAcc, chainSize, shardSize, slaveSize, geneRHeights)
		bootNodes = make([]*enode.Node, 0, 0)
		priv      *ecdsa.PrivateKey
	)

	urls := strings.Split(fakeBootNode, ",")
	for idx, url := range urls {
		if idx >= clstrNum {
			break
		}
		node, err := enode.ParseV4(url)
		if err != nil {
			utils.Fatalf("Bootstrap URL invalid", "enode", url, "err", err)
		}
		bootNodes = append(bootNodes, node)
	}

	// slave nodes
	for idx, slaveCfg := range clstrCfg.SlaveList {
		var svrCfg = new(service.Config)
		_ = common.DeepCopy(&svrCfg, defaultNodeConfig())
		svrCfg.P2P.PrivateKey = priv
		svrCfg.P2P.BootstrapNodes = bootNodes
		svrCfg.P2P.ListenAddr = fmt.Sprintf("127.0.0.1:%d", clstrCfg.P2PPort)
		svrCfg.P2P.MaxPeers = int(clstrCfg.P2P.MaxPeers)
		svrCfg.IPCPath = ""
		svrCfg.Name = slaveCfg.ID
		svrCfg.SvrHost = slaveCfg.IP
		svrCfg.SvrPort = slaveCfg.Port
		svrCfg.DataDir = fmt.Sprintf("./data/%s_%d_%d", slaveCfg.ID, clstrNum, idx)
		node, err := service.New(svrCfg)
		if err != nil {
			utils.Fatalf("Failed to create the slave_%s: %v", svrCfg.Name, err)
		}
		node.SetIsMaster(false)
		utils.RegisterSlaveService(node, clstrCfg, slaveCfg)
		nodeList[svrCfg.Name] = node
	}

	// master node
	var svrCfg = new(service.Config)
	_ = common.DeepCopy(svrCfg, defaultNodeConfig())
	svrCfg.P2P.PrivateKey = priv
	svrCfg.P2P.BootstrapNodes = bootNodes
	svrCfg.P2P.ListenAddr = fmt.Sprintf("127.0.0.1:%d", clstrCfg.P2PPort)
	svrCfg.P2P.MaxPeers = int(clstrCfg.P2P.MaxPeers)
	svrCfg.IPCPath = ""
	svrCfg.Name = clientIdentifier
	svrCfg.SvrHost = clstrCfg.Quarkchain.Root.GRPCHost
	svrCfg.SvrPort = clstrCfg.Quarkchain.Root.GRPCPort
	svrCfg.DataDir = fmt.Sprintf("./data/%s_%d", clientIdentifier, clstrNum)
	node, err := service.New(svrCfg)
	if err != nil {
		utils.Fatalf("Failed to create the master: %v", err)
	}
	node.SetIsMaster(true)
	utils.RegisterMasterService(node, clstrCfg)
	nodeList[clientIdentifier] = node
	return clstrCfg, nodeList
}

func makeClusterNode(index uint16, geneAcc *account.Account, chainSize, shardSize, slaveSize uint32,
	geneRHeights map[uint32]uint32) (*config.ClusterConfig, map[string]*service.Node) {
	var (
		nodeList  = make(map[string]*service.Node)
		clstrCfg  = getClusterConfig(index, geneAcc, chainSize, shardSize, slaveSize, geneRHeights)
		bootNodes = make([]*enode.Node, 0, 0)
		priv      = getPrivKeyByIndex(int(index))
	)
	/*jsonCfg, _ := json.MarshalIndent(clstrCfg, "", "\t")
	fmt.Println("--------- cluster config", string(jsonCfg))*/

	if index != 0 && clstrCfg.P2P.BootNodes != "" {
		urls := strings.Split(clstrCfg.P2P.BootNodes, ",")
		for _, url := range urls {
			node, err := enode.ParseV4(url)
			if err != nil {
				utils.Fatalf("Bootstrap URL invalid", "enode", url, "err", err)
			}
			bootNodes = append(bootNodes, node)
		}
	}

	// slave nodes
	for idx, slaveCfg := range clstrCfg.SlaveList {
		var svrCfg = new(service.Config)
		_ = common.DeepCopy(&svrCfg, defaultNodeConfig())
		svrCfg.P2P.PrivateKey = priv
		svrCfg.P2P.BootstrapNodes = bootNodes
		svrCfg.P2P.ListenAddr = fmt.Sprintf("127.0.0.1:%d", clstrCfg.P2PPort)
		svrCfg.P2P.MaxPeers = int(clstrCfg.P2P.MaxPeers)
		svrCfg.IPCPath = ""
		svrCfg.Name = slaveCfg.ID
		svrCfg.SvrHost = slaveCfg.IP
		svrCfg.SvrPort = slaveCfg.Port
		svrCfg.DataDir = fmt.Sprintf("./data/%s_%d_%d", slaveCfg.ID, index, idx)
		node, err := service.New(svrCfg)
		if err != nil {
			utils.Fatalf("Failed to create the slave_%s: %v", svrCfg.Name, err)
		}
		node.SetIsMaster(false)
		utils.RegisterSlaveService(node, clstrCfg, slaveCfg)
		nodeList[svrCfg.Name] = node
	}

	// master node
	var svrCfg = defaultNodeConfig()
	svrCfg.P2P.PrivateKey = priv
	svrCfg.P2P.BootstrapNodes = bootNodes
	svrCfg.P2P.ListenAddr = fmt.Sprintf("127.0.0.1:%d", clstrCfg.P2PPort)
	svrCfg.P2P.MaxPeers = int(clstrCfg.P2P.MaxPeers)
	svrCfg.IPCPath = ""
	svrCfg.Name = clientIdentifier
	svrCfg.SvrHost = clstrCfg.Quarkchain.Root.GRPCHost
	svrCfg.SvrPort = clstrCfg.Quarkchain.Root.GRPCPort
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
	clusterList := make([]*clusterNode, numCluster+1, numCluster+1)
	geneAcc := getAccByIndex(0)
	var g errgroup.Group
	for i := 0; i <= numCluster; i++ {
		i := i
		g.Go(func() error {
			var (
				cfg      *config.ClusterConfig
				nodeList map[string]*service.Node
			)
			if i < numCluster {
				cfg, nodeList = makeClusterNode(uint16(i), geneAcc, chainSize, shardSize, slaveSize, geneRHeights)
			} else {
				cfg, nodeList = fakeClusterNode(numCluster, geneAcc, chainSize, shardSize, slaveSize, nil)
			}
			clusterList[i] = &clusterNode{index: i, clstrCfg: cfg, services: nodeList}
			return nil
		})
	}
	defer g.Wait()
	return geneAcc, clusterList
}

func (c *clusterNode) Stop() {
	/*if err := c.services[clientIdentifier].Stop(); err != nil {
		utils.Fatalf("Failed to stop %s: %v", clientIdentifier, err)
	}*/
	if !c.status {
		return
	}
	c.status = false
	for key, node := range c.services {
		if key != clientIdentifier {
			if err := node.Stop(); err != nil {
				utils.Fatalf("Failed to stop %s: %v", key, err)
			}
		}
	}
}

func (c *clusterNode) Start() (err error) {
	var (
		started = make([]*service.Node, len(c.services), len(c.services))
		g       errgroup.Group
		idx     int
	)
	if c.status {
		return
	}

	for key, node := range c.services {
		if key == clientIdentifier {
			continue
		}
		node := node
		i := idx
		g.Go(func() error {
			err := node.Start()
			if err == nil {
				started[i] = node
			}
			return err
		})
		idx++
	}

	stop := func() {
		for _, nd := range started {
			if nd != nil {
				if err := nd.Stop(); err != nil {
					fmt.Println("failed to stop slave", "err", err)
				}
			}
		}
	}
	if err = g.Wait(); err != nil {
		stop()
		return
	} else {
		if err = c.services[clientIdentifier].Start(); err != nil {
			stop()
			return
		}
	}

	mstr := c.GetMaster()
	if err = mstr.Start(); err == nil {
		c.status = true
	}
	return err
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

func (c *clusterNode) createAllShardsBlock(fullShardIds []uint32) {
	for _, fullShardId := range fullShardIds {
		shrd := c.GetShard(fullShardId)
		if shrd == nil {
			debug.PrintStack()
			utils.Fatalf("has no such shard, fullShardId: %d", fullShardId)
		}
		iBlock, err := shrd.CreateBlockToMine()
		if err != nil {
			utils.Fatalf("can't create minor block, fullShardId: %d, err: %v", fullShardId, err)
		}
		if err := shrd.AddMinorBlock(iBlock.(*types.MinorBlock)); err != nil {
			utils.Fatalf("failed to add minor block, err: %v", err)
		}
	}
}

func (c *clusterNode) CreateAndInsertBlocks(fullShards []uint32, seconds time.Duration) (rBlock *types.RootBlock) {
	if fullShards != nil && len(fullShards) > 0 {
		c.createAllShardsBlock(fullShards)
	}
	time.Sleep(seconds * time.Second)
	// insert root block
	iBlock, err := c.GetMaster().CreateBlockToMine()
	if err != nil {
		goto FAILED
	}
	rBlock = iBlock.(*types.RootBlock)
	if err = c.GetMaster().AddRootBlock(rBlock); err != nil {
		goto FAILED
	}

	return
FAILED:
	utils.Fatalf("failed to create and add root/minor block, err: %v", err)
	return
}

func (c *clusterNode) getProtocol() *p2p.Protocol {
	mstr := c.GetMaster()
	subProtocols := mstr.Protocols()
	return &subProtocols[0]
}

func (c *clusterNode) getP2PServer() *p2p.Server {
	return c.services[clientIdentifier].Server()
}
