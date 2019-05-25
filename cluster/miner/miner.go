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
	logger   string
}

func New(api MinerAPI, engine consensus.Engine) *Miner {
	miner := &Miner{
		api:      api,
		engine:   engine,
		resultCh: make(chan types.IBlock),
		startCh:  make(chan struct{}, 1),
		exitCh:   make(chan struct{}),
		logger:   "Miner work loop",
	}

	go miner.minerLoop()

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
				log.Error(m.logger, "CreateBlockAsyncFunc operation error: ", err)
				m.ReMine()
			}
			_ = m.engine.Seal(nil, block, m.resultCh, stopCh)

		case rBlock := <-m.resultCh:
			_ = m.api.AddBlockAsyncFunc(rBlock)
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

	m.isMining = mining
	m.engine.SetThreads(1)
	m.startCh <- struct{}{}
}

func (m *Miner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.isMining = false
	m.engine.SetThreads(-1)
	if m.exitCh != nil {
		m.exitCh <- struct{}{}
		m.exitCh = nil
	}
}

func (m *Miner) ReMine() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.isMining {
		m.startCh <- struct{}{}
	}
}
