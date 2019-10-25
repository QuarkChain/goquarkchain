package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"hash"
	"reflect"
	"sync"
)

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

type hashBuf struct {
	bytes *[]byte
	hw    hash.Hash
}

func newBuf() *hashBuf {
	return &hashBuf{bytes: new([]byte), hw: sha3.NewKeccak256()}
}

func (h *hashBuf) reset() {
	*h.bytes = (*h.bytes)[:0]
	h.hw.Reset()
}

func (h *hashBuf) getHash() (hash common.Hash) {
	h.hw.Write(*h.bytes)
	h.hw.Sum(hash[:0])
	return
}

var (
	bufPool = sync.Pool{
		New: func() interface{} { return &hashBuf{bytes: new([]byte), hw: sha3.NewKeccak256()} },
	}
)

func serHash(val interface{}, excludeList map[string]bool) (h common.Hash) {
	buf := bufPool.Get().(*hashBuf)
	if buf == nil {
		buf = newBuf()
	}
	buf.reset()
	defer bufPool.Put(buf)
	serialize.SerializeStructWithout(reflect.ValueOf(val), buf.bytes, excludeList)
	return buf.getHash()
}
