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
	"time"
)

const (
	deadtime = 120
)

var (
	threads = runtime.NumCPU()
)

type Miner struct {
	api    MinerAPI
	engine consensus.Engine

	resultCh  chan types.IBlock
	workCh    chan types.IBlock
	startCh   chan struct{}
	exitCh    chan struct{}
	mu        sync.RWMutex
	timestamp *time.Time
	isMining  bool
	stopCh    chan struct{}
	logInfo   string
}

func New(ctx *service.ServiceContext, api MinerAPI, engine consensus.Engine) *Miner {
	miner := &Miner{
		api:       api,
		engine:    engine,
		timestamp: &ctx.Timestamp,
		resultCh:  make(chan types.IBlock, 1),
		workCh:    make(chan types.IBlock, 1),
		startCh:   make(chan struct{}, 1),
		exitCh:    make(chan struct{}),
		stopCh:    make(chan struct{}),
		logInfo:   "miner",
	}
	miner.engine.SetThreads(1)
	go miner.mainLoop()
	return miner
}
func (m *Miner) getTip() uint64 {
	return m.api.GetTip()
}

// interrupt aborts the minering work
func (m *Miner) interrupt() {
	if m.stopCh != nil {
		close(m.stopCh)
		m.stopCh = make(chan struct{})
	}
}

func (m *Miner) allowMining() bool {
	if !m.IsMining() ||
		m.api.IsSyncIng() ||
		time.Now().Sub(*m.timestamp).Seconds() > deadtime {
		return false
	}
	return true
}

func (m *Miner) commit() {
	// don't allow to mine
	if !m.allowMining() {
		return
	}
	m.interrupt()
	block, err := m.api.CreateBlockToMine()
	if err != nil {
		log.Error(m.logInfo, "create block to mine err", err)
		// retry to create block to mine
		time.Sleep(2 * time.Second)
		m.startCh <- struct{}{}
		return
	}
	tip := m.getTip()
	if block.NumberU64() <= tip {
		log.Error(m.logInfo, "block's height small than tipHeight after commit blockNumber ,no need to seal", block.NumberU64(), "tip", m.getTip())
		return
	}
	m.workCh <- block
}

func (m *Miner) mainLoop() {

	for {
		select {
		case <-m.startCh:
			m.commit()

		case work := <-m.workCh:
			log.Info(m.logInfo, "ready to seal height", work.NumberU64())
			if err := m.engine.Seal(nil, work, m.resultCh, m.stopCh); err != nil {
				log.Error(m.logInfo, "Seal block to mine err", err)
				m.commit()
			}

		case rBlock := <-m.resultCh:
			log.Info(m.logInfo, "seal succ number", rBlock.NumberU64(), "hash", rBlock.Hash().String())
			if err := m.api.InsertMinedBlock(rBlock); err != nil {
				log.Error(m.logInfo, "add minered block err block hash", rBlock.Hash().Hex(), "err", err)
				time.Sleep(time.Duration(3) * time.Second)
				m.commit()
			}

		case <-m.exitCh:
			return
		}
	}
}

func (m *Miner) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isMining = false
	close(m.exitCh)
}

// TODO when p2p is syncing block how to stop miner.
func (m *Miner) SetMining(mining bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isMining = mining
	if mining {
		m.startCh <- struct{}{}
	}
}

func (m *Miner) GetWork() (*consensus.MiningWork, error) {
	if !m.IsMining() {
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
	if !m.IsMining() || m.api.IsSyncIng() {
		return false
	}
	return m.engine.SubmitWork(nonce, hash, digest)
}

func (m *Miner) HandleNewTip() {
	log.Info(m.logInfo, "handle new tip: height", m.getTip())
	m.commit()
}

func (m *Miner) IsMining() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isMining
}
