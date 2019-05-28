package miner

import (
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/log"
	"runtime"
	"sync"
)

var (
	threads = runtime.NumCPU()
)

type Miner struct {
	api      MinerAPI
	engine   consensus.Engine
	resultCh chan types.IBlock
	startCh  chan struct{}
	exitCh   chan struct{}
	mu       sync.RWMutex
	isMining bool
}

func New(api MinerAPI, engine consensus.Engine) *Miner {
	miner := &Miner{
		api:      api,
		engine:   engine,
		resultCh: make(chan types.IBlock),
	}
	return miner
}

func (m *Miner) minerLoop() {
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
	for {
		select {
		case <-m.startCh:
			interrupt()
			block, err := m.api.CreateBlockAsyncFunc()
			if err != nil {
				log.Error("create block to mine", "err", err)
				continue
			}
			_ = m.engine.Seal(nil, block, m.resultCh, stopCh)

		case rBlock := <-m.resultCh:
			if err := m.api.AddBlockAsyncFunc(rBlock); err != nil {
				log.Error("add minered block", "block hash", rBlock.Hash().Hex(), "err", err)
				continue
			}
			m.ReMine()

		case <-m.exitCh:
			interrupt()
			return
		}
	}
}

func (m *Miner) Start(mining bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isMining || !mining {
		return
	}
	m.isMining = mining

	m.startCh = make(chan struct{}, 1)
	m.exitCh = make(chan struct{})
	m.engine.SetThreads(threads / 2)

	go m.minerLoop()
	m.startCh <- struct{}{}
}

func (m *Miner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.engine.SetThreads(-1)
	m.isMining = false
	if m.exitCh != nil {
		close(m.exitCh)
	}
}

func (m *Miner) ReMine() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.isMining {
		m.startCh <- struct{}{}
	}
}
