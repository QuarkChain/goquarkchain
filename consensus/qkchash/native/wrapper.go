package native

import "errors"

// Cache is the type of native qkchash impl.
type Cache struct {
	ptr *uintptr
}

// Destroy calls the underlying cpp func to free the memory.
func (c *Cache) Destroy() {
	if c.ptr == nil {
		panic("destroy non-existent native cache")
	}
	Cache_destroy(*c.ptr)
	c.ptr = nil
}

// NewCache creates the qkchash cache for cpp impl.
func NewCache(rawCache []uint64) *Cache {
	nativeCache := Cache_create(rawCache)
	return &Cache{&nativeCache}
}

// Hash wraps the native qkchash algorithm.
func Hash(cache *Cache, seed [8]uint64) (ret [4]uint64, err error) {
	if cache == nil || cache.ptr == nil {
		return ret, errors.New("invoking native qkchash on empty cache")
	}

	Qkc_hash(*cache.ptr, seed[:], ret[:])
	return ret, nil
}
