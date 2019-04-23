package master

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
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
func (api *PrivateP2PAPI) BroadcastMinorBlock(branch uint32, block *types.MinorBlock) error {
	if block == nil {
		return errors.New("input block is nil")
	}
	if block.Branch().Value != branch {
		return errors.New("branch mismatch")
	}
	for _, peer := range api.peers.peers {
		peer.AsyncSendNewMinorBlock(branch, block)
	}
	return nil
}

// BroadcastTransactions only be called when run performance test which the txs
// are created by shard itself, so broadcast to all the peer
func (api *PrivateP2PAPI) BroadcastTransactions(branch uint32, txs []*types.Transaction) {
	for _, peer := range api.peers.peers {
		peer.AsyncSendTransactions(branch, txs)
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
	for _, peer := range api.peers.peers {
		peer.AsyncSendNewTip(branch, &p2p.Tip{RootBlockHeader: rootBlockHeader, MinorBlockHeaderList: minorBlockHeaderList})
	}
	return nil
}

func (api *PrivateP2PAPI) GetMinorBlocks(hashList []common.Hash, branch uint32, peerId string) ([]*types.MinorBlock, error) {
	peer := api.peers.Peer(peerId)
	if peer != nil {
		return nil, errNotRegistered
	}
	blocks, err := peer.GetMinorBlockList(hashList, branch)
	//if err != nil {
	//api.peers.Unregister(peerId)
	//}
	return blocks, err
}

func (api *PrivateP2PAPI) GetMinorBlockHeaders(hash common.Hash, amount uint32, branch uint32, reverse bool, peerId string) ([]*types.MinorBlockHeader, error) {
	peer := api.peers.Peer(peerId)
	if peer != nil {
		return nil, errNotRegistered
	}
	headers, err := peer.GetMinorBlockHeaderList(hash, amount, branch, reverse)
	//if err != nil {
	//	api.peers.Unregister(peerId)
	//}
	return headers, err
}
