package sync

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/core/types"
)

// TODO a mock struct for peers, should have real Peer type in P2P module.
type peer struct {
	id       string
	hostport string
}

// Use root block header and source peer to represent a sync task.
type task struct {
	rootBlock *types.RootBlock
	peer      peer
}

// Synchronizer will sync blocks for the master server when receiving new root blocks from peers.
type Synchronizer interface {
	AddTask(*types.RootBlock, peer) error
	Close() error
}

type synchronizer struct {
	taskRecvCh   chan *task
	taskAssignCh chan *task
	abortCh      chan struct{}
}

// AddTask sends a root block from peers to the main loop for processing.
func (s *synchronizer) AddTask(rootBlock *types.RootBlock, p peer) error {
	s.taskRecvCh <- &task{rootBlock, p}
	return nil
}

// Close will stop all on-going channels in the synchronizer.
func (s *synchronizer) Close() error {
	s.abortCh <- struct{}{}
	return nil
}

func (s *synchronizer) loop() {
	logger := log.New("synchronizer", "root")

	go func() {
		for t := range s.taskAssignCh {
			if err := s.runTask(t); err != nil {
				logger.Error("Running sync task failed", "error", err)
			} else {
				logger.Info("Done sync task", "height")
			}
		}
	}()

	taskMap := make(map[peer]*task)
	for {
		var currTask *task
		var assignCh chan *task
		if len(taskMap) > 0 {
			// Enable sending through the channel.
			currTask = s.getNextTask(taskMap)
			assignCh = s.taskAssignCh
		}

		select {
		case task := <-s.taskRecvCh:
			taskMap[task.peer] = task
		case assignCh <- currTask:
			delete(taskMap, currTask.peer)
		case <-s.abortCh:
			close(s.taskAssignCh)
			return
		}
	}
}

// Find the task with highest root heigh.
func (s *synchronizer) getNextTask(taskMap map[peer]*task) *task {
	// TODO: actually find the highest heigh.
	for _, t := range taskMap {
		return t
	}
	return nil
}

// Run the sync task.
func (s *synchronizer) runTask(t *task) error {
	// TODO: do something here.
	return nil
}

// NewSynchronizer returns a new synchronizer instance.
func NewSynchronizer() Synchronizer {
	s := &synchronizer{
		taskRecvCh:   make(chan *task),
		taskAssignCh: make(chan *task),
		abortCh:      make(chan struct{}),
	}
	go s.loop()
	return s
}
