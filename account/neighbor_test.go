package account

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"math/big"
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

func TestCreatAddressFromIdentity(t *testing.T) {
	a := new(big.Int).SetUint64(1)
	b := new(big.Int).SetUint64(2)

	c := new(big.Int).Div(a, b)
	fmt.Println("a,b,c", a, b, c)
}
