package filters

import (
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
	"time"
)

type delEvent struct {
	id uint32
	tp Type
}

type addEvent struct {
	delEvent
	broadcast func(interface{})
}

type subscribe struct {
	backend SlaveBackend
	events  map[uint32]map[Type]subackend

	exitCh chan struct{}
	addCh  chan *addEvent
	delCh  chan *delEvent
}

func NewSubScribe(backend SlaveBackend) *subscribe {

	sub := &subscribe{
		backend: backend,
		events:  make(map[uint32]map[Type]subackend),

		exitCh: make(chan struct{}),
		addCh:  make(chan *addEvent, 8),
		delCh:  make(chan *delEvent, 8),
	}

	go sub.eventsloop()
	return sub
}

func (s *subscribe) Subscribe(id uint32, tp Type, broadcast func(interface{})) {
	go func() {
		s.addCh <- &addEvent{
			delEvent:  delEvent{id: id, tp: tp},
			broadcast: broadcast,
		}
	}()
}

func (s *subscribe) Unsubscribe(id uint32, tp Type) {
	go func() { s.delCh <- &delEvent{id: id, tp: tp} }()
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
			if subEv, ok := s.events[del.id]; ok {
				if ev, ok := subEv[del.tp]; ok {
					delete(subEv, del.tp)
					ev.freech()
				}
				if len(subEv) == 0 {
					delete(s.events, del.id)
				}
			}

		case add := <-s.addCh:
			shrd, err := s.backend.GetShardBackend(add.id)
			if err != nil {
				break
			}
			if _, ok := s.events[add.id]; !ok {
				s.events[add.id] = make(map[Type]subackend)
			}
			tpEvent := s.events[add.id]
			if _, ok := tpEvent[add.tp]; !ok {
				sub := s.newSubEvent(shrd, add.tp, add.broadcast)
				if qcom.IsNil(sub) {
					break
				}
				tpEvent[add.tp] = sub
			}

		case <-ticker.C:
			for id := range s.events {
				id := id
				g.Go(func() error {
					for tp, subEv := range s.events[id] {
						if err := subEv.getch(); err != nil {
							s.delCh <- &delEvent{id: id, tp: tp}
							return err
						}
					}
					return nil
				})
			}
			if err := g.Wait(); err != nil {
				log.Error("subscribe event error: ", err)
			}
		}
	}
}
