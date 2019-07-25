//+build !gm

package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"github.com/ethereum/go-ethereum/crypto"
)

const CryptoType = "nogm"

// Ecrecover returns the uncompressed public key that created the given signature.
func Ecrecover(hash, sig []byte) ([]byte, error) {
	return crypto.Ecrecover(hash, sig)
}

// SigToPub returns the public key that created the given signature.
func SigToPub(hash, sig []byte) (*ecdsa.PublicKey, error) {
	return crypto.SigToPub(hash, sig)
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
	return crypto.Sign(hash, prv)
}

// VerifySignature checks that the given public key created signature over hash.
// The public key should be in compressed (33 bytes) or uncompressed (65 bytes) format.
// The signature should have the 64 byte [R || S] format.
func VerifySignature(pubkey, hash, signature []byte) bool {
	return crypto.VerifySignature(pubkey, hash, signature)
}

// DecompressPubkey parses a public key in the 33-byte compressed format.
func DecompressPubkey(pubkey []byte) (*ecdsa.PublicKey, error) {
	return crypto.DecompressPubkey(pubkey)
}

// CompressPubkey encodes a public key to the 33-byte compressed format.
func CompressPubkey(pubkey *ecdsa.PublicKey) []byte {
	return crypto.CompressPubkey(pubkey)
}

// S256 returns an instance of the secp256k1 curve.
func S256() elliptic.Curve {
	return crypto.S256()
}
