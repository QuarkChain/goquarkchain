package types

import (
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/serialize"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"math/big"
	"sort"
)

//type TokenType struct {
//	Key uint64
//}
//
//func NewTokenType(value uint64) TokenType {
//	return TokenType{
//		Key: value,
//	}
//}

type TokenBalanceMap struct {
	BalanceMap map[uint64]*big.Int
}

func NewTokenBalanceMap() *TokenBalanceMap {
	return &TokenBalanceMap{
		BalanceMap: make(map[uint64]*big.Int),
	}
}
func (t *TokenBalanceMap) Add(other map[uint64]*big.Int) {
	for k, v := range other {
		if data, ok := t.BalanceMap[k]; ok {
			t.BalanceMap[k] = new(big.Int).Add(v, data)
		} else {
			t.BalanceMap[k] = new(big.Int).Set(v)
		}
	}
}

func (t *TokenBalanceMap) GetDefaultTokenBalance() *big.Int {
	return new(big.Int).Set(t.BalanceMap[common.TokenIDEncode("QKC")])
}

func (t *TokenBalanceMap) Copy() *TokenBalanceMap {
	data := NewTokenBalanceMap()
	data.BalanceMap = make(map[uint64]*big.Int)
	for k, v := range t.BalanceMap {
		data.BalanceMap[k] = v
	}
	return data
}

func (t *TokenBalanceMap) Serialize(w *[]byte) error {
	keys := make([]uint64, 0, len(t.BalanceMap))
	num := uint32(0)
	for k, v := range t.BalanceMap {
		if v.Cmp(ethCommon.Big0) == 0 {
			continue
		}
		keys = append(keys, k)
		num++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	if err := serialize.Serialize(w, num); err != nil {
		return err
	}
	for _, key := range keys {
		v := t.BalanceMap[key]
		if err := serialize.Serialize(w, new(big.Int).SetUint64(key)); err != nil {
			return err
		}
		if err := serialize.Serialize(w, v); err != nil {
			return err
		}
	}
	return nil
}

func (t *TokenBalanceMap) Deserialize(bb *serialize.ByteBuffer) error {
	t.BalanceMap = make(map[uint64]*big.Int)
	num, err := bb.GetUInt32()
	if err != nil {
		return err
	}
	for index := uint32(0); index < num; index++ {
		k := new(big.Int)
		if err := serialize.Deserialize(bb, k); err != nil {
			return err
		}
		v := new(big.Int)

		if err := serialize.Deserialize(bb, v); err != nil {
			return err
		}
		if v.Cmp(ethCommon.Big0) == 0 {
			continue
		}
		t.BalanceMap[k.Uint64()] = v
	}
	return nil
}

type XShardTxCursorInfo struct {
	RootBlockHeight    uint64
	MinorBlockIndex    uint64
	XShardDepositIndex uint64
}
