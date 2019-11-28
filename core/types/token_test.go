package types

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	qCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/ethereum/go-ethereum/ethdb"

	"github.com/ethereum/go-ethereum/common"
)

func TestNewTokenBalanceMap(t *testing.T) {
	m0 := make(map[uint64]*big.Int)
	m0[3234] = big.NewInt(1000)
	m0[0] = big.NewInt(0)
	m0[3567] = big.NewInt(0)
	tb := NewTokenBalancesWithMap(m0)
	t.Logf("token balance map：%v", tb.balances)
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

func TestEncodingInTrie(t *testing.T) {
	encoding := common.FromHex("0x01848d4271e44ea41466fe355561dd43b166c927d2eca0a8dd901a8e6469ecdeb1")
	mapping := make(map[uint64]*big.Int, 0)
	for index := 0; index < 17; index++ {
		mapping[qCommon.TokenIDEncode("Q"+string(byte(65+index)))] = new(big.Int).SetUint64(uint64(index*1000 + 42))
	}

	db := trie.NewDatabase(ethdb.NewMemDatabase())
	b0, err := NewTokenBalances(nil, db)
	if err != nil {
		panic(encoding)
	}
	b0.Add(mapping)
	b0.Commit()
	sData, err := b0.SerializeToBytes()
	if err != nil {
		panic(encoding)
	}
	assert.Equal(t, sData, encoding)
	assert.True(t, b0.tokenTrie != nil)

	b1, err := NewTokenBalances(encoding, db)
	assert.NoError(t, err)

	sData, err = b1.SerializeToBytes()
	assert.NoError(t, err)
	assert.Equal(t, sData, encoding)

	assert.Equal(t, len(b1.balances), 0)
	assert.Equal(t, b1.tokenTrie != nil, true)
	assert.Equal(t, !b1.IsBlank(), true)
	assert.Equal(t, b1.GetTokenBalance(qCommon.TokenIDEncode("QC")), mapping[qCommon.TokenIDEncode("QC")])
	assert.Equal(t, len(b1.balances), 1)

	_, err = b1.SerializeToBytes()
	assert.Error(t, err)
	b1.Commit()
	data, err := b1.SerializeToBytes()
	assert.NoError(t, err)
	assert.Equal(t, data, encoding)
}
