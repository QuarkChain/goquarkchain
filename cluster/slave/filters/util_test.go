package filters

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"net"
	"time"
)

type testBackend struct {
	db         ethdb.Database
	endpoint   string
	curShardId uint32
	config     *config.ClusterConfig
	mGenesis   *types.MinorBlock

	handler  *ethrpc.Server
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
		handler:  ethrpc.NewServer(),
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

	go ethrpc.NewWSServer([]string{"*"}, bak.handler).Serve(bak.listener)

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
		b.chainHeadFeed.Send(core.MinorChainHeadEvent{Block: blk})
	}
	return mBlocks, nil
}

func (b *testBackend) subscribeTxs(method string, subch chan *types.Transaction) error {

	client, err := ethrpc.Dial(b.endpoint)
	if err != nil {
		return err
	}

	var sub *ethrpc.ClientSubscription
	subscribeTxs := func(client *ethrpc.Client, method string, subch chan *types.Transaction) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sub, err = client.Subscribe(ctx, "ws", subch, method, "0x10001")
		if err != nil {
			return err
		}
		return nil
	}

	ticker := time.NewTicker(2 * time.Second)
	go func() {
		select {
		case <-b.exitCh:
			sub.Unsubscribe()
			return
		case <-sub.Err():
			sub.Unsubscribe()
			return
		case <-ticker.C:
			if err = subscribeTxs(client, method, subch); err != nil {
				sub.Unsubscribe()
				return
			}
		}
	}()
	return nil
}

func (b *testBackend) subscribeMinorHeaders(method string, subch chan *types.MinorBlockHeader) error {

	client, err := ethrpc.Dial(b.endpoint)
	if err != nil {
		return err
	}

	var sub *ethrpc.ClientSubscription
	subscribeTxs := func(client *ethrpc.Client, method string, subch chan *types.MinorBlockHeader) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sub, err = client.Subscribe(ctx, "qkc", subch, method, hexutil.EncodeUint64(uint64(b.curShardId)))
		return err
	}

	if err = subscribeTxs(client, method, subch); err != nil {
		return err
	}
	ticker := time.NewTicker(2 * time.Second)
	go func() {
		select {
		case <-b.exitCh:
			sub.Unsubscribe()
			return
		case err = <-sub.Err():
			sub.Unsubscribe()
			return
		case <-ticker.C:
			if err = subscribeTxs(client, method, subch); err != nil {
				sub.Unsubscribe()
				return
			}
		}
	}()
	return err
}

func (b *testBackend) GetShardBackend(fullShardId uint32) (ShardBackend, error) {
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
