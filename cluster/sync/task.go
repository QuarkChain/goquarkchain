package sync

import "github.com/QuarkChain/goquarkchain/core/types"

// Task represents a synchronization task for the synchronizer.
type Task interface {
	// TODO: master server and root state are needed as arguments.
	Run() error
	Peer() peer
	Priority() uint
}

// All of the sync tasks to are to catch up with the root chain from peers.
type task struct {
	rootBlock *types.RootBlock
	peer
}

func (t *task) Run() error {
	panic("not implemented")
}

func (t *task) Priority() uint {
	panic("not implemented")
}

func (t *task) Peer() peer {
	return t.peer
}
