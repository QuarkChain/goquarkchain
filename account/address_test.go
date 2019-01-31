package account

import (
	"encoding/hex"
	"fmt"
	"testing"
)

type AddressTestStruct struct {
	Type         string `json:"type"`
	IsEmpty      bool   `json:"isEmpty"`
	ToHex        string `json:"toHex"`
	FullSizeData uint32 `json:"T_FullSize_Data"`
	FullShardID  uint32 `json:"T_FullShardId"`
	BranchData   uint32 `json:"T_Branch_Data"`
	BranchToHex  string `json:"T_Branch_toHex"`
	TShard       uint32 `json:"T_Shard"`
	ShardToHex   string `json:"T_shard_toHex"`
	TKey         string `json:"tKey"`
	FullShardKey uint32 `json:"t_full_shard_key"`
}

func CheckAddressUnitTest(data AddressTestStruct) bool {
	tAddress := Address{}
	switch data.Type {
	case "empty":
		tAddress = CreatEmptyAddress(data.FullShardKey) //test creatEmptyAccount
	case "bs":
		bs, err := hex.DecodeString(data.TKey)
		if err != nil {
			fmt.Println("decodeString bs failed:err", err)
			return false
		}
		tAddress, err = CreatAddressFromBytes(bs) //create address from bs
		if err != nil {
			fmt.Println("create address from bs failed err", err)
			return false
		}
	case "identity":
		tkey, err := hex.DecodeString(data.TKey) //create address from special key
		if err != nil {
			fmt.Println("decodeString tKey failed err", err)
			return false
		}
<<<<<<< HEAD
		keyType,err:=BytesToIdentityKey(tkey)
		if err!=nil{
			fmt.Println("BytesToIdentityKey failed")
			return false
		}
=======
		keyType := BytesToIdentityKey(tkey)
>>>>>>> 08842181cc246dd74f95cd30a4ddc23474a35c80
		tIdentity, err := CreatIdentityFromKey(keyType)
		tAddress = CreatAddressFromIdentity(tIdentity, data.FullShardKey)
	}

	if tAddress.IsEmpty() != data.IsEmpty { //checkIsEmpty
		fmt.Println("tAddress.IsEmpty is not match")
		return false
	}
	toHex := tAddress.ToHex()
	if hex.EncodeToString(toHex) != data.ToHex { //checkToHex
		fmt.Println("toHex is not match")
		return false
	}
	fullShardID, err := tAddress.GetFullShardID(data.FullSizeData) //checkFullSizeData
	if err != nil {
		fmt.Println("GetFullShardID err", err)
		return false
	}

	if fullShardID != data.FullShardID {
		fmt.Println("fullShardId is not match")
		return false
	}

	tBranch := Branch{
<<<<<<< HEAD
		Value:data.BranchData,
=======
		Value: data.BranchData,
>>>>>>> 08842181cc246dd74f95cd30a4ddc23474a35c80
	}
	addressInBranch := tAddress.AddressInBranch(tBranch) //check address's toHex depend addressInBranch
	toHex = addressInBranch.ToHex()
	if hex.EncodeToString(toHex) != data.BranchToHex {
		fmt.Println("addressInBranch.Tohex is not match ")
		return false
	}

	addressInShard := tAddress.AddressInShard(data.TShard) //checkShardIDInBranch
	toHex = addressInShard.ToHex()
	if hex.EncodeToString(toHex) != data.ShardToHex {
		fmt.Printf("addressInShard is not match : unexcepted %s,excepted %s\n", hex.EncodeToString(toHex), data.ShardToHex)
		return false
	}
	return true

}

//1.python generate testdata
//   1.1 empty address
//   1.2 address from bytes
//   1.3 address from special key
//2.go.exe to check
//   2.1 checkIsEmpty
//   2.2 checkToHex
//   2.3 checkFullShardID
//   2.4 checkAddressInBranch
//   2.5 checkShardIDInBranch
func TestAddress(t *testing.T) {
	JSONParse := NewJSONStruct()
	data := []AddressTestStruct{}
	err := JSONParse.Load("./testdata/testAddress.json", &data) //analysis test data
	if err != nil {
		panic(err)
	}
	count := 0
	for _, v := range data {
		if err := CheckAddressUnitTest(v); err == false { //unit test
			panic(err)
		}
		count++
	}
	fmt.Println("TestAddress:success test num:", count)
}
