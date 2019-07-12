package sync

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"math/big"
	"math/rand"
	"testing"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/assert"
)

func (p *mockpeer) GetMinorBlockHeaderList(hash common.Hash, limit, branch uint32, reverse bool) ([]*types.MinorBlockHeader, error) {
	if p.downloadHeaderError != nil {
		return nil, p.downloadHeaderError
	}
	// May return a subset.
	for i, h := range p.retMHeaders {
		if h.Hash() == hash {
			ret := p.retMHeaders[i:len(p.retMHeaders)]
			return ret, nil
		}
	}
	panic("lolwut")
}

func (p *mockpeer) GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	if p.downloadBlockError != nil {
		return nil, p.downloadBlockError
	}
	return p.retMBlocks, nil
}

func newMinorBlockChain(sz int) (blockchain, ethdb.Database) {
	var (
		db            = ethdb.NewMemDatabase()
		clusterConfig = config.NewClusterConfig()
		rootBlock     = genesis.CreateRootBlock()
		fullShardID   = qkcconfig.Chains[0].ShardSize | 0
	)
	clusterConfig.Quarkchain = qkcconfig
	qkcconfig.SkipRootCoinbaseCheck = true
	qkcconfig.SkipMinorDifficultyCheck = true
	qkcconfig.SkipRootDifficultyCheck = true
	addr0 := account.NewAddress(account.BytesToIdentityRecipient(common.Address{0}.Bytes()), 0)
	ids := qkcconfig.GetGenesisShardIds()
	for _, v := range ids {
		shardConfig := qkcconfig.GetShardConfigByFullShardID(v)
		addr := addr0.AddressInBranch(account.Branch{Value: v})
		shardConfig.Genesis.Alloc[addr] = big.NewInt(0)
	}
	minorGenesis := genesis.MustCommitMinorBlock(db, rootBlock, fullShardID)
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
	if _, err := blockchain.InsertChain(blocks); err != nil {
		panic(fmt.Sprintf("failed to insert minor blocks: %v", err))
	}

	return &mockblockchain{mbc: blockchain}, db
}

func TestMinorChainTaskRun(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc, db := newMinorBlockChain(5)
	mbc := bc.(*mockblockchain).mbc
	currHeader := bc.CurrentHeader().(*types.MinorBlockHeader)
	var mt Task = NewMinorChainTask(p, currHeader)

	// Prepare future blocks for downloading.
	mbChain, mhChain := makeMinorChains(mbc.CurrentBlock(), db, false)

	// No error if already have the target block.
	assert.NoError(t, mt.Run(bc))
	// Happy path.
	mt.(*minorChainTask).header = mbChain[4].Header()
	v := &mockvalidator{}
	bc.(*mockblockchain).validator = v
	p.retMHeaders, p.retMBlocks = reverseMHeaders(mhChain), reverseMBlocks(mbChain)
	assert.NoError(t, mt.Run(bc))
	// Confirm 5 more blocks are successfully added to existing 5-block chain.
	assert.Equal(t, uint64(10), bc.CurrentHeader().NumberU64())

	// Rollback and test unhappy path.
	mbc.SetHead(5)
	assert.Equal(t, mhChain[0].ParentHash, bc.CurrentHeader().Hash())
	// Get errors when downloading headers.
	mt.(*minorChainTask).header = mbChain[4].Header()
	p.downloadHeaderError = errors.New("download error")
	assert.Error(t, mt.Run(bc))
	// Downloading headers succeeds, but block validation failed.
	p.downloadHeaderError = nil
	v.err = errors.New("validate error")
	assert.Error(t, mt.Run(bc))
	// Block validation succeeds, but block header list not correct.
	v.err = nil
	wrongHeaders := reverseMHeaders(mhChain)
	rand.Shuffle(len(wrongHeaders), func(i, j int) {
		wrongHeaders[i], wrongHeaders[j] = wrongHeaders[j], wrongHeaders[i]
	})
	p.retMHeaders = wrongHeaders
	assert.Error(t, mt.Run(bc))
	// Validation succeeds. Should be downloading actual blocks. Mock some errors.
	p.retMHeaders = reverseMHeaders(mhChain)
	p.downloadBlockError = errors.New("download error")
	assert.Error(t, mt.Run(bc))
	// Downloading blocks succeeds. But make the returned blocks miss one. Insertion should fail.
	p.downloadBlockError = nil
	missing := p.retMBlocks[len(p.retMBlocks)-1]
	p.retMBlocks = p.retMBlocks[0 : len(p.retMBlocks)-1]
	assert.Error(t, mt.Run(bc))
	// Add back that missing block. Happy again.
	p.retMBlocks = append(p.retMBlocks, missing)
	assert.NoError(t, mt.Run(bc))
	assert.Equal(t, uint64(10), bc.CurrentHeader().NumberU64())

	// Sync older forks. Starting from block 6, up to 11.
	mbChain, mhChain = makeMinorChains(mbChain[0], db, true)
	for _, rh := range mhChain {
		assert.False(t, bc.HasBlock(rh.Hash()))
	}
	mt.(*minorChainTask).header = mbChain[4].Header()
	p.retMHeaders, p.retMBlocks = reverseMHeaders(mhChain), reverseMBlocks(mbChain)
	assert.NoError(t, mt.Run(bc))
	for _, mh := range mhChain {
		assert.True(t, bc.HasBlock(mh.Hash()))
	}
	// Tip should be updated.
	assert.Equal(t, uint64(11), bc.CurrentHeader().NumberU64())
}

/*
 Test helpers.
*/

func makeMinorChains(parent *types.MinorBlock, db ethdb.Database, random bool) ([]*types.MinorBlock, []*types.MinorBlockHeader) {
	var gen func(config *config.QuarkChainConfig, i int, b *core.MinorBlockGen)
	if random {
		gen = func(config *config.QuarkChainConfig, i int, b *core.MinorBlockGen) {
			b.SetExtra([]byte{byte(i)})
		}
	}
	blockchain, _ := core.GenerateMinorBlockChain(params.TestChainConfig, qkcconfig, parent, engine, db, 5, gen)
	var headerchain []*types.MinorBlockHeader
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
