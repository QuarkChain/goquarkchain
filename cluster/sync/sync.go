package sync

import (
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"sync"
)

// A lightweight wrapper over shard chain or root chain.
type blockchain interface {
	HasBlock(common.Hash) bool
	AddBlock(types.IBlock) error
	CurrentHeader() types.IHeader
	Validator() core.Validator
}

// Adds rootchain specific logic.
type rootblockchain interface {
	blockchain
	AddValidatedMinorBlockHeader(common.Hash)
	IsMinorBlockValidated(common.Hash) bool
}

// Synchronizer will sync blocks for the master server when receiving new root blocks from peers.
type Synchronizer interface {
	AddTask(Task) error
	Close() error
	IsSyncing() bool
}

type synchronizer struct {
	blockchain   blockchain
	taskRecvCh   chan Task
	taskAssignCh chan Task
	abortCh      chan struct{}

	mu      sync.RWMutex
	running bool
}

func (s *synchronizer) IsSyncing() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *synchronizer) setSyncing(isSync bool) {
	s.mu.Lock()
	s.running = isSync
	s.mu.Unlock()
}

// AddTask sends a root block from peers to the main loop for processing.
func (s *synchronizer) AddTask(task Task) error {
	s.taskRecvCh <- task
	return nil
}

// Close will stop all on-going channels in the synchronizer.
func (s *synchronizer) Close() error {
	s.abortCh <- struct{}{}
	return nil
}

func (s *synchronizer) loop() {
	go func() {
		logger := log.New("synchronizer", "runner")
		for t := range s.taskAssignCh {
			if !s.IsSyncing() {
				s.setSyncing(true)
			}
			if err := t.Run(s.blockchain); err != nil {
				logger.Error("Running sync task failed", "error", err)
			} else {
				logger.Info("Done sync task", "priority", t.Priority())
			}
			s.setSyncing(false)
		}
	}()

	taskMap := make(map[string]Task)
	for {
		var currTask Task
		var assignCh chan Task
		if len(taskMap) > 0 {
			// Enable sending through the channel.
			currTask = getNextTask(taskMap)
			assignCh = s.taskAssignCh
		}

		select {
		case task := <-s.taskRecvCh:
			taskMap[task.PeerID()] = task
		case assignCh <- currTask:
			delete(taskMap, currTask.PeerID())
		case <-s.abortCh:
			close(s.taskAssignCh)
			return
		}
	}
}

// Find the next task according to their priorities.
func getNextTask(taskMap map[string]Task) (ret Task) {
	prio := new(big.Int)
	for _, t := range taskMap {
		newPrio := t.Priority()
		if ret == nil || newPrio.Cmp(prio) > 0 {
			ret = t
			prio = newPrio
		}
	}
	return
}

// NewSynchronizer returns a new synchronizer instance.
func NewSynchronizer(bc blockchain) Synchronizer {
	s := &synchronizer{
		blockchain:   bc,
		taskRecvCh:   make(chan Task),
		taskAssignCh: make(chan Task),
		abortCh:      make(chan struct{}),
	}
	go s.loop()
	return s
}
