package miner

import (
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/log"
	"runtime"
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
	isMining bool
	logger   string
}

func New(api MinerAPI, engine consensus.Engine) *Miner {
	miner := &Miner{
		api:      api,
		engine:   engine,
		resultCh: make(chan types.IBlock),
		startCh:  make(chan struct{}),
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
				log.Error(m.logger, "CreateBlockAsyncFunc operation", "err", err)
				m.Stop()
			}
			_ = m.engine.Seal(nil, block, m.resultCh, stopCh)

		case rBlock := <-m.resultCh:
			if err := m.api.AddBlockAsyncFunc(rBlock); err != nil {
				log.Error(m.logger, "AddBlockAsyncFunc operation", "err", err)
				m.Stop()
			}
			m.ReMine()

		case <-m.exitCh:
			interrupt()
			return
		}
	}
}

func (m *Miner) Start(mining bool) {
	m.isMining = mining
	m.engine.SetThreads(1)
	m.startCh <- struct{}{}
}

func (m *Miner) Stop() {
	m.isMining = false
	m.engine.SetThreads(-1)
	if m.exitCh != nil {
		m.exitCh <- struct{}{}
		m.exitCh = nil
	}
}

func (m *Miner) ReMine() {
	if m.isMining {
		m.startCh <- struct{}{}
	}
}
