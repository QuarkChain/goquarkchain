package deploy

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"golang.org/x/sync/errgroup"
)

var (
	clusterPath       = "../../../cmd/cluster/cluster"
	clusterConfigPath = "./cluster_config_template.json"
)

func CheckErr(err error) {
	if err != nil {
		panic(err)
	}
}

type ToolManager struct {
	localConfig *LocalConfig
	SSHSession  map[string]*SSHSession
}

func NewToolManager(config *LocalConfig) *ToolManager {
	tool := &ToolManager{
		localConfig: config,
		SSHSession:  make(map[string]*SSHSession),
	}
	tool.check()
	tool.init()
	return tool
}

func (t *ToolManager) check() {
	if len(t.localConfig.Hosts) < 1 {
		panic("t.localConfig.IPList should >=1")
	}
	if t.localConfig.ShardNumber < t.localConfig.ChainNumber {
		log.Error("check err", "chainNumber", t.localConfig.ChainNumber, "shardNumber", t.localConfig.ShardNumber)
		panic("shardNumber should > chainNumber")
	}
	if t.localConfig.ShardNumber%t.localConfig.ChainNumber != 0 {
		log.Error("check err", "chainNumber", t.localConfig.ChainNumber, "shardNumber", t.localConfig.ShardNumber)
		panic("shardNumber%chainNumber should=0")
	}

	if t.localConfig.ExtraClusterConfig.GasLimit == 0 {
		t.localConfig.ExtraClusterConfig.GasLimit = 120000
	}
	if t.localConfig.ExtraClusterConfig.TargetMinorBlockTime == 0 {
		t.localConfig.ExtraClusterConfig.TargetMinorBlockTime = 10
	}
	if t.localConfig.ExtraClusterConfig.TargetRootBlockTime == 0 {
		t.localConfig.ExtraClusterConfig.TargetRootBlockTime = 60
	}
}

func (t *ToolManager) init() {
	for _, ip := range t.localConfig.Hosts {
		t.SSHSession[ip.IP] = NewSSHConnect(ip.User, ip.Password, ip.IP, int(ip.Port))
	}
	log.Info("init", "IP list", t.localConfig.Hosts)
}

func (t *ToolManager) GetIpListDependTag(tag string) []string {
	ipList := make([]string, 0)
	for _, v := range t.localConfig.Hosts {
		if strings.Contains(v.Service, tag) {
			ipList = append(ipList, v.IP)
		}
	}
	return ipList
}

func (t *ToolManager) GenClusterConfig() {
	clusterConfig := GenConfigDependInitConfig(t.localConfig.ChainNumber, t.localConfig.ShardNumber/t.localConfig.ChainNumber, t.GetIpListDependTag("slave"), t.localConfig.ExtraClusterConfig)

	if t.localConfig.BootNode == "" {
		sk, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
		CheckErr(err)
		clusterConfig.P2P.PrivKey = hex.EncodeToString(sk.D.Bytes())
		db, err := enode.OpenDB("")
		CheckErr(err)
		node := enode.NewLocalNode(db, sk)
		log.Info("bootnode info", "info", node.Node().String())
	} else {
		clusterConfig.P2P.BootNodes = t.localConfig.BootNode
	}
	WriteConfigToFile(clusterConfig, "./cluster_config_template.json")
}

func (t *ToolManager) SendFileToCluster() {
	var g errgroup.Group
	for _, session := range t.SSHSession {
		v := session
		g.Go(func() error {
			v.RunCmd("rm -rf  /tmp/QKC")
			v.RunCmd("mkdir /tmp/QKC")

			v.RunCmd("docker stop $(docker ps -a|grep bjqkc |awk '{print $1}')")
			v.RunCmd("docker  rm $(docker ps -a|grep bjqkc |awk '{print $1}')")
			v.RunCmd("docker pull " + t.localConfig.DockerName)
			v.RunCmd("docker run -itd --name bjqkc --network=host " + t.localConfig.DockerName)

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

func (t *ToolManager) StartCluster() {
	masterIp := t.GetIpListDependTag("master")[0]
	slaveIpLists := make([]*SlaveInfo, 0)
	cfg := config.NewClusterConfig()
	err := LoadClusterConfig("./cluster_config_template.json", cfg)
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
	session := t.SSHSession[ip]
	session.RunCmd("docker exec -itd bjqkc /bin/bash -c 'chmod +x /tmp/QKC/cluster && /tmp/QKC/cluster --cluster_config /tmp/QKC/cluster_config_template.json --json_rpc_host 0.0.0.0 --json_rpc_private_host 0.0.0.0 >>master.log 2>&1 '")
}

func (t *ToolManager) startSlave(ipList []*SlaveInfo) {
	for _, v := range ipList {
		session := t.SSHSession[v.IP]
		cmd := "docker exec -itd bjqkc /bin/bash -c 'chmod +x /tmp/QKC/cluster && /tmp/QKC/cluster --cluster_config /tmp/QKC/cluster_config_template.json --service " + v.ServiceName + ">> " + v.ServiceName + ".log 2>&1  '"
		session.RunCmd(cmd)
	}
}
