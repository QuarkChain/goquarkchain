package qkchash

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/QuarkChain/goquarkchain/consensus/qkchash/native"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	accessRound   = 64
	cacheEntryCnt = 1024 * 64
)

var (
	cacheSeed = common.Hash{}
)

// qkcCache is the union type of cache for qkchash algo.
// Note in Go impl, `nativeCache` will be empty.
type qkcCache struct {
	ls          []uint64
	set         map[uint64]struct{}
	nativeCache native.Cache
}

// fnv64 is an algorithm inspired by the FNV hash, which in some cases is used as
// a non-associative substitute for XOR. This is 64-bit version.
func fnv64(a, b uint64) uint64 {
	return a*0x100000001b3 ^ b
}

// generateCache generates cache for qkchash. Will also generate underlying cache
// in native c++ impl if needed.
func generateCache(cnt int, seed []byte, genNativeCache bool) qkcCache {
	fmt.Println("Seed", hex.EncodeToString(seed), len(seed))
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

	if genNativeCache {
		return qkcCache{nil, nil, native.NewCache(ls)}
	}

	// Sort is needed for Go impl
	sort.Slice(ls, func(i, j int) bool { return ls[i] < ls[j] })
	return qkcCache{ls, set, nil}
}

// qkcHashNative calls the native c++ implementation through SWIG.
func qkcHashNative(seed []byte, cache qkcCache) (digest []byte, result []byte, err error) {
	// Combine header+nonce into a seed
	seed = crypto.Keccak512(seed)
	var seedArray [8]uint64
	for i := 0; i < 8; i++ {
		seedArray[i] = binary.LittleEndian.Uint64(seed[i*8:])
	}
	hashRes, err := native.Hash(cache.nativeCache, seedArray)
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

// qkcHashGo is the Go implementation.
func qkcHashGo(seed []byte, cache qkcCache) (digest []byte, result []byte, err error) {
	const mixBytes = 128
	// Copy the cache since modification is needed
	cacheLs := make([]uint64, len(cache.ls))
	copy(cacheLs, cache.ls)
	fmt.Println("LLLLLLLLLLL", len(cache.ls), cache.ls)
	cacheSet := make(map[uint64]struct{})
	for k, v := range cache.set {
		cacheSet[k] = v
	}
	seed = crypto.Keccak512(seed)
	seedHead := binary.LittleEndian.Uint64(seed)

	// Start the mix with replicated seed
	mix := make([]uint64, mixBytes/8)
	for i := 0; i < len(mix); i++ {
		mix[i] = binary.LittleEndian.Uint64(seed[i%8*8:])
	}

	// TODO: can be improved using balanced tree
	for i := 0; i < accessRound; i++ {
		newData := make([]uint64, mixBytes/8)
		p := fnv64(uint64(i)^seedHead, mix[i%len(mix)])
		for j := 0; j < len(mix); j++ {
			idx := p % uint64(len(cacheLs))
			v := cacheLs[idx]
			newData[j] = v
			cacheLs = append(cacheLs[:idx], cacheLs[idx+1:]...)
			delete(cacheSet, v)

			// Generate a random item and insert
			p = fnv64(p, v)
			if _, ok := cacheSet[p]; !ok {
				// Insert in a sorted order
				sz := len(cacheLs)
				idxLess := sort.Search(sz, func(i int) bool { return cacheLs[i] > p })
				switch idxLess {
				case 0:
					cacheLs = append([]uint64{p}, cacheLs...)
				case sz:
					cacheLs = append(cacheLs, p)
				default:
					cacheLs = append(cacheLs, 0)
					copy(cacheLs[idxLess+1:], cacheLs[idxLess:])
					cacheLs[idxLess] = p
				}
				cacheSet[p] = struct{}{}
			}
			// Continue next search
			p = fnv64(p, v)
		}

		for j := 0; j < len(mix); j++ {
			mix[j] = fnv64(mix[j], newData[j])
		}
	}
	fmt.Println("mixxxxxx", mix)
	// Compress mix
	for i := 0; i < len(mix); i += 4 {
		mix[i/4] = fnv64(fnv64(fnv64(mix[i], mix[i+1]), mix[i+2]), mix[i+3])
	}
	mix = mix[:len(mix)/4]

	digest = make([]byte, common.HashLength)
	for i, val := range mix {
		binary.LittleEndian.PutUint64(digest[i*8:], val)
	}
	result = crypto.Keccak256(append(seed, digest...))
	return digest, result, nil
}
