//+build gm

package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/goscn/gos/crypto/sm2"
	"github.com/goscn/gos/crypto/sm3"
	"io"
	"io/ioutil"
	"math/big"
	"os"
)

var errInvalidPubkey = errors.New("invalid sm2 public key")

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	return crypto.Keccak256(data...)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h common.Hash) {
	return crypto.Keccak256Hash(data...)
}

// SM3 calculates and returns the SM3 hash of the input data.
func SM3(data ...[]byte) []byte {
	fmt.Println("----------------------CryptoType:gm; Fun: SM3----------------------")
	s := sm3.NewSM3Hash()
	for _, b := range data {
		s.Write(b)
	}
	return s.Sum(nil)
}

// SM3Hash calculates and returns the SM3 hash of the input data,
// converting it to an internal Hash data structure.
func SM3Hash(data ...[]byte) (h common.Hash) {
	fmt.Println("----------------------CryptoType:gm; Fun: SM3Hash------------------")
	s := sm3.NewSM3Hash()
	for _, b := range data {
		s.Write(b)
	}
	s.Sum(h[:0])
	return h
}

// CreateAddress2 creates an qkc address given the address bytes, initial
//// contract code hash and a salt.
func CreateAddress2(b common.Address, salt [32]byte, inithash []byte) common.Address {
	return common.BytesToAddress(SM3([]byte{0xff}, b.Bytes(), salt[:], inithash)[12:])
}

// ToECDSA creates a private key with the given D value.
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	fmt.Println("----------------------CryptoType:gm; Fun: ToECDSA------------------")
	priv := new(ecdsa.PrivateKey)
	priv.Curve = sm2.Sm2Curve()
	if 8*len(d) != priv.Curve.Params().BitSize {
		return nil, fmt.Errorf("invalid length, need %d bits", priv.Curve.Params().BitSize)
	}
	priv.D = new(big.Int).SetBytes(d)

	// The priv.D must < N
	if priv.D.Cmp(priv.Curve.Params().N) >= 0 {
		return nil, fmt.Errorf("invalid private key, >=N")
	}
	// The priv.D must not be zero or negative.
	if priv.D.Sign() <= 0 {
		return nil, fmt.Errorf("invalid private key, zero or negative")
	}

	priv.PublicKey.X, priv.PublicKey.Y = priv.Curve.ScalarBaseMult(d)
	if priv.PublicKey.X == nil {
		return nil, errors.New("invalid private key")
	}
	return priv, nil

}

// FromECDSA exports a private key into a binary dump.
func FromECDSA(priv *ecdsa.PrivateKey) []byte {
	fmt.Println("----------------------CryptoType:gm; Fun: FromECDSA------------------")
	if priv == nil {
		return nil
	}
	return math.PaddedBigBytes(priv.D, priv.Curve.Params().BitSize/8)
}

// UnmarshalPubkey converts bytes to a public key.
func UnmarshalPubkey(pub []byte) (*ecdsa.PublicKey, error) {
	fmt.Println("----------------------CryptoType:gm; Fun: UnmarshalPubkey------------------")
	x, y := elliptic.Unmarshal(sm2.Sm2Curve(), pub)
	if x == nil {
		return nil, errInvalidPubkey
	}
	return &ecdsa.PublicKey{Curve: sm2.Sm2Curve(), X: x, Y: y}, nil
}

func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	fmt.Println("----------------------CryptoType:gm; Fun: FromECDSAPub------------------")
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return elliptic.Marshal(sm2.Sm2Curve(), pub.X, pub.Y)
}

// HexToECDSA parses a private key.
func HexToECDSA(hexkey string) (*ecdsa.PrivateKey, error) {
	fmt.Println("----------------------CryptoType:gm; Fun: HexToECDSA------------------")
	b, err := hex.DecodeString(hexkey)
	if err != nil {
		return nil, errors.New("invalid hex string")
	}
	return ToECDSA(b)
}

// LoadECDSA loads a private key from the given file.
func LoadECDSA(file string) (*ecdsa.PrivateKey, error) {
	buf := make([]byte, 64)
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	if _, err := io.ReadFull(fd, buf); err != nil {
		return nil, err
	}

	key, err := hex.DecodeString(string(buf))
	if err != nil {
		return nil, err
	}
	return ToECDSA(key)
}

// SaveECDSA saves a private key to the given file with
// restrictive permissions. The key data is saved hex-encoded.
func SaveECDSA(file string, key *ecdsa.PrivateKey) error {
	k := hex.EncodeToString(FromECDSA(key))
	return ioutil.WriteFile(file, []byte(k), 0600)
}

func GenerateKey() (*ecdsa.PrivateKey, error) {
	fmt.Println("----------------------CryptoType:gm; Fun: GenerateKey------------------")
	return sm2.GenerateKey(rand.Reader)
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func ValidateSignatureValues(v byte, r, s *big.Int, homestead bool) bool {
	fmt.Println("----------------------CryptoType:gm; Fun: ValidateSignatureValues------------------")
	curve := sm2.Sm2Curve()
	if r.Cmp(common.Big1) < 0 || s.Cmp(common.Big1) < 0 {
		return false
	}

	// Frontier: allow s to be in full N range
	return r.Cmp(curve.N) < 0 && s.Cmp(curve.N) < 0 && (v == 0 || v == 1)
}
func PubkeyToAddress(p ecdsa.PublicKey) common.Address {
	fmt.Println("----------------------CryptoType:gm; Fun: PubkeyToAddress------------------")
	pubBytes := FromECDSAPub(&p)
	return common.BytesToAddress(SM3(pubBytes[1:])[12:])
}
