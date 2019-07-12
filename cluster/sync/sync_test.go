package sync

import (
	"fmt"
	"math/big"
	"testing"
)

// For test purpose.
type trivialTask struct {
	id   uint
	prio uint
	// To pause / resume the task running.
	executeSwitch chan struct{}
}

func (t *trivialTask) Run(_ blockchain) error {
	<-t.executeSwitch
	return nil
}

func (t *trivialTask) PeerID() string {
	return fmt.Sprintf("%d", t.id)
}

func (t *trivialTask) Priority() *big.Int {
	return new(big.Int).SetUint64(uint64(t.prio))
}

func TestRunOneTask(t *testing.T) {
	s := NewSynchronizer(nil)
	tt := &trivialTask{executeSwitch: make(chan struct{})}

	s.AddTask(tt)
	tt.executeSwitch <- struct{}{}
	s.Close()
}

func TestRunTasksByPriority(t *testing.T) {
	s := NewSynchronizer(nil)
	coldStarter := &trivialTask{prio: 999, executeSwitch: make(chan struct{})}

	// Use cold starter to block the running goroutine.
	s.AddTask(coldStarter)

	tasks := []*trivialTask{}
	for i := uint(1); i <= 10; i++ {
		tt := &trivialTask{id: i, prio: i, executeSwitch: make(chan struct{})}
		tasks = append(tasks, tt)
		s.AddTask(tt)
	}

	// Release the cold starter later and let the synchronizer to sort tasks.
	coldStarter.executeSwitch <- struct{}{}

	// Release switches according to priorities: reverse order of the slice.
	for i := 0; i < len(tasks); i++ {
		tt := tasks[len(tasks)-i-1]
		tt.executeSwitch <- struct{}{}
	}
	s.Close()
}
