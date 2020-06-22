package native

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"
)

// Cache is the type of cache in native qkchash impl.
type Cache = *cache

type cache struct {
	ptr *uintptr
}

// NewCache creates the qkchash cache for cpp impl.
func NewCache(rawCache []uint64) Cache {
	nativeCache := Cache_create(rawCache)
	ret := &cache{&nativeCache}
	runtime.SetFinalizer(ret, func(c *cache) {
		if c.ptr != nil {
			Cache_destroy(*c.ptr)
			c.ptr = nil
		}
	})
	return ret
}

// Hashx wraps the native qkchashx algorithm.
func HashWithRotationStats(cache Cache, seed []byte, useX bool) (ret [4]uint64, err error) {
	if cache == nil || cache.ptr == nil {
		return ret, errors.New("invoking native qkchash on empty cache")
	}

	if len(seed) != 64 {
		return ret, fmt.Errorf("invoking native qkchash on invalid seed %v", len(seed))
	}

	var seedArray [8]uint64
	for i := 0; i < 8; i++ {
		seedArray[i] = binary.LittleEndian.Uint64(seed[i*8:])
	}
	if useX {
		Qkc_hash_with_rotation_stats(*cache.ptr, seedArray[:], ret[:])
	} else {
		Qkc_hash(*cache.ptr, seedArray[:], ret[:])
	}

	return ret, nil
}
