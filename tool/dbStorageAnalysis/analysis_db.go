package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"os"
	"path/filepath"
	"time"

	"github.com/QuarkChain/goquarkchain/qkcdb"
)

var (
	dbPath = flag.String("path", "", "db_path")
)

type Cal struct {
	Num    int
	Length int
}

func rangeDB(db *qkcdb.QKCDataBase) {
	calMap := make(map[byte]*Cal, 0)
	characterCal := new(Cal)
	sumCal := new(Cal)

	preLen:=0

	it := db.NewIteratorWithPrefix([]byte("cntM"))
	for it.Next() {
		size := len(it.Key())
		firstByte := it.Key()[0]
		size+=len(it.Value())



		if len(it.Value())!=preLen{
			preLen=len(it.Value())
			infoList := new(account.CoinbaseStatses)
			if err := serialize.DeserializeFromBytes(it.Value(), infoList); err != nil {
				panic(err) //TODO delete later unexpected err
			}
			fmt.Println("====-start")
			for _,v:=range infoList.CoinbaseStatsList{
				fmt.Println("vvv",v.Addr.String(),v.Cnt,hex.EncodeToString(it.Key()))
			}
			fmt.Println("====-end")
			time.Sleep(3*time.Second)
		}
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
		//if sumCal.Num%1000000 == 0 {
			fmt.Println( sumCal.Num,sumCal.Length,len(it.Key()),len(it.Value()), string(firstByte),hex.EncodeToString(it.Key()))
		//}

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

func DirSizeB(path string) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("path:", path, "size-B:", size, "size-KB:", size/1024, "size-MB:", size/1024/1024, "size-GB:", size/1024/1024/1024)
}

// eg: go run analysis_db.go --path=/mnt/hgfs/GOPATH/UbuntuTest/MainnetTest/qkc-data/mainnet/S0/shard-1/db
// eg: go run analysis_db.go --path=/mnt/hgfs/GOPATH/UbuntuTest/MainnetTest/qkc-data/mainnet/master/db
func main() {
	flag.Parse()

	dbfile := *dbPath
	if dbfile == "" {
		panic("please set right dbfile")
	}
	db, err := qkcdb.NewDatabase(dbfile, false, false)
	if err != nil {
		panic(db)
	}
	rangeDB(db)
	//DirSizeB(*dbPath)
}
