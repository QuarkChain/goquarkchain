package sync

import (
	"errors"
	"fmt"
	"math/rand"
	"testing"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/assert"
)

func (p *mockpeer) GetMinorBlockHeaderList(req *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	if p.downloadHeaderError != nil {
		return nil, p.downloadHeaderError
	}

	if req.Limit <= 0 || req.Limit > 2*MinorBlockHeaderListLimit {
		return nil, errors.New("Bad limit ")
	}
	if req.Direction != qcom.DirectionToGenesis && req.Direction != qcom.DirectionToTip {
		return nil, errors.New("Bad direction ")
	}

	var (
		hash   = req.GetHash()
		height = req.GetHeight()
	)
	if hash == (common.Hash{}) && height == nil {
		return nil, errors.New("Bad params minor block hash and height ")
	}

	sign := 0
	mBHeaders := make([]*types.MinorBlockHeader, 0, req.Limit)
	for i, hd := range p.retMHeaders {
		if hd.Hash() == hash || hd.Number == *height {
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

	return mBHeaders, nil
}

func (p *mockpeer) GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	if p.downloadBlockError != nil {
		return nil, p.downloadBlockError
	}

	if len(hashes) > 2*MinorBlockBatchSize {
		return nil, fmt.Errorf("bad number of minor blocks requested. branch: %d; limit: %d; expected limit: %d",
			branch, MinorBlockHeaderListLimit, MinorBlockBatchSize)
	}
	mBlocks := make([]*types.MinorBlock, 0, len(hashes))
	sign := 0
	for _, mhash := range hashes {
		for ; sign < len(p.retMBlocks); sign++ {
			if mhash == p.retMBlocks[sign].Hash() {
				mBlocks = append(mBlocks, p.retMBlocks[sign])
				break
			}
		}
	}
	return mBlocks, nil
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
	defer blockchain.Stop()
	_, err = blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(fmt.Sprintf("failed to init minor blockchain: %v", err))
	}
	if _, err := blockchain.InsertChain(blocks, false); err != nil {
		panic(fmt.Sprintf("failed to insert minor blocks: %v", err))
	}

	return &mockblockchain{mbc: blockchain}, db
}

func TestMinorChainTask(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc, db := newMinorBlockChain(10)
	v := &mockvalidator{}
	bc.(*mockblockchain).validator = v
	mbc := bc.(*mockblockchain).mbc

	retMBlocks, retMHeaders := makeMinorChains(mbc.GetBlockByNumber(0).(*types.MinorBlock), 20, db, false)

	// No error if already have the target block.
	var mt = NewMinorChainTask(p, bc.CurrentHeader().(*types.MinorBlockHeader))
	mTask := mt.(*minorChainTask)
	assert.NoError(t, mt.Run(bc))

	// Happy path.
	p.retMBlocks, p.retMHeaders = retMBlocks, retMHeaders
	mTask.header = retMHeaders[4]
	// Confirm 5 more blocks are successfully added to existing 5-block chain.
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(10))

	// Rollback and test unhappy path.
	mbc.SetHead(5)
	assert.Equal(t, bc.CurrentHeader().NumberU64(), retMHeaders[5].NumberU64())

	// Get errors when downloading headers.
	mTask.header = retMHeaders[11]
	p.downloadHeaderError = errors.New("download error")
	assert.Error(t, mt.Run(bc))

	// Downloading headers succeeds, but block validation failed.
	p.downloadHeaderError = nil
	v.err = errors.New("validate error")
	assert.Error(t, mt.Run(bc))

	// Block validation succeeds, but block header list not correct.
	v.err = nil
	wrongHeaders := reverseMHeaders(retMHeaders)
	rand.Shuffle(len(wrongHeaders), func(i, j int) {
		wrongHeaders[i], wrongHeaders[j] = wrongHeaders[j], wrongHeaders[i]
	})
	p.retMHeaders = wrongHeaders
	assert.Error(t, mt.Run(bc))

	// Validation succeeds. Should be downloading actual blocks. Mock some errors.
	p.retMHeaders = retMHeaders
	p.downloadBlockError = errors.New("download error")
	assert.Error(t, mt.Run(bc))

	// Downloading blocks succeeds. But make the returned blocks miss one. Insertion should fail.
	p.downloadBlockError = nil
	missing := p.retMBlocks[len(p.retMBlocks)-4:]
	p.retMBlocks = p.retMBlocks[:len(p.retMBlocks)-4]
	mTask.header = retMHeaders[len(retMHeaders)-1]
	assert.Error(t, mt.Run(bc))

	// Add back that missing block. Happy again.
	p.retMBlocks = append(p.retMBlocks, missing...)
	assert.NoError(t, mt.Run(bc))
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(20))

	// add maxSyncStaleness minor blocks
	mTask.maxSyncStaleness = 1000
	retMBlocks, retMHeaders = makeMinorChains(retMBlocks[len(retMBlocks)-1], 1000, db, false)
	p.retMHeaders = append(p.retMHeaders, retMHeaders[1:]...)
	p.retMBlocks = append(p.retMBlocks, retMBlocks[1:]...)

	// just sync 10 minor blocks.
	mTask.header = retMHeaders[10]
	assert.NoError(t, mt.Run(bc))
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(20+10))

	// sync the last all 990 minor blocks.
	mTask.header = retMHeaders[len(retMHeaders)-1]
	assert.NoError(t, mt.Run(bc))
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(20+1000))

	// Sync older forks. Starting from block 6, up to maxSyncStaleness.
	// retMBlocks, retMHeaders = makeRootChains(retMBlocks[len(retMBlocks)-1], 1000, true)
	retMBlocks, retMHeaders = makeMinorChains(retMBlocks[len(retMBlocks)-1], 1000, db, true)
	for _, rh := range retMHeaders[1:] {
		assert.False(t, bc.HasBlock(rh.Hash()))
	}

	mTask.header = retMHeaders[len(retMHeaders)-1]
	p.retMHeaders, p.retMBlocks = append(p.retMHeaders[20:], retMHeaders...), append(p.retMBlocks[20:], retMBlocks...)
	assert.NoError(t, mt.Run(bc))
	for _, rh := range retMHeaders {
		assert.True(t, bc.HasBlock(rh.Hash()))
	}

	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(2000+20))
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
