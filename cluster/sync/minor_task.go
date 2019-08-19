package sync

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type minorSyncerPeer interface {
	GetMinorBlockHeaderList(hash common.Hash, limit, branch uint32, reverse bool) ([]*types.MinorBlockHeader, error)
	GetMinorBlockHeaderListWithSkip(tp uint8, data common.Hash, limit, skip uint32, branch uint32, direction uint8) (*p2p.GetMinorBlockHeaderListResponse, error)
	GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error)
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
		header: header,
		peer:   p,
	}
	mChain.task = task{
		name:             fmt.Sprintf("shard-%d", header.Branch.GetShardID()),
		maxSyncStaleness: 22500 * 6, // TODO: derive from root chain?
		findAncestor: func(bc blockchain) (types.IHeader, error) {
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
			limit := MinorBlockHeaderListLimit
			heightDiff := mChain.header.Number - mHeader.Number
			if limit > heightDiff {
				limit = heightDiff
			}

			mBHeaders, err := mChain.downloadBlockHeaderListAndCheck(startheader.NumberU64(), 0, limit, mHeader.Branch.Value)
			if err != nil {
				return nil, err
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
	data := big.NewInt(int64(height)).Bytes()
	resp, err := m.peer.GetMinorBlockHeaderListWithSkip(1, common.BytesToHash(data), uint32(limit), uint32(skip), branch, DirectionToTip)
	if err != nil {
		return nil, err
	}

	if resp.ShardTip.Difficulty.Cmp(m.header.Difficulty) < 0 {
		return nil, errors.New("Bad peer sending minor block tip with lower TD ")
	}

	if len(resp.BlockHeaderList) == 0 {
		return nil, errors.New("Remote chain reorg causing empty minor block headers ")
	}

	newLimit := (resp.ShardTip.Number + 1 - height) / (skip + 1)
	if newLimit < limit {
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

	end := mtip.Number
	start := mtip.Number - m.maxStaleness
	if start < 0 {
		start = 0
	}
	if m.header.Number < end {
		end = m.header.Number
	}

	var bestAncestor *types.MinorBlockHeader
	for end >= start {
		span := (end-start)/MinorBlockHeaderListLimit + 1

		mBHeaders, err := m.downloadBlockHeaderListAndCheck(start, span-1, MinorBlockHeaderListLimit, m.header.Branch.Value)
		if err != nil {
			return nil, err
		}

		var preHeader *types.MinorBlockHeader
		for i := len(mBHeaders); i >= 0; i-- {
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
		}
	}
	return bestAncestor, nil
}
