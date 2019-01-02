// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// package minorchain contains data types related to Ethereum consensus.
package types

import (
	"encoding/binary"
	"io"
	"math/big"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/syndtr/goleveldb/leveldb/errors"
)

type Branch struct {
	ShardSize   uint32
	ShardId     uint32
}

func (b *Branch)IsInShard(fullShardId uint32) bool {
	return (fullShardId & (b.ShardSize - 1)) == b.ShardId
}

func CreateBranch(shardSize, shardId uint32) (*Branch, error)  {
	if IsP2(shardSize){
		return nil, errors.New("shard size should be power of 2")
	}

	if shardSize <= shardId{
		return nil, errors.New("shardId should larger than shard size")
	}

	return &Branch{ShardId:shardId, ShardSize:shardSize}, nil
}
//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// MinorBlockHeader represents a minor block header in the QuarkChain.
type MinorBlockHeader struct {
	Version           uint32              `json:"version"                    gencodec:"required"`
	Branch            Branch              `json:"branch"                     gencodec:"required"`
	Number            *big.Int            `json:"number"                     gencodec:"required"`
	Coinbase          common.Address      `json:"miner"                      gencodec:"required"`
	CoinbaseAmount    *big.Int            `json:"coinbaseAmount"             gencodec:"required"`
	ParentHash        common.Hash         `json:"parentHash"                 gencodec:"required"`
	PrevRootBlockHash common.Hash         `json:"prevRootBlockHash"          gencodec:"required"`
	GasLimit          *big.Int            `json:"gasLimit"                   gencodec:"required"`
	MetaHash          common.Hash         `json:"metaHash"                   gencodec:"required"`
	Time              *big.Int            `json:"timestamp"                  gencodec:"required"`
	Difficulty        *big.Int            `json:"difficulty"                 gencodec:"required"`
	Nonce             types.BlockNonce    `json:"nonce"`
	Bloom             Bloom               `json:"logsBloom"                  gencodec:"required"`
	Extra             []byte              `json:"extraData"                  gencodec:"required"`
	MixDigest         common.Hash         `json:"mixHash"`
}

type MinorBlockMeta struct {
	Root              common.Hash          `json:"stateRoot"                  gencodec:"required"`
	TxHash            common.Hash          `json:"transactionsRoot"           gencodec:"required"`
	ReceiptHash       common.Hash          `json:"receiptsRoot"               gencodec:"required"`
	GasUsed           *big.Int             `json:"gasUsed"                    gencodec:"required"`
	CrossShardGasUsed *big.Int             `json:"crossShardGasUsed"          gencodec:"required"` // ("evm_cross_shard_receive_gas_used", uint256) in pyquarkchain
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *MinorBlockHeader) Hash() common.Hash {
	return RlpHash(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *MinorBlockHeader) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) +
		common.StorageSize(len(h.Extra)+
			(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen() + h.GasLimit.BitLen() + h.CoinbaseAmount.BitLen())/8)
}

// MinorBlockHeaders is a MinorBlockHeader slice type for basic sorting.
type MinorBlockHeaders  []*MinorBlockHeader

// Len returns the length of s.
func (s MinorBlockHeaders) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s MinorBlockHeaders) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s MinorBlockHeaders) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// MinorBlock represents an entire block in the Ethereum blockchain.
type MinorBlock struct {
	header       *MinorBlockHeader
	meta         *MinorBlockMeta
	transactions Transactions

	// caches
	hash atomic.Value
	size atomic.Value

	// Td is used by package core to store the total difficulty
	// of the chain up to and including the block.
	td *big.Int

	// These fields are used by package eth to track
	// inter-peer block relay.
	ReceivedAt   time.Time
	ReceivedFrom interface{}
}

// "external" block encoding. used for eth protocol, etc.
type extminorblock struct {
	Header *MinorBlockHeader
	Meta   *MinorBlockMeta
	Txs    Transactions
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs and receipts.
func NewMinorBlock(header *MinorBlockHeader, meta *MinorBlockMeta, txs []*Transaction, receipts []*Receipt) *MinorBlock {
	b := &MinorBlock{header: CopyMinorBlockHeader(header), meta: CopyMinorBlockMeta(meta), td: new(big.Int)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.meta.TxHash = EmptyRootHash
	} else {
		b.meta.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.meta.ReceiptHash = EmptyRootHash
	} else {
		b.meta.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewMinorBlockWithHeader(header *MinorBlockHeader, meta *MinorBlockMeta) *MinorBlock {
	return &MinorBlock{header: CopyMinorBlockHeader(header), meta: CopyMinorBlockMeta(meta)}
}

// CopyHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyMinorBlockHeader(h *MinorBlockHeader) *MinorBlockHeader {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if cpy.CoinbaseAmount = new(big.Int); h.CoinbaseAmount != nil {
		cpy.CoinbaseAmount.Set(h.CoinbaseAmount)
	}
	if cpy.GasLimit = new(big.Int); h.GasLimit != nil {
		cpy.GasLimit.Set(h.GasLimit)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}

	return &cpy  //todo verify the copy for struct
}

func CopyMinorBlockMeta(m *MinorBlockMeta) *MinorBlockMeta {
	cpy := *m
	if cpy.GasUsed = new(big.Int); m.GasUsed != nil {
		cpy.GasUsed.Set(m.GasUsed)
	}
	if cpy.CrossShardGasUsed = new(big.Int); m.CrossShardGasUsed != nil {
		cpy.CrossShardGasUsed.Set(m.CrossShardGasUsed)
	}
	return &cpy
}

// DecodeRLP decodes the Ethereum
func (b *MinorBlock) DecodeRLP(s *rlp.Stream) error {
	var eb extminorblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.meta, b.transactions = eb.Header, eb.Meta, eb.Txs
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the Ethereum RLP block format.
func (b *MinorBlock) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extminorblock{
		Header: b.header,
		Txs:    b.transactions,
		Meta:   b.meta,
	})
}

// TODO: copies

func (b *MinorBlock) Transactions() Transactions { return b.transactions }

func (b *MinorBlock) Transaction(hash common.Hash) *Transaction {
	for _, transaction := range b.transactions {
		if transaction.Hash() == hash {
			return transaction
		}
	}
	return nil
}
//header properties
func (b *MinorBlock) Version() uint32     { return b.header.Version }
func (b *MinorBlock) Branch() Branch     { return b.header.Branch }
func (b *MinorBlock) Number() *big.Int     { return new(big.Int).Set(b.header.Number) }
func (b *MinorBlock) NumberU64() uint64        { return b.header.Number.Uint64() }
func (b *MinorBlock) Coinbase() common.Address { return b.header.Coinbase }
func (b *MinorBlock) CoinbaseAmount() *big.Int     { return new(big.Int).Set(b.header.CoinbaseAmount) }
func (b *MinorBlock) ParentHash() common.Hash  { return b.header.ParentHash }
func (b *MinorBlock) PrevRootBlockHash() common.Hash  { return b.header.PrevRootBlockHash }
func (b *MinorBlock) GasLimit() *big.Int     { return b.header.GasLimit }
func (b *MinorBlock) MetaHash() common.Hash  { return b.header.MetaHash }
func (b *MinorBlock) Time() *big.Int       { return new(big.Int).Set(b.header.Time) }
func (b *MinorBlock) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }
func (b *MinorBlock) Nonce() uint64            { return binary.BigEndian.Uint64(b.header.Nonce[:]) }
func (b *MinorBlock) Extra() []byte            { return common.CopyBytes(b.header.Extra) }
func (b *MinorBlock) Bloom() Bloom             { return b.header.Bloom }
func (b *MinorBlock) MixDigest() common.Hash   { return b.header.MixDigest }
//meta properties
func (b *MinorBlock) Root() common.Hash        { return b.meta.Root }
func (b *MinorBlock) TxHash() common.Hash      { return b.meta.TxHash }
func (b *MinorBlock) ReceiptHash() common.Hash { return b.meta.ReceiptHash }
func (b *MinorBlock) GasUsed() *big.Int      { return b.meta.GasUsed }
func (b *MinorBlock) CrossShardGasUsed() *big.Int      { return b.meta.CrossShardGasUsed }


func (b *MinorBlock) Header() *MinorBlockHeader { return CopyMinorBlockHeader(b.header) }

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *MinorBlock) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, b)
	b.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *MinorBlock) WithSeal(header *MinorBlockHeader, meta *MinorBlockMeta) *MinorBlock {
	cpyheader := *header
	cpymeta := *meta
	return &MinorBlock{
		header:       &cpyheader,
		meta:         &cpymeta,
		transactions: b.transactions,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *MinorBlock) WithBody(transactions []*Transaction) *MinorBlock {
	block := &MinorBlock{
		header:       CopyMinorBlockHeader(b.header),
		meta:         CopyMinorBlockMeta(b.meta),
		transactions: make([]*Transaction, len(transactions)),
	}
	copy(block.transactions, transactions)
	return block
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *MinorBlock) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}
