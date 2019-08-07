package test

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/master"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"net"
	"time"
)

type Clusterlist []*clusterNode

func (cl Clusterlist) Start(duration time.Duration, prCtrol bool) {
	length := len(cl)
	if !prCtrol {
		length -= 1
	}
	var (
		started = make([]*clusterNode, length, length)
	)
	for i := 0; i < length; i++ {
		idx := i
		err := cl[idx].Start()
		if err == nil {
			started[idx] = cl[idx]
		} else {
			for idx, nd := range started {
				if nd != nil {
					nd.Stop()
					cl[idx] = nil
				}
			}
			utils.Fatalf("failed to start clusters, err: %v", err)
		}
	}

	// wait p2p connection for at last $duration seconds.
	if prCtrol {
		mntorClstr := cl[len(cl)-1]
		if mntorClstr != nil {
			p2pSvr := mntorClstr.getP2PServer()
			now := time.Now()
			for p2pSvr.PeerCount() < mntorClstr.index && time.Now().Sub(now) < duration {
				time.Sleep(500 * time.Millisecond)
			}
			fmt.Printf("start %d clusters successful\n\n", p2pSvr.PeerCount())
		}
	}
}

func (cl Clusterlist) Stop() {
	for idx, clstr := range cl {
		if clstr != nil {
			clstr.Stop()
			cl[idx] = nil
		}
	}
}

func (cl Clusterlist) PrintPeerList() {
	for _, c := range cl {
		p2pSvr := c.getP2PServer()
		if p2pSvr == nil {
			continue
		}
		for _, pr := range p2pSvr.Peers() {
			fmt.Printf("cluster id: %d\tpeer node: %s\n", c.index, pr.Node().String())
		}
	}
}

func (cl Clusterlist) GetPeerByIndex(idx int) (peer *master.Peer) {
	mstr := cl[len(cl)-1].GetMaster()
	peers := mstr.GetPeerList()
	for _, pr := range peers {
		tcp := pr.RemoteAddr().(*net.TCPAddr)
		if defaultP2PPort+idx == tcp.Port {
			return pr
		}
	}
	return
}
