package sync

import (
	"fmt"

	"golang.org/x/sync/errgroup"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

const (
	// Number of root block headers to download from peers.
	headerDownloadSize = 500
	// Number root blocks to download from peers.
	blockDownloadSize = 100
)

type rootSyncerPeer interface {
	GetRootBlockHeaderList(hash common.Hash, amount uint32, reverse bool) ([]*types.RootBlockHeader, error)
	GetRootBlockList(hashes []common.Hash) ([]*types.RootBlock, error)
	PeerId() string
}

// All of the sync tasks to are to catch up with the root chain from peers.
type rootChainTask struct {
	task
	peer rootSyncerPeer
}

// NewRootChainTask returns a sync task for root chain.
func NewRootChainTask(
	p rootSyncerPeer,
	header *types.RootBlockHeader,
	statusChan chan *rpc.ShardStatus,
	getShardConnFunc func(fullShardId uint32) []rpc.ShardConnForP2P,
) Task {
	return &rootChainTask{
		task: task{
			header:           header,
			name:             "root",
			maxSyncStaleness: 22500,
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
			syncBlock: func(block types.IBlock, bc blockchain) error {
				rb := block.(*types.RootBlock)
				rbc := bc.(rootblockchain)
				return syncMinorBlocks(p.PeerId(), rbc, rb, statusChan, getShardConnFunc)
			},
		},
		peer: p,
	}
}

func (r *rootChainTask) Priority() uint {
	// TODO: should use total diff
	return uint(r.task.header.NumberU64())
}

func (r *rootChainTask) PeerID() string {
	return r.peer.PeerId()
}

func syncMinorBlocks(
	peerID string,
	rbc rootblockchain,
	rootBlock *types.RootBlock,
	statusChan chan *rpc.ShardStatus,
	getShardConnFunc func(fullShardId uint32) []rpc.ShardConnForP2P,
) error {
	downloadMap := make(map[uint32][]common.Hash)
	for _, header := range rootBlock.MinorBlockHeaders() {
		hash := header.Hash()
		if !rbc.IsMinorBlockValidated(hash) {
			downloadMap[header.Branch.Value] = append(downloadMap[header.Branch.Value], hash)
		}
	}

	var g errgroup.Group
	for branch, hashes := range downloadMap {
		b, hashList := branch, hashes
		conns := getShardConnFunc(b)
		if len(conns) == 0 {
			return fmt.Errorf("shard connection for branch %d is missing", b)
		}
		// TODO Support to multiple connections
		g.Go(func() error {
			status, err := conns[0].AddBlockListForSync(&rpc.AddBlockListForSyncRequest{Branch: b, PeerId: peerID, MinorBlockHashList: hashList})
			if err == nil {
				statusChan <- status
			}
			return err
		})
	}
	err := g.Wait()
	if err != nil {
		return err
	}

	for _, hashes := range downloadMap {
		for _, hash := range hashes {
			rbc.AddValidatedMinorBlockHeader(hash)
		}
	}
	return nil
}
