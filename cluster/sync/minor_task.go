package sync

import (
	"errors"
	"fmt"
	"math/big"
	
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
)

type minorSyncerPeer interface {
	GetMinorBlockHeaderList(gReq *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error)
	// GetMinorBlockHeaderList(hash common.Hash, limit, branch uint32, reverse bool) ([]*types.MinorBlockHeader, error)
	GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error)
	PeerID() string
}

type minorChainTask struct {
	task
	stats  *BlockSychronizerStats
	peer   minorSyncerPeer
	header *types.MinorBlockHeader
}

// NewMinorChainTask returns a sync task for minor chain.
func NewMinorChainTask(
	p minorSyncerPeer,
	header *types.MinorBlockHeader,
) Task {
	mTask := &minorChainTask{
		header: header,
		stats:  &BlockSychronizerStats{},
		peer:   p,
	}
	mTask.task = task{
		name:             fmt.Sprintf("shard-%d", header.Branch.GetShardID()),
		header:           header,
		maxSyncStaleness: 22500 * 6, // TODO: derive from root chain?
		batchSize:        MinorBlockHeaderListLimit,
		findAncestor: func(bc blockchain) (types.IHeader, error) {

			if bc.HasBlock(mTask.header.Hash()) {
				return nil, nil
			}

			ancestor, err := mTask.findAncestor(bc)
			if err != nil {
				mTask.stats.AncestorNotFoundCount += 1
				return nil, err
			}

			if !bc.HasBlock(ancestor.Hash()) {
				return nil, errors.New("Bad ancestor ")
			}

			return ancestor, nil
		},
		getHeaders: func(startheader types.IHeader) ([]types.IHeader, error) {
			if startheader.NumberU64() >= mTask.header.Number {
				return nil, nil
			}

			mHeader := startheader.(*types.MinorBlockHeader)
			limit := uint64(MinorBlockHeaderListLimit)
			heightDiff := mTask.header.Number - mHeader.Number
			if limit > heightDiff {
				limit = heightDiff
			}

			mBHeaders, err := mTask.downloadBlockHeaderListAndCheck(mHeader.Number+1, 0, limit, mHeader.Branch.Value)
			if err != nil {
				return nil, err
			}

			if len(mBHeaders) == 0 {
				return nil, errors.New("Remote chain reorg causing empty minor block headers ")
			}

			if mBHeaders[0].ParentHash != mHeader.Hash() {
				return nil, errors.New("Bad peer sending incorrect canonical minor headers ")
			}

			iHeaders := make([]types.IHeader, 0, len(mBHeaders))
			for _, hd := range mBHeaders {
				hd := hd
				iHeaders = append(iHeaders, hd)
			}
			return iHeaders, nil
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
		needSkip: func(b blockchain) bool {
			if mTask.header.NumberU64() <= b.CurrentHeader().NumberU64() || b.HasBlock(mTask.header.Hash()) {
				return true
			}

			bc, ok := b.(*core.MinorBlockChain)
			if !ok {
				return false
			}
			// Do not download if the prev root block is not synced
			rootBlockHeader := bc.GetRootBlockByHash(mTask.header.PrevRootBlockHash)
			if rootBlockHeader == nil {
				return true
			}

			// Do not download if the new header's confirmed root is lower then current root tip last header's confirmed root
			// This means the minor block's root is a fork, which will be handled by master sync
			if bc.GetMinorTip() == nil {
				return false
			}
			confirmedRootHeader := bc.GetRootBlockByHash(bc.GetMinorTip().PrevRootBlockHash())
			if confirmedRootHeader != nil && confirmedRootHeader.NumberU64() > rootBlockHeader.NumberU64() {
				return true
			}
			return false
		},
	}
	return mTask
}

func (m *minorChainTask) Priority() *big.Int {
	return new(big.Int).SetUint64(m.header.NumberU64())
}

func (m *minorChainTask) PeerID() string {
	return m.peer.PeerID()
}

func (m *minorChainTask) downloadBlockHeaderListAndCheck(height, skip, limit uint64, branch uint32) ([]*types.MinorBlockHeader, error) {
	req := &rpc.GetMinorBlockHeaderListWithSkipRequest{
		GetMinorBlockHeaderListWithSkipRequest: p2p.GetMinorBlockHeaderListWithSkipRequest{
			Limit:     uint32(limit),
			Skip:      uint32(skip),
			Direction: qcom.DirectionToTip,
			Branch:    account.Branch{Value: branch},
		},
		PeerID: m.PeerID(),
	}
	req.SetHeight(height)
	mHeaders, err := m.peer.GetMinorBlockHeaderList(req)
	if err != nil {
		return nil, err
	}
	m.stats.HeadersDownloaded += uint64(len(mHeaders))

	if len(mHeaders) == 0 {
		return nil, errors.New("Remote chain reorg causing empty minor block headers ")
	}

	newLimit := (m.header.Number + 1 - height + skip) / (skip + 1)
	if newLimit > limit {
		newLimit = limit
	}

	if len(mHeaders) != int(newLimit) {
		return nil, fmt.Errorf("Bad peer sending incorrect number of minor block headers expect: %d, actual: %d", newLimit, len(mHeaders))
	}

	return mHeaders, nil
}

func (m *minorChainTask) findAncestor(bc blockchain) (*types.MinorBlockHeader, error) {
	mtip := bc.CurrentHeader().(*types.MinorBlockHeader)
	if m.header.Hash() == mtip.Hash() {
		return mtip, nil
	}

	end := mtip.Number
	start := end - m.task.maxSyncStaleness
	if end < m.task.maxSyncStaleness {
		start = 0
	}
	if m.header.Number < end {
		end = m.header.Number
	}

	var bestAncestor *types.MinorBlockHeader
	for end >= start {
		m.stats.AncestorLookupRequests += 1
		span := (end-start)/MinorBlockHeaderListLimit + 1
		mBHeaders, err := m.downloadBlockHeaderListAndCheck(start, span-1, (end+1-start+span-1)/span, m.header.Branch.Value)
		if err != nil {
			return nil, err
		}

		var preHeader *types.MinorBlockHeader
		for i := len(mBHeaders) - 1; i >= 0; i-- {
			mh := mBHeaders[i]
			if mh.Number < start || mh.Number > end {
				return nil, errors.New("Bad peer returning minor block height out of range ")
			}

			if preHeader != nil && mh.Number > preHeader.Number {
				return nil, errors.New("Bad peer returning minor block height must be ordered ")
			}
			preHeader = mh

			if !bc.HasBlock(mh.Hash()) {
				end = mh.Number - 1
				continue
			}
			if mh.Number == end {
				return mh, nil
			}

			start = mh.Number + 1
			bestAncestor = mh
			if start > end {
				return nil, errors.New("Bad order start and end to download minor blocks ")
			}
			break
		}
	}
	return bestAncestor, nil
}
