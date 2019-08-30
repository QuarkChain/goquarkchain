package types

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func TestNewTokenBalanceMap(t *testing.T) {
	m0 := NewEmptyTokenBalances()
	m0.SetValue(new(big.Int).SetUint64(10), 3234)
	m0.SetValue(new(big.Int).SetUint64(0), 0)
	m0.SetValue(new(big.Int).SetUint64(0), 3567)

	m1 := NewEmptyTokenBalances()
	m1.SetValue(new(big.Int).SetUint64(10), 3234)

	data0, err0 := serialize.SerializeToBytes(m0)
	data1, err1 := serialize.SerializeToBytes(m1)
	assert.Equal(t, data0, data1)
	assert.Equal(t, err0, err1)

}

func TestNewEmptyTokenBalances(t *testing.T) {
	var block IBlock
	if block == nil {
		fmt.Println("block is nil")
	} else {
		fmt.Println("block is not ni")
	}
}
