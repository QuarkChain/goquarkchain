package deploy

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/QuarkChain/goquarkchain/common"

	"github.com/ybbus/jsonrpc"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"golang.org/x/sync/errgroup"
)

var (
	clusterPath       = "../../../cmd/cluster/cluster"
	clusterConfigPath = "./cluster_config_template.json"
	gensisAccountPath = "../../tests/loadtest/accounts"
)

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

type ToolManager struct {
	LocalConfig *LocalConfig
	BootNode    string

	SSHSession   []map[string]*SSHSession
	ClusterIndex int
}

func NewToolManager(config *LocalConfig) *ToolManager {
	tool := &ToolManager{
		LocalConfig: config,
		SSHSession:  make([]map[string]*SSHSession, len(config.Hosts)),
	}
	tool.check()
	tool.init()
	return tool
}

func (t *ToolManager) check() {
	clusterLen := len(t.LocalConfig.Hosts)
	for index := 0; index < clusterLen; index++ {
		if _, ok := t.LocalConfig.Hosts[index]; !ok {
			panic(fmt.Errorf("need clusterID %v", index))
		}
		t.ClusterIndex = index
		if len(t.GetIpListDependTag("master")) == 0 {
			panic(fmt.Errorf("clusterID %v need master", index))
		}
		lenSlave := len(t.GetIpListDependTag("slave"))
		if lenSlave == 0 {
			panic(fmt.Errorf("clusterID %v need slave", index))
		}
		if lenSlave > int(t.LocalConfig.ChainSize) {
			panic(fmt.Errorf("slave's count %d should <= chainSize %d", lenSlave, t.LocalConfig.ChainSize))
		}
		if !common.IsP2(uint32(lenSlave)) {
			panic(fmt.Errorf("slave's count %d must be power of 2", lenSlave))
		}
	}
	t.ClusterIndex = 0

	if len(t.LocalConfig.Hosts) < 1 {
		panic("t.localConfig.IPList should >=1")
	}

	if !common.IsP2(t.LocalConfig.ShardSize) {
		panic(fmt.Errorf("ShardSize %d must be power of 2", t.LocalConfig.ShardSize))
	}

	if t.LocalConfig.ExtraClusterConfig.GasLimit == 0 {
		t.LocalConfig.ExtraClusterConfig.GasLimit = 120000
	}
	if t.LocalConfig.ExtraClusterConfig.TargetMinorBlockTime == 0 {
		t.LocalConfig.ExtraClusterConfig.TargetMinorBlockTime = 10
	}
	if t.LocalConfig.ExtraClusterConfig.TargetRootBlockTime == 0 {
		t.LocalConfig.ExtraClusterConfig.TargetRootBlockTime = 60
	}
}

func (t *ToolManager) init() {
	for index, cluster := range t.LocalConfig.Hosts {
		t.SSHSession[index] = make(map[string]*SSHSession)
		for _, ip := range cluster {
			t.SSHSession[index][ip.IP] = NewSSHConnect(ip.User, ip.Password, ip.IP, int(ip.Port))
		}
	}
}

func (t *ToolManager) GetIpListDependTag(tag string) []string {
	if t.ClusterIndex >= len(t.LocalConfig.Hosts) {
		panic(fmt.Errorf("index:%d host's len:%d", t.ClusterIndex, len(t.LocalConfig.Hosts)))
	}
	ipList := make([]string, 0)
	for _, v := range t.LocalConfig.Hosts[t.ClusterIndex] {
		if strings.Contains(v.Service, tag) {
			ipList = append(ipList, v.IP)
		}
	}
	return ipList
}

func (t *ToolManager) GenClusterConfig() {
	clusterConfig := GenConfigDependInitConfig(t.LocalConfig.ChainSize, t.LocalConfig.ShardSize, t.GetIpListDependTag("slave"), t.LocalConfig.ExtraClusterConfig)

	if t.BootNode == "" {
		sk, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
		CheckErr(err)
		clusterConfig.P2P.PrivKey = hex.EncodeToString(sk.D.Bytes())
		db, err := enode.OpenDB("")
		CheckErr(err)
		node := enode.NewLocalNode(db, sk)
		t.BootNode = node.Node().String()
	} else {
		tIndex := t.ClusterIndex
		t.ClusterIndex = 0
		clusterConfig.P2P.BootNodes = t.BootNode + "@" + t.GetIpListDependTag("master")[0] + ":38291"
		t.ClusterIndex = tIndex
	}
	WriteConfigToFile(clusterConfig, clusterConfigPath)
}

func (t *ToolManager) SendFileToCluster() {
	var g errgroup.Group
	for _, session := range t.SSHSession[t.ClusterIndex] {
		v := session
		g.Go(func() error {
			v.RunCmd("rm -rf  /tmp/QKC")
			v.RunCmd("mkdir /tmp/QKC")

			v.RunCmdIgnoreErr("docker stop $(docker ps -a|grep bjqkc |awk '{print $1}')")
			v.RunCmdIgnoreErr("docker  rm $(docker ps -a|grep bjqkc |awk '{print $1}')")
			log.Debug("==== begin pulling docker image...")
			v.RunCmd("docker pull " + t.LocalConfig.DockerName)
			v.RunCmd("docker run -itd --name bjqkc --network=host " + t.LocalConfig.DockerName)

			v.SendFile(clusterPath, "/tmp/QKC")
			v.SendFile(clusterConfigPath, "/tmp/QKC")

			v.RunCmd("docker exec -itd bjqkc  /bin/bash -c  'mkdir /tmp/QKC/'")
			v.RunCmd("docker cp /tmp/QKC/cluster bjqkc:/tmp/QKC")
			v.RunCmd("docker cp /tmp/QKC/cluster_config_template.json bjqkc:/tmp/QKC")
			return nil
		})
	}
	err := g.Wait()
	CheckErr(err)
}

type SlaveInfo struct {
	IP          string
	ServiceName string
}

func (t *ToolManager) StartCluster(clusterIndex int) {
	masterIp := t.GetIpListDependTag("master")[0]
	slaveIpLists := make([]*SlaveInfo, 0)
	cfg := config.NewClusterConfig()
	err := LoadClusterConfig(clusterConfigPath, cfg)
	CheckErr(err)

	for _, v := range cfg.SlaveList {
		slaveIpLists = append(slaveIpLists, &SlaveInfo{
			IP:          v.IP,
			ServiceName: v.ID,
		})
	}

	t.startSlave(slaveIpLists)
	time.Sleep(5 * time.Second)
	t.startMaster(masterIp)

}

func (t *ToolManager) startMaster(ip string) {
	session := t.SSHSession[t.ClusterIndex][ip]
	session.RunCmd("docker exec -itd bjqkc /bin/bash -c 'chmod +x /tmp/QKC/cluster && /tmp/QKC/cluster --cluster_config /tmp/QKC/cluster_config_template.json --json_rpc_host 0.0.0.0 --json_rpc_private_host 0.0.0.0 >>master.log 2>&1 '")
}

func (t *ToolManager) startSlave(ipList []*SlaveInfo) {
	for _, v := range ipList {
		session := t.SSHSession[t.ClusterIndex][v.IP]
		cmd := "docker exec -itd bjqkc /bin/bash -c 'chmod +x /tmp/QKC/cluster && /tmp/QKC/cluster --cluster_config /tmp/QKC/cluster_config_template.json --service " + v.ServiceName + ">> " + v.ServiceName + ".log 2>&1  '"
		session.RunCmd(cmd)
	}
}

func (t *ToolManager) StartClusters() {
	for index := 0; index < len(t.LocalConfig.Hosts); index++ {
		log.Info("============begin start cluster============", "ClusterID", index)
		log.Info("==== begin gen config")
		t.GenClusterConfig()
		log.Info("==== begin send file to others cluster")
		t.SendFileToCluster()
		log.Info("==== begin start cluster")
		t.StartCluster(index)
		log.Info("============end start cluster============", "ClusterID", index)
		t.ClusterIndex++
	}
}

func (t *ToolManager) CheckPeerStatus() {
	for true {
		time.Sleep(10 * time.Second)
		t.ClusterIndex = 0
		for index := 0; index < len(t.LocalConfig.Hosts); index++ {
			masterIP := t.GetIpListDependTag("master")[0]
			pubUrl := fmt.Sprintf("http://%s:38491", masterIP)
			client := jsonrpc.NewClient(pubUrl)
			resp, err := client.Call("getPeers")
			if err != nil {
				panic(fmt.Errorf("getPeer from ip %v err %v", masterIP, err))
			}
			if resp == nil {
				panic(fmt.Errorf("getPeer from ip %v resp==nil", masterIP))
			}
			if resp.Error != nil {
				panic(fmt.Errorf("getPeer from ip %v err %v", masterIP, resp.Error))
			}
			peerInfo := resp.Result.(map[string]interface{})["peers"].([]interface{})
			log.Info("check peer status", "masterIP", masterIP, "peers len", len(peerInfo), "data", peerInfo)
			t.ClusterIndex++
			if len(peerInfo) == len(t.LocalConfig.Hosts)-1 {
				log.Info("========start cluster successfully", "cluster cnt", len(t.LocalConfig.Hosts), "peer number", len(peerInfo))
				return
			}
		}
	}
}
