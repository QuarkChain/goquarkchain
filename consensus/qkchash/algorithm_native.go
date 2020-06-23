//+build nt

package qkchash

import (
	"encoding/binary"

	"github.com/QuarkChain/goquarkchain/consensus/qkchash/native"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// qkcCache is the union type of cache for qkchash algo.
type qkcCache struct {
	nativeCache native.Cache
	seed        []byte
}

func newCache(seed []byte, ls []uint64) qkcCache {
	return qkcCache{native.NewCache(ls), seed}
}

// qkcHashX calls the native c++ implementation through SWIG.
func qkcHashX(seed []byte, cache qkcCache, useX bool) (digest []byte, result []byte, err error) {
	// Combine header+nonce into a seed
	seed = crypto.Keccak512(seed)
	hashRes, err := native.HashX(cache.nativeCache, seed, useX)

	if err != nil {
		return nil, nil, err
	}

	digest = make([]byte, common.HashLength)
	for i, val := range hashRes {
		binary.LittleEndian.PutUint64(digest[i*8:], val)
	}
	result = crypto.Keccak256(append(seed, digest...))
	return digest, result, nil
}
