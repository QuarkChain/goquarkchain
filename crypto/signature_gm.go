//+build gm

package crypto

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/QuarkChain/gos/crypto/sm2"
	"math/big"
)

const CryptoType = "gm"

var (
	halfN = new(big.Int).Rsh(sm2.Sm2Curve().N, 1)
)

// Ecrecover returns the uncompressed public key that created the given signature.
func Ecrecover(hash, sig []byte) ([]byte, error) {
	pub, err := SigToPub(hash, sig)
	if err != nil {
		return nil, err
	}
	bytes := pub.SerializeUncompressed()
	return bytes, err
}

// SigToPub returns the public key that created the given signature.
func SigToPub(hash, sig []byte) (*sm2.PublicKey, error) {
	// Convert to btcec input format with 'recovery id' v at the beginning.
	sign := make([]byte, 65)
	sign[0] = sig[64] + 27
	copy(sign[1:], sig)

	pub, _, err := sm2.RecoverCompact(sm2.Sm2Curve(), sign, hash)
	return pub, err
}

// Sm2Sign calculates an ECDSA signature.
//
// This function is susceptible to chosen plaintext attacks that can leak
// information about the private key that is used for signing. Callers must
// be aware that the given hash cannot be chosen by an adversery. Common
// solution is to hash any input before calculating the signature.
//
// The produced signature is in the [R || S || V] format where V is 0 or 1.
func Sign(hash []byte, prv *ecdsa.PrivateKey) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash is required to be exactly 32 bytes (%d)", len(hash))
	}
	if prv.Curve != sm2.Sm2Curve() {
		return nil, fmt.Errorf("private key curve is not Sm2Curve")
	}
	sig, err := sm2.SignCompact(sm2.Sm2Curve(), (*sm2.PrivateKey)(prv), hash, false)
	if err != nil {
		return nil, err
	}
	// Convert to Ethereum signature format with 'recovery id' v at the end.
	v := sig[0] - 27
	copy(sig, sig[1:])
	sig[64] = v
	return sig, nil
}

// VerifySignature checks that the given public key created signature over hash.
// The public key should be in compressed (33 bytes) or uncompressed (65 bytes) format.
// The signature should have the 64 byte [R || S] format.
func VerifySignature(pubkey, hash, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}
	r := new(big.Int).SetBytes(signature[:32])
	s := new(big.Int).SetBytes(signature[32:])
	key, err := sm2.ParsePubKey(pubkey)
	if err != nil {
		return false
	}
	// Reject malleable signatures.
	if s.Cmp(halfN) > 0 {
		return false
	}
	return ecdsa.Verify((*ecdsa.PublicKey)(key), hash, r, s)
}

// DecompressPubkey parses a public key in the 33-byte compressed format.
func DecompressPubkey(pubkey []byte) (*ecdsa.PublicKey, error) {
	if len(pubkey) != 33 {
		return nil, errors.New("invalid compressed public key length")
	}
	key, err := sm2.ParsePubKey(pubkey)
	if err != nil {
		return nil, err
	}
	return (*ecdsa.PublicKey)(key), nil
}

// CompressPubkey encodes a public key to the 33-byte compressed format.
func CompressPubkey(pubkey *ecdsa.PublicKey) []byte {
	return (*sm2.PublicKey)(pubkey).SerializeCompressed()
}

// S256 returns an instance of the sm2 curve.
func S256() elliptic.Curve {
	return sm2.Sm2Curve()
}
