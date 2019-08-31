package sync

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/p2p"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
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

func (p *mockpeer) GetRootBlockHeaderList(request *p2p.GetRootBlockHeaderListWithSkipRequest) (*p2p.GetRootBlockHeaderListResponse, error) {
	if p.downloadHeaderError != nil {
		return nil, p.downloadHeaderError
	}

	if request.Limit <= 0 || request.Limit > 2*RootBlockHeaderListLimit {
		return nil, errors.New("Bad limit ")
	}
	if request.Direction != qcom.DirectionToGenesis && request.Direction != qcom.DirectionToTip {
		return nil, errors.New("Bad direction ")
	}

	// May return a subset.
	var (
		sign   = 0
		hash   = request.GetHash()
		height = request.GetHeight()
	)
	if hash == (common.Hash{}) && height == nil {
		return nil, errors.New("useless hash and height")
	}

	rBHeaders := make([]*types.RootBlockHeader, 0, request.Limit)
	for i, hd := range p.retRHeaders {
		if hd.Hash() == hash || hd.Number == *height {
			sign = i
			break
		}
	}

	direction := int(request.Skip + 1)
	if request.Direction == qcom.DirectionToGenesis {
		direction = 0 - direction
	}

	for ; sign >= 0 && sign < len(p.retRHeaders) && len(rBHeaders) < cap(rBHeaders); sign += direction {
		rBHeaders = append(rBHeaders, p.retRHeaders[sign])
	}

	// make root block headers be ordered
	sort.Slice(rBHeaders, func(i, j int) bool {
		return rBHeaders[i].Number < rBHeaders[j].Number
	})

	return &p2p.GetRootBlockHeaderListResponse{
		RootTip:         p.retRHeaders[len(p.retRHeaders)-1],
		BlockHeaderList: rBHeaders,
	}, nil
}

func (p *mockpeer) GetRootBlockList(hashes []common.Hash) ([]*types.RootBlock, error) {
	if p.downloadBlockError != nil {
		return nil, p.downloadBlockError
	}

	if len(hashes) > 2*RootBlockBatchSize {
		return nil, fmt.Errorf("len of RootBlockHashList is larger than expected, limit: %d, want: %d", len(hashes), RootBlockBatchSize)
	}
	rBlocks := make([]*types.RootBlock, 0, len(hashes))
	sign := 0
	for _, rhash := range hashes {
		for ; sign < len(p.retRBlocks); sign++ {
			if rhash == p.retRBlocks[sign].Hash() {
				rBlocks = append(rBlocks, p.retRBlocks[sign])
				break
			}
		}
	}
	return rBlocks, nil
}

func (p *mockpeer) PeerID() string {
	return p.name
}

func (p *mockpeer) GetRootBlockHeaderListWithSkip(tp uint8, data common.Hash, limit, skip uint32,
	direction uint8) (*p2p.GetRootBlockHeaderListResponse, error) {
	return &p2p.GetRootBlockHeaderListResponse{}, nil
}

func (p *mockpeer) RootHead() *types.RootBlockHeader {
	return p.retRHeaders[len(p.retRHeaders)-1]
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
	_, err := bc.mbc.InsertChain([]types.IBlock{block}, nil)
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

func (bc *mockblockchain) AddValidatedMinorBlockHeader(hash common.Hash, coinbaseToken *types.TokenBalances) {
	bc.rbc.AddValidatedMinorBlockHeader(hash, coinbaseToken)
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

func (v *mockvalidator) ValidateBlock(types.IBlock, bool) error {
	return v.err
}
func (v *mockvalidator) ValidateSeal(mHeader types.IHeader) error {
	return v.err
}

func newRootBlockChain(sz int) blockchain {
	qkcconfig.SkipRootCoinbaseCheck = true
	db := ethdb.NewMemDatabase()
	genesisBlock := genesis.MustCommitRootBlock(db)
	blockchain, err := core.NewRootBlockChain(db, qkcconfig, engine, nil)
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

func TestRootChainTask(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc := newRootBlockChain(10)
	v := &mockvalidator{}
	bc.(*mockblockchain).validator = v
	rbc := bc.(*mockblockchain).rbc

	retRBlocks, retRHeaders := makeRootChains(rbc.GetBlockByNumber(0).(*types.RootBlock), 20, false)

	// No error if already have the target block.
	var rt = NewRootChainTask(p, bc.CurrentHeader().(*types.RootBlockHeader), &BlockSychronizerStats{}, nil, nil)
	rTask := rt.(*rootChainTask)
	assert.NoError(t, rt.Run(bc))

	// Happy path.
	p.retRBlocks, p.retRHeaders = retRBlocks, retRHeaders
	rTask.header = retRHeaders[4]
	// Confirm 5 more blocks are successfully added to existing 5-block chain.
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(10))

	// Rollback and test unhappy path.
	rbc.SetHead(5)
	assert.Equal(t, bc.CurrentHeader().NumberU64(), retRHeaders[5].NumberU64())

	// Get errors when downloading headers.
	rTask.header = retRHeaders[11]
	p.downloadHeaderError = errors.New("download error")
	assert.Error(t, rt.Run(bc))

	// Downloading headers succeeds, but block validation failed.
	p.downloadHeaderError = nil
	v.err = errors.New("validate error")
	assert.Error(t, rt.Run(bc))

	// Block validation succeeds, but block header list not correct.
	v.err = nil
	wrongHeaders := reverseRHeaders(retRHeaders)
	rand.Shuffle(len(wrongHeaders), func(i, j int) {
		wrongHeaders[i], wrongHeaders[j] = wrongHeaders[j], wrongHeaders[i]
	})
	p.retRHeaders = wrongHeaders
	assert.Error(t, rt.Run(bc))

	// Validation succeeds. Should be downloading actual blocks. Mock some errors.
	p.retRHeaders = retRHeaders
	p.downloadBlockError = errors.New("download error")
	assert.Error(t, rt.Run(bc))

	// Downloading blocks succeeds. But make the returned blocks miss one. Insertion should fail.
	p.downloadBlockError = nil
	missing := p.retRBlocks[len(p.retRBlocks)-1]
	p.retRBlocks = p.retRBlocks[:len(p.retRBlocks)-1]
	assert.Error(t, rt.Run(bc))

	// Add back that missing block. Happy again.
	p.retRBlocks = append(p.retRBlocks, missing)
	assert.NoError(t, rt.Run(bc))
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(20))

	// add maxSyncStaleness root blocks
	rTask.maxSyncStaleness = 1000
	retRBlocks, retRHeaders = makeRootChains(retRBlocks[len(retRBlocks)-1], 1000, false)
	p.retRHeaders = append(p.retRHeaders, retRHeaders[1:]...)
	p.retRBlocks = append(p.retRBlocks, retRBlocks[1:]...)
	rTask.header = retRHeaders[2]
	assert.NoError(t, rt.Run(bc))
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(1000+20))

	// Sync older forks. Starting from block 20, up to 2*maxSyncStaleness.
	retRBlocks, retRHeaders = makeRootChains(retRBlocks[len(retRBlocks)-1], 1000, true)
	for _, rh := range retRHeaders[1:] {
		assert.False(t, bc.HasBlock(rh.Hash()))
	}

	rt.(*rootChainTask).header = retRHeaders[4]
	p.retRHeaders, p.retRBlocks = append(p.retRHeaders[20:], retRHeaders...), append(p.retRBlocks[20:], retRBlocks...)
	assert.NoError(t, rt.Run(bc))
	for _, rh := range retRHeaders {
		assert.True(t, bc.HasBlock(rh.Hash()))
	}

	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(2000+20))
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

		AddBlockListForSyncFunc := func(request *rpc.AddBlockListForSyncRequest) (*rpc.ShardStatus, error) {
			for _, header := range block.MinorBlockHeaders() {
				rbc.AddValidatedMinorBlockHeader(header.Hash(), header.CoinbaseAmount)
			}
			return &rpc.ShardStatus{
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
			}, nil
		}

		for _, conn := range shardConns {
			conn.(*mock_master.MockShardConnForP2P).EXPECT().AddBlockListForSync(gomock.Any()).DoAndReturn(AddBlockListForSyncFunc).Times(1)
		}

		var rt = NewRootChainTask(&mockpeer{name: "chunfeng"}, nil, nil, statusChan, func(fullShardId uint32) []rpc.ShardConnForP2P {
			return shardConns
		})
		rTask := rt.(*rootChainTask)
		err := rTask.syncMinorBlocks(bc.(rootblockchain), block)
		assert.NoError(t, err)

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

func makeRootChains(parent *types.RootBlock, height int, random bool) ([]*types.RootBlock, []*types.RootBlockHeader) {
	var gen func(i int, b *core.RootBlockGen)
	if random {
		gen = func(i int, b *core.RootBlockGen) {
			b.SetExtra([]byte{byte(i)})
		}
	}

	var (
		headerchain []*types.RootBlockHeader
		blockchain  []*types.RootBlock
	)
	blockchain = append(blockchain, parent)

	blockchain = append(blockchain, core.GenerateRootBlockChain(parent, engine, height, gen)...)
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
