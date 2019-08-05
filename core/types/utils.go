package types

import (
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"reflect"
)

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func serHash(val interface{}, excludeList map[string]bool) (h common.Hash) {
	bytes := new([]byte)
	serialize.SerializeStructWithout(reflect.ValueOf(val), bytes, excludeList)
	return crypto.Hash256Hash(*bytes)
}
