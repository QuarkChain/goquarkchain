package common

import (
	"encoding/hex"
	"golang.org/x/crypto/sha3"
)

// Lengths of hashes and addresses in bytes.
const (
	// quarkchain address length
	QAddrLength = 24
)

/////////// Address

// Address represents the 24 byte address of an Quarkchain account.
type QAddress [QAddrLength]byte

// SetBytes sets the address to the value of b.
// If b is larger than len(a) it will panic.
func (a *QAddress) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-QAddrLength:]
	}
	copy(a[QAddrLength-len(b):], b)
}

func BytesToQAddress(b []byte) QAddress {
	var a QAddress
	a.SetBytes(b)
	return a
}

// Hex returns an EIP55-compliant hex string representation of the address.
func (a QAddress) Hex() string {
	unconsumed := hex.EncodeToString(a[:])
	sha := sha3.NewLegacyKeccak256()
	sha.Write([]byte(unconsumed))
	hash := sha.Sum(nil)

	result := []byte(unconsumed)
	for i := 0; i < len(result); i++ {
		hashByte := hash[i/2]
		if i%2 == 0 {
			hashByte = hashByte >> 4
		} else {
			hashByte &= 0xf
		}
		if result[i] > '9' && hashByte > 7 {
			result[i] -= 32
		}
	}
	return "0x" + string(result)
}
