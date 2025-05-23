package slave

import (
	"context"
	"net"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/slave/filters"
	"github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/rpc"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
)

type testBackend struct {
	db         ethdb.Database
	endpoint   string
	curShardId uint32
	config     *config.ClusterConfig
	mGenesis   *types.MinorBlock

	handler  *rpc.Server
	listener net.Listener

	txFeed        event.Feed
	rmLogsFeed    event.Feed
	logsFeed      event.Feed
	chainFeed     event.Feed
	chainHeadFeed event.Feed
	syncFeed      event.Feed

	mBlock *types.MinorBlock

	exitCh chan struct{}
}

func newTestBackend() (*testBackend, error) {
	bak := &testBackend{
		db:       ethdb.NewMemDatabase(),
		endpoint: "ws://127.0.0.1:38191",
		config:   config.NewClusterConfig(),
		handler:  rpc.NewServer(),
		exitCh:   make(chan struct{}),
	}
	bak.curShardId = bak.config.Quarkchain.GetGenesisShardIds()[0]
	genesis := core.NewGenesis(bak.config.Quarkchain)
	bak.mGenesis = genesis.MustCommitMinorBlock(bak.db, genesis.CreateRootBlock(), bak.curShardId)

	bak.config.Quarkchain.SkipRootCoinbaseCheck = true
	bak.config.Quarkchain.SkipMinorDifficultyCheck = true
	bak.config.Quarkchain.SkipRootCoinbaseCheck = true

	err := bak.handler.RegisterName("qkc", NewPublicFilterAPI(bak, 1))
	if err != nil {
		return nil, err
	}

	bak.listener, err = net.Listen("tcp", bak.endpoint[5:])
	if err != nil {
		return nil, err
	}

	go rpc.NewWSServer([]string{"*"}, bak.handler).Serve(bak.listener)

	return bak, nil
}

func (t *testBackend) stop() {
	close(t.exitCh)
	t.handler.Stop()
	t.listener.Close()
}

func (b *testBackend) cresteMinorBlocks(size int) ([]*types.MinorBlock, error) {
	mBlocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, b.config.Quarkchain, b.mGenesis, new(consensus.FakeEngine), b.db, size, nil)
	for _, blk := range mBlocks {
		blk := blk
		b.chainHeadFeed.Send(core.MinorChainHeadEvent{Block: blk})
	}
	return mBlocks, nil
}

func (b *testBackend) creatSyncing(results []*sync.SyncingResult) {
	for _, re := range results {
		b.syncFeed.Send(re)
	}
}

func (b *testBackend) createTxs(txs []*types.Transaction) {
	mBlocks, _ := core.GenerateMinorBlockChain(params.TestChainConfig, b.config.Quarkchain, b.mGenesis, new(consensus.FakeEngine), b.db, 1, nil)
	b.mBlock = mBlocks[0]
	for _, tx := range txs {
		b.mBlock.AddTx(tx)
	}
	b.txFeed.Send(core.NewTxsEvent{Txs: txs})
}

func (b *testBackend) subscribeEvent(method string, channel interface{}) (err error) {
	client, err := rpc.Dial(b.endpoint)
	if err != nil {
		return err
	}

	stopTicker := time.NewTicker(5 * time.Second)

	go func() {
		select {
		case <-stopTicker.C:
			return

		default:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			sub, err := client.QkcSubscribe(ctx, channel, method, hexutil.EncodeUint64(uint64(b.curShardId)))
			if err != nil {
				return
			}
			err = <-sub.Err()
			if err != nil {
				sub.Unsubscribe()
				return
			}
		}
	}()
	return
}

func (b *testBackend) GetShardFilter(fullShardId uint32) (filters.ShardFilter, error) {
	return b, nil
}

func (b *testBackend) GetFullShardList() []uint32 {
	return []uint32{b.curShardId}
}

func (b *testBackend) GetHeaderByNumber(height rpc.BlockNumber) (*types.MinorBlockHeader, error) {
	panic("not implemented")
}

func (b *testBackend) GetHeaderByHash(blockHash common.Hash) (*types.MinorBlockHeader, error) {
	panic("not implemented")
}

func (b *testBackend) GetReceiptsByHash(hash common.Hash) (types.Receipts, error) {
	panic("not implemented")
}

func (b *testBackend) GetLogs(hash common.Hash) ([][]*types.Log, error) {
	panic("not implemented")
}

func (b *testBackend) GetMinorBlock(mHash common.Hash, height *uint64) (mBlock *types.MinorBlock, err error) {
	panic("not implemented")
}

func (b *testBackend) AddTransaction(tx *types.Transaction) error {
	panic("not implemented")
}

func (b *testBackend) AddTransactionAndBroadcast(tx *types.Transaction) error {
	panic("not implemented")
}

func (b *testBackend) GetEthChainID() uint32 {
	panic("not implemented")
}

func (b *testBackend) GetNetworkId() uint32 {
	panic("not implemented")
}

func (b *testBackend) GetTransactionByHash(txHash common.Hash) (*types.MinorBlock, uint32) {
	txs := b.mBlock.GetTransactions()
	for idx, tx := range txs {
		if tx.Hash() == txHash {
			return b.mBlock, uint32(idx)
		}
	}
	panic("not implemented")
}

func (b *testBackend) GetTransactionCount(address common.Address, blockNrOrHash qrpc.BlockNumberOrHash) (*uint64, error) {
	panic("not implemented")
}

func (s *testBackend) ExecuteTx(tx *types.Transaction, fromAddress *account.Address, height *uint64) ([]byte, error) {
	panic("not implemented")
}

func (s *testBackend) EstimateGas(tx *types.Transaction, fromAddress *account.Address) (uint32, error) {
	panic("not implemented")
}

func (s *testBackend) GasPrice(tokenID uint64) (uint64, error) {
	panic("not implemented")
}

func (s *testBackend) GetCode(recipient account.Recipient, height *uint64) ([]byte, error) {
	panic("not implemented")
}

func (b *testBackend) SubscribeChainHeadEvent(ch chan<- core.MinorChainHeadEvent) event.Subscription {
	return b.chainHeadFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeLogsEvent(ch chan<- core.LoglistEvent) event.Subscription {
	return b.logsFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.rmLogsFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeChainEvent(ch chan<- core.MinorChainEvent) event.Subscription {
	return b.chainFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.txFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeSyncEvent(ch chan<- *sync.SyncingResult) event.Subscription {
	return b.syncFeed.Subscribe(ch)
}
