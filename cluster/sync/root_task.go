package sync

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/sync/errgroup"
)

type rootSyncerPeer interface {
	GetRootBlockHeaderList(*p2p.GetRootBlockHeaderListWithSkipRequest) (*p2p.GetRootBlockHeaderListResponse, error)
	GetRootBlockList(hashes []common.Hash) ([]*types.RootBlock, error)
	RootHead() *types.RootBlockHeader
	PeerID() string
}

// All of the sync tasks to are to catch up with the root chain from peers.
type rootChainTask struct {
	task
	header     *types.RootBlockHeader
	peer       rootSyncerPeer
	stats      *BlockSychronizerStats
	statusChan chan *rpc.ShardStatus
	slaveConns rpc.ConnManager
}

// NewRootChainTask returns a sync task for root chain.
func NewRootChainTask(
	p rootSyncerPeer,
	header *types.RootBlockHeader,
	stats *BlockSychronizerStats,
	statusChan chan *rpc.ShardStatus,
	slaveConns rpc.ConnManager,
) Task {
	rTask := &rootChainTask{
		peer:       p,
		header:     header,
		stats:      stats,
		statusChan: statusChan,
		slaveConns: slaveConns,
	}
	rTask.task = task{
		name:             "root",
		header:           header,
		maxSyncStaleness: 22500,
		// if the shard size is large (like 1024), the block size would be large
		// and multi root block will exceed the p2p up limit
		// change the batch size to 3 for tps test
		batchSize: 3, // RootBlockBatchSize,
		findAncestor: func(bc blockchain) (types.IHeader, error) {

			if bc.HasBlock(rTask.header.Hash()) {
				return nil, nil
			}

			ancestor, err := rTask.findAncestor(bc)
			if err != nil {
				rTask.stats.AncestorNotFoundCount += 1
				return nil, err
			}

			if !bc.HasBlock(ancestor.Hash()) {
				return nil, errors.New("Bad ancestor ")
			}

			if rTask.header.ToTalDifficulty.Cmp(ancestor.ToTalDifficulty) < 0 {
				return nil, errors.New("ancestor's total difficulty is bigger than current")
			}
			return ancestor, nil
		},
		getHeaders: func(startHeader types.IHeader) ([]types.IHeader, error) {
			if startHeader.NumberU64() >= uint64(rTask.header.Number) {
				return nil, nil
			}

			limit := uint32(RootBlockHeaderListLimit)
			heightDiff := rTask.header.Number - uint32(startHeader.NumberU64())
			if limit > heightDiff {
				limit = heightDiff
			}

			rBHeaders, err := rTask.downloadBlockHeaderListAndCheck(uint32(startHeader.NumberU64()+1), 0, limit)
			if err != nil {
				return nil, err
			}

			if len(rBHeaders) == 0 {
				return nil, errors.New("Remote chain reorg causing empty root block headers ")
			}

			if rBHeaders[0].ParentHash != startHeader.Hash() {
				return nil, errors.New("Bad peer sending incorrect canonical root headers ")
			}

			iHeaders := make([]types.IHeader, 0, len(rBHeaders))
			for _, hd := range rBHeaders {
				hd := hd
				iHeaders = append(iHeaders, hd)
			}

			return iHeaders, nil
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
		syncBlock: func(bc blockchain, block types.IBlock) error {
			rb := block.(*types.RootBlock)
			rbc := bc.(rootblockchain)
			return rTask.syncMinorBlocks(rbc, rb)
		},
		needSkip: func(b blockchain) bool {
			if rTask.header.GetTotalDifficulty().Cmp(b.CurrentHeader().GetTotalDifficulty()) <= 0 {
				return true
			}
			return false
		},
	}
	return rTask
}

func (r *rootChainTask) Priority() *big.Int {
	return r.header.GetTotalDifficulty()
}

func (r *rootChainTask) PeerID() string {
	return r.peer.PeerID()
}

func (r *rootChainTask) downloadBlockHeaderListAndCheck(start uint32, skip,
	limit uint32) ([]*types.RootBlockHeader, error) {
	req := &p2p.GetRootBlockHeaderListWithSkipRequest{
		Skip:      skip,
		Limit:     limit,
		Direction: qcom.DirectionToTip,
	}
	req.SetHeight(start)
	resp, err := r.peer.GetRootBlockHeaderList(req)
	if err != nil {
		return nil, err
	}
	r.stats.HeadersDownloaded += uint64(len(resp.BlockHeaderList))

	// check total difficulty
	if resp.RootTip.ToTalDifficulty.Cmp(r.header.GetDifficulty()) <= 0 {
		return nil, errors.New("Bad peer sending root block tip with lower TD ")
	}

	if len(resp.BlockHeaderList) == 0 {
		return nil, errors.New("Remote chain reorg causing empty root block headers ")
	}

	newLimit := (resp.RootTip.Number + 1 - start + skip) / (skip + 1)
	if newLimit > limit {
		newLimit = limit
	}
	if len(resp.BlockHeaderList) != int(newLimit) {
		return nil, fmt.Errorf("Bad peer sending incorrect number of root block headers expect: %d, actual: %d\n", newLimit, len(resp.BlockHeaderList))
	}

	if resp.RootTip.Hash() != r.header.Hash() {
		r.header = resp.RootTip
	}
	return resp.BlockHeaderList, nil
}

func (r *rootChainTask) findAncestor(bc blockchain) (*types.RootBlockHeader, error) {
	rtip := bc.CurrentHeader().(*types.RootBlockHeader)
	if r.header.ParentHash == rtip.Hash() {
		return rtip, nil
	}

	end := rtip.Number
	maxSyncStaleness := uint32(r.task.maxSyncStaleness)
	start := end - maxSyncStaleness
	if end < maxSyncStaleness {
		start = 0
	}
	if r.header.Number < end {
		end = r.header.Number
	}

	var bestAncestor *types.RootBlockHeader
	for end >= start {
		r.stats.AncestorLookupRequests += 1
		span := (end-start)/RootBlockHeaderListLimit + 1
		blocklist, err := r.downloadBlockHeaderListAndCheck(start, span-1, (end+1-start)/span)
		if err != nil {
			return nil, err
		}

		var preHeader *types.RootBlockHeader
		for i := len(blocklist) - 1; i >= 0; i-- {
			rh := blocklist[i]
			if rh.Number < start || rh.Number > end {
				return nil, errors.New("Bad peer returning root block height out of range ")
			}

			if preHeader != nil && rh.Number >= preHeader.Number {
				return nil, errors.New("Bad peer returning root block height must be ordered ")
			}
			preHeader = rh

			if !bc.HasBlock(rh.Hash()) {
				end = rh.Number - 1
				continue
			}

			if rh.Number == end {
				return rh, nil
			}

			start = rh.Number + 1
			bestAncestor = rh
			if start > end {
				return nil, errors.New("Bad order start and end to download root blocks ")
			}
			break
		}
	}
	return bestAncestor, nil
}

func (r *rootChainTask) syncMinorBlocks(
	rbc rootblockchain,
	rootBlock *types.RootBlock,
) error {
	downloadMap := make(map[uint32][]common.Hash)
	for _, header := range rootBlock.MinorBlockHeaders() {
		hash := header.Hash()
		downloadMap[header.Branch.Value] = append(downloadMap[header.Branch.Value], hash)
	}

	var g errgroup.Group
	for branch, hashes := range downloadMap {
		b, hashList := branch, hashes
		conns := r.slaveConns.GetSlaveConnsById(b)
		if len(conns) == 0 {
			return fmt.Errorf("shard connection for branch %d is missing", b)
		}
		// TODO Support to multiple connections
		g.Go(func() error {
			status, err := conns[0].AddBlockListForSync(&rpc.AddBlockListForSyncRequest{Branch: b, PeerId: r.PeerID(), MinorBlockHashList: hashList})
			if err == nil {
				r.statusChan <- status
			}
			return err
		})
	}
	err := g.Wait()
	if err != nil {
		return err
	}

	//for _, hashes := range downloadMap {
	//	for _, hash := range hashes {
	//		rbc.AddValidatedMinorBlockHeader(hash, nil) //TODO @sync to modify
	//	}
	//}
	return nil
}
