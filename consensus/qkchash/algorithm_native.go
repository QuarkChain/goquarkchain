//+build nt

package qkchash

import (
	"encoding/binary"
	"sort"
	"sync"

	"github.com/QuarkChain/goquarkchain/consensus/qkchash/native"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	cacheEntryCnt   = 1024 * 64
	cacheAheadRound = 64 // 64*30000
)

var (
	EpochLength = uint64(30000) //blocks pre epoch
)

type cacheSeed struct {
	mu     sync.RWMutex
	caches []qkcCache
}

func NewcacheSeed() *cacheSeed {
	firstCache := generateCache(cacheEntryCnt, common.Hash{}.Bytes())
	caches := make([]qkcCache, 0)
	caches = append(caches, firstCache)
	return &cacheSeed{
		caches: caches,
	}
}

func (c *cacheSeed) getCacheFromHeight(block uint64) qkcCache {
	epoch := int(block / EpochLength)
	lenCaches := len(c.caches)
	if epoch < lenCaches {
		return c.caches[epoch]
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	needAddCnt := epoch - lenCaches + cacheAheadRound
	seed := c.caches[len(c.caches)-1].seed
	for i := 0; i < needAddCnt; i++ {
		seed = crypto.Keccak256(seed)
		c.caches = append(c.caches, generateCache(cacheEntryCnt, seed))
	}
	return c.caches[epoch]
}

// qkcCache is the union type of cache for qkchash algo.
// Note in Go impl, `nativeCache` will be empty.
type qkcCache struct {
	nativeCache native.Cache
	seed        []byte
}

// generateCache generates cache for qkchash. Will also generate underlying cache
// in native c++ impl if needed.
func generateCache(cnt int, seed []byte) qkcCache {
	ls := []uint64{}
	set := make(map[uint64]struct{})
	for i := uint32(0); i < uint32(cnt/8); i++ {
		iBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(iBytes, i)
		bs := crypto.Keccak512(append(seed, iBytes...))
		// Read 8 bytes as uint64
		for j := 0; j < len(bs); j += 8 {
			ele := binary.LittleEndian.Uint64(bs[j:])
			if _, ok := set[ele]; !ok {
				ls = append(ls, ele)
				set[ele] = struct{}{}
			}
		}
	}
	sort.Slice(ls, func(i, j int) bool { return ls[i] < ls[j] })
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
