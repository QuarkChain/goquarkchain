package state

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
	"math/big"
	"sort"
)

type TokenBalancePair struct {
	TokenID *big.Int
	Balance *big.Int
}

type TokenBalances struct {
	//TODO:store token balances in trie when TOKEN_TRIE_THRESHOLD is crossed
	Balances map[uint64]*big.Int
	enum     byte
}

func NewTokenBalances(data []byte) (*TokenBalances, error) {
	tokenBalances := &TokenBalances{
		Balances: make(map[uint64]*big.Int, 0),
	}
	if len(data) == 0 {
		return tokenBalances, nil
	}

	tokenBalances.enum = data[0]
	switch data[0] {
	case byte(0):
		balanceList := make([]*TokenBalancePair, 0)
		if err := rlp.DecodeBytes(data[1:], balanceList); err != nil {
			return nil, err
		}
		for _, v := range balanceList {
			tokenBalances.Balances[v.TokenID.Uint64()] = v.Balance
		}
	case byte(1):
		return nil, fmt.Errorf("Token balance trie is not yet implemented")
	default:
		return nil, fmt.Errorf("Unknown enum byte in token_balances:%v", data[0])

	}
	return tokenBalances, nil
}

func (b *TokenBalances) Serialize(w *[]byte) error {
	if len(b.Balances) == 0 {
		return nil
	}
	*w = append(*w, b.enum)
	switch b.enum {
	case byte(0):
		list := make([]*TokenBalancePair, 0)
		for k, v := range b.Balances {
			if v.Cmp(new(big.Int)) == 0 {
				continue
			}
			list = append(list, &TokenBalancePair{
				TokenID: new(big.Int).SetUint64(k),
				Balance: v,
			})
		}
		sort.Slice(list, func(i, j int) bool { return list[i].TokenID.Cmp(list[j].TokenID) < 0 })
		rlpData, err := rlp.EncodeToBytes(list)
		if err != nil {
			return err
		}
		*w = append(*w, rlpData...)
	case byte(1):
		return fmt.Errorf("Token balance trie is not yet implemented")
	default:
		return fmt.Errorf("Unknown enum byte in token_balances")

	}
	return nil
}

func (b *TokenBalances) EncodeRLP(w io.Writer) error {
	data, err := serialize.SerializeToBytes(b)
	if err != nil {
		return nil
	}
	_, err = w.Write(data)
	return err
}

func (b *TokenBalances) DecodeRLP(s *rlp.Stream) error {
	data, err := s.Raw()
	if err != nil {
		return err
	}
	t, err := NewTokenBalances(data)
	if err != nil {
		return err
	}
	b = t
	return err
}

func (b *TokenBalances) Balance(tokenID uint64) *big.Int {
	balance, ok := b.Balances[tokenID]
	if !ok {
		return new(big.Int)
	}
	return balance
}

func (b *TokenBalances) IsEmpty() bool {
	flag := true
	for _, v := range b.Balances {
		if v.Cmp(new(big.Int)) != 0 {
			return false
		}
	}
	return flag
}

func (b *TokenBalances) Copy() *TokenBalances {
	t := &TokenBalances{
		Balances: make(map[uint64]*big.Int),
		enum:     b.enum,
	}
	for k, v := range b.Balances {
		t.Balances[k] = v
	}
	return t
}
