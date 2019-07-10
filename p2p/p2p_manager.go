package p2p

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"math/big"
	"strings"
	"sync"
	"time"
)

var (
	pManagerLog     = "P2PManager"
	QKCProtocolName = "quarkchain"
)

// P2PManager p2p manager
type P2PManager struct {
	preferredNodes []*enode.Node
	Server         *Server
	selfID         []byte
	log            string
	lock           sync.RWMutex
	stop           chan struct{}
}

//NewP2PManager new p2p manager
func NewP2PManager(env config.ClusterConfig, protocol Protocol) (*P2PManager, error) {
	var err error
	server := &Server{
		Config: Config{
			Name:       QKCProtocolName,
			MaxPeers:   int(env.P2P.MaxPeers),
			ListenAddr: fmt.Sprintf(":%v", env.P2PPort),
			Protocols:  []Protocol{protocol},
		},
		newTransport: NewQKCRlp,
	}

	p2pManager := &P2PManager{
		Server: server,
		log:    pManagerLog,
	}

	p2pManager.Server.BootstrapNodes, err = getNodesFromConfig(env.P2P.BootNodes)
	if err != nil {
		return nil, err
	}

	p2pManager.Server.PrivateKey, err = GetPrivateKeyFromConfig(env.P2P.PrivKey)
	if err != nil {
		return nil, err
	}

	p2pManager.preferredNodes, err = getNodesFromConfig(env.P2P.PreferredNodes)
	if err != nil {
		return nil, err
	}

	//used in HelloCommand.peer_id
	p2pManager.selfID = crypto.FromECDSAPub(&p2pManager.Server.PrivateKey.PublicKey)[1:33]
	p2pManager.stop = make(chan struct{})

	return p2pManager, nil
}

//Start start p2p manager
func (p *P2PManager) Start() error {
	log.Info(p.log, "this server:Node", p.Server.NodeInfo().Enode)
	err := p.Server.Start()
	if err != nil {
		log.Info(p.log, "pManager start err", err)
		return err
	}
	go func() {
		for true {
			Peers := p.Server.Peers()
			log.Info(msgHandleLog, "==============", "===========")
			log.Info(msgHandleLog, "peer number", len(Peers))
			for _, v := range Peers {
				log.Info(msgHandleLog, "peerInfo", v.String())
			}
			log.Info(msgHandleLog, "==============", "===========")
			time.Sleep(10 * time.Second)
		}
	}()
	return nil
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
		}
		enodeList = append(enodeList, node)
	}
	return enodeList, nil
}

func GetPrivateKeyFromConfig(configKey string) (*ecdsa.PrivateKey, error) {
	if configKey == "" {
		return crypto.GenerateKey()
	}
	configKeyValue, err := hex.DecodeString(configKey)
	if err != nil {
		return nil, err
	}
	keyValue := new(big.Int).SetBytes(configKeyValue)
	if err != nil {
		return nil, err
	}
	return crypto.ToECDSA(keyValue.Bytes())
}

// Stop stop p2p manager
func (p *P2PManager) Stop() {
	close(p.stop)
}

// Wait wait for p2p manager
func (p *P2PManager) Wait() {
	p.lock.RLock()
	if p.Server == nil {
		p.lock.RUnlock()
		return
	}
	reStop := p.stop
	p.lock.RUnlock()
	<-reStop

}
