package main

import (
	"encoding/hex"
	"fmt"

	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/qkcdb"
	"github.com/QuarkChain/goquarkchain/serialize"
)

func main() {
	paths := make([]string, 8)
	paths[0] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S0/shard-1/db"
	paths[1] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S1/shard-65537/db"
	paths[2] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S2/shard-131073/db"
	paths[3] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S3/shard-196609/db"
	paths[4] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S0/shard-262145/db"
	paths[5] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S1/shard-327681/db"
	paths[6] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S2/shard-393217/db"
	paths[7] = "/home/gocode/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet/S3/shard-458753/db"

	for idx, path := range paths {
		GetBalances(idx, path)
	}
}

func GetBalances(idx int, path string) {
	fmt.Println("--------", idx)
	db, err := qkcdb.NewDatabase(path, false, false)
	if err != nil {
		panic(err)
	}
	fmt.Println("11---")
	it := db.NewIterator()
	it.Seek([]byte{})
	fmt.Println("seek", it.Key(), len(it.Key()), it.Valid())
	distribute := make(map[int]int)
	for it.Valid() {
		//fmt.Println("22")
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
	fmt.Println("map len", len(distribute))
	for key, value := range distribute {
		fmt.Println(key, value)
	}
}
