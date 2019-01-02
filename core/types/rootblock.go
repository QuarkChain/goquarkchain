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

// package rootchain contains data types related to Ethereum consensus.
package types

import (
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type ShardInfo struct {
	ShardSize     uint32
	ReshardVote   bool
}

func Create(shardSize uint32, reshardVote bool) (*ShardInfo, error) {
	if IsP2(shardSize) {
		return nil, errors.New("shard size should be power of 2")
	}

	return &ShardInfo{ShardSize:shardSize, ReshardVote:reshardVote}, nil
}

// RootBlockHeader represents a root block header in the QuarkChain.
type RootBlockHeader struct {
	Version         uint32             `json:"version"          gencodec:"required"`
	Number          *big.Int           `json:"number"           gencodec:"required"`
	ShardInfo       ShardInfo          `json:"shardinfo"        gencodec:"required"`
	ParentHash      common.Hash        `json:"parentHash"       gencodec:"required"`
	Root            common.Hash        `json:"stateRoot"        gencodec:"required"`
	Coinbase        common.Address     `json:"miner"            gencodec:"required"`
	CoinbaseAmount  *big.Int           `json:"coinbaseAmount"   gencodec:"required"`
	Time            *big.Int           `json:"timestamp"        gencodec:"required"`
	Difficulty      *big.Int           `json:"difficulty"       gencodec:"required"`
	Nonce           types.BlockNonce   `json:"nonce"`
	Extra           []byte             `json:"extraData"        gencodec:"required"`
	MixDigest       common.Hash        `json:"mixHash"`
	Signature       [65]byte           `json:"signature"        gencodec:"required"`   //todo
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *RootBlockHeader) Hash() common.Hash {
	return RlpHash(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *RootBlockHeader) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Signature)) +
		common.StorageSize(len(h.Extra)+
			(h.Difficulty.BitLen()+h.Number.BitLen()+h.Time.BitLen() + h.CoinbaseAmount.BitLen())/8)
}

// Block represents an entire block in the QuarkChain.
type RootBlock struct {
	header              *RootBlockHeader
	minorBlockHeaders   MinorBlockHeaders

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
type extrootblock struct {
	Header                *RootBlockHeader
	MinorBlockHeaders     MinorBlockHeaders
}


// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of TxHash, UncleHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewRootBlock(header *RootBlockHeader, mbHeaders MinorBlockHeaders) *RootBlock {
	b := &RootBlock{header: CopyRootBlockHeader(header), td: new(big.Int)}

	if len(mbHeaders) == 0 {
		b.header.Root = EmptyRootHash
	} else {
		b.header.Root = DeriveSha(MinorBlockHeaders(mbHeaders))
		b.minorBlockHeaders = make(MinorBlockHeaders, len(mbHeaders))
		copy(b.minorBlockHeaders, mbHeaders)
	}

	return b
}

// NewBlockWithHeader creates a block with the given header data. The
// header data is copied, changes to header and to the field values
// will not affect the block.
func NewRootBlockWithHeader(header *RootBlockHeader) *RootBlock {
	return &RootBlock{header: CopyRootBlockHeader(header)}
}

// CopyRootHeader creates a deep copy of a block header to prevent side effects from
// modifying a header variable.
func CopyRootBlockHeader(h *RootBlockHeader) *RootBlockHeader {
	cpy := *h
	if cpy.Time = new(big.Int); h.Time != nil {
		cpy.Time.Set(h.Time)
	}
	if cpy.CoinbaseAmount = new(big.Int); h.CoinbaseAmount != nil {
		cpy.CoinbaseAmount.Set(h.CoinbaseAmount)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.Number = new(big.Int); h.Number != nil {
		cpy.Number.Set(h.Number)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	cpy.Signature = [65]byte{}
	copy(cpy.Signature[:], h.Signature[:])

	return &cpy
}

// DecodeRLP decodes the QuarkChain
func (b *RootBlock) DecodeRLP(s *rlp.Stream) error {
	var eb extrootblock
	_, size, _ := s.Kind()
	if err := s.Decode(&eb); err != nil {
		return err
	}
	b.header, b.minorBlockHeaders = eb.Header, eb.MinorBlockHeaders
	b.size.Store(common.StorageSize(rlp.ListSize(size)))
	return nil
}

// EncodeRLP serializes b into the QuarkChain RLP block format.
func (b *RootBlock) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, extrootblock{
		Header:               b.header,
		MinorBlockHeaders:    b.minorBlockHeaders,
	})
}

// TODO: copies

func (b *RootBlock) MinorBlockHeaders() MinorBlockHeaders { return b.minorBlockHeaders }

func (b *RootBlock) MinorBlockHeader(hash common.Hash) *MinorBlockHeader {
	for _, minorBlockHeader := range b.minorBlockHeaders {
		if minorBlockHeader.Hash() == hash {
			return minorBlockHeader
		}
	}
	return nil
}

func (b *RootBlock) Version() uint32          { return b.header.Version }
func (b *RootBlock) Number() *big.Int         { return new(big.Int).Set(b.header.Number) }
func (b *RootBlock) NumberU64() uint64        { return b.header.Number.Uint64() }
func (b *RootBlock) ShardInfo() ShardInfo     { return b.header.ShardInfo    }
func (b *RootBlock) ParentHash() common.Hash  { return b.header.ParentHash }
func (b *RootBlock) Root() common.Hash        { return b.header.Root }
func (b *RootBlock) Coinbase() common.Address { return b.header.Coinbase }
func (b *RootBlock) CoinbaseAmount() *big.Int     { return b.header.CoinbaseAmount }
func (b *RootBlock) Time() *big.Int       { return new(big.Int).Set(b.header.Time) }
func (b *RootBlock) Difficulty() *big.Int { return new(big.Int).Set(b.header.Difficulty) }
func (b *RootBlock) Nonce() uint64            { return binary.BigEndian.Uint64(b.header.Nonce[:]) }
func (b *RootBlock) Extra() []byte            { return common.CopyBytes(b.header.Extra) }
func (b *RootBlock) MixDigest() common.Hash   { return b.header.MixDigest }
func (b *RootBlock) Signature() [65]byte             { return b.header.Signature }

func (b *RootBlock) Header() *RootBlockHeader { return CopyRootBlockHeader(b.header) }

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *RootBlock) Size() common.StorageSize {
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
func (b *RootBlock) WithSeal(header *RootBlockHeader) *RootBlock {
	cpy := *header

	return &RootBlock{
		header:              &cpy,
		minorBlockHeaders:   b.minorBlockHeaders,
	}
}

// WithBody returns a new block with the given minorBlockHeaders contents.
func (b *RootBlock) WithBody(minorBlockHeaders MinorBlockHeaders, uncles []*RootBlockHeader) *RootBlock {
	block := &RootBlock{
		header:            CopyRootBlockHeader(b.header),
		minorBlockHeaders: make(MinorBlockHeaders, len(minorBlockHeaders)),
	}
	copy(block.minorBlockHeaders, minorBlockHeaders)

	return block
}

// Hash returns the keccak256 hash of b's header.
// The hash is computed on the first call and cached thereafter.
func (b *RootBlock) Hash() common.Hash {
	if hash := b.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.header.Hash()
	b.hash.Store(v)
	return v
}
