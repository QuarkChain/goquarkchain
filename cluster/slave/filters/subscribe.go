package filters

import (
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

type delnode struct {
	fullShardId uint32
	tp          Type
}

type subscribe struct {
	backend SlaveBackend

	events map[uint32]map[Type]subackend

	mu     sync.RWMutex
	exitCh chan struct{}
	delCh  chan *delnode
}

func NewSubScribe(backend SlaveBackend) *subscribe {

	sub := &subscribe{
		backend: backend,

		events: make(map[uint32]map[Type]subackend),

		exitCh: make(chan struct{}),
		delCh:  make(chan *delnode, len(backend.GetFullShardList())),
	}

	go sub.eventsloop()
	return sub
}

func (s *subscribe) Subscribe(fullShardId uint32, tp Type, broadcast func(interface{})) {
	shrd, err := s.backend.GetShardBackend(fullShardId)
	if err != nil {
		return
	}
	if _, ok := s.events[fullShardId]; !ok {
		s.events[fullShardId] = make(map[Type]subackend)
	}
	tpEvent := s.events[fullShardId]
	if _, ok := tpEvent[tp]; ok {
		return
	}

	switch tp {
	case LogsSubscription:
		logsCh := make(chan []*types.Log, logsChanSize)
		logsSub := shrd.MinorBlockChain.SubscribeLogsEvent(logsCh)
		tpEvent[tp] = &subLogsEvent{
			ch:        logsCh,
			sub:       logsSub,
			broadcast: broadcast,
		}
	case PendingTransactionsSubscription:
		panic("not implemented")
		//txsCh := make(chan core.NewTxsEvent, txChanSize)
		//txsSub := shrd.MinorBlockChain.SubscribeTxsEvent(txsCh)
		//tpEvent[tp] = &subTxsEvent{
		//	ch:        txsCh,
		//	sub:       txsSub,
		//	broadcast: broadcast,
		//}
	case BlocksSubscription:
		headersCh := make(chan core.MinorChainHeadEvent, chainEvChanSize)
		headersSub := shrd.MinorBlockChain.SubscribeChainHeadEvent(headersCh)
		tpEvent[tp] = &subMinorBlockHeadersEvent{
			ch:        headersCh,
			sub:       headersSub,
			broadcast: broadcast,
		}
	}
	return
}

func (s *subscribe) Unsubscribe(fullShardId uint32, tp Type) {
	go func() { s.delCh <- &delnode{fullShardId: fullShardId, tp: tp} }()
}

func (s *subscribe) Stop() {
	close(s.exitCh)
}

func (s *subscribe) eventsloop() {
	var (
		g      errgroup.Group
		ticker = time.NewTicker(3 * time.Second)
	)
	defer func() {
		for id := range s.events {
			for tp, subEv := range s.events[id] {
				subEv.freech()
				delete(s.events[id], tp)
			}
			delete(s.events, id)
		}
		ticker.Stop()
	}()
	for {
		select {
		case <-s.exitCh:
			return
		case del := <-s.delCh:
			if subEv, ok := s.events[del.fullShardId]; ok {
				ev := subEv[del.tp]
				if !qcom.IsNil(ev) {
					delete(subEv, del.tp)
					ev.freech()
				}
			}
		case <-ticker.C:
			for fullShardId := range s.events {
				id := fullShardId
				g.Go(func() error {
					for tp, subEv := range s.events[id] {
						if err := subEv.getch(); err != nil {
							s.delCh <- &delnode{fullShardId: id, tp: tp}
							return err
						}
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				// TODO print error
			}
		}
	}
}
