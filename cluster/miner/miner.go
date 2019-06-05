package miner

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	deadtime = 120
)

var (
	threads = runtime.NumCPU()
)

type Miner struct {
	api           MinerAPI
	engine        consensus.Engine
	minerInterval time.Duration

	resultCh  chan types.IBlock
	workCh    chan types.IBlock
	startCh   chan struct{}
	exitCh    chan struct{}
	now       time.Time
	mu        sync.RWMutex
	timestamp *int64
	isMining  uint32
}

func New(ctx *service.ServiceContext, api MinerAPI, engine consensus.Engine, interval uint32) *Miner {
	miner := &Miner{
		api:           api,
		engine:        engine,
		minerInterval: time.Duration(interval) * time.Second,
		timestamp:     &ctx.Timestamp,
		resultCh:      make(chan types.IBlock, 1),
		workCh:        make(chan types.IBlock, 1),
		startCh:       make(chan struct{}, 1),
		exitCh:        make(chan struct{}),
	}
	go miner.mainLoop(miner.minerInterval)
	return miner
}

func (m *Miner) mainLoop(recommit time.Duration) {
	var (
		stopCh chan struct{}
	)

	// interrupt aborts the minering work
	interrupt := func() {
		if stopCh != nil {
			close(stopCh)
			stopCh = make(chan struct{})
		}
	}
	commit := func() {
		// don't allow to mine
		if atomic.LoadUint32(&m.isMining) == 0 || time.Now().Unix()-deadtime > atomic.LoadInt64(m.timestamp) {
			return
		}
		interrupt()
		block, err := m.api.CreateBlockToMine()
		if err != nil {
			log.Error("create block to mine", "err", err)
			// retry to create block to mine
			time.Sleep(2 * time.Second)
			m.startCh <- struct{}{}
			return
		}
		m.workCh <- block
		log.Info("create new block to mine", "height", block.NumberU64())
		m.now = time.Now()
	}

	for {
		select {
		case <-m.startCh:
			commit()

		case work := <-m.workCh:
			if err := m.engine.Seal(nil, work, m.resultCh, stopCh); err != nil {
				log.Error("Seal block to mine", "err", err)
				commit()
			}

		case rBlock := <-m.resultCh:
			if err := m.api.InsertMinedBlock(rBlock); err != nil {
				log.Error("add minered block", "block hash", rBlock.Hash().Hex(), "err", err)
				time.Sleep(time.Duration(3) * time.Second)
			}
			commit()

		case <-m.exitCh:
			return
		}
	}
}

func (m *Miner) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.engine.SetThreads(1)
}

func (m *Miner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	close(m.exitCh)
}

// TODO when p2p is syncing block how to stop miner.
func (m *Miner) SetMining(mining bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if mining {
		atomic.StoreUint32(&m.isMining, 1)
		m.startCh <- struct{}{}
	} else {
		atomic.StoreUint32(&m.isMining, 0)
	}
}

func (m *Miner) GetWork() (*consensus.MiningWork, error) {
	if atomic.LoadUint32(&m.isMining) == 0 {
		return nil, fmt.Errorf("Should only be used for remote miner ")
	}
	work, err := m.engine.GetWork()
	if err == nil {
		return work, nil
	}
	if err == consensus.ErrNoMiningWork {
		m.startCh <- struct{}{}
		time.Sleep(2)
	}
	return m.engine.GetWork()
}

func (m *Miner) SubmitWork(nonce uint64, hash, digest common.Hash) bool {
	if atomic.LoadUint32(&m.isMining) == 0 {
		return false
	}
	return m.engine.SubmitWork(nonce, hash, digest)
}
