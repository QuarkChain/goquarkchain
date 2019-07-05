package sync

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/QuarkChain/goquarkchain/core/types"
)

type minorSyncerPeer interface {
	GetMinorBlockHeaderList(hash common.Hash, limit, branch uint32, reverse bool) ([]*types.MinorBlockHeader, error)
	GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error)
	PeerID() string
}

type minorChainTask struct {
	task
	peer minorSyncerPeer
}

// NewMinorChainTask returns a sync task for minor chain.
func NewMinorChainTask(
	p minorSyncerPeer,
	header *types.MinorBlockHeader,
) Task {
	return &minorChainTask{
		task: task{
			header:           header,
			name:             fmt.Sprintf("shard-%d", header.Branch.GetShardID()),
			maxSyncStaleness: 22500 * 6, // TODO: derive from root chain?
			getHeaders: func(hash common.Hash, limit uint32) (ret []types.IHeader, err error) {
				mheaders, err := p.GetMinorBlockHeaderList(hash, limit, header.Branch.Value, true)
				if err != nil {
					return nil, err
				}
				for _, mh := range mheaders {
					ret = append(ret, mh)
				}
				return ret, nil
			},
			getBlocks: func(hashes []common.Hash) (ret []types.IBlock, err error) {
				mblocks, err := p.GetMinorBlockList(hashes, header.Branch.Value)
				if err != nil {
					return nil, err
				}
				for _, mb := range mblocks {
					ret = append(ret, mb)
				}
				return ret, nil
			},
		},
		peer: p,
	}
}

func (m *minorChainTask) Priority() *big.Int {
	return new(big.Int).SetUint64(m.task.header.NumberU64())
}

func (m *minorChainTask) PeerID() string {
	return m.peer.PeerID()
}
