package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"reflect"
	"sort"
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

type BlockBy func(b1, b2 IBlock) bool

func (self BlockBy) Sort(blocks Blocks) {
	bs := blockSorter{
		blocks: blocks,
		by:     self,
	}
	sort.Sort(bs)
}

type blockSorter struct {
	blocks Blocks
	by     func(b1, b2 IBlock) bool
}

func (self blockSorter) Len() int { return len(self.blocks) }
func (self blockSorter) Swap(i, j int) {
	self.blocks[i], self.blocks[j] = self.blocks[j], self.blocks[i]
}
func (self blockSorter) Less(i, j int) bool { return self.by(self.blocks[i], self.blocks[j]) }

func Number(b1, b2 IBlock) bool { return b1.IHeader().NumberU64() < b2.IHeader().NumberU64() }
