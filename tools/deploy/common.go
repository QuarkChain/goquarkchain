package deploy

import (
	"crypto/ecdsa"
	"crypto/rand"
	"github.com/ethereum/go-ethereum/crypto"
)

func Checkerr(err error) {
	if err != nil {
		panic(err)
	}
}
func GetPrivateKeyFromConfig() (*ecdsa.PrivateKey, error) {
	sk, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	return sk, err
}
