package sync

import (
	"fmt"
	"testing"
)

// For test purpose.
type trivialTask struct {
	id   uint
	prio uint
	// To pause / resume the task running.
	executeSwitch chan struct{}
}

func (t *trivialTask) Run() error {
	<-t.executeSwitch
	return nil
}

func (t *trivialTask) Peer() peer {
	return peer{id: fmt.Sprintf("%d", t.id)}
}

func (t *trivialTask) Priority() uint {
	return t.prio
}

func TestRunOneTask(t *testing.T) {
	s := NewSynchronizer()
	tt := &trivialTask{executeSwitch: make(chan struct{})}

	s.AddTask(tt)
	tt.executeSwitch <- struct{}{}
	s.Close()
}

func TestTaskRunByPriority(t *testing.T) {
	s := NewSynchronizer()
	coldStarter := &trivialTask{prio: 999, executeSwitch: make(chan struct{})}

	// Use cold starter to block the running goroutine.
	s.AddTask(coldStarter)
	// Release the cold starter later and let the synchronizer to sort tasks.
	// go func() {
	// time.Sleep(500 * time.Millisecond)
	// coldStarter.executeSwitch <- struct{}{}
	// }()

	tasks := []*trivialTask{}
	for i := uint(1); i <= 10; i++ {
		tt := &trivialTask{id: i, prio: i, executeSwitch: make(chan struct{})}
		tasks = append(tasks, tt)
		s.AddTask(tt)
	}

	coldStarter.executeSwitch <- struct{}{}

	// Release switches according to priorities.
	for i := 9; i >= 0; i-- {
		tt := tasks[i]
		tt.executeSwitch <- struct{}{}
	}
	s.Close()
}
