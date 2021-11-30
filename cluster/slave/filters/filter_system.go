// Modified from go-ethereum under GNU Lesser General Public License
package filters

import (
	"errors"
	"math/big"
	"sync"
	"time"

	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

// Type determines the kind of filter and is used to put the filter in to
// the correct bucket when added.
type Type byte

const (
	// UnknownSubscription indicates an unknown subscription type
	UnknownSubscription Type = iota
	// LogsSubscription queries for new or removed (chain reorg) logs
	LogsSubscription
	// PendingLogsSubscription queries for logs in pending blocks
	PendingLogsSubscription
	// MinedAndPendingLogsSubscription queries for logs in mined and pending blocks.
	MinedAndPendingLogsSubscription
	// PendingTransactionsSubscription queries tx hashes for pending
	// transactions entering the pending state
	PendingTransactionsSubscription
	// BlocksSubscription queries hashes for blocks that are imported
	BlocksSubscription
	// SyncingSubscription queries syncResult when syncing
	SyncingSubscription
	// LastSubscription keeps track of the last index
	LastIndexSubscription
)

const (
	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	TxsChanSize = 5
	// rmLogsChanSize is the size of channel listening to RemovedLogsEvent.
	RmLogsChanSize = 10
	// logsChanSize is the size of channel listening to LogsEvent.
	LogsChanSize = 10
	// chainEvChanSize is the size of channel listening to ChainEvent.
	ChainEvChanSize = 10
	// syncSize is the size of channel listening to SubscribeSyncEvent.
	SyncSize = 5
)

var (
	ErrInvalidSubscriptionID = errors.New("invalid id")
)

type SlaveFilter interface {
	GetTransactionByHash(txHash common.Hash, branch uint32) (*types.MinorBlock, uint32, error)
	GetShardFilter(fullShardId uint32) (ShardFilter, error)
	GetFullShardList() []uint32
}

type ShardFilter interface {
	GetHeaderByNumber(height qrpc.BlockNumber) (*types.MinorBlockHeader, error)
	GetReceiptsByHash(hash common.Hash) (types.Receipts, error)
	GetLogs(hash common.Hash) ([][]*types.Log, error)
	GetMinorBlock(mHash common.Hash, height *uint64) (mBlock *types.MinorBlock, err error)
	AddTransaction(tx *types.Transaction) error
	GetEthChainID() uint32
	GetTransactionByHash(hash common.Hash) (*types.MinorBlock, uint32)

	SubscribeChainHeadEvent(ch chan<- core.MinorChainHeadEvent) event.Subscription
	SubscribeLogsEvent(chan<- core.LoglistEvent) event.Subscription
	SubscribeChainEvent(ch chan<- core.MinorChainEvent) event.Subscription
	SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription
	SubscribeSyncEvent(ch chan<- *qsync.SyncingResult) event.Subscription
}

type subscription struct {
	id          rpc.ID
	typ         Type
	created     time.Time
	fullShardId uint32
	logsCrit    qrpc.FilterQuery
	logsCh      chan core.LoglistEvent
	txlistCh    chan []*types.Transaction
	blocksCh    chan *types.MinorBlock
	syncCh      chan *qsync.SyncingResult
	installed   chan struct{} // closed when the filter is installed
	err         chan error    // closed when the filter is uninstalled
}

// EventSystem creates subscriptions, processes events and broadcasts them to the
// subscription which match the subscription criteria.
type EventSystem struct {
	backend  SlaveFilter
	lastHead *types.MinorBlockHeader

	// Channels
	install    chan *subscription // install filter for event notification
	uninstall  chan *subscription // remove filter for event notification
	subManager *subscribe

	mu sync.RWMutex
	// chainCh   chan *sub       // Channel to receive new chain event
}

// NewEventSystem creates a new manager that listens for event on the given mux,
// parses and filters them. It uses the all map to retrieve filter changes. The
// work loop holds its own index that is used to forward events to filters.
//
// The returned manager has a loop that needs to be stopped with the Stop function
// or by stopping the given mux.
func NewEventSystem(backend SlaveFilter) *EventSystem {
	m := &EventSystem{
		backend:    backend,
		install:    make(chan *subscription),
		uninstall:  make(chan *subscription),
		subManager: NewSubScribe(backend),
	}

	go m.loop()
	return m
}

// Subscription is created when the client registers itself for a particular event.
type Subscription struct {
	ID        rpc.ID
	f         *subscription
	es        *EventSystem
	unsubOnce sync.Once
}

// Err returns a channel that is closed when unsubscribed.
func (sub *Subscription) Err() <-chan error {
	return sub.f.err
}

// Unsubscribe uninstalls the subscription from the event broadcast loop.
func (sub *Subscription) Unsubscribe() {
	sub.unsubOnce.Do(func() {
	uninstallLoop:
		for {
			// write uninstall request and consume logs/hashes. This prevents
			// the eventLoop broadcast method to deadlock when writing to the
			// filter event channel while the subscription loop is waiting for
			// this method to return (and thus not reading these events).
			select {
			case sub.es.uninstall <- sub.f:
				break uninstallLoop
			default:
				if sub.f.logsCh != nil {
					<-sub.f.logsCh
				} else if sub.f.blocksCh != nil {
					<-sub.f.blocksCh
				} else if sub.f.txlistCh != nil {
					<-sub.f.txlistCh
				} else if sub.f.syncCh != nil {
					<-sub.f.syncCh
				}
			}
		}

		// wait for filter to be uninstalled in work loop before returning
		// this ensures that the manager won't use the event channel which
		// will probably be closed by the client asap after this method returns.
		<-sub.Err()
	})
}

// subscribe installs the subscription in the event broadcast loop.
func (es *EventSystem) subscribe(sub *subscription) *Subscription {
	es.install <- sub
	<-sub.installed
	return &Subscription{ID: sub.id, f: sub, es: es}
}

// SubscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel. Default value for the from and to
// block is "latest". If the fromBlock > toBlock an error is returned.
func (es *EventSystem) SubscribeLogs(crit qrpc.FilterQuery, logs chan core.LoglistEvent) (*Subscription, error) {
	if crit.FromBlock == nil {
		crit.FromBlock = new(big.Int).SetInt64(qrpc.LatestBlockNumber.Int64())
	}
	if crit.ToBlock == nil {
		crit.ToBlock = new(big.Int).SetInt64(qrpc.LatestBlockNumber.Int64())
	}
	return es.subscribeLogs(crit, logs), nil
}

// subscribeMinedPendingLogs creates a subscription that returned mined and
// pending logs that match the given criteria.
func (es *EventSystem) subscribeMinedPendingLogs(crit qrpc.FilterQuery, logs chan core.LoglistEvent) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       MinedAndPendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logsCh:    logs,
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// subscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel.
func (es *EventSystem) subscribeLogs(crit qrpc.FilterQuery, logs chan core.LoglistEvent) *Subscription {
	sub := &subscription{
		id:          rpc.NewID(),
		fullShardId: crit.FullShardId,
		typ:         LogsSubscription,
		logsCrit:    crit,
		created:     time.Now(),
		logsCh:      logs,
		installed:   make(chan struct{}),
		err:         make(chan error),
	}
	return es.subscribe(sub)
}

// subscribePendingLogs creates a subscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) subscribePendingLogs(crit qrpc.FilterQuery, logs chan core.LoglistEvent) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logsCh:    logs,
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribeNewHeads creates a subscription that writes the header of a block that is
// imported in the chain.
func (es *EventSystem) SubscribeNewHeads(blocks chan *types.MinorBlock, fullShardId uint32) *Subscription {
	sub := &subscription{
		id:          rpc.NewID(),
		fullShardId: fullShardId,
		typ:         BlocksSubscription,
		created:     time.Now(),
		blocksCh:    blocks,
		installed:   make(chan struct{}),
		err:         make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribePendingTxs creates a subscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) SubscribePendingTxs(txs chan []*types.Transaction, fullShardId uint32) *Subscription {
	sub := &subscription{
		id:          rpc.NewID(),
		fullShardId: fullShardId,
		typ:         PendingTransactionsSubscription,
		created:     time.Now(),
		txlistCh:    txs,
		installed:   make(chan struct{}),
		err:         make(chan error),
	}
	return es.subscribe(sub)
}

func (es *EventSystem) SubscribeSyncing(status chan *qsync.SyncingResult, fullShardId uint32) *Subscription {
	sub := &subscription{
		id:          rpc.NewID(),
		fullShardId: fullShardId,
		typ:         SyncingSubscription,
		created:     time.Now(),
		syncCh:      status,
		installed:   make(chan struct{}),
		err:         make(chan error),
	}
	return es.subscribe(sub)
}

type filterIndex map[Type]map[rpc.ID]*subscription

// broadcast event to filters that match criteria.
func (es *EventSystem) broadcast(filters filterIndex, ev interface{}) {
	if ev == nil {
		return
	}
	es.mu.RLock()
	defer es.mu.RUnlock()

	switch e := ev.(type) {
	case core.LoglistEvent:
		if len(e.Logs) > 0 {
			for _, f := range filters[LogsSubscription] {
				var loglist = make([][]*types.Log, 0, len(e.Logs))
				for _, logs := range e.Logs {
					if matchedLogs := core.FilterLogs(logs, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
						loglist = append(loglist, matchedLogs)
					}
				}
				if len(loglist) > 0 {
					f.logsCh <- core.LoglistEvent{Logs: loglist, IsRemoved: e.IsRemoved}
				}
			}
		}

	case []*types.Transaction:
		for _, f := range filters[PendingTransactionsSubscription] {
			f.txlistCh <- e
		}

	case *types.MinorBlock:
		for _, f := range filters[BlocksSubscription] {
			f.blocksCh <- e
		}

	case *qsync.SyncingResult:
		for _, f := range filters[SyncingSubscription] {
			f.syncCh <- e
		}
	}
}

type slavefilterIndex map[uint32]filterIndex

// eventLoop (un)installs filters and processes mux events.
func (es *EventSystem) loop() {
	// Ensure all subscriptions get cleaned up
	defer func() {
		es.subManager.Stop()
	}()

	index := make(slavefilterIndex)
	for _, id := range es.backend.GetFullShardList() {
		filter := make(filterIndex)
		for i := UnknownSubscription + 1; i < LastIndexSubscription; i++ {
			filter[i] = make(map[rpc.ID]*subscription)
		}
		index[id] = filter
	}

	for {
		select {
		case f := <-es.install:
			if idx, ok := index[f.fullShardId]; ok {
				if _, ok := idx[f.typ]; ok {
					es.mu.Lock()
					idx[f.typ][f.id] = f
					es.mu.Unlock()
					es.subManager.Subscribe(f.fullShardId, f.typ, func(val interface{}) {
						es.broadcast(idx, val)
					})
				}
			}
			close(f.installed)

		case f := <-es.uninstall:
			if idx, ok := index[f.fullShardId]; ok {
				es.mu.Lock()
				delete(idx[f.typ], f.id)
				es.mu.Unlock()
				if len(idx[f.typ]) == 0 {
					es.subManager.Unsubscribe(f.fullShardId, f.typ)
				}
			}
			close(f.err)
		}
	}
}
