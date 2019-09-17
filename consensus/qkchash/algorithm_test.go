package qkchash

import (
	"encoding/hex"
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
}

func TestQKCHash(t *testing.T) {

	fakeQkcHashGo := func(a []byte, b qkcCache, c bool) ([]byte, []byte, error) {
		return qkcHashGo(a, b)
	}
	assert := assert.New(t)

	// Failure case, mismatched native flag and hash algo
	cache := generateCache(cacheEntryCnt, nil, false /* not native */)
	seed := make([]byte, 40)
	_, _, err := qkcHashNative(seed, cache, false) // Native
	assert.Error(err,
		"should have error because native cache is not populated by wrong flag")

	// Successful test cases
	testcases := []struct {
		useNative   bool
		qkcHashAlgo func([]byte, qkcCache, bool) ([]byte, []byte, error)
	}{
		{false, fakeQkcHashGo},
		{true, qkcHashNative},
	}
	for _, tc := range testcases {
		cache = generateCache(cacheEntryCnt, nil, tc.useNative)

		seed = make([]byte, 40)
		digest, result, err := tc.qkcHashAlgo(seed, cache, false)
		assert.NoError(err)
		assert.Equal(
			"22da7bf17b573e402c71211a9c96e5631dafcbeda1fc5b7812a2d6529408b207",
			fmt.Sprintf("%x", digest),
		)
		assert.Equal(
			"776fb98b9328713a3d45f5e2e6a3e2238acc55749ad9b4c6d21bfbf8c940ab60",
			fmt.Sprintf("%x", result),
		)

		seed = make([]byte, 40)
		copy(seed, []byte("Hello World!"))
		digest, result, err = tc.qkcHashAlgo(seed, cache, false)
		assert.NoError(err)
		assert.Equal(
			"37e6b7575e9bcf572bb9f4f60baacb738a75d0f1692f3be6c526488d30fe198f",
			fmt.Sprintf("%x", digest),
		)
		assert.Equal(
			"bf36c170967632ce8d55c6bb7f2dafbe1d1a5d94fa542a671362e17f803940ce",
			fmt.Sprintf("%x", result),
		)
	}
}

func TestGetSeedFromBlockNumber(t *testing.T) {
	checkRes := func(height uint64, res string) {
		if res != hex.EncodeToString(getSeedFromBlockNumber(height)) {
			panic("res is not match")
		}
	}
	checkRes(0, "0000000000000000000000000000000000000000000000000000000000000000")
	checkRes(1, "0000000000000000000000000000000000000000000000000000000000000000")
	checkRes(2999, "0000000000000000000000000000000000000000000000000000000000000000")
	checkRes(3000, "0000000000000000000000000000000000000000000000000000000000000000")
	checkRes(3001, "0000000000000000000000000000000000000000000000000000000000000000")
	checkRes(959999, "7e7d8bef9d86983accafa937aed08042391d6f435bc640e50a9c38927de9b299")
	checkRes(960000, "5a46dd85298b0beff65ccf3006c4b5d2f7fa6ca8a5885a7f2132714fe7a48602")
	checkRes(960001, "5a46dd85298b0beff65ccf3006c4b5d2f7fa6ca8a5885a7f2132714fe7a48602")
	checkRes(1919999, "1e484311694bb35f3eb466a15767b60533ec91a7971f53a09837bc946bd861db")
	checkRes(1920000, "4220f7b47dc9e1f91e2d7c117a12e9158ce7a78185c805d21338759838f6f55d")
	checkRes(1920001, "4220f7b47dc9e1f91e2d7c117a12e9158ce7a78185c805d21338759838f6f55d")
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
		seed := make([]byte, 40)
		copy(seed, []byte("HELLOW"))
		_, _, err = qkcHashGo(seed, cache)
	}
	benchErr = err
}

func BenchmarkQKCHashNative(b *testing.B) {
	cache := generateCache(cacheEntryCnt, nil, true)
	var err error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seed := make([]byte, 40)
		copy(seed, []byte("HELLOW"))
		_, _, err = qkcHashNative(seed, cache, false)
		// Note native cache is not destroyed
	}
	benchErr = err
}
