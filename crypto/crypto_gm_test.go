//+build gm

package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/gos/crypto/sm2"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

var testAddrHex = "d187504d7eef58c1f1d0e72ebd077acfecc9b22e"
var testPrivHex = "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032"

func TestToECDSAErrors(t *testing.T) {
	if _, err := HexToECDSA("0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("HexToECDSA should've returned error")
	}
	if _, err := HexToECDSA("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"); err == nil {
		t.Fatal("HexToECDSA should've returned error")
	}
}


func TestUnmarshalPubkey(t *testing.T) {
	key, err := UnmarshalPubkey(nil)
	if err != errInvalidPubkey || key != nil {
		t.Fatalf("expected error, got %v, %v", err, key)
	}
	key, err = UnmarshalPubkey([]byte{1, 2, 3})
	if err != errInvalidPubkey || key != nil {
		t.Fatalf("expected error, got %v, %v", err, key)
	}

	var (
		enc, _ = hex.DecodeString("04FF6712D3A7FC0D1B9E01FF471A87EA87525E47C7775039D19304E554DEFE0913F632025F692776D4C13470ECA36AC85D560E794E1BCCF53D82C015988E0EB956")
		dec    = &ecdsa.PublicKey{
			Curve: sm2.Sm2Curve(),
			X:     hexutil.MustDecodeBig("0xFF6712D3A7FC0D1B9E01FF471A87EA87525E47C7775039D19304E554DEFE0913"),
			Y:     hexutil.MustDecodeBig("0xF632025F692776D4C13470ECA36AC85D560E794E1BCCF53D82C015988E0EB956"),
		}
	)
	key, err = UnmarshalPubkey(enc)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !reflect.DeepEqual(key, dec) {
		t.Fatal("wrong result")
	}
}

func TestSign(t *testing.T) {
	key, _ := HexToECDSA(testPrivHex)
	addr := common.HexToAddress(testAddrHex)

	msg := SM3([]byte("foo"))
	sig, err := Sign(msg, key)
	if err != nil {
		t.Errorf("Sign error: %s", err)
	}
	recoveredPub, err := Ecrecover(msg, sig)
	if err != nil {
		t.Errorf("ECRecover error: %s", err)
	}
	pubKey, _ := UnmarshalPubkey(recoveredPub)
	recoveredAddr := PubkeyToAddress(*pubKey)
	if addr != recoveredAddr {
		t.Errorf("Address mismatch: want: %x have: %x", addr, recoveredAddr)
	}

	// should be equal to SigToPub
	recoveredPub2, err := SigToPub(msg, sig)
	ecdsaPub := (*ecdsa.PublicKey)(recoveredPub2)
	if err != nil {
		t.Errorf("ECRecover error: %s", err)
	}
	recoveredAddr2 := PubkeyToAddress(*ecdsaPub)
	if addr != recoveredAddr2 {
		t.Errorf("Address mismatch: want: %x have: %x", addr, recoveredAddr2)
	}
}

func TestInvalidSign(t *testing.T) {
	if _, err := Sign(make([]byte, 1), nil); err == nil {
		t.Errorf("expected sign with hash 1 byte to error")
	}
	if _, err := Sign(make([]byte, 33), nil); err == nil {
		t.Errorf("expected sign with hash 33 byte to error")
	}
}

func TestLoadECDSAFile(t *testing.T) {
	keyBytes := common.FromHex(testPrivHex)
	fileName0 := "test_key0"
	fileName1 := "test_key1"
	checkKey := func(k *ecdsa.PrivateKey) {
		assert.Equal(t, PubkeyToAddress(k.PublicKey), common.HexToAddress(testAddrHex))
		loadedKeyBytes := FromECDSA(k)
		if !bytes.Equal(loadedKeyBytes, keyBytes) {
			t.Fatalf("private key mismatch: want: %x have: %x", keyBytes, loadedKeyBytes)
		}
	}

	ioutil.WriteFile(fileName0, []byte(testPrivHex), 0600)
	defer os.Remove(fileName0)

	key0, err := LoadECDSA(fileName0)
	if err != nil {
		t.Fatal(err)
	}
	checkKey(key0)

	// again, this time with SaveECDSA instead of manual save:
	err = SaveECDSA(fileName1, key0)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(fileName1)

	key1, err := LoadECDSA(fileName1)
	if err != nil {
		t.Fatal(err)
	}
	checkKey(key1)
}

func TestPerformance(t *testing.T) {
	privKey, _ := GenerateKey()
	curve := sm2.Sm2Curve()
	curve.IsOnCurve(privKey.X, privKey.Y)

	if !curve.IsOnCurve(privKey.X, privKey.Y) {
		t.Errorf("public key (x, y) is not on Curve")
	}
	hash := common.Hex2Bytes("ce0677bb30baa8cf067c88db9811f4333d131bf8bcf12fe7065d211dce971008")

	fmt.Println("hash: ", common.Bytes2Hex(hash))
	signresult, err := Sign(hash, privKey)
	fmt.Println(common.Bytes2Hex(signresult))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	if !ValidateSignatureValues(signresult[64], new(big.Int).SetBytes(signresult[:32]), new(big.Int).SetBytes(signresult[32:64])) {
		t.Errorf("ValidateSignatureValues failed ")
	}
		VerifySignature( (*sm2.PublicKey)(&privKey.PublicKey).SerializeUncompressed(), hash, signresult )

	pubKey, _ := Ecrecover(hash, signresult)

	x := new(big.Int).SetBytes(pubKey[1:33])
	y := new(big.Int).SetBytes(pubKey[33:])
	if !curve.IsOnCurve(x, y) {
		t.Errorf("recovered public key (x, y) is not on Curve")
	}

	fmt.Println(common.Bytes2Hex(pubKey))
	fmt.Println(common.Bytes2Hex(privKey.PublicKey.X.Bytes()))
	fmt.Println(common.Bytes2Hex(privKey.PublicKey.Y.Bytes()))
}
