package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func TestNewTokenBalanceMap(t *testing.T) {
	m0 := NewTokenBalanceMap()
	m0.BalanceMap[3234] = new(big.Int).SetUint64(10)
	m0.BalanceMap[0] = new(big.Int).SetUint64(0)
	m0.BalanceMap[3567] = new(big.Int).SetUint64(0)

	m1 := NewTokenBalanceMap()
	m1.BalanceMap[3234] = new(big.Int).SetUint64(10)

	data0, err0 := serialize.SerializeToBytes(m0)
	data1, err1 := serialize.SerializeToBytes(m1)
	assert.Equal(t, data0, data1)
	assert.Equal(t, err0, err1)

}
