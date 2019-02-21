package p2p

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"math/big"
	"strings"
	"sync"
	"time"
)

var (
	PManagerLog="P2PManager"
)


type PManager struct {
	config         config.ClusterConfig
	preferredNodes []*enode.Node
	Server *Server

	selfId []byte
	log string

	lock sync.RWMutex
	stop chan struct{}
}

func NewP2PManager(env config.ClusterConfig) (*PManager, error) {
	config := Config{
		Name:       "quarkchain",
		MaxPeers:   10,
		ListenAddr: ":38291",
		Protocols:  []Protocol{MyProtocol()},
	}

	server:=&Server{
		Config:config,
		newTransport:NewqkcRLPX,

	}
	p2pManager := &PManager{
		Server:server,
		log:PManagerLog,
	}
	var err error

	p2pManager.Server.BootstrapNodes, err = getNodesFromConfig(env.P2P.BootNodes)
	if err != nil {
		return nil, err
	}

	p2pManager.Server.PrivateKey, err = getPrivateKeyFromConfig(env.P2P.PrivKey)
	if err != nil {
		return nil, err
	}

	p2pManager.preferredNodes, err = getNodesFromConfig(env.P2P.PreferredNodes)
	if err != nil {
		return nil, err
	}

	//used in HelloCommand.peer_id
	p2pManager.selfId = crypto.FromECDSAPub(&p2pManager.Server.PrivateKey.PublicKey)[1:33]
	p2pManager.stop=make(chan struct{})
	return p2pManager, nil
}

func (self *PManager) Start() {
//	log.Info(self.log, "this server:enode", self.Server.NodeInfo().Enode)
	err := self.Server.Start()
	if err!=nil{
		//TODO  panic
		panic(err)
	}
	go func() {
		for true {
			Peers := self.Server.Peers()
			fmt.Println("开始展示peer信息", "peer个数", len(Peers))
			for _, v := range Peers {
				fmt.Println("peerInfo", v.String())
			}
			fmt.Println("结束展示peer信息")
			time.Sleep(5 * time.Second)
		}
	}()
}


func getNodesFromConfig(configNodes string) ([]*enode.Node, error) {
	if configNodes == "" {
		return make([]*enode.Node, 0), nil
	}

	NodeList := strings.Split(configNodes, ",")
	enodeList := make([]*enode.Node, 0, len(NodeList))
	for _, url := range NodeList {
		node, err := enode.ParseV4(url)
		if err != nil {
			return nil, err
		} else {
			log.Error("Node add", "url", url)
		}
		enodeList = append(enodeList, node)
	}
	return enodeList, nil
}

func getPrivateKeyFromConfig(configKey string) (*ecdsa.PrivateKey, error) {
	if configKey == "" {
		sk, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
		return sk, err
	}
	configKeyValue, err := hex.DecodeString(configKey)
	if err != nil {
		return nil, err
	}
	keyValue := new(big.Int).SetBytes(configKeyValue)
	if err != nil {
		return nil, err
	}
	sk := new(ecdsa.PrivateKey)
	sk.PublicKey.Curve = crypto.S256()
	sk.D = keyValue
	sk.PublicKey.X, sk.PublicKey.Y = crypto.S256().ScalarBaseMult(keyValue.Bytes())
	return sk, nil
}

func (self *PManager)Stop(){
	close(self.stop)
}

func(self *PManager)Wait(){
	self.lock.RLock()
	if self.Server==nil{
		self.lock.RUnlock()
		return
	}
	reStop:=self.stop
	self.lock.RUnlock()
	<-reStop

}