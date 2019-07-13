package types

import (
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/serialize"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"math/big"
)

type TokenBalanceMap struct {
	BalanceMap map[*big.Int]*big.Int
}

func NewTokenBalanceMap() *TokenBalanceMap {
	return &TokenBalanceMap{
		BalanceMap: make(map[*big.Int]*big.Int),
	}
}
func (t *TokenBalanceMap) Add(other map[*big.Int]*big.Int) {
	for k, v := range other {
		prevAmount := new(big.Int)
		if data, ok := t.BalanceMap[k]; ok {
			prevAmount = prevAmount.Add(prevAmount, data)
		}
		prevAmount = prevAmount.Add(prevAmount, v)
		t.BalanceMap[k] = prevAmount
	}
}

func (t *TokenBalanceMap) GetDefaultTokenBalance() *big.Int {
	return new(big.Int).Set(t.BalanceMap[common.TokenIDEncode("QKC")])
}

func (t *TokenBalanceMap) Serialize(w *[]byte) error {
	num := uint32(0)
	for _, v := range t.BalanceMap {
		if v.Cmp(ethCommon.Big0) == 0 {
			continue
		}
		num++
	}

	if err := serialize.Serialize(w, num); err != nil {
		return err
	}
	for k, v := range t.BalanceMap {
		if v.Cmp(ethCommon.Big0) == 0 {
			continue
		}
		if err := serialize.Serialize(w, k); err != nil {
			return err
		}
		if err := serialize.Serialize(w, v); err != nil {
			return err
		}
	}
	return nil
}

func (t *TokenBalanceMap) Deserialize(bb *serialize.ByteBuffer) error {
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
		t.BalanceMap[k] = v
	}
	return nil
}

type XShardTxCursorInfo struct {
	RootBlockHeight    uint64
	MinorBlockIndex    uint64
	XShardDepositIndex uint64
}
