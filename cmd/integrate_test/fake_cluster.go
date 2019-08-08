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
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"math/rand"
	"runtime"
	"runtime/debug"
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

func makeClusterNode(index uint16, clstrCfg *config.ClusterConfig, bootNodes []*enode.Node) *clusterNode {
	var (
		nodeList = make(map[string]*service.Node)
		priv     = getPrivKeyByIndex(int(index))
	)
	rand.Seed(time.Now().Unix())
	random := rand.Int() % 100000
	// slave nodes
	for idx, slaveCfg := range clstrCfg.SlaveList {
		var svrCfg = defaultNodeConfig()
		svrCfg.P2P.PrivateKey = priv
		svrCfg.P2P.BootstrapNodes = bootNodes
		svrCfg.P2P.ListenAddr = fmt.Sprintf("127.0.0.1:%d", clstrCfg.P2PPort)
		svrCfg.P2P.MaxPeers = int(clstrCfg.P2P.MaxPeers)
		svrCfg.IPCPath = ""
		svrCfg.Name = slaveCfg.ID
		svrCfg.SvrHost = slaveCfg.IP
		svrCfg.SvrPort = slaveCfg.Port
		svrCfg.DataDir = fmt.Sprintf("/tmp/integrate_test/%d/%s_%d_%d", random, slaveCfg.ID, index, idx)
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
	svrCfg.DataDir = fmt.Sprintf("/tmp/integrate_test/%d/%s_%d", random, clientIdentifier, index)
	node, err := service.New(svrCfg)
	if err != nil {
		utils.Fatalf("Failed to create the master: %v", err)
	}
	node.SetIsMaster(true)
	utils.RegisterMasterService(node, clstrCfg)
	nodeList[clientIdentifier] = node
	return &clusterNode{index: int(index), clstrCfg: clstrCfg, services: nodeList}
}

func CreateClusterList(numCluster int, clstrCfg []*config.ClusterConfig) (*account.Account, Clusterlist) {
	runtime.GOMAXPROCS(runtime.NumCPU() * 4)
	debug.SetGCPercent(60)
	clusterList := make([]*clusterNode, numCluster+1, numCluster+1)
	geneAcc := getAccByIndex(0)
	for i := 0; i <= numCluster; i++ {
		idx := i
		nodes := getBootNodes(clstrCfg[idx].P2P.BootNodes)
		if idx == numCluster && len(nodes) > numCluster {
			nodes = nodes[:idx+1]
		}
		clusterList[idx] = makeClusterNode(uint16(idx), clstrCfg[idx], nodes)
	}
	return geneAcc, clusterList
}

func (c *clusterNode) Stop() {
	if !c.status {
		return
	}
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
	c.status = false
}

func (c *clusterNode) Start() (err error) {
	var (
		started = make(map[string]*service.Node)
	)
	if c.status {
		return
	}

	for key, node := range c.services {
		if key == clientIdentifier {
			continue
		}
		key := key
		err := node.Start()
		if err == nil {
			started[key] = node
		} else {
			c.Stop()
		}
	}

	time.Sleep(1 * time.Second)
	if err = c.services[clientIdentifier].Start(); err != nil {
		c.Stop()
		return
	}

	if err = c.GetMaster().Start(); err != nil {
		c.Stop()
		return
	}

	c.status = true
	return
}

func (c *clusterNode) GetMaster() *master.QKCMasterBackend {
	if c.master != nil {
		return c.master
	}
	if err := c.services[clientIdentifier].Service(&c.master); err != nil {
		utils.Fatalf("master service not running, cluster index: %d, err: %v", c.index, err)
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
			debug.PrintStack()
			utils.Fatalf("slave service not running, cluster index: %d, slave id: %s, err: %v", c.index, slv.ID, err)
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
			utils.Fatalf("has no such shard, fullShardId: %d", fullShardId)
		}
		iBlock, _, err := shrd.CreateBlockToMine()
		if err != nil {
			utils.Fatalf("can't create minor block, fullShardId: %d, err: %v", fullShardId, err)
		}
		if err := shrd.AddMinorBlock(iBlock.(*types.MinorBlock)); err != nil {
			utils.Fatalf("failed to add minor block, err: %v", err)
		}
	}
}

func (c *clusterNode) CreateAndInsertBlocks(fullShards []uint32) (rBlock *types.RootBlock) {

	if fullShards != nil && len(fullShards) > 0 {
		c.createAllShardsBlock(fullShards)
	}
	start := time.Now().Unix() - 2
	var seconds int64 = 0
	fullShardIdList := c.clstrCfg.Quarkchain.GetGenesisShardIds()
	for _, id := range fullShardIdList {
		shrd := c.GetShard(id)
		if shrd == nil {
			continue
		}
		mBlock := shrd.MinorBlockChain.CurrentBlock()
		diff := int64(mBlock.Time()) - start
		if diff > seconds {
			seconds = diff
		}
	}

	time.Sleep(time.Duration(seconds) * time.Second)
	// insert root block
	iBlock, _, err := c.GetMaster().CreateBlockToMine()
	if err != nil {
		utils.Fatalf("failed to create and add root/minor block, err: %v", err)
	}
	rBlock = iBlock.(*types.RootBlock)
	if err = c.GetMaster().AddRootBlock(rBlock); err != nil {
		utils.Fatalf("failed to create and add root/minor block, err: %v", err)
	}
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
