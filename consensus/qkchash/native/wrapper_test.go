package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCacheCreateAndDestroy(t *testing.T) {
	cache := NewCache([]uint64{})
	assert.NotNil(t, cache.ptr)

	cache.Destroy()
	assert.Nil(t, cache.ptr)
}

func TestHash(t *testing.T) {
	assert := assert.New(t)

	// Failure case
	cache := &Cache{nil}
	_, err := Hash(cache, [8]uint64{}) // Invalid cache
	assert.Error(err)

	// Success
	rawCache := make([]uint64, 1024*64)
	// Fake cache
	for i := 0; i < len(rawCache); i++ {
		rawCache[i] = uint64(i * 2)
	}
	cache = NewCache(rawCache)
	defer cache.Destroy()

	seed := [8]uint64{}
	for i := 0; i < len(seed); i++ {
		seed[i] = uint64(i + 1)
	}

	ret, err := Hash(cache, seed)
	// Verified with cpp-py version
	expected := [4]uint64{0x720e8da04bf3b232, 0x8dbfff09fd460b1c, 0xa6355a7ce4041df8, 0x968d37a76ffa20ee}
	assert.NoError(err)
	assert.ElementsMatch(expected, ret)
}
