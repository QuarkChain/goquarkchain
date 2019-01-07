package qkchash

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateCache(t *testing.T) {
	cache := generateCache(cacheEntryCnt, nil, false /* not native */)
	assert.Equal(t, cacheEntryCnt, len(cache.ls))
	for i := 1; i < len(cache.ls); i++ {
		assert.True(t, cache.ls[i-1] < cache.ls[i])
	}
	assert.Equal(t, uint64(71869947341538), cache.ls[0])
	assert.Nil(t, cache.nativeCache)

	cache = generateCache(cacheEntryCnt, nil, true /* gen native cache*/)
	assert.Nil(t, cache.ls)
	assert.Nil(t, cache.set)
	assert.NotNil(t, cache.nativeCache)
	cache.nativeCache.Destroy()
}

func TestQKCHash(t *testing.T) {
	assert := assert.New(t)

	// Failure case, mismatched native flag and hash algo
	cache := generateCache(cacheEntryCnt, nil, false /* not native */)
	_, _, err := qkcHashNative([]byte{}, []byte{}, cache) // Native
	assert.Error(err,
		"should have error because native cache is not populated by wrong flag")

	// Successful test cases
	testcases := []struct {
		useNative   bool
		qkcHashAlgo func([]byte, []byte, qkcCache) ([]byte, []byte, error)
	}{
		{false, qkcHashGo},
		{true, qkcHashNative},
	}
	for _, tc := range testcases {
		cache = generateCache(cacheEntryCnt, nil, tc.useNative)
		if cache.nativeCache != nil {
			defer cache.nativeCache.Destroy()
		}

		digest, result, err := tc.qkcHashAlgo([]byte{}, []byte{}, cache)
		assert.NoError(err)
		assert.Equal(
			"be3913b35b8815a61146f4d8dcce89a2bd8a262698c7263a56abe14b8e29595c",
			fmt.Sprintf("%x", digest),
		)
		assert.Equal(
			"bee2c41fce7183e73823dbefa846439817af68d28891f58ce8331cca7a871504",
			fmt.Sprintf("%x", result),
		)

		digest, result, err = tc.qkcHashAlgo([]byte("Hello World!"), []byte{}, cache)
		assert.NoError(err)
		assert.Equal(
			"53423968b44d02b1861e9a000d035d74bc0fa515e8f702261bcfcc3e5fb4bd3d",
			fmt.Sprintf("%x", digest),
		)
		assert.Equal(
			"992923af6a261ef3f0b7c82b57ab28c055e31021d0d6258f7dcfd9d6195e0c70",
			fmt.Sprintf("%x", result),
		)
	}
}

// Use following to avoid compiler optimization
var (
	benchErr   error
	benchCache qkcCache
)

func BenchmarkGenerateCacheGo(b *testing.B) {
	var cache qkcCache
	for i := 0; i < b.N; i++ {
		cache = generateCache(cacheEntryCnt, nil, false /* not native */)
	}
	benchCache = cache
}

func BenchmarkGenerateCacheNative(b *testing.B) {
	var cache qkcCache
	for i := 0; i < b.N; i++ {
		cache = generateCache(cacheEntryCnt, nil, false /* not native */)
		// Note native cache is not destroyed
	}
	benchCache = cache
}

func BenchmarkQKCHashGo(b *testing.B) {
	cache := generateCache(cacheEntryCnt, nil, false)
	var err error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err = qkcHashGo([]byte("HELLOW"), []byte{}, cache)
	}
	benchErr = err
}

func BenchmarkQKCHashNative(b *testing.B) {
	cache := generateCache(cacheEntryCnt, nil, true)
	var err error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err = qkcHashNative([]byte("HELLOW"), []byte{}, cache)
		// Note native cache is not destroyed
	}
	benchErr = err
}
