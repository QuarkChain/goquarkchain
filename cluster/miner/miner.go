package miner

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
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

type workAdjusted struct {
	block              types.IBlock
	adjustedDifficulty *big.Int
	optionalDivider    uint64
}

type Miner struct {
	api    MinerAPI
	engine consensus.Engine

	resultCh  chan types.IBlock
	workCh    chan workAdjusted
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
		workCh:    make(chan workAdjusted, 1),
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
		time.Now().Sub(*m.timestamp).Seconds() > deadtime {
		fmt.Println("SSSSSSSSSSSSSSSSS",m.isMining,m.api.IsSyncIng(),time.Now().Sub(*m.timestamp).Seconds(),deadtime,time.Now().Sub(*m.timestamp).Seconds() > deadtime)
		return false
	}
	return true
}

func (m *Miner) commit(addr *account.Address) {
	// don't allow to mine
	if !m.allowMining() {
		fmt.Println("?????????-1")
		return
	}
	sleepTime:=0
	for m.api.IsSyncIng(){
		time.Sleep(1*time.Second)
		sleepTime++
		if sleepTime>=100{
			panic("sleepTime>=100 ")//TODO this line need ti delete
		}
	}
		m.interrupt()
	block, diff, optionalDivider, err := m.api.CreateBlockToMine(addr)
	if err != nil {
		log.Error(m.logInfo, "create block to mine err", err)
		// retry to create block to mine
		time.Sleep(2 * time.Second)
		m.startCh <- struct{}{}
		fmt.Println("?????????-2")
		return
	}
	tip := m.getTip()
	if block.NumberU64() <= tip {
		log.Error(m.logInfo, "block's height small than tipHeight after commit blockNumber ,no need to seal", block.NumberU64(), "tip", m.getTip())
		fmt.Println("?????????-4")
		return
	}
	fmt.Println("??????????-succ")
	m.workCh <- workAdjusted{block, diff, optionalDivider}
}

func (m *Miner) mainLoop() {

	for {
		select {
		case <-m.startCh:
			m.commit(nil)

		case work := <-m.workCh: //to discuss:need this?
			log.Info(m.logInfo, "ready to seal height", work.block.NumberU64(), "coinbase", work.block.IHeader().GetCoinbase().ToHex())
			if err := m.engine.Seal(nil, work.block, work.adjustedDifficulty, work.optionalDivider, m.resultCh, m.stopCh); err != nil {
				log.Error(m.logInfo, "Seal block to mine err", err)
				coinbase := work.block.IHeader().GetCoinbase()
				m.commit(&coinbase)
			}

		case block := <-m.resultCh:
			log.Info(m.logInfo, "seal succ number", block.NumberU64(), "hash", block.Hash().String())
			if err := m.api.InsertMinedBlock(block); err != nil {
				log.Error(m.logInfo, "add minered block err block hash", block.Hash().Hex(), "err", err)
				time.Sleep(time.Duration(3) * time.Second)
				coinbase := block.IHeader().GetCoinbase()
				m.commit(&coinbase)
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
	m.isMining = mining
	m.mu.Unlock()
	if mining {
		m.startCh <- struct{}{}
	} else {
		m.interrupt()
	}
}

func (m *Miner) GetWork(coinbaseAddr *account.Address) (*consensus.MiningWork, error) {
	addrForGetWork := m.api.GetDefaultCoinbaseAddress()
	if coinbaseAddr != nil && !account.IsSameAddress(*coinbaseAddr, m.api.GetDefaultCoinbaseAddress()) {
		addrForGetWork = *coinbaseAddr
	}

	work, err := m.engine.GetWork(addrForGetWork)
	if err != nil {
		if err == consensus.ErrNoMiningWork {
			block, diff, optionalDivider, err := m.api.CreateBlockToMine(&addrForGetWork)
			if err == nil {
				m.workCh <- workAdjusted{block, diff, optionalDivider}
				return &consensus.MiningWork{HeaderHash: block.IHeader().SealHash(), Number: block.NumberU64(),
					OptionalDivider: optionalDivider, Difficulty: diff}, nil
			}
			return nil, err
		}
		return nil, err
	}
	return work, nil
}

func (m *Miner) SubmitWork(nonce uint64, hash, digest common.Hash, signature *[65]byte) bool {
	if !m.IsMining() || m.api.IsSyncIng() {
		return false
	}
	return m.engine.SubmitWork(nonce, hash, digest, signature)
}

func (m *Miner) HandleNewTip() {
	log.Info(m.logInfo, "handle new tip: height", m.getTip())
	m.engine.RefreshWork(m.api.GetTip())
	m.commit(nil)
}

func (m *Miner) IsMining() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isMining
}
