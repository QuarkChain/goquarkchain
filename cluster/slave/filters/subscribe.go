package filters

import (
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"golang.org/x/sync/errgroup"
	"sync"
)

type delnode struct {
	fullShardId uint32
	tp          Type
}

type subscribe struct {
	shardlist map[uint32]*shard.ShardBackend

	events map[uint32]map[Type]subackend

	mu     sync.RWMutex
	exitCh chan struct{}
	delCh  chan *delnode
}

func NewSubScribe(shardlist []*shard.ShardBackend) *subscribe {

	shards := make(map[uint32]*shard.ShardBackend)
	for _, shrd := range shardlist {
		shards[shrd.Config.GetFullShardId()] = shrd
	}

	sub := &subscribe{
		shardlist: shards,

		events: make(map[uint32]map[Type]subackend),

		exitCh: make(chan struct{}),
		delCh:  make(chan *delnode, len(shardlist)),
	}

	go sub.eventsloop()
	return sub
}

func (s *subscribe) Subscribe(fullShardId uint32, tp Type, broadcast func(interface{})) {
	if _, ok := s.shardlist[fullShardId]; !ok {
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
		logsSub := s.shardlist[fullShardId].MinorBlockChain.SubscribeLogsEvent(logsCh)
		tpEvent[tp] = &subLogsEvent{
			ch:        logsCh,
			sub:       logsSub,
			broadcast: broadcast,
		}
	case PendingTransactionsSubscription:
		txsCh := make(chan core.NewTxsEvent, txChanSize)
		txsSub := s.shardlist[fullShardId].MinorBlockChain.SubscribeTxsEvent(txsCh)
		tpEvent[tp] = &subTxsEvent{
			ch:        txsCh,
			sub:       txsSub,
			broadcast: broadcast,
		}
	case BlocksSubscription:
		headersCh := make(chan core.MinorChainHeadEvent, chainEvChanSize)
		headersSub := s.shardlist[fullShardId].MinorBlockChain.SubscribeChainHeadEvent(headersCh)
		tpEvent[tp] = &subMinorBlockHeadersEvent{
			ch:headersCh,
			sub:headersSub,
			broadcast: broadcast,
		}
	}
}

func (s *subscribe) Unsubscribe(fullShardId uint32, tp Type) {
	go func() { s.delCh <- &delnode{fullShardId: fullShardId, tp: tp} }()
}

func (s *subscribe) Stop() {
	close(s.exitCh)
}

func (s *subscribe) eventsloop() {
	var g errgroup.Group
	defer func() {
		for id := range s.events {
			for _, subEv := range s.events[id] {
				subEv.freech()
			}
		}
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
		default:
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
