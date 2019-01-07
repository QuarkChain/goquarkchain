package qkchash

import (
	"fmt"
	"testing"

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
	assert := assert.New(t)
	cache := generateCache(cacheEntryCnt, nil)
	digest, result := qkcHash([]byte{}, []byte{}, cache)
	assert.Equal(
		"be3913b35b8815a61146f4d8dcce89a2bd8a262698c7263a56abe14b8e29595c",
		fmt.Sprintf("%x", digest),
	)
	assert.Equal(
		"bee2c41fce7183e73823dbefa846439817af68d28891f58ce8331cca7a871504",
		fmt.Sprintf("%x", result),
	)

	digest, result = qkcHash([]byte("Hello World!"), []byte{}, cache)
	assert.Equal(
		"53423968b44d02b1861e9a000d035d74bc0fa515e8f702261bcfcc3e5fb4bd3d",
		fmt.Sprintf("%x", digest),
	)
	assert.Equal(
		"992923af6a261ef3f0b7c82b57ab28c055e31021d0d6258f7dcfd9d6195e0c70",
		fmt.Sprintf("%x", result),
	)
}
