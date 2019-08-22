package sync

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/assert"
	"testing"
)

func (p *mockpeer) GetMinorBlockHeaderList(req *rpc.GetMinorBlockHeaderListRequest) (*p2p.GetMinorBlockHeaderListResponse, error) {
	if p.downloadHeaderError != nil {
		return nil, p.downloadHeaderError
	}

	if req.Limit <= 0 || req.Limit > 2*MinorBlockHeaderListLimit {
		return nil, errors.New("Bad limit ")
	}
	if req.Direction != qcom.DirectionToGenesis && req.Direction != qcom.DirectionToTip {
		return nil, errors.New("Bad direction ")
	}

	if req.Hash == (common.Hash{}) && req.Height == nil {
		return nil, errors.New("Bad params minor block hash and height ")
	}

	sign := 0
	mBHeaders := make([]*types.MinorBlockHeader, 0, req.Limit)
	for i, hd := range p.retMHeaders {
		if hd.Hash() == req.Hash || hd.Number == *req.Height {
			sign = i
			break
		}
	}

	direction := int(req.Skip + 1)
	if req.Direction == qcom.DirectionToGenesis {
		direction = 0 - direction
	}

	for ; sign >= 0 && sign < len(p.retMHeaders) && len(mBHeaders) < cap(mBHeaders); sign += direction {
		mBHeaders = append(mBHeaders, p.retMHeaders[sign])
	}

	return &p2p.GetMinorBlockHeaderListResponse{
		ShardTip:        p.retMHeaders[len(p.retMHeaders)-1],
		BlockHeaderList: mBHeaders,
	}, nil
}

func (p *mockpeer) GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	mBlocks := make([]*types.MinorBlock, 0, len(hashes))
	sign := 0
	for _, mhash := range hashes {
		for ; sign < len(p.retMHeaders); sign++ {
			if mhash == p.retMBlocks[sign].Hash() {
				mBlocks = append(mBlocks, p.retMBlocks[sign])
				break
			}
		}
	}
	return mBlocks, nil
}

func (p *mockpeer) MinorHead(branch uint32) (*types.MinorBlockHeader, error) {
	return p.retMHeaders[len(p.retMHeaders)-1], nil
}

func newMinorBlockChain(sz int) (blockchain, ethdb.Database) {
	var (
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		rootBlock     = genesis.CreateRootBlock()
		fullShardID   = qkcconfig.Chains[0].ShardSize | 0
		minorGenesis  = genesis.MustCommitMinorBlock(db, rootBlock, fullShardID)
	)
	clusterConfig.Quarkchain = qkcconfig
	qkcconfig.SkipRootCoinbaseCheck = true
	qkcconfig.SkipMinorDifficultyCheck = true
	qkcconfig.SkipRootDifficultyCheck = true
	minorBlocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, qkcconfig, minorGenesis, engine, db, sz, nil)

	var blocks []types.IBlock
	for _, mb := range minorBlocks {
		blocks = append(blocks, mb)
	}

	blockchain, err := core.NewMinorBlockChain(db, nil, params.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, fullShardID)
	if err != nil {
		panic(fmt.Sprintf("failed to create minor blockchain: %v", err))
	}
	_, err = blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(fmt.Sprintf("failed to init minor blockchain: %v", err))
	}
	if _, err := blockchain.InsertChain(blocks, nil); err != nil {
		panic(fmt.Sprintf("failed to insert minor blocks: %v", err))
	}

	return &mockblockchain{mbc: blockchain}, db
}

func TestMinorChainTaskEmpty(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc, _ := newMinorBlockChain(5)
	currHeader := bc.CurrentHeader().(*types.MinorBlockHeader)
	var mt = NewMinorChainTask(p, currHeader)

	// No error if already have the target block.
	assert.NoError(t, mt.Run(bc))
}

func TestMinorChainTaskBigerThanLimit(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc, db := newMinorBlockChain(5)
	v := &mockvalidator{}
	bc.(*mockblockchain).validator = v
	mbc := bc.(*mockblockchain).mbc

	// Prepare future blocks for downloading.
	p.retMBlocks, p.retMHeaders = makeMinorChains(mbc.GetBlockByNumber(0).(*types.MinorBlock), 100, db, false)

	var mt = NewMinorChainTask(p, p.retMHeaders[51])

	// No error if already have the target block.
	assert.NoError(t, mt.Run(bc))

}

/*
 Test helpers.
*/

func makeMinorChains(parent *types.MinorBlock, height int, db ethdb.Database, random bool) ([]*types.MinorBlock, []*types.MinorBlockHeader) {
	var gen func(config *config.QuarkChainConfig, i int, b *core.MinorBlockGen)
	if random {
		gen = func(config *config.QuarkChainConfig, i int, b *core.MinorBlockGen) {
			b.SetExtra([]byte{byte(i)})
		}
	}

	var (
		headerchain []*types.MinorBlockHeader
		blockchain  []*types.MinorBlock
	)
	blockchain = append(blockchain, parent)

	tBlocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, qkcconfig, parent, engine, db, height, gen)
	blockchain = append(blockchain, tBlocks...)
	for _, mb := range blockchain {
		headerchain = append(headerchain, mb.Header())
	}

	return blockchain, headerchain
}

func reverseMBlocks(ls []*types.MinorBlock) []*types.MinorBlock {
	ret := make([]*types.MinorBlock, len(ls), len(ls))
	copy(ret, ls)
	for left, right := 0, len(ls)-1; left < right; left, right = left+1, right-1 {
		ret[left], ret[right] = ret[right], ret[left]
	}
	return ret
}

func reverseMHeaders(ls []*types.MinorBlockHeader) []*types.MinorBlockHeader {
	ret := make([]*types.MinorBlockHeader, len(ls), len(ls))
	copy(ret, ls)
	for left, right := 0, len(ls)-1; left < right; left, right = left+1, right-1 {
		ret[left], ret[right] = ret[right], ret[left]
	}
	return ret
}
