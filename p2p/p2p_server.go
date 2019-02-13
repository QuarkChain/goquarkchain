package p2p

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"time"
)

type BaseServer struct {
	Server
	port             uint
	networkID        int
	maxPeers         uint
	bootstrapNodes   []*enode.Node
	preferredNodes   []*enode.Node
	useDiscv5        bool
	upnp             bool
	allowDialInRatio float32

	logName string
}

func NewBaseServer(private *ecdsa.PrivateKey, port uint, networkId int, maxPeers uint, bootstrapNodes []*enode.Node, preferredNodes []*enode.Node, useDiscv5 bool, upnp bool, allowDialInRatio float32) BaseServer {
	return BaseServer{
		port:             port,
		networkID:        networkId,
		maxPeers:         maxPeers,
		bootstrapNodes:   bootstrapNodes,
		preferredNodes:   preferredNodes,
		useDiscv5:        useDiscv5,
		upnp:             upnp,
		allowDialInRatio: allowDialInRatio,
		logName:          "BaseServer",
	}
}

func (self BaseServer) Run() error {
	log.Info(self.logName, "Running Server", "start")

	if err := self.Start(); err != nil {
		return err
	}
	go func() {
		for true {
			Peers := self.Peers()
			fmt.Println("开始展示peer信息", "peer个数", len(Peers))
			for _, v := range Peers {
				fmt.Println("peerInfo", v.String())
			}
			fmt.Println("结束展示peer信息")
			time.Sleep(5 * time.Second)
		}
	}()
	log.Info(self.logName, "this server:enode", self.localnode.Node())

	return nil
}
