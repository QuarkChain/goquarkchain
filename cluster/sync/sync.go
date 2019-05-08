package sync

import (
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/core/types"
)

// TODO a mock struct for peers, should have real peer connection type in P2P module.
type peer interface {
	GetRootBlockHeaderList(hash common.Hash, amount uint32, reverse bool) ([]*types.RootBlockHeader, error)
	GetRootBlockList(hashes []common.Hash) ([]*types.RootBlock, error)
	PeerId() string
}

// blockchain is a lightweight wrapper over shard chain or root chain.
type blockchain interface {
	HasBlock(common.Hash) bool
	InsertChain([]types.IBlock) (int, error)
	CurrentHeader() types.IHeader
	Validator() core.Validator
}

// Synchronizer will sync blocks for the master server when receiving new root blocks from peers.
type Synchronizer interface {
	AddTask(Task) error
	Close() error
}

type synchronizer struct {
	blockchain   blockchain
	taskRecvCh   chan Task
	taskAssignCh chan Task
	abortCh      chan struct{}
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
			if err := t.Run(s.blockchain); err != nil {
				logger.Error("Running sync task failed", "error", err)
			} else {
				logger.Info("Done sync task", "height")
			}
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
			taskMap[task.Peer().PeerId()] = task
		case assignCh <- currTask:
			delete(taskMap, currTask.Peer().PeerId())
		case <-s.abortCh:
			close(s.taskAssignCh)
			return
		}
	}
}

// Find the next task according to their priorities.
func getNextTask(taskMap map[string]Task) (ret Task) {
	prio := uint(0)
	for _, t := range taskMap {
		newPrio := t.Priority()
		if ret == nil || newPrio > prio {
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
