package main

import (
	"encoding/hex"
	"fmt"

	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/qkcdb"
	"github.com/QuarkChain/goquarkchain/serialize"
)

func main() {
	str := "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S0/shard-262145/db"
	db, err := qkcdb.NewDatabase(str, false, false)
	if err != nil {
		panic(err)
	}
	fmt.Println("11---")
	it := db.NewIterator()
	it.Seek([]byte{})
	fmt.Println("seek", it.Key(), len(it.Key()), it.Valid())
	distribute := make(map[int]int)
	for it.Valid() {
		fmt.Println("22")
		size := len(it.Key())
		distribute[size]++
		if size == 20 {
			value, err := db.Get(it.Key())
			if err != nil {
				panic(err)
			}
			acc := new(state.Account)
			err1 := serialize.DeserializeFromBytes(value, acc)
			if err1 != nil {
				panic(err1)
			}
			fmt.Println("acc", hex.EncodeToString(it.Key()), acc.TokenBalances.GetBalanceMap())
		}
		it.Next()
	}
	fmt.Println("33")
	for key, value := range distribute {
		fmt.Println(key, value)
	}
}
