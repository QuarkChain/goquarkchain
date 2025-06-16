//+build !nt

package qkchash

import (
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/consensus/qkchash/native"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestGenerateCache(t *testing.T) {
	cache := generateCache(cacheEntryCnt, nil)
	assert.Equal(t, cacheEntryCnt, len(cache.ls))
	for i := 1; i < len(cache.ls); i++ {
		assert.True(t, cache.ls[i-1] < cache.ls[i])
	}
	assert.Equal(t, uint64(71869947341538), cache.ls[0])
}

func TestQKCHash(t *testing.T) {
	fakeQkcHashGo := func(a []byte, b *qkcCache, c bool) ([]byte, []byte, error) {
		return qkcHashX(a, b, false)
	}
	assert := assert.New(t)

	cache := generateCache(cacheEntryCnt, nil)
	seed := make([]byte, 40)

	// Successful test cases

	cache = generateCache(cacheEntryCnt, nil)

	seed = make([]byte, 40)
	digest, result, err := fakeQkcHashGo(seed, cache, false)
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
	digest, result, err = fakeQkcHashGo(seed, cache, false)
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

func TestGetSeedFromBlockNumber(t *testing.T) {
	q := New(nil, false, nil, 100)
	checkRes := func(height uint64, res string) {
		if res != hex.EncodeToString(q.cache.getCacheFromHeight(height).seed) {
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
	benchCache *qkcCache
)

func BenchmarkGenerateCache(b *testing.B) {
	var cache *qkcCache
	for i := 0; i < b.N; i++ {
		cache = generateCache(cacheEntryCnt, nil)
	}
	benchCache = cache
}

func BenchmarkQKCHashX(b *testing.B) {
	cache := generateCache(cacheEntryCnt, nil)
	var err error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seed := make([]byte, 40)
		copy(seed, []byte("HELLO"))
		_, _, err = qkcHashX(seed, cache, false)
	}
	benchErr = err
}

func BenchmarkQKCHashX_UseX(b *testing.B) {
	cache := generateCache(cacheEntryCnt, nil)
	var err error
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		seed := make([]byte, 40)
		copy(seed, []byte("HELLO"))
		_, _, err = qkcHashX(seed, cache, true)
	}
	benchErr = err
}

func TestQKCHashXResult(t *testing.T) {
	digest := []byte{137, 242, 237, 21, 58, 130, 196, 151, 10, 128, 37, 84, 90, 57, 27, 133, 37, 239, 50, 72, 74, 245, 145, 134, 210, 94, 30, 55, 205, 247, 248, 24}
	result := []byte{45, 160, 166, 176, 163, 16, 103, 190, 69, 209, 193, 89, 157, 138, 89, 214, 43, 240, 248, 189, 88, 12, 148, 15, 176, 31, 157, 136, 126, 110, 98, 1}
	cache := generateCache(cacheEntryCnt, nil)
	seed := make([]byte, 40)
	copy(seed, []byte("HELLO"))
	digestX, resultX, err := qkcHashX(seed, cache, false)
	assert.NoError(t, err)
	assert.Equal(t, digest, digestX)
	assert.Equal(t, result, resultX)
}

func TestQKCHashXResult_UseX(t *testing.T) {
	digest := []byte{239, 165, 0, 103, 143, 248, 227, 49, 236, 152, 168, 236, 59, 59, 118, 63, 197, 68, 50, 48, 57, 241, 225, 147, 38, 177, 63, 221, 115, 174, 216, 177}
	result := []byte{227, 252, 40, 42, 175, 149, 150, 225, 16, 184, 64, 85, 21, 230, 53, 127, 165, 81, 200, 158, 57, 125, 34, 242, 21, 29, 114, 7, 38, 40, 215, 202}
	cache := generateCache(cacheEntryCnt, nil)
	seed := make([]byte, 40)
	copy(seed, []byte("HELLO"))
	digestX, resultX, err := qkcHashX(seed, cache, true)
	assert.NoError(t, err)
	assert.Equal(t, digest, digestX)
	assert.Equal(t, result, resultX)
}

func TestQkcHashXCompare(t *testing.T) {
	ux := []bool{false, true}
	c := NewcacheSeed()
	tm := time.Now()
	round := uint64(200)
	for i := uint64(0); i < round; i++ {
		cache := c.getCacheFromHeight(i * EpochLength / 2)
		for _, usex := range ux {
			if err := compareQkcHashBetweenGoAndNative(cache, usex); err != nil {
				assert.NoError(t, err, fmt.Sprintf("comparison error for index %d; \r\n\tseed: %v;"+
					"\r\n\tcache: %v; \r\n\tusex: %v;\r\n\terror: %v", i, cache.seed, cache.ls, usex, err.Error()))
				return
			}
		}
	}
	len := uint64(len(c.caches))
	for i := uint64(0); i < len; i++ {
		if i+2*cacheAheadRound < len {
			assert.Nil(t, c.caches[i].ls, "cache should be shrunk")
		} else {
			assert.NotNil(t, c.caches[i].ls, "cache should not be shrunk")
		}
	}
	for i := uint64(0); i < round; i++ {
		cache := c.getCacheFromHeight(i * EpochLength / 2)
		for _, usex := range ux {
			if err := compareQkcHashBetweenGoAndNative(cache, usex); err != nil {
				assert.NoError(t, err,
					fmt.Sprintf("Second round of comparison error for index %d; \r\n\tseed: %v;"+
						"\r\n\tcache: %v; \r\n\tusex: %v;\r\n\terror: %v",
						i, cache.seed, cache.ls, usex, err.Error()))
				return
			}
		}
	}
	fmt.Printf("test done, using %v seconds for %d round", time.Now().Sub(tm).Seconds(), round)
}

func compareQkcHashBetweenGoAndNative(cache *qkcCache, useX bool) error {
	seed := crypto.Keccak512(cache.seed)
	resultN, errN := native.HashWithRotationStats(native.NewCache(cache.ls), seed, useX)
	resultG, errG := HashWithRotationStats(cache.ls, seed, useX)
	if errN != nil {
		return fmt.Errorf("errN: %v", errN.Error())
	}
	if errG != nil {
		return fmt.Errorf("errG: %v", errG.Error())
	}
	for i := 0; i < 4; i++ {
		if resultG[i] != resultN[i] {
			return fmt.Errorf("diff : %v vs %v", resultN, resultG)
		}
	}
	return nil
}
