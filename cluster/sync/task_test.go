package sync

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/stretchr/testify/assert"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
)

var (
	qkcconfig = config.NewQuarkChainConfig()
	genesis   = core.NewGenesis(qkcconfig)
	engine    = new(consensus.FakeEngine)
)

type mockpeer struct {
	name                string
	downloadHeaderError error
	downloadBlockError  error
	retHeaders          []*types.RootBlockHeader // Order: descending.
	retBlocks           []*types.RootBlock       // Order: descending.
}

func (p *mockpeer) downloadRootHeadersFromHash(hash common.Hash, sz uint64) ([]*types.RootBlockHeader, error) {
	if p.downloadHeaderError != nil {
		return nil, p.downloadHeaderError
	}
	// May return a subset.
	for i, h := range p.retHeaders {
		if h.Hash() == hash {
			ret := p.retHeaders[i:len(p.retHeaders)]
			return ret, nil
		}
	}
	panic("lolwut")
}

func (p *mockpeer) downloadRootBlocks([]*types.RootBlockHeader) ([]*types.RootBlock, error) {
	if p.downloadBlockError != nil {
		return nil, p.downloadBlockError
	}
	return p.retBlocks, nil
}

func (p *mockpeer) id() string {
	return p.name
}

type mockblockchain struct {
	rbc       *core.RootBlockChain
	validator headerValidator
}

func (bc *mockblockchain) HasBlock(hash common.Hash) bool {
	header := bc.rbc.GetHeader(hash)
	return !(header == nil || reflect.ValueOf(header).IsNil())
}

func (bc *mockblockchain) InsertChain(blocks []types.IBlock) (int, error) {
	return bc.rbc.InsertChain(blocks)
}

func (bc *mockblockchain) CurrentHeader() types.IHeader {
	return bc.rbc.CurrentHeader()
}

func (bc *mockblockchain) Validator() headerValidator {
	return bc.validator
}

type mockvalidator struct {
	err error
}

func (v *mockvalidator) ValidateHeader(types.IHeader) error {
	return v.err
}

func newBlockChain(sz int) blockchain {
	qkcconfig.SkipRootCoinbaseCheck = true
	db := ethdb.NewMemDatabase()
	genesisBlock := genesis.MustCommitRootBlock(db)
	blockchain, err := core.NewRootBlockChain(db, nil, qkcconfig, engine, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to generate root blockchain: %v", err))
	}
	rootBlocks := core.GenerateRootBlockChain(genesisBlock, engine, sz, nil)
	var blocks []types.IBlock
	for _, rb := range rootBlocks {
		blocks = append(blocks, rb)
	}

	_, err = blockchain.InsertChain(blocks)
	if err != nil {
		panic(fmt.Sprintf("failed to insert headers: %v", err))
	}
	return &mockblockchain{rbc: blockchain}
}

func TestRootChainTaskRun(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc := newBlockChain(5)
	rbc := bc.(*mockblockchain).rbc
	var rt Task = &rootChainTask{peer: p}

	// Prepare future blocks for downloading.
	rbChain, rhChain := makeChains(rbc.CurrentBlock(), false)

	// No error if already have the target block.
	rt.(*rootChainTask).header = bc.CurrentHeader().(*types.RootBlockHeader)
	assert.NoError(t, rt.Run(bc))
	// Happy path.
	rt.(*rootChainTask).header = rbChain[4].Header()
	v := &mockvalidator{}
	bc.(*mockblockchain).validator = v
	p.retHeaders, p.retBlocks = reverserHeaders(rhChain), reverserBlocks(rbChain)
	assert.NoError(t, rt.Run(bc))
	// Confirm 5 more blocks are successfully added to existing 5-block chain.
	assert.Equal(t, uint64(10), bc.CurrentHeader().NumberU64())

	// Rollback and test unhappy path.
	rbc.SetHead(5)
	assert.Equal(t, rhChain[0].ParentHash, bc.CurrentHeader().Hash())
	// Get errors when downloading headers.
	rt.(*rootChainTask).header = rbChain[4].Header()
	p.downloadHeaderError = errors.New("download error")
	assert.Error(t, rt.Run(bc))
	// Downloading headers succeeds, but block validation failed.
	p.downloadHeaderError = nil
	v.err = errors.New("validate error")
	assert.Error(t, rt.Run(bc))
	// Block validation succeeds, but block header list not correct.
	v.err = nil
	wrongHeaders := reverserHeaders(rhChain)
	rand.Shuffle(len(wrongHeaders), func(i, j int) {
		wrongHeaders[i], wrongHeaders[j] = wrongHeaders[j], wrongHeaders[i]
	})
	p.retHeaders = wrongHeaders
	assert.Error(t, rt.Run(bc))
	// Validation succeeds. Should be downloading actual blocks. Mock some errors.
	p.retHeaders = reverserHeaders(rhChain)
	p.downloadBlockError = errors.New("download error")
	assert.Error(t, rt.Run(bc))
	// Downloading blocks succeeds. But make the returned blocks miss one. Insertion should fail.
	p.downloadBlockError = nil
	missing := p.retBlocks[len(p.retBlocks)-1]
	p.retBlocks = p.retBlocks[0 : len(p.retBlocks)-1]
	assert.Error(t, rt.Run(bc))
	// Add back that missing block. Happy again.
	p.retBlocks = append(p.retBlocks, missing)
	assert.NoError(t, rt.Run(bc))
	assert.Equal(t, uint64(10), bc.CurrentHeader().NumberU64())

	// Sync older forks. Starting from block 6, up to 11.
	rbChain, rhChain = makeChains(rbChain[0], true)
	for _, rh := range rhChain {
		assert.False(t, bc.HasBlock(rh.Hash()))
	}
	rt.(*rootChainTask).header = rbChain[4].Header()
	p.retHeaders, p.retBlocks = reverserHeaders(rhChain), reverserBlocks(rbChain)
	assert.NoError(t, rt.Run(bc))
	for _, rh := range rhChain {
		assert.True(t, bc.HasBlock(rh.Hash()))
	}
	// Tip should be updated.
	assert.Equal(t, uint64(11), bc.CurrentHeader().NumberU64())
}

/*
 Test helpers.
*/

func makeChains(parent *types.RootBlock, random bool) ([]*types.RootBlock, []*types.RootBlockHeader) {
	var gen func(i int, b *core.RootBlockGen)
	if random {
		gen = func(i int, b *core.RootBlockGen) {
			b.SetExtra([]byte{byte(i)})
		}
	}
	blockchain := core.GenerateRootBlockChain(parent, engine, 5, gen)
	var headerchain []*types.RootBlockHeader
	for _, rb := range blockchain {
		headerchain = append(headerchain, rb.Header())
	}

	return blockchain, headerchain
}

func reverserBlocks(ls []*types.RootBlock) []*types.RootBlock {
	ret := make([]*types.RootBlock, len(ls), len(ls))
	copy(ret, ls)
	for left, right := 0, len(ls)-1; left < right; left, right = left+1, right-1 {
		ret[left], ret[right] = ret[right], ret[left]
	}
	return ret
}

func reverserHeaders(ls []*types.RootBlockHeader) []*types.RootBlockHeader {
	ret := make([]*types.RootBlockHeader, len(ls), len(ls))
	copy(ret, ls)
	for left, right := 0, len(ls)-1; left < right; left, right = left+1, right-1 {
		ret[left], ret[right] = ret[right], ret[left]
	}
	return ret
}
