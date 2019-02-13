package p2p

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type QuarkPeerPool struct {
	privateKey *ecdsa.PrivateKey
	context    string
	listenPort uint
}

type QuarkServer struct {
	BaseServer
}

func (self *QuarkServer) makePeerPool() *QuarkPeerPool {
	return &QuarkPeerPool{
		privateKey: self.PrivateKey,
		context:    "quarkchain",
		listenPort: self.port,
	}
}

type P2PManager struct {
	config         config.ClusterConfig
	preferredNodes []*enode.Node
	QuarkServer

	selfId []byte
	ip     string
	port   uint64
}

func NewP2PManager(env config.ClusterConfig) (*P2PManager, error) {
	config := Config{
		Name:       "quarkchain",
		MaxPeers:   10,
		ListenAddr: ":38291",
		Protocols:  []Protocol{MyProtocol()},
	}
	p2pManager := &P2PManager{}
	var err error

	p2pManager.Config = config
	p2pManager.QuarkServer.newTransport = NewqkcRLPX
	p2pManager.BootstrapNodes, err = getNodesFromConfig(env.P2P.BootNodes)
	if err != nil {
		return nil, err
	}

	p2pManager.PrivateKey, err = getPrivateKeyFromConfig(env.P2P.PrivKey)
	if err != nil {
		return nil, err
	}

	p2pManager.preferredNodes, err = getNodesFromConfig(env.P2P.PreferredNodes)
	if err != nil {
		return nil, err
	}

	//used in HelloCommand.peer_id
	p2pManager.selfId = crypto.FromECDSAPub(&p2pManager.PrivateKey.PublicKey)[1:33]
	//TODO
	//osHostname,err:=os.Hostname()
	//p2pManager.ip,err=net.LookupHost(osHostname)
	p2pManager.port = env.P2Port
	return p2pManager, nil
}

func (self *P2PManager) Start() {
	fmt.Println("*********************** self p2pMananger start")
	err := self.Run()

	fmt.Println("*********************** self p2pManager  end", err)
}
