package account

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsNeighbor(t *testing.T) {
	b1 := NewBranch(2<<16 | 2 | 1)
	b2 := NewBranch(2<<16 | 2 | 0)
	assert.True(t, IsNeighbor(b1, b2, 33))

	b1 = NewBranch(1<<16 | 2 | 1)
	b2 = NewBranch(3<<16 | 2 | 1)
	assert.True(t, IsNeighbor(b1, b2, 33))

	b1 = NewBranch(1<<16 | 2 | 0)
	b2 = NewBranch(3<<16 | 2 | 1)
	assert.True(t, IsNeighbor(b1, b2, 32))

	b1 = NewBranch(1<<16 | 2 | 0)
	b2 = NewBranch(3<<16 | 2 | 1)
	assert.False(t, IsNeighbor(b1, b2, 33))
}

const (
	deadtime = 120
)

func TestNewBranch(t *testing.T) {
	//ts := time.Now()
	//time.Sleep(1231111 * time.Microsecond)
	//diff := time.Now().Sub(ts).Seconds()
	diff := float64(9.223372036854776e+09)
	fmt.Println("diff", int(diff))
	fmt.Println("diff", diff)
	fmt.Println("a,", diff > deadtime)
	fmt.Println("a,", int(diff) > deadtime)
}
