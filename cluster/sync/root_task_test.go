package sync

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/p2p"
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

func (p *mockpeer) GetRootBlockHeaderList(request *rpc.GetRootBlockHeaderListRequest) (*p2p.GetRootBlockHeaderListResponse, error) {
	if p.downloadHeaderError != nil {
		return nil, p.downloadHeaderError
	}
	if request.Limit <= 0 || request.Limit > 2*RootBlockHeaderListLimit {
		return nil, errors.New("Bad limit ")
	}
	if request.Direction != qcom.DirectionToGenesis && request.Direction != qcom.DirectionToTip {
		return nil, errors.New("Bad direction ")
	}

	if request.Hash == (common.Hash{}) && request.Height == nil {
		return nil, errors.New("Bad params root block hash and height ")
	}

	// May return a subset.
	sign := 0
	rBHeaders := make([]*types.RootBlockHeader, 0, request.Limit)
	for i, hd := range p.retRHeaders {
		if hd.Hash() == request.Hash || hd.Number == *request.Height {
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

func (v *mockvalidator) ValidateBlock(types.IBlock) error {
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

func TestRootChainTaskEmpty(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc := newRootBlockChain(5)
	var rt = NewRootChainTask(p, bc.CurrentHeader().(*types.RootBlockHeader), &RootBlockSychronizerStats{}, nil, nil)
	// No error if already have the target block.
	err := rt.Run(bc)
	assert.NoError(t, err)
}

func TestRootChainTaskBigerThanLimit(t *testing.T) {
	p := &mockpeer{name: "chunfeng"}
	bc := newRootBlockChain(5)
	v := &mockvalidator{}
	bc.(*mockblockchain).validator = v
	rbc := bc.(*mockblockchain).rbc

	p.retRBlocks, p.retRHeaders = makeRootChains(rbc.GetBlockByNumber(0).(*types.RootBlock), 1000, false)

	var stats = &RootBlockSychronizerStats{}
	var rt = NewRootChainTask(p, p.retRHeaders[800], stats, nil, nil)

	err := rt.Run(bc)
	assert.NoError(t, err)
	assert.Equal(t, bc.CurrentHeader().NumberU64(), uint64(p.retRHeaders[len(p.retRHeaders)-1].Number))
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
