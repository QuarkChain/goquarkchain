package account

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsNeighbor(t *testing.T) {
	b1 := NewBranch(2<<16 | 2 | 1)
	b2 := NewBranch(2<<16 | 2 | 0)
	assert.Equal(t, IsNeighbor(b1, b2, 33), true)

	b1 = NewBranch(1<<16 | 2 | 1)
	b2 = NewBranch(3<<16 | 2 | 1)

	assert.Equal(t, IsNeighbor(b1, b2, 33), true)

	b1 = NewBranch(1<<16 | 2 | 0)
	b2 = NewBranch(3<<16 | 2 | 1)
	assert.Equal(t, IsNeighbor(b1, b2, 32), true)

	b1 = NewBranch(1<<16 | 2 | 0)
	b2 = NewBranch(3<<16 | 2 | 1)
	assert.Equal(t, IsNeighbor(b1, b2, 33), false)
}
