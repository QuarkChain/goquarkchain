package account

import (
	"errors"
	"testing"
)

func TestIsNeighbor(t *testing.T) {
	b1 := NewBranch(2<<16 | 2 | 1)
	b2 := NewBranch(2<<16 | 2 | 0)
	if IsNeighbor(b1, b2, 33) == false {
		panic(errors.New("should true"))
	}

	b1 = NewBranch(1<<16 | 2 | 1)
	b2 = NewBranch(3<<16 | 2 | 1)
	if IsNeighbor(b1, b2, 33) == false {
		panic(errors.New("should true"))
	}

	b1 = NewBranch(1<<16 | 2 | 0)
	b2 = NewBranch(3<<16 | 2 | 1)
	if IsNeighbor(b1, b2, 32) == false {
		panic(errors.New("should true"))
	}

	b1 = NewBranch(1<<16 | 2 | 0)
	b2 = NewBranch(3<<16 | 2 | 1)
	if IsNeighbor(b1, b2, 33) == true {
		panic(errors.New("should false"))
	}
}
