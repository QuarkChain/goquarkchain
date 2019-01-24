package account

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
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
	keyType, err := BytesToIdentityKey(key)
	if err != nil {
		fmt.Println("BytesToIdentityKey err")
		return false
	}

	identity, err := CreatIdentityFromKey(keyType)
	if err := checkPublicToRecipient(keyType, identity.Recipient); err != nil {
		fmt.Println("checkPublicToRecipient err", err)
		return false
	}

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

func checkPublicToRecipient(key Key, recipient Recipient) error {
	keyValue := big.NewInt(0)
	keyValue.SetBytes(key.Bytes())
	sk := new(ecdsa.PrivateKey)
	sk.PublicKey.Curve = crypto.S256()
	sk.D = keyValue
	sk.PublicKey.X, sk.PublicKey.Y = crypto.S256().ScalarBaseMult(keyValue.Bytes())

	recipientData, err := PublicKeyToRecipient(sk.PublicKey)
	if err != nil {
		return err
	}
	if bytes.Equal(recipientData.Bytes(), recipient.Bytes()) {
		return nil
	}
	return errors.New("check public to recipient failed")
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
