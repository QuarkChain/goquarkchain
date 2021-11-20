// Modified from go-ethereum under GNU Lesser General Public License
package filters

import (
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/ethereum/go-ethereum/event"
)

type subackend interface {
	getch() error
	freech()
}

type subBaseEvent struct {
	sub       event.Subscription
	broadcast func(interface{})
}

func (s subBaseEvent) freech() {
	s.sub.Unsubscribe()
}

type subTxsEvent struct {
	ch chan core.NewTxsEvent
	subBaseEvent
}

func (s *subTxsEvent) getch() error {
	for {
		select {
		case ev := <-s.ch:
			s.broadcast(ev.Txs)
		case err := <-s.sub.Err():
			return err
		default:
			return nil
		}
	}
}

type subLogsEvent struct {
	ch chan core.LoglistEvent
	subBaseEvent
}

func (s *subLogsEvent) getch() error {
	for {
		select {
		case ev := <-s.ch:
			s.broadcast(ev)
		case err := <-s.sub.Err():
			return err
		default:
			return nil
		}
	}
}

type subMinorBlocksEvent struct {
	ch chan core.MinorChainHeadEvent
	subBaseEvent
}

func (s *subMinorBlocksEvent) getch() error {
	for {
		select {
		case ev := <-s.ch:
			s.broadcast(ev.Block)
		case err := <-s.sub.Err():
			return err
		default:
			return nil
		}
	}
}

type subSyncingEvent struct {
	ch chan *qsync.SyncingResult
	subBaseEvent
}

func (s *subSyncingEvent) getch() error {
	for {
		select {
		case ev := <-s.ch:
			s.broadcast(ev)
		case err := <-s.sub.Err():
			return err
		default:
			return nil
		}
	}
}

func (s *subscribe) newSubEvent(shrd ShardFilter, tp Type, broadcast func(interface{})) subackend {
	switch tp {
	case LogsSubscription:
		logsCh := make(chan core.LoglistEvent, LogsChanSize)
		sub := shrd.SubscribeLogsEvent(logsCh)
		return &subLogsEvent{
			ch: logsCh,
			subBaseEvent: subBaseEvent{
				sub:       sub,
				broadcast: broadcast,
			},
		}
	case PendingTransactionsSubscription:
		txsCh := make(chan core.NewTxsEvent, TxsChanSize)
		sub := shrd.SubscribeNewTxsEvent(txsCh)
		return &subTxsEvent{
			ch: txsCh,
			subBaseEvent: subBaseEvent{
				sub:       sub,
				broadcast: broadcast,
			},
		}
	case BlocksSubscription:
		blocksCh := make(chan core.MinorChainHeadEvent, ChainEvChanSize)
		sub := shrd.SubscribeChainHeadEvent(blocksCh)
		return &subMinorBlocksEvent{
			ch: blocksCh,
			subBaseEvent: subBaseEvent{
				sub:       sub,
				broadcast: broadcast,
			},
		}
	case SyncingSubscription:
		syncCh := make(chan *qsync.SyncingResult, SyncSize)
		sub := shrd.SubscribeSyncEvent(syncCh)
		return &subSyncingEvent{
			ch: syncCh,
			subBaseEvent: subBaseEvent{
				sub:       sub,
				broadcast: broadcast,
			},
		}
	}
	return nil
}
