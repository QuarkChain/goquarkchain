package sync

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/p2p"
	"math/big"

	"golang.org/x/sync/errgroup"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

type rootSyncerPeer interface {
	GetRootBlockHeaderList(*rpc.GetRootBlockHeaderListRequest) (*p2p.GetRootBlockHeaderListResponse, error)
	GetRootBlockList(hashes []common.Hash) ([]*types.RootBlock, error)
	RootHead() *types.RootBlockHeader
	PeerID() string
}

// All of the sync tasks to are to catch up with the root chain from peers.
type rootChainTask struct {
	task
	header           *types.RootBlockHeader
	peer             rootSyncerPeer
	stats            *BlockSychronizerStats
	statusChan       chan *rpc.ShardStatus
	getShardConnFunc func(fullShardId uint32) []rpc.ShardConnForP2P
	maxStaleness     uint32
}

// NewRootChainTask returns a sync task for root chain.
func NewRootChainTask(
	p rootSyncerPeer,
	header *types.RootBlockHeader,
	stats *BlockSychronizerStats,
	statusChan chan *rpc.ShardStatus,
	getShardConnFunc func(fullShardId uint32) []rpc.ShardConnForP2P,
) Task {
	rChain := &rootChainTask{
		peer:             p,
		header:           header,
		stats:            stats,
		statusChan:       statusChan,
		getShardConnFunc: getShardConnFunc,
		maxStaleness:     22500,
	}
	rChain.task = task{
		name:             "root",
		maxSyncStaleness: 22500,
		findAncestor: func(bc blockchain) (types.IHeader, error) {

			if bc.HasBlock(rChain.header.Hash()) {
				return nil, nil
			}

			ancestor, err := rChain.findAncestor(bc)
			if err != nil {
				rChain.stats.AncestorNotFoundCount += 1
				return nil, err
			}

			if !bc.HasBlock(ancestor.Hash()) {
				return nil, errors.New("Bad ancestor ")
			}

			if rChain.header.ToTalDifficulty.Cmp(ancestor.ToTalDifficulty) < 0 {
				return nil, errors.New("ancestor's total difficulty is bigger than current")
			}
			return ancestor, nil
		},
		getHeaders: func(startHeader types.IHeader) ([]types.IHeader, error) {
			if startHeader.NumberU64() >= uint64(rChain.header.Number) {
				return nil, nil
			}

			limit := uint32(RootBlockHeaderListLimit)
			heightDiff := rChain.header.Number - uint32(startHeader.NumberU64())
			if limit > heightDiff {
				limit = heightDiff
			}

			rBHeaders, err := rChain.downloadBlockHeaderListAndCheck(uint32(startHeader.NumberU64()+1), 0, limit)
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
			return rChain.syncMinorBlocks(rbc, rb)
		},
		needSkip: func(b blockchain) bool {
			if rChain.header.GetTotalDifficulty().Cmp(b.CurrentHeader().GetTotalDifficulty()) <= 0 {
				return true
			}
			return false
		},
	}
	return rChain
}

func (r *rootChainTask) Priority() *big.Int {
	return r.header.GetTotalDifficulty()
}

func (r *rootChainTask) PeerID() string {
	return r.peer.PeerID()
}

func (r *rootChainTask) downloadBlockHeaderListAndCheck(start uint32, skip,
limit uint32) ([]*types.RootBlockHeader, error) {
	resp, err := r.peer.GetRootBlockHeaderList(&rpc.GetRootBlockHeaderListRequest{
		Height:    &start,
		Skip:      skip,
		Limit:     limit,
		Direction: qcom.DirectionToTip,
	})
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

	newLimit := (resp.RootTip.Number + 1 - start) / (skip + 1)
	if newLimit > limit {
		newLimit = limit
	}
	if len(resp.BlockHeaderList) != int(newLimit) {
		return nil, errors.New("Bad peer sending incorrect number of root block headers ")
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

	end := r.peer.RootHead().Number
	start := end - r.maxStaleness
	if end < r.maxStaleness {
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
		conns := r.getShardConnFunc(b)
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
