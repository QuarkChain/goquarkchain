package sync

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

const (
	// Number of root block headers to download from peers.
	headerDownloadSize = 500
	// Number root blocks to download from peers.
	blockDownloadSize = 100
)

var (
	// TODO: should use config.
	maxSyncStaleness = 22500
)

// All of the sync tasks to are to catch up with the root chain from peers.
type rootChainTask struct {
	task
}

// NewRootChainTask returns a sync task for root chain.
func NewRootChainTask(p peer, header *types.RootBlockHeader) Task {
	return &rootChainTask{
		task{
			header: header,
			peer:   p,
			name: "root",
			getHeaders: func(hash common.Hash, limit uint32) (ret []types.IHeader, err error) {
				rheaders, err := p.GetRootBlockHeaderList(hash, limit, true)
				if err != nil {
					return nil, err
				}
				for _, rh := range rheaders {
					ret = append(ret, rh)
				}
				return ret, nil
			},
			getBlocks: func(hashes []common.Hash) (ret []types.IBlock, err error) {
				rblocks, err := p.GetRootBlockList(hashes)
				if err != nil {
					return nil, err
				}
				for _, rb := range rblocks {
					ret = append(ret, rb)
				}
				return ret, nil
			},
			syncBlock: func(types.IBlock) error {
				// TODO(ping): to be implemented
				return nil
			},
		},
	}
}

func (r *rootChainTask) Priority() uint {
	// TODO: should use total diff
	return uint(r.task.header.NumberU64())
}
