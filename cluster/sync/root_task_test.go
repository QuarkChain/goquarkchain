package sync

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/mocks/mock_master"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
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
	retRHeaders         []*types.RootBlockHeader  // Order: descending.
	retRBlocks          []*types.RootBlock        // Order: descending.
	retMHeaders         []*types.MinorBlockHeader // Order: descending.
	retMBlocks          []*types.MinorBlock       // Order: descending.
}

func (p *mockpeer) GetRootBlockHeaderList(hash common.Hash, amount uint32, reverse bool) ([]*types.RootBlockHeader, error) {
	if p.downloadHeaderError != nil {
		return nil, p.downloadHeaderError
	}
	// May return a subset.
	for i, h := range p.retRHeaders {
		if h.Hash() == hash {
			ret := p.retRHeaders[i:len(p.retRHeaders)]
			return ret, nil
		}
	}
	panic("lolwut")
}

func (p *mockpeer) GetRootBlockList(hashes []common.Hash) ([]*types.RootBlock, error) {
	if p.downloadBlockError != nil {
		return nil, p.downloadBlockError
	}
	return p.retRBlocks, nil
}

func (p *mockpeer) PeerID() string {
	return p.name
}

// Only one of `rbc` or `mbc` should be initialized.
type mockblockchain struct {
	rbc       *core.RootBlockChain
	mbc       *core.MinorBlockChain
	validator core.Validator
}

func (bc *mockblockchain) HasBlock(hash common.Hash) bool {
	if bc.rbc != nil {
		header := bc.rbc.GetHeader(hash)
		return !(header == nil || reflect.ValueOf(header).IsNil())
	}
	header := bc.mbc.GetHeader(hash)
	return !(header == nil || reflect.ValueOf(header).IsNil())

}

func (bc *mockblockchain) AddBlock(block types.IBlock) error {
	if bc.rbc != nil {
		_, err := bc.rbc.InsertChain([]types.IBlock{block})
		return err
	}
	_, err := bc.mbc.InsertChain([]types.IBlock{block})
	return err
}

func (bc *mockblockchain) CurrentHeader() types.IHeader {
	if bc.rbc != nil {
		return bc.rbc.CurrentHeader()
	}
	return bc.mbc.CurrentHeader()
}

func (bc *mockblockchain) Validator() core.Validator {
	return bc.validator
}

func (bc *mockblockchain) AddValidatedMinorBlockHeader(hash common.Hash) {
	bc.rbc.AddValidatedMinorBlockHeader(hash)
}

func (bc *mockblockchain) IsMinorBlockValidated(hash common.Hash) bool {
	return bc.rbc.IsMinorBlockValidated(hash)
}

type mockvalidator struct {
	err error
}

func (v *mockvalidator) ValidateHeader(types.IHeader) error {
	return v.err
}
func (v *mockvalidator) ValidateState(block, parent types.IBlock, state *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	return v.err
}

func (v *mockvalidator) ValidateBlock(types.IBlock) error {
	return v.err
}

func newRootBlockChain(sz int) blockchain {
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
	bc := newRootBlockChain(5)
	rbc := bc.(*mockblockchain).rbc
	var rt Task = NewRootChainTask(p, nil, nil, nil)

	// Prepare future blocks for downloading.
	rbChain, rhChain := makeRootChains(rbc.CurrentBlock(), false)

	// No error if already have the target block.
	rt.(*rootChainTask).header = bc.CurrentHeader().(*types.RootBlockHeader)
	assert.NoError(t, rt.Run(bc))
	// Happy path.
	rt.(*rootChainTask).header = rbChain[4].Header()
	v := &mockvalidator{}
	bc.(*mockblockchain).validator = v
	p.retRHeaders, p.retRBlocks = reverseRHeaders(rhChain), reverseRBlocks(rbChain)
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
	wrongHeaders := reverseRHeaders(rhChain)
	rand.Shuffle(len(wrongHeaders), func(i, j int) {
		wrongHeaders[i], wrongHeaders[j] = wrongHeaders[j], wrongHeaders[i]
	})
	p.retRHeaders = wrongHeaders
	assert.Error(t, rt.Run(bc))
	// Validation succeeds. Should be downloading actual blocks. Mock some errors.
	p.retRHeaders = reverseRHeaders(rhChain)
	p.downloadBlockError = errors.New("download error")
	assert.Error(t, rt.Run(bc))
	// Downloading blocks succeeds. But make the returned blocks miss one. Insertion should fail.
	p.downloadBlockError = nil
	missing := p.retRBlocks[len(p.retRBlocks)-1]
	p.retRBlocks = p.retRBlocks[0 : len(p.retRBlocks)-1]
	assert.Error(t, rt.Run(bc))
	// Add back that missing block. Happy again.
	p.retRBlocks = append(p.retRBlocks, missing)
	assert.NoError(t, rt.Run(bc))
	assert.Equal(t, uint64(10), bc.CurrentHeader().NumberU64())

	// Sync older forks. Starting from block 6, up to 11.
	rbChain, rhChain = makeRootChains(rbChain[0], true)
	for _, rh := range rhChain {
		assert.False(t, bc.HasBlock(rh.Hash()))
	}
	rt.(*rootChainTask).header = rbChain[4].Header()
	p.retRHeaders, p.retRBlocks = reverseRHeaders(rhChain), reverseRBlocks(rbChain)
	assert.NoError(t, rt.Run(bc))
	for _, rh := range rhChain {
		assert.True(t, bc.HasBlock(rh.Hash()))
	}
	// Tip should be updated.
	assert.Equal(t, uint64(11), bc.CurrentHeader().NumberU64())
}

func TestSyncMinorBlocks(t *testing.T) {
	bc := newRootBlockChain(5)
	rbc := bc.(*mockblockchain).rbc
	statusChan := make(chan *rpc.ShardStatus, 2)
	index := 0
	// Prepare future blocks for downloading.
	var gen = func(i int, b *core.RootBlockGen) {
		for j := 0; j < 3; j++ {
			header := types.MinorBlockHeader{Coinbase: account.Address{Recipient: account.Recipient{byte(index)}}}
			index++
			b.Headers = append(b.Headers, &header)
		}
	}
	blocks := core.GenerateRootBlockChain(rbc.CurrentBlock(), engine, 2, gen)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	shardConns := getShardConnForP2P(1, ctrl)
	for _, block := range blocks {
		for _, header := range block.MinorBlockHeaders() {
			if rbc.IsMinorBlockValidated(header.Hash()) {
				t.Errorf("validated minor block hash in block %d is exist", block.NumberU64())
			}
		}

		for _, conn := range shardConns {
			conn.(*mock_master.MockShardConnForP2P).EXPECT().AddBlockListForSync(gomock.Any()).Return(
				&rpc.ShardStatus{
					Branch:             account.Branch{Value: 0},
					Height:             block.NumberU64(),
					Difficulty:         block.Difficulty(),
					CoinbaseAddress:    block.Coinbase(),
					Timestamp:          block.Time(),
					TotalTxCount:       0,
					TxCount60s:         0,
					PendingTxCount:     0,
					BlockCount60s:      2,
					StaleBlockCount60s: 2,
					LastBlockTime:      block.Time(),
				}, nil).Times(1)
		}

		syncMinorBlocks("", bc.(rootblockchain), block, statusChan, func(fullShardId uint32) []rpc.ShardConnForP2P {
			return shardConns
		})

		select {
		case status := <-statusChan:
			if status.Branch.Value != 0 || status.Height != block.NumberU64() {
				t.Errorf("status result is wrong")
			}
		}

		for _, header := range block.MinorBlockHeaders() {
			if !rbc.IsMinorBlockValidated(header.Hash()) {
				t.Errorf("validated minor block hash in block %d is missing", block.NumberU64())
			}
		}
	}
}

func getShardConnForP2P(n int, ctrl *gomock.Controller) []rpc.ShardConnForP2P {
	shardConns := make([]rpc.ShardConnForP2P, 0, n)
	for i := 0; i < n; i++ {
		sc := mock_master.NewMockShardConnForP2P(ctrl)
		shardConns = append(shardConns, sc)
	}

	return shardConns
}

/*
 Test helpers.
*/

func makeRootChains(parent *types.RootBlock, random bool) ([]*types.RootBlock, []*types.RootBlockHeader) {
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

func reverseRBlocks(ls []*types.RootBlock) []*types.RootBlock {
	ret := make([]*types.RootBlock, len(ls), len(ls))
	copy(ret, ls)
	for left, right := 0, len(ls)-1; left < right; left, right = left+1, right-1 {
		ret[left], ret[right] = ret[right], ret[left]
	}
	return ret
}

func reverseRHeaders(ls []*types.RootBlockHeader) []*types.RootBlockHeader {
	ret := make([]*types.RootBlockHeader, len(ls), len(ls))
	copy(ret, ls)
	for left, right := 0, len(ls)-1; left < right; left, right = left+1, right-1 {
		ret[left], ret[right] = ret[right], ret[left]
	}
	return ret
}
