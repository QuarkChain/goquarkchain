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
		s.broadcast(ev.Block)
		return nil
	case err := <-s.sub.Err():
		return err
	}
}

func (s *subMinorBlockHeadersEvent) freech() {
	if err := s.getch(); err == nil {
		s.sub.Unsubscribe()
	}
}
