package main

import (
	"flag"
	"fmt"

	"github.com/QuarkChain/goquarkchain/qkcdb"
)

var (
	dbPath = flag.String("path", "", "db_path")
)

type Cal struct {
	Num    int
	Length int
}

func rangeDB(db *qkcdb.RDBDatabase) {
	calMap := make(map[byte]*Cal, 0)
	characterCal := new(Cal)
	sumCal := new(Cal)

	it := db.NewIterator()
	it.SeekToFirst()
	for it.Valid() {
		size := it.Value().Size()
		firstByte := it.Key().Data()[0]

		sumCal.Num++
		sumCal.Length += size
		if (firstByte >= 'a' && firstByte <= 'z') || (firstByte >= 'A' && firstByte <= 'Z') {
			characterCal.Num++
			characterCal.Length += size
			if _, ok := calMap[firstByte]; !ok {
				calMap[firstByte] = new(Cal)
			}
			calMap[firstByte].Num++
			calMap[firstByte].Length += size
		}
		it.Next()
		if sumCal.Num%1000000 == 0 {
			fmt.Println("currIndexSum", sumCal.Num, "currIndex", firstByte)
		}
	}

	for index := 'a'; index <= 'z'; index++ {
		fmt.Println("character", string(index), "data info", calMap[byte(index)])
	}
	for index := 'A'; index <= 'Z'; index++ {
		fmt.Println("character", string(index), "data info", calMap[byte(index)])
	}

	fmt.Println("AllInfo  ", "key num", sumCal.Num, "value all length", sumCal.Length)
	fmt.Println("Character", "key num", characterCal.Num, "value all length", characterCal.Length)
}

// eg: go run analysis_db.go --path=/mnt/hgfs/GOPATH/UbuntuTest/MainnetTest/qkc-data/mainnet/S0/shard-1/db
// eg: go run analysis_db.go --path=/mnt/hgfs/GOPATH/UbuntuTest/MainnetTest/qkc-data/mainnet/master/db
func main() {
	flag.Parse()

	dbfile := *dbPath
	if dbfile == "" {
		panic("please set right dbfile")
	}
	db, err := qkcdb.NewRDBDatabase(dbfile, false, false)
	if err != nil {
		panic(db)
	}
	rangeDB(db)
}
