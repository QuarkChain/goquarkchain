package types

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
	"math/big"
	"sort"
)

type TokenBalancePair struct {
	TokenID uint64
	Balance *big.Int
}

type TokenBalances struct {
	//TODO:store token balances in trie when TOKEN_TRIE_THRESHOLD is crossed
	balances map[uint64]*big.Int
	Enum     byte
	//TODO need to lock balances?
}

func NewEmptyTokenBalances() *TokenBalances {
	return &TokenBalances{
		balances: make(map[uint64]*big.Int),
		Enum:     byte(0),
	}
}

func NewTokenBalancesWithMap(data map[uint64]*big.Int) *TokenBalances {
	t := &TokenBalances{
		balances: data,
		Enum:     byte(0),
	}
	t.checkZero()
	return t
}

func NewTokenBalances(data []byte) (*TokenBalances, error) {
	tokenBalances := NewEmptyTokenBalances()
	if len(data) == 0 {
		return tokenBalances, nil
	}

	tokenBalances.Enum = data[0]
	switch data[0] {
	case byte(0):
		balanceList := make([]*TokenBalancePair, 0)
		if err := rlp.DecodeBytes(data[1:], &balanceList); err != nil {
			return nil, err
		}
		for _, v := range balanceList {
			tokenBalances.balances[v.TokenID] = v.Balance
		}
	case byte(1):
		return nil, fmt.Errorf("Token balance trie is not yet implemented")
	default:
		return nil, fmt.Errorf("Unknown enum byte in token_balances:%v", data[0])

	}
	return tokenBalances, nil
}

func (b *TokenBalances) checkZero() {
	for k, v := range b.balances {
		if v.Cmp(new(big.Int)) == 0 {
			delete(b.balances, k)
		}
	}
}

func (b *TokenBalances) Add(other map[uint64]*big.Int) {
	for k, v := range other {
		if data, ok := b.balances[k]; ok {
			b.balances[k] = new(big.Int).Add(v, data)
		} else {
			b.balances[k] = new(big.Int).Set(v)
		}
	}
	b.checkZero()
}

func (b *TokenBalances) SetValue(amount *big.Int, tokenID uint64) {
	if amount.Cmp(new(big.Int)) < 0 {
		panic("?////////////")
	}
	if amount.Cmp(new(big.Int)) == 0 {
		delete(b.balances, tokenID)
		return
	}
	b.balances[tokenID] = amount
}

func (b *TokenBalances) GetTokenBalance(tokenID uint64) *big.Int {
	data, ok := b.balances[tokenID]
	if !ok {
		return new(big.Int)
	}
	return data
}

func (b *TokenBalances) GetBalanceMap() map[uint64]*big.Int {
	data := b.Copy()
	return data.balances
}

func (b *TokenBalances) Len() int {
	return len(b.balances)
}

func (b *TokenBalances) IsEmpty() bool {
	flag := true
	for _, v := range b.balances {
		if v.Cmp(new(big.Int)) != 0 {
			return false
		}
	}
	return flag
}

func (b *TokenBalances) Copy() *TokenBalances {
	balancesCopy := make(map[uint64]*big.Int)
	for k, v := range b.balances {
		balancesCopy[k] = v
	}
	return &TokenBalances{
		balances: balancesCopy,
		Enum:     b.Enum,
	}
}

func (b *TokenBalances) SerializeToBytes() ([]byte, error) {
	w := make([]byte, 0)
	if b.Len() == 0 {
		return nil, nil
	}
	w = append(w, b.Enum)
	switch b.Enum {
	case byte(0):
		list := make([]*TokenBalancePair, 0)
		for k, v := range b.balances {
			list = append(list, &TokenBalancePair{
				TokenID: k,
				Balance: v,
			})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].TokenID < (list[j].TokenID) })
		rlpData, err := rlp.EncodeToBytes(list)
		if err != nil {
			return nil, err
		}
		w = append(w, rlpData...)
	case byte(1):
		return nil, fmt.Errorf("Token balance trie is not yet implemented")
	default:
		return nil, fmt.Errorf("Unknown enum byte in token_balances")

	}
	return w, nil
}

func (b *TokenBalances) EncodeRLP(w io.Writer) error {
	dataSer, err := b.SerializeToBytes()
	if err != nil {
		return err
	}
	dataRlp, err := rlp.EncodeToBytes(dataSer)
	if err != nil {
		return err
	}
	_, err = w.Write(dataRlp)
	return err
}

func (b *TokenBalances) DecodeRLP(s *rlp.Stream) error {
	dataRawBytes, err := s.Raw()
	if err != nil {
		return err
	}
	dataRlp := new([]byte)
	err = rlp.DecodeBytes(dataRawBytes, dataRlp)
	if err != nil {
		panic(err)
	}
	t, err := NewTokenBalances(*dataRlp)
	if err != nil {
		return err
	}
	(*b).balances = (*t).balances
	(*b).Enum = (*t).Enum
	return err
}
func (b *TokenBalances) Serialize(w *[]byte) error {
	keys := make([]uint64, 0, b.Len())
	num := uint32(0)
	for k, _ := range b.balances {
		keys = append(keys, k)
		num++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	if err := serialize.Serialize(w, num); err != nil {
		return err
	}
	for _, key := range keys {
		v := b.balances[key]
		if err := serialize.Serialize(w, new(big.Int).SetUint64(key)); err != nil {
			return err
		}
		if err := serialize.Serialize(w, v); err != nil {
			return err
		}
	}
	return nil
}

func (b *TokenBalances) Deserialize(bb *serialize.ByteBuffer) error {
	b.balances = make(map[uint64]*big.Int)
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
		if v.Cmp(common.Big0) == 0 {
			continue
		}
		b.balances[k.Uint64()] = v
	}
	return nil
}
