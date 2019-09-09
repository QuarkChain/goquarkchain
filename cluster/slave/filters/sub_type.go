package filters

import (
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/event"
)

type subackend interface {
	getch() error
	freech()
}

type subTxsEvent struct {
	ch        chan core.NewTxsEvent
	sub       event.Subscription
	broadcast func(interface{})
}

func (s *subTxsEvent) getch() error {
	select {
	case ev := <-s.ch:
		s.broadcast(ev.Txs)
		return nil
	case err := <-s.sub.Err():
		return err
	default:
		return nil
	}
}

func (s *subTxsEvent) freech() {
	s.sub.Unsubscribe()
}

type subLogsEvent struct {
	ch        chan []*types.Log
	sub       event.Subscription
	broadcast func(interface{})
}

func (s *subLogsEvent) getch() error {
	select {
	case ev := <-s.ch:
		s.broadcast(ev)
		return nil
	case err := <-s.sub.Err():
		return err
	default:
		return nil
	}
}

func (s *subLogsEvent) freech() {
	if err := s.getch(); err == nil {
		s.sub.Unsubscribe()
	}
}

type subMinorBlockHeadersEvent struct {
	ch        chan core.MinorChainHeadEvent
	sub       event.Subscription
	broadcast func(interface{})
}

func (s *subMinorBlockHeadersEvent) getch() error {
	select {
	case ev := <-s.ch:
		s.broadcast(ev.Block.Header())
		return nil
	case err := <-s.sub.Err():
		return err
	default:
		return nil
	}
}

func (s *subMinorBlockHeadersEvent) freech() {
	if err := s.getch(); err == nil {
		s.sub.Unsubscribe()
	}
}

func (s *subscribe) newSubEvent(shrd ShardBackend, tp Type, broadcast func(interface{})) subackend {
	switch tp {
	case LogsSubscription:
		logsCh := make(chan []*types.Log, logsChanSize)
		logsSub := shrd.SubscribeLogsEvent(logsCh)
		return &subLogsEvent{
			ch:        logsCh,
			sub:       logsSub,
			broadcast: broadcast,
		}
	case PendingTransactionsSubscription:
		txsCh := make(chan core.NewTxsEvent, txChanSize)
		txsSub := shrd.SubscribeNewTxsEvent(txsCh)
		return &subTxsEvent{
			ch:        txsCh,
			sub:       txsSub,
			broadcast: broadcast,
		}
	case BlocksSubscription:
		headersCh := make(chan core.MinorChainHeadEvent, chainEvChanSize)
		headersSub := shrd.SubscribeChainHeadEvent(headersCh)
		return &subMinorBlockHeadersEvent{
			ch:        headersCh,
			sub:       headersSub,
			broadcast: broadcast,
		}
	}
	return nil
}
