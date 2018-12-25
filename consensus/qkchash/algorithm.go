package qkchash

import (
	"encoding/binary"
	"sort"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	accessRound   = 64
	cacheEntryCnt = 1024 * 64
)

var (
	cacheSeed = []byte{}
)

type qkcCache struct {
	ls  []uint64
	set map[uint64]struct{}
}

// fnv64 is an algorithm inspired by the FNV hash, which in some cases is used as
// a non-associative substitute for XOR. This is 64-bit version.
func fnv64(a, b uint64) uint64 {
	return a*0x100000001b3 ^ b
}

// generateQKCCache is a simpler implementation for generating cache for qkchash.
// Can be improved using a balanced tree.
func generateQKCCache(cnt int, seed []byte) qkcCache {
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
	return qkcCache{ls, set}
}

// qkcHash runs the order-statistics-based hash algorithm.
func qkcHash(hash, nonceBytes []byte, cache qkcCache) (digest []byte, result []byte) {
	const mixBytes = 128
	// Copy the cache since modification is needed
	cacheLs := make([]uint64, len(cache.ls))
	copy(cacheLs, cache.ls)
	cacheSet := make(map[uint64]struct{})
	for k, v := range cache.set {
		cacheSet[k] = v
	}
	// Combine header+nonce into a seed
	seed := append(hash, nonceBytes...)
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
	return digest, result
}
