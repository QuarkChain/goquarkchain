package master

import (
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/pkg/errors"
)

type PrivateP2PAPI struct {
	peers *peerSet
}

// NewPrivateP2PAPI creates a new peer shard p2p protocol API.
func NewPrivateP2PAPI(peers *peerSet) *PrivateP2PAPI {
	return &PrivateP2PAPI{peers}
}

//BroadcastMinorBlock will be called when a minor block first time added to a chain
func (api *PrivateP2PAPI) BroadcastMinorBlock(res *rpc.P2PRedirectRequest) error {
	for _, peer := range api.peers.Peers() {
		peer.AsyncSendNewMinorBlock(res)
	}
	return nil
}

// BroadcastTransactions only be called when run performance test which the txs
// are created by shard itself, so broadcast to all the peer
func (api *PrivateP2PAPI) BroadcastTransactions(txsBatch *rpc.P2PRedirectRequest, peerID string) {
	for _, peer := range api.peers.Peers() {
		if peer.id != peerID {
			peer.AsyncSendTransactions(txsBatch)
		}
	}
}

func (api *PrivateP2PAPI) BroadcastNewTip(branch uint32, rootBlockHeader *types.RootBlockHeader, minorBlockHeaderList []*types.MinorBlockHeader) error {
	if rootBlockHeader == nil {
		return errors.New("input block is nil")
	}
	if len(minorBlockHeaderList) != 1 {
		return errors.New("minor block count in minorBlockHeaderList should be 1")
	}
	if minorBlockHeaderList[0].Branch.Value != branch {
		return errors.New("branch mismatch")
	}
	for _, peer := range api.peers.Peers() {
		if minorTip := peer.MinorHead(branch); minorTip != nil && minorTip.RootBlockHeader != nil {
			if minorTip.RootBlockHeader.Number > rootBlockHeader.Number {
				continue
			}
			if minorTip.RootBlockHeader.Number == rootBlockHeader.Number &&
				minorTip.MinorBlockHeaderList[0].Number > minorBlockHeaderList[0].Number {
				continue
			}
			if minorTip.MinorBlockHeaderList[0].Hash() == minorBlockHeaderList[0].Hash() {
				continue
			}
		}
		peer.AsyncSendNewTip(branch, &p2p.Tip{RootBlockHeader: rootBlockHeader, MinorBlockHeaderList: minorBlockHeaderList})
	}
	return nil
}

func (api *PrivateP2PAPI) GetMinorBlockList(req *rpc.P2PRedirectRequest) ([]byte, error) {
	peer := api.peers.Peer(req.PeerID)
	if peer == nil {
		return nil, errNotRegistered
	}
	data, err := peer.GetMinorBlockList(req)
	return data, err
}

func (api *PrivateP2PAPI) GetMinorBlockHeaderListWithSkip(req *rpc.P2PRedirectRequest) ([]byte, error) {
	peer := api.peers.Peer(req.PeerID)
	if peer == nil {
		return nil, errNotRegistered
	}
	return peer.GetMinorBlockHeaderListWithSkip(req)
}

func (api *PrivateP2PAPI) GetMinorBlockHeaderList(req *rpc.P2PRedirectRequest) ([]byte, error) {
	peer := api.peers.Peer(req.PeerID)
	if peer == nil {
		return nil, errNotRegistered
	}
	return peer.GetMinorBlockHeaderList(req)
}

func (api *PrivateP2PAPI) MinorHead(req *rpc.MinorHeadRequest) (*types.MinorBlockHeader, error) {
	peer := api.peers.Peer(req.PeerID)
	if peer == nil {
		return nil, errNotRegistered
	}
	if mTip := peer.MinorHead(req.Branch); mTip != nil && mTip.MinorBlockHeaderList != nil {
		return mTip.MinorBlockHeaderList[0], nil
	}
	return nil, errors.New("empty minor block header")
}
