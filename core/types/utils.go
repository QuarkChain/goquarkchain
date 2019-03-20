package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"reflect"
)

func IsP2(v uint32) bool {
	return (v & (v - 1)) == 0
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func serHash(val interface{}, excludeList map[string]bool) (h common.Hash) {
	bytes := new([]byte)
	serialize.SerializeStructWithout(reflect.ValueOf(val), bytes, excludeList)
	hw := sha3.NewKeccak256()
	hw.Write(*bytes)
	hw.Sum(h[:0])
	return h
}

type Blocks []IBlock
