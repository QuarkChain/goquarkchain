package main

import (
	"fmt"
	//	"github.com/QuarkChain/goquarkchain/qkcdb"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	//	"github.com/ethereum/go-ethereum/common"
)

func main() {
	// db, err := qkcdb.NewDatabase("/mnt/volume_sgp1_01/data/mainnet/S0/shard-1/db", false, true)
	db, err := NewDatabase("/mnt/volume_sgp1_01/data/mainnet/S0/shard-1/db", false, true)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	fmt.Println(rawdb.ReadHeadBlockHash(db))

	/*	fmt.Println(rawdb.ReadMinorBlock(db, common.HexToHash("0x157f7601150ff5261aee852faaf9938cdeb360c4a56f15eacb56d9be1c93ee0b")))

		receipts := rawdb.ReadReceipts(db, common.HexToHash("0x157f7601150ff5261aee852faaf9938cdeb360c4a56f15eacb56d9be1c93ee0b"))
		if err != nil {
			fmt.Println(err.Error())
			db.Close()
		}

		fmt.Println(receipts)*/
}
