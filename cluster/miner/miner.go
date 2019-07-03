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
	mu        sync.RWMutex
	timestamp *time.Time
	isMining  uint32
	stopCh    chan struct{}
	logInfo   string
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
		stopCh:        make(chan struct{}),
		logInfo:       "miner",
	}
	go miner.mainLoop(miner.minerInterval)
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

func (m *Miner) commit() {
	// don't allow to mine
	if atomic.LoadUint32(&m.isMining) == 0 || time.Now().Sub(*m.timestamp).Seconds() > deadtime {
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

func (m *Miner) mainLoop(recommit time.Duration) {

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

func (m *Miner) Init() {
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

func (m *Miner) HandleNewTip() {
	log.Info(m.logInfo, "handle new tip: height", m.getTip())
	m.commit()

}
