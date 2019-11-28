package types

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"runtime/debug"
	"sort"
	"strings"

	qCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

const (
	TokenTrieThreshold = 16
)

type TokenBalancePair struct {
	TokenID uint64
	Balance *big.Int
}

type TokenBalances struct {
	db        *trie.Database
	tokenTrie *trie.SecureTrie
	balances  map[uint64]*big.Int
}

type TokenBalancesAlias TokenBalances

func (t *TokenBalances) MarshalJSON() ([]byte, error) {
	balances := ""
	for key, val := range t.balances {
		bal := fmt.Sprintf("%d:%d", key, val.Uint64())
		if balances == "" {
			balances = bal
		} else {
			balances += "," + bal
		}
	}
	jsoncfg := struct {
		TokenBalancesAlias
		Balances string `json:"balances"`
	}{TokenBalancesAlias: TokenBalancesAlias(*t), Balances: balances}
	return json.Marshal(jsoncfg)
}

func (t *TokenBalances) UnmarshalJSON(input []byte) error {
	var jsoncfg struct {
		TokenBalancesAlias
		Balances string `json:"balances"`
	}
	if err := json.Unmarshal(input, &jsoncfg); err != nil {
		return err
	}
	*t = TokenBalances(jsoncfg.TokenBalancesAlias)
	t.balances = make(map[uint64]*big.Int)
	balList := strings.Split(jsoncfg.Balances, ",")
	for _, val := range balList {
		var (
			key     int
			balance int
		)
		_, err := fmt.Fscanf(strings.NewReader(val), "%d:%d", &key, &balance)
		if err != nil {
			return err
		}
		t.balances[uint64(key)] = big.NewInt(int64(balance))
	}
	return nil
}

func NewEmptyTokenBalances() *TokenBalances {
	return &TokenBalances{
		balances: make(map[uint64]*big.Int),
	}
}

func NewTokenBalancesWithMap(data map[uint64]*big.Int) *TokenBalances {
	t := &TokenBalances{
		balances: data,
	}
	return t
}

func NewTokenBalances(data []byte, db *trie.Database) (*TokenBalances, error) {
	tokenBalances := NewEmptyTokenBalances()
	tokenBalances.db = db
	if len(data) == 0 {
		return tokenBalances, nil
	}

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
		var err error
		tokenBalances.tokenTrie, err = trie.NewSecure(common.BytesToHash(data[1:]), db, 0)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("Unknown enum byte in token_balances:%v", data[0])

	}
	return tokenBalances, nil
}

func (t *TokenBalances) Commit() {
	if t.notUsingTrie() {
		return
	}
	if t.tokenTrie == nil {
		var err error
		t.tokenTrie, err = trie.NewSecure(common.Hash{}, t.db, 0)
		if err != nil {
			panic(err)
		}
	}
	for tokenID, bal := range t.balances {
		k := qCommon.EncodeInt32(tokenID)
		if bal.Cmp(common.Big0) > 0 {
			val, err := rlp.EncodeToBytes(bal)
			if err != nil {
				panic(err)
			}
			t.tokenTrie.Update(k, val)
		} else {
			t.tokenTrie.Delete(k)
		}
	}
	if _, err := t.tokenTrie.Commit(nil); err != nil {
		panic(err)
	}
	t.balances = make(map[uint64]*big.Int, 0)
}

func (t *TokenBalances) Add(other map[uint64]*big.Int) {
	//TODO only for test? need to delete
	for k, v := range other {
		if data, ok := t.balances[k]; ok {
			t.balances[k] = new(big.Int).Add(v, data)
		} else {
			t.balances[k] = new(big.Int).Set(v)
		}
	}
}

func (t *TokenBalances) SetValue(amount *big.Int, tokenID uint64) {
	if amount.Cmp(new(big.Int)) < 0 {
		panic("serious bug !!!!!!!!!!!!!")
	}
	t.balances[tokenID] = amount
}

func (t *TokenBalances) GetTokenBalance(tokenID uint64) *big.Int {
	data, ok := t.balances[tokenID]
	if ok {
		return data
	}

	if t.tokenTrie != nil {
		v := t.tokenTrie.Get(qCommon.EncodeInt32(tokenID))
		ret := new(big.Int)
		if len(v) != 0 {
			if err := rlp.DecodeBytes(v, ret); err != nil {
				panic(err)
			}
		}
		t.balances[tokenID] = ret
		return ret
	}
	return new(big.Int)
}

func (t *TokenBalances) GetBalanceMap() map[uint64]*big.Int {
	data := t.Copy()
	return data.balances
}

func (t *TokenBalances) Len() int {
	return len(t.balances)
}

func (t *TokenBalances) nonZeroEntriesInBalancesCache() int {
	sum := 0
	for _, v := range t.balances {
		if v.Cmp(common.Big0) > 0 {
			sum++
		}
	}
	return sum
}

func (t *TokenBalances) IsBlank() bool {
	return t.tokenTrie == nil && t.nonZeroEntriesInBalancesCache() == 0
}

func (t *TokenBalances) CopyWithDB() *TokenBalances {
	data := t.Copy()
	data.db = t.db
	data.tokenTrie = t.tokenTrie
	return data
}

func (t *TokenBalances) Copy() *TokenBalances {
	balancesCopy := make(map[uint64]*big.Int)
	for k, v := range t.balances {
		balancesCopy[k] = v
	}
	return &TokenBalances{
		balances: balancesCopy,
	}
}

func (t *TokenBalances) notUsingTrie() bool {
	return t.tokenTrie == nil && t.nonZeroEntriesInBalancesCache() <= TokenTrieThreshold
}

func (t *TokenBalances) SerializeToBytes() ([]byte, error) {
	readyBeforeSer := func() bool {
		if t.notUsingTrie() {
			return true
		}

		if t.tokenTrie != nil && len(t.balances) == 0 {
			return true
		}
		return false
	}
	if !readyBeforeSer() {
		return nil, errors.New("bug here")
	}
	w := make([]byte, 0)
	if t.tokenTrie != nil {
		w = append(w, byte(1))
		w = append(w, t.tokenTrie.Hash().Bytes()...)
		return w, nil
	}

	if t.Len() == 0 {
		return nil, nil
	}
	w = append(w, byte(0))
	list := make([]*TokenBalancePair, 0)
	for k, v := range t.balances {
		if v.Cmp(common.Big0) == 0 {
			continue
		}
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
	return w, nil
}

func (t *TokenBalances) EncodeRLP(w io.Writer) error {
	dataSer, err := t.SerializeToBytes()
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

func (t *TokenBalances) DecodeRLP(s *rlp.Stream) error {
	dataRawBytes, err := s.Raw()
	if err != nil {
		return err
	}
	dataRlp := new([]byte)
	err = rlp.DecodeBytes(dataRawBytes, dataRlp)
	if err != nil {
		panic(err)
	}
	if t.db == nil {
		debug.PrintStack()
		panic("bug here")
	}
	//!!!need to set db before decode
	b, err := NewTokenBalances(*dataRlp, t.db)
	if err != nil {
		return err
	}
	(*t).balances = (*b).balances
	return err
}

func (t *TokenBalances) Serialize(w *[]byte) error {
	keys := make([]uint64, 0, t.Len())
	num := uint32(0)
	for k := range t.balances {
		keys = append(keys, k)
		num++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	if err := serialize.Serialize(w, num); err != nil {
		return err
	}
	for _, key := range keys {
		v := t.balances[key]
		if err := serialize.Serialize(w, new(big.Int).SetUint64(key)); err != nil {
			return err
		}
		if err := serialize.Serialize(w, v); err != nil {
			return err
		}
	}
	return nil
}

func (t *TokenBalances) Deserialize(bb *serialize.ByteBuffer) error {
	t.balances = make(map[uint64]*big.Int)
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
		t.balances[k.Uint64()] = v
	}
	return nil
}
