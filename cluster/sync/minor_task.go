package sync

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type minorSyncerPeer interface {
	GetMinorBlockHeaderList(gReq *rpc.GetMinorBlockHeaderListRequest) (*p2p.GetMinorBlockHeaderListResponse, error)
	// GetMinorBlockHeaderList(hash common.Hash, limit, branch uint32, reverse bool) ([]*types.MinorBlockHeader, error)
	GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error)
	MinorHead(branch uint32) *p2p.Tip
	PeerID() string
}

type minorChainTask struct {
	task
	maxStaleness uint64
	stat         *MinorBlockSychronizerStats
	peer         minorSyncerPeer
	header       *types.MinorBlockHeader
}

// NewMinorChainTask returns a sync task for minor chain.
func NewMinorChainTask(
	p minorSyncerPeer,
	header *types.MinorBlockHeader,
) Task {
	mChain := &minorChainTask{
		header:       header,
		peer:         p,
		maxStaleness: 22500 * 6,
	}
	mChain.task = task{
		name:             fmt.Sprintf("shard-%d", header.Branch.GetShardID()),
		maxSyncStaleness: 22500 * 6, // TODO: derive from root chain?
		findAncestor: func(bc blockchain) (types.IHeader, error) {

			if bc.HasBlock(mChain.header.Hash()) {
				return nil, nil
			}

			ancestor, err := mChain.findAncestor(bc)
			if err != nil {
				return nil, err
			}
			return ancestor, nil
		},
		getHeaders: func(startheader types.IHeader) ([]types.IHeader, error) {
			if startheader.NumberU64() >= mChain.header.Number {
				return nil, nil
			}

			mHeader := startheader.(*types.MinorBlockHeader)
			limit := uint64(MinorBlockHeaderListLimit)
			heightDiff := mChain.header.Number - mHeader.Number
			if limit > heightDiff {
				limit = heightDiff
			}

			mBHeaders, err := mChain.downloadBlockHeaderListAndCheck(mHeader.Number+1, 0, limit, mHeader.Branch.Value)
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
	}
	return mChain
}

func (m *minorChainTask) Priority() *big.Int {
	return new(big.Int).SetUint64(m.header.NumberU64())
}

func (m *minorChainTask) PeerID() string {
	return m.peer.PeerID()
}

func (m *minorChainTask) downloadBlockHeaderListAndCheck(height, skip,
limit uint64, branch uint32) ([]*types.MinorBlockHeader, error) {
	req := &rpc.GetMinorBlockHeaderListRequest{
		Height:    &height,
		Hash:      common.Hash{},
		Skip:      uint32(skip),
		Limit:     uint32(limit),
		Direction: qcom.DirectionToTip,
		Branch:    branch,
	}
	resp, err := m.peer.GetMinorBlockHeaderList(req)
	if err != nil {
		return nil, err
	}

	if len(resp.BlockHeaderList) == 0 {
		return nil, errors.New("Remote chain reorg causing empty minor block headers ")
	}

	newLimit := (resp.ShardTip.Number + 1 - height) / (skip + 1)
	if newLimit > limit {
		newLimit = limit
	}

	if len(resp.BlockHeaderList) != int(newLimit) {
		return nil, errors.New("Bad peer sending incorrect number of root block headers ")
	}

	if resp.ShardTip.Hash() != m.header.Hash() {
		m.header = resp.ShardTip
	}
	return resp.BlockHeaderList, nil
}

func (m *minorChainTask) findAncestor(bc blockchain) (*types.MinorBlockHeader, error) {
	mtip := bc.CurrentHeader().(*types.MinorBlockHeader)
	if m.header.Hash() == mtip.Hash() {
		return mtip, nil
	}

	end := m.peer.MinorHead(m.header.Branch.Value).MinorBlockHeaderList[0].Number
	start := end - m.maxStaleness
	if end < m.maxStaleness {
		start = 0
	}
	if m.header.Number < end {
		end = m.header.Number
	}

	var bestAncestor *types.MinorBlockHeader
	for end >= start {
		span := (end-start)/MinorBlockHeaderListLimit + 1
		mBHeaders, err := m.downloadBlockHeaderListAndCheck(start, span-1, (end+1-start)/span, m.header.Branch.Value)
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
