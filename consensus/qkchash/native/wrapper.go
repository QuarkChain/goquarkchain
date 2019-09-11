package native

import (
	"errors"
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

// Hash wraps the native qkchash algorithm.
func Hash(cache Cache, seed [8]uint64) (ret [4]uint64, err error) {
	if cache == nil || cache.ptr == nil {
		return ret, errors.New("invoking native qkchash on empty cache")
	}

	Qkc_hash(*cache.ptr, seed[:], ret[:])
	return ret, nil
}

// Hashx wraps the native qkchashx algorithm.
func HashWithRotationStats(cache Cache, seed [8]uint64) (ret [4]uint64, err error) {
	if cache == nil || cache.ptr == nil {
		return ret, errors.New("invoking native qkchash on empty cache")
	}

	Qkc_hash_with_rotation_stats(*cache.ptr, seed[:], ret[:])
	return ret, nil
}
