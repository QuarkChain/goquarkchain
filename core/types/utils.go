package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/sha3"
)

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

func serHash(val interface{}) (h common.Hash) {
	bytes := new([]byte)
	serialize.Serialize(bytes, val)
	hw := sha3.NewKeccak256()
	hw.Write(*bytes)
	hw.Sum(h[:0])
	return h
}
