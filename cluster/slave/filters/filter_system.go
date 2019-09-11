// Modified from go-ethereum under GNU Lesser General Public License
package filters

import (
	"errors"
	"fmt"
	"sync"
	"time"

	qrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core/types"
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
	txChanSize = 4096
	// rmLogsChanSize is the size of channel listening to RemovedLogsEvent.
	rmLogsChanSize = 10
	// logsChanSize is the size of channel listening to LogsEvent.
	logsChanSize = 10
	// chainEvChanSize is the size of channel listening to ChainEvent.
	chainEvChanSize = 10
	// syncSize is the size of channel listening to SubscribeSyncEvent.
	syncSize = 5
)

var (
	ErrInvalidSubscriptionID = errors.New("invalid id")
)

type subscription struct {
	id          rpc.ID
	typ         Type
	created     time.Time
	fullShardId uint32
	logsCrit    qrpc.FilterQuery
	logs        chan []*types.Log
	txlist      chan *types.Transaction
	headers     chan *types.MinorBlockHeader
	syncCh      chan *qsync.SyncingResult
	installed   chan struct{} // closed when the filter is installed
	err         chan error    // closed when the filter is uninstalled
}

// EventSystem creates subscriptions, processes events and broadcasts them to the
// subscription which match the subscription criteria.
type EventSystem struct {
	backend  SlaveBackend
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
func NewEventSystem(backend SlaveBackend) *EventSystem {
	m := &EventSystem{
		backend:    backend,
		install:    make(chan *subscription),
		uninstall:  make(chan *subscription),
		subManager: NewSubScribe(backend),
	}

	go m.eventLoop()
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
				if sub.f.logs != nil {
					<-sub.f.logs
				} else if sub.f.headers != nil {
					<-sub.f.headers
				} else if sub.f.txlist != nil {
					<-sub.f.txlist
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
func (es *EventSystem) SubscribeLogs(crit qrpc.FilterQuery, logs chan []*types.Log) (*Subscription, error) {
	var from, to qrpc.BlockNumber
	if crit.FromBlock == nil {
		from = qrpc.LatestBlockNumber
	} else {
		from = qrpc.BlockNumber(crit.FromBlock.Int64())
	}
	if crit.ToBlock == nil {
		to = qrpc.LatestBlockNumber
	} else {
		to = qrpc.BlockNumber(crit.ToBlock.Int64())
	}

	// only interested in new mined logs
	if from == qrpc.LatestBlockNumber && to == qrpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
	// only interested in mined logs within a specific block range
	if from >= 0 && to >= 0 && to >= from {
		return es.subscribeLogs(crit, logs), nil
	}
	// interested in logs from a specific block number to new mined blocks
	if from >= 0 && to == qrpc.LatestBlockNumber {
		return es.subscribeLogs(crit, logs), nil
	}
	return nil, fmt.Errorf("invalid from and to block combination: from > to")
}

// subscribeMinedPendingLogs creates a subscription that returned mined and
// pending logs that match the given criteria.
func (es *EventSystem) subscribeMinedPendingLogs(crit qrpc.FilterQuery, logs chan []*types.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       MinedAndPendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// subscribeLogs creates a subscription that will write all logs matching the
// given criteria to the given logs channel.
func (es *EventSystem) subscribeLogs(crit qrpc.FilterQuery, logs chan []*types.Log) *Subscription {
	sub := &subscription{
		id:          rpc.NewID(),
		fullShardId: crit.FullShardId,
		typ:         LogsSubscription,
		logsCrit:    crit,
		created:     time.Now(),
		logs:        logs,
		installed:   make(chan struct{}),
		err:         make(chan error),
	}
	return es.subscribe(sub)
}

// subscribePendingLogs creates a subscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) subscribePendingLogs(crit qrpc.FilterQuery, logs chan []*types.Log) *Subscription {
	sub := &subscription{
		id:        rpc.NewID(),
		typ:       PendingLogsSubscription,
		logsCrit:  crit,
		created:   time.Now(),
		logs:      logs,
		installed: make(chan struct{}),
		err:       make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribeNewHeads creates a subscription that writes the header of a block that is
// imported in the chain.
func (es *EventSystem) SubscribeNewHeads(headers chan *types.MinorBlockHeader, fullShardId uint32) *Subscription {
	sub := &subscription{
		id:          rpc.NewID(),
		fullShardId: fullShardId,
		typ:         BlocksSubscription,
		created:     time.Now(),
		headers:     headers,
		installed:   make(chan struct{}),
		err:         make(chan error),
	}
	return es.subscribe(sub)
}

// SubscribePendingTxs creates a subscription that writes transaction hashes for
// transactions that enter the transaction pool.
func (es *EventSystem) SubscribePendingTxs(txs chan *types.Transaction, fullShardId uint32) *Subscription {
	sub := &subscription{
		id:          rpc.NewID(),
		fullShardId: fullShardId,
		typ:         PendingTransactionsSubscription,
		created:     time.Now(),
		txlist:      txs,
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
	case []*types.Log:
		if len(e) > 0 {
			for _, f := range filters[LogsSubscription] {
				if matchedLogs := filterLogs(e, f.logsCrit.FromBlock, f.logsCrit.ToBlock, f.logsCrit.Addresses, f.logsCrit.Topics); len(matchedLogs) > 0 {
					f.logs <- matchedLogs
				}
			}
		}

	case []*types.Transaction:
		for _, tx := range e {
			for _, f := range filters[PendingTransactionsSubscription] {
				f.txlist <- tx
			}
		}

	case *types.MinorBlockHeader:
		for _, f := range filters[BlocksSubscription] {
			f.headers <- e
		}

	case *qsync.SyncingResult:
		for _, f := range filters[SyncingSubscription] {
			f.syncCh <- e
		}
	}
}

type slavefilterIndex map[uint32]filterIndex

// eventLoop (un)installs filters and processes mux events.
func (es *EventSystem) eventLoop() {
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
