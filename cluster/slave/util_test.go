package slave

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/slave/filters"
	"github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"net"
	"time"
)

type testBackend struct {
	db         ethdb.Database
	endpoint   string
	curShardId uint32
	config     *config.ClusterConfig
	mGenesis   *types.MinorBlock

	handler  *qrpc.Server
	listener net.Listener

	txFeed        event.Feed
	rmLogsFeed    event.Feed
	logsFeed      event.Feed
	chainFeed     event.Feed
	chainHeadFeed event.Feed
	syncFeed      event.Feed

	exitCh chan struct{}
}

func newTestBackend() (*testBackend, error) {
	bak := &testBackend{
		db:       ethdb.NewMemDatabase(),
		endpoint: "ws://127.0.0.1:38191",
		config:   config.NewClusterConfig(),
		handler:  qrpc.NewServer(),
		exitCh:   make(chan struct{}),
	}
	bak.curShardId = bak.config.Quarkchain.GetGenesisShardIds()[0]
	genesis := core.NewGenesis(bak.config.Quarkchain)
	bak.mGenesis = genesis.MustCommitMinorBlock(bak.db, genesis.CreateRootBlock(), bak.curShardId)

	bak.config.Quarkchain.SkipRootCoinbaseCheck = true
	bak.config.Quarkchain.SkipMinorDifficultyCheck = true
	bak.config.Quarkchain.SkipRootCoinbaseCheck = true

	err := bak.handler.RegisterName("qkc", NewPublicFilterAPI(bak))
	if err != nil {
		return nil, err
	}

	bak.listener, err = net.Listen("tcp", bak.endpoint[5:])
	if err != nil {
		return nil, err
	}

	go qrpc.NewWSServer([]string{"*"}, bak.handler).Serve(bak.listener)

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
	b.txFeed.Send(core.NewTxsEvent{Txs: txs})
}

func (b *testBackend) subscribeEvent(method string, channel interface{}) (err error) {
	client, err := qrpc.Dial(b.endpoint)
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
			sub, err := client.Subscribe(ctx, "qkc", channel, method, hexutil.EncodeUint64(uint64(b.curShardId)))
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

func (b *testBackend) GetShardBackend(fullShardId uint32) (filters.ShardFilter, error) {
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

func (b *testBackend) SubscribeChainHeadEvent(ch chan<- core.MinorChainHeadEvent) event.Subscription {
	return b.chainHeadFeed.Subscribe(ch)
}

func (b *testBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
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
