package deploy

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"strings"

	"time"
)

var (
	dockerName        = "sunchunfeng/goqkc:deploy-v2"
	remoteDir         = "/tmp/QKC"
	clusterPath       = "./cluster"
	clusterConfigPath = "./cluster_config_template.json"
)

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
	if len(t.localConfig.IPList) < 1 {
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
	for _, ip := range t.localConfig.IPList {
		t.SSHSession[ip.IP] = NewSSHConnect(ip.User, ip.Password, ip.IP, int(ip.Port))
	}
	log.Info("init", "IP list", t.localConfig.IPList)
}

func (t *ToolManager) GetIpListDependTag(tag string) []string {
	ipList := make([]string, 0)
	for _, v := range t.localConfig.IPList {
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
		Checkerr(err)
		clusterConfig.P2P.PrivKey = hex.EncodeToString(sk.D.Bytes())
		db, err := enode.OpenDB("")
		Checkerr(err)
		node := enode.NewLocalNode(db, sk)
		log.Info("bootnode info", "info", node.Node().String())
	} else {
		clusterConfig.P2P.BootNodes = t.localConfig.BootNode
	}
	WriteConfigToFile(clusterConfig, "./cluster_config_template.json")
}

func (t *ToolManager) MakeClusterExe() {
	for _, v := range t.SSHSession {
		v.RunCmd("rm -rf /tmp/QKC/*")

		v.RunCmd("docker stop $(docker ps -a|grep bjqkc |awk '{print $1}')")
		v.RunCmd("docker  rm $(docker ps -a|grep bjqkc |awk '{print $1}')")
		v.RunCmd("docker pull " + dockerName)
		v.RunCmd("docker run -itd --name bjqkc --network=host  " + dockerName)

		v.RunCmd("mkdir /tmp/QKC")

		v.RunCmd("docker exec -itd bjqkc /bin/bash -c  'cd /root/go/src/github.com/Quarkchain/goquarkchain/cmd/cluster/  && go build -o /tmp/QKC/cluster && chmod +x /tmp/QKC/cluster '")
		time.Sleep(30 * time.Second)
		v.RunCmd("docker cp bjqkc:/tmp/QKC/cluster /tmp/QKC") //checkout
		time.Sleep(5 * time.Second)
		v.GetFile("./", "/tmp/QKC/cluster")
		v.RunCmd("rm -rf /tmp/QKC/*")
		break
	}
}

func (t *ToolManager) SendFileToCluster() {
	for _, v := range t.SSHSession {
		v.RunCmd("rm -rf " + remoteDir)
		v.RunCmd("mkdir " + remoteDir)

		v.RunCmd("docker stop $(docker ps -a -q)")
		v.RunCmd("docker  rm $(docker ps -a -q)")
		v.RunCmd("docker pull " + dockerName)
		v.RunCmd("docker run -itd --name bjqkc --network=host " + dockerName)

		v.SendFile(clusterPath, remoteDir)
		v.SendFile(clusterConfigPath, remoteDir)

		v.RunCmd("docker cp " + remoteDir + "/cluster bjqkc:" + remoteDir)
		v.RunCmd("docker cp " + remoteDir + "/cluster_config_template.json bjqkc:" + remoteDir)

		v.RunCmd("docker exec -itd bjqkc  /bin/bash -c  'cd /root/go/src/github.com/Quarkchain/goquarkchain/consensus/qkchash/native/ && make clean && make '") //checkout
	}
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
	Checkerr(err)

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
