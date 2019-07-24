package types

import (
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/serialize"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"math/big"
	"sort"
	"sync"
)

type TokenBalanceMap struct {
	balanceMap map[uint64]*big.Int
	mu         sync.RWMutex
}

func NewTokenBalanceMap() *TokenBalanceMap {
	return &TokenBalanceMap{
		balanceMap: make(map[uint64]*big.Int),
	}
}
func (t *TokenBalanceMap) Add(other map[uint64]*big.Int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for k, v := range other {
		if data, ok := t.balanceMap[k]; ok {
			t.balanceMap[k] = new(big.Int).Add(v, data)
		} else {
			t.balanceMap[k] = new(big.Int).Set(v)
		}
	}
}

func (t *TokenBalanceMap) SetValue(amount *big.Int, tokenID uint64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	t.balanceMap[tokenID] = amount
}

func (t *TokenBalanceMap) Len() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.balanceMap)
}

func (t *TokenBalanceMap) IsEmpty() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	flag := true
	for _, v := range t.balanceMap {
		if v.Cmp(new(big.Int)) != 0 {
			return false
		}
	}
	return flag
}
func (t *TokenBalanceMap) GetBalancesFromTokenID(tokenID uint64) *big.Int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	data, ok := t.balanceMap[tokenID]
	if !ok {
		return new(big.Int)
	}
	return new(big.Int).Set(data)
}

func (t *TokenBalanceMap) GetDefaultTokenBalance() *big.Int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return new(big.Int).Set(t.balanceMap[common.TokenIDEncode("QKC")])
}

func (t *TokenBalanceMap) SetBalanceMap(data map[uint64]*big.Int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.balanceMap = data
}

func (t *TokenBalanceMap) GetBalanceMap() map[uint64]*big.Int {
	data := t.Copy()
	return data.balanceMap
}

func (t *TokenBalanceMap) Copy() *TokenBalanceMap {
	t.mu.RLock()
	defer t.mu.RUnlock()
	data := NewTokenBalanceMap()
	data.balanceMap = make(map[uint64]*big.Int)
	for k, v := range t.balanceMap {
		data.balanceMap[k] = v
	}
	return data
}

func (t *TokenBalanceMap) Serialize(w *[]byte) error {
	keys := make([]uint64, 0, len(t.balanceMap))
	num := uint32(0)
	for k, v := range t.balanceMap {
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
		v := t.balanceMap[key]
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
	t.balanceMap = make(map[uint64]*big.Int)
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
		t.balanceMap[k.Uint64()] = v
	}
	return nil
}

type XShardTxCursorInfo struct {
	RootBlockHeight    uint64
	MinorBlockIndex    uint64
	XShardDepositIndex uint64
}
