//+build !gm

package crypto

import (
	"crypto/ecdsa"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
)

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	return crypto.Keccak256(data...)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h common.Hash) {
	return crypto.Keccak256Hash(data...)
}

// CreateAddress2 creates an qkc address given the address bytes, initial
//// contract code hash and a salt.
func CreateAddress2(b common.Address, salt [32]byte, inithash []byte) common.Address {
	return crypto.CreateAddress2(b, salt, inithash)
}

// ToECDSA creates a private key with the given D value.
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	return crypto.ToECDSA(d)
}

// FromECDSA exports a private key into a binary dump.
func FromECDSA(priv *ecdsa.PrivateKey) []byte {
	return crypto.FromECDSA(priv)
}

// UnmarshalPubkey converts bytes to a public key.
func UnmarshalPubkey(pub []byte) (*ecdsa.PublicKey, error) {
	return crypto.UnmarshalPubkey(pub)
}

func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	return crypto.FromECDSAPub(pub)
}

// HexToECDSA parses a private key.
func HexToECDSA(hexkey string) (*ecdsa.PrivateKey, error) {
	return crypto.HexToECDSA(hexkey)
}

// LoadECDSA loads a private key from the given file.
func LoadECDSA(file string) (*ecdsa.PrivateKey, error) {
	return crypto.LoadECDSA(file)
}

// SaveECDSA saves a private key to the given file with
// restrictive permissions. The key data is saved hex-encoded.
func SaveECDSA(file string, key *ecdsa.PrivateKey) error {
	return crypto.SaveECDSA(file, key)
}

func GenerateKey() (*ecdsa.PrivateKey, error) {
	return crypto.GenerateKey()
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func ValidateSignatureValues(v byte, r, s *big.Int, homestead bool) bool {
	return crypto.ValidateSignatureValues(v, r, s, homestead)
}

func PubkeyToAddress(p ecdsa.PublicKey) common.Address {
	return crypto.PubkeyToAddress(p)
}
