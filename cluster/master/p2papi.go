package master

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

type PrivateP2PAPI struct {
	peers *peerSet
}

// NewPrivateP2PAPI creates a new peer shard p2p protocol API.
func NewPrivateP2PAPI(peers *peerSet) *PrivateP2PAPI {
	return &PrivateP2PAPI{peers}
}

func (api *PrivateP2PAPI) BroadcastBlock(branch uint32, block *types.MinorBlock) error {
	for _, peer := range api.peers.peers {
		peer.AsyncSendNewMinorBlock(branch, block)
	}

	return nil
}

func (api *PrivateP2PAPI) BroadcastTransactions(branch uint32, txs []*types.Transaction) {
	for _, peer := range api.peers.peers {
		peer.AsyncSendTransactions(branch, txs)
	}
}

func (api *PrivateP2PAPI) GetMinorBlocks(hashList []common.Hash, branch uint32, peerId string) []*types.MinorBlock {
	peer := api.peers.Peer(peerId)
	blocks, err := peer.GetMinorBlockList(hashList, branch)
	if err != nil {
		api.peers.Unregister(peerId)
	}
	return blocks
}
