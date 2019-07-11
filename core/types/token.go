package types

import (
	"github.com/QuarkChain/goquarkchain/common"
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

func (t *TokenBalanceMap)GetDefaultTokenBalance()*big.Int  {
	return new(big.Int).Set(t.BalanceMap[common.TokenIDEncode("QKC")])
}

type XShardTxCursorInfo struct {
	RootBlockHeight    uint64
	MinorBlockIndex    uint64
	XShardDepositIndex uint64
}
