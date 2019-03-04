package types

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"math/big"
	"sort"
)

type IHeader interface {
	Hash() common.Hash
	SealHash() common.Hash
	NumberU64() uint64
	GetParentHash() common.Hash
	GetCoinbase() account.Address
	GetTime() uint64
	GetDifficulty() *big.Int
	GetNonce() uint64
	GetExtra() []byte
	SetCoinbase(account.Address)
	SetExtra([]byte)
	SetDifficulty(*big.Int)
	SetNonce(uint64)
	GetMixDigest() common.Hash
	ValidateHeader() error
}

type IBlock interface {
	ValidateBlock() error
	Hash() common.Hash
	NumberU64() uint64
	IHeader() IHeader
	WithMingResult(nonce uint64, mixDigest common.Hash) IBlock
	HashItems() []IHashItem
}

type IHashItem interface {
	Hash() common.Hash
}

func IsP2(v uint32) bool {
	return (v & (v - 1)) == 0
}

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
