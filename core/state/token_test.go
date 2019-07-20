package state

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	"testing"
)

type AAccount struct {
	Nonce         uint64
	TokenBalances *TokenBalances
	Root          common.Hash // merkle root of the storage trie
	CodeHash      []byte
	FullShardKey  uint32
}

func TestRLP(t *testing.T) {
	t1 := AAccount{
		Nonce:         9,
		TokenBalances: NewEmptyTokenBalances(),
		Root:          common.BytesToHash(new(big.Int).SetUint64(7).Bytes()),
		CodeHash:      []byte{1},
		FullShardKey:  2,
	}
	//t1.TokenBalances.Balances[1] = new(big.Int).SetUint64(1)
	//t1.TokenBalances.Balances[2] = new(big.Int).SetUint64(2)
	//t1.TokenBalances.Enum = byte(0)

	data, err := rlp.EncodeToBytes(t1)
	fmt.Println("err", err, "data", hex.EncodeToString(data))

	tt1 := new(AAccount)
	err = rlp.DecodeBytes(data, tt1)

	fmt.Println("err", err)
	fmt.Println("Nonce", tt1.Nonce)
	fmt.Println("TokenBalances", tt1.TokenBalances)
	fmt.Println("Root", tt1.Root.String())
	fmt.Println("CodeHash", tt1.CodeHash)
	fmt.Println("FullShardKey", tt1.FullShardKey)

}
