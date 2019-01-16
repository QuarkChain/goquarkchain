package account

import (
	"encoding/hex"
	"fmt"
	"testing"
)

type IdentityTestStruct struct {
	Key       string `json:"key"`
	IDKey     string `json:"idkey"`
	Recipient string `json:"recipient"`
}

func CheckIdentityUnitTest(data IdentityTestStruct) bool {
	key, err := hex.DecodeString(data.Key)
	if err != nil {
		fmt.Println("DecodeString failed err", err)
		return false
	}
	identity, err := CreatIdentityFromKey(BytesToIdentityKey(key))
	if err != nil {
		fmt.Println("creatFromKey Failed err", err)
		return false
	}
	if hex.EncodeToString(identity.GetRecipient().Bytes()) != data.Recipient { //checkRecipent
		fmt.Printf("recipient is not match : unexcepted:%s , excepted %s", hex.EncodeToString(identity.GetRecipient().Bytes()), data.Recipient)
		return false
	}
	return true
}

//1.python generate testdata
//   1.1 identity from key
//2.go.exe to check
//   2.1 checkRecipent
func TestIdentity(t *testing.T) {
	JSONParse := NewJSONStruct()
	v := []IdentityTestStruct{}
	err := JSONParse.Load("./testdata/testIdentity.json", &v) //analysis test data
	if err != nil {
		panic(err)
	}
	count := 0
	for _, v := range v {
		err := CheckIdentityUnitTest(v) //unit test
		if err == false {
			panic(err)
		}
		count++
	}
	fmt.Println("TestIdentity:success test num:", count)

}
