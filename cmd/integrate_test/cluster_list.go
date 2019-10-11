package test

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/master"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"golang.org/x/sync/errgroup"
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
		e       errgroup.Group
	)
	for i := 0; i < length; i++ {
		idx := i
		e.Go(func() error {
			err := cl[idx].Start()
			if err == nil {
				started[idx] = cl[idx]
			}
			return err
		})
	}
	if err := e.Wait(); err != nil {
		for idx, nd := range started {
			if nd != nil {
				nd.Stop()
				cl[idx] = nil
			}
		}
		utils.Fatalf("failed to start clusters, err: %v", err)
	}

	// wait p2p connection for at last $duration seconds.
	if prCtrol {
		mntorClstr := cl[len(cl)-1]
		p2pSvr := mntorClstr.getP2PServer()
		now := time.Now()
		for p2pSvr.PeerCount() < mntorClstr.index && time.Now().Sub(now) < duration {
			time.Sleep(200 * time.Millisecond)
			p2pSvr = mntorClstr.getP2PServer()
		}
		if p2pSvr.PeerCount() < mntorClstr.index {
			utils.Fatalf("monitor's peers connection start failed, expect sum: %d", p2pSvr.PeerCount())
		} else {
			fmt.Printf("start %d clusters successful\n\n", p2pSvr.PeerCount())
		}
	}
}

func (cl Clusterlist) Stop() {
	time.Sleep(200 * time.Millisecond)
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
	for _, pr0 := range mstr.GetPeerList() {
		for _, pr1 := range cl[idx].GetMaster().GetPeerList() {
			if pr0.ID() == pr1.ID() {
				return pr0
			}
		}
	}

	return
}
