package types

import (
	"math/big"
	"reflect"
	"testing"
)

func TestNewTokenBalanceMap(t *testing.T) {
	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	m0 := make(map[uint64]*big.Int)
	m0[3234] = big.NewInt(1000)
	m0[0] = big.NewInt(0)
	m0[3567] = big.NewInt(0)
	tb := NewTokenBalancesWithMap(m0)
	tbb, _ := tb.SerializeToBytes()
	m1 := make(map[uint64]*big.Int)
	m1[3234] = big.NewInt(10)
	tb1 := NewTokenBalancesWithMap(m1)
	tbb1, _ := tb1.SerializeToBytes()

	check("NewTokenBalancesWithMap", tbb, tbb1)

}
func TestTokenBalances_Add(t *testing.T) {
	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	m0 := make(map[uint64]*big.Int)
	m0[3567] = big.NewInt(0)
	tb := NewTokenBalancesWithMap(m0)
	m1 := make(map[uint64]*big.Int)
	m1[3234] = big.NewInt(10)
	tb1 := NewTokenBalancesWithMap(m1)
	tb.Add(tb1.balances)
	m3 := make(map[uint64]*big.Int)
	m3[3567] = big.NewInt(0)
	m3[3234] = big.NewInt(10)
	check("", tb.balances, m3)

}
