// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"crypto/ecdsa"
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/ethereum/go-ethereum/common"
)

type ShardInfo struct {
	ShardSize   uint32
	ReshardVote bool
}

func Create(shardSize uint32, reshardVote bool) (*ShardInfo, error) {
	if IsP2(shardSize) {
		return nil, errors.New("shard size should be power of 2")
	}

	return &ShardInfo{ShardSize: shardSize, ReshardVote: reshardVote}, nil
}

// RootBlockHeader represents a root block header in the QuarkChain.
type RootBlockHeader struct {
	Version         uint32             `json:"version"          gencodec:"required"`
	Number          uint32             `json:"number"           gencodec:"required"`
	ParentHash      common.Hash        `json:"parentHash"       gencodec:"required"`
	MinorHeaderHash common.Hash        `json:"transactionsRoot" gencodec:"required"`
	Coinbase        account.Address    `json:"miner"            gencodec:"required"`
	CoinbaseAmount  *serialize.Uint256 `json:"coinbaseAmount"   gencodec:"required"`
	Time            uint64             `json:"timestamp"        gencodec:"required"`
	Difficulty      *big.Int           `json:"difficulty"       gencodec:"required"`
	Nonce           uint64             `json:"nonce"`
	Extra           []byte             `json:"extraData"        gencodec:"required"   bytesize:"2"`
	MixDigest       common.Hash        `json:"mixHash"`
	Signature       [65]byte           `json:"signature"        gencodec:"required"`
}

type rootBlockHeaderForHash struct {
	Version         uint32
	Number          uint32
	ParentHash      common.Hash
	MinorHeaderHash common.Hash
	Coinbase        account.Address
	CoinbaseAmount  *serialize.Uint256
	Time            uint64
	Difficulty      *big.Int
	Nonce           uint64
	Extra           []byte `bytesize:"2"`
	MixDigest       common.Hash
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// RLP encoding.
func (h *RootBlockHeader) Hash() common.Hash {
	header := rootBlockHeaderForHash{
		Version:         h.Version,
		Number:          h.Number,
		ParentHash:      h.ParentHash,
		MinorHeaderHash: h.MinorHeaderHash,
		Coinbase:        h.Coinbase,
		CoinbaseAmount:  h.CoinbaseAmount,
		Time:            h.Time,
		Difficulty:      h.Difficulty,
		Nonce:           h.Nonce,
		Extra:           h.Extra,
		MixDigest:       h.MixDigest,
	}

	return serHash(header)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *RootBlockHeader) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) + common.StorageSize(len(h.Signature)) +
		common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen())/8)
}

func (h *RootBlockHeader) SignWithPrivateKey(prv *ecdsa.PrivateKey) error {
	hash := h.Hash()
	sig, err := crypto.Sign(hash[:], prv)
	if err != nil {
		return err
	}

	copy(h.Signature[:], sig)
	return nil
}

func (h *RootBlockHeader) NumberUI64() uint64 { return uint64(h.Number) }

// Block represents an entire block in the QuarkChain.
type RootBlock struct {
	header            *RootBlockHeader
	minorBlockHeaders MinorBlockHeaders
	trackingdata      []byte

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
	Header            *RootBlockHeader
	MinorBlockHeaders MinorBlockHeaders `bytesize:"4"`
	Trackingdata      []byte            `bytesize:"2"`
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of MinorHeaderHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs, uncles
// and receipts.
func NewRootBlock(header *RootBlockHeader, mbHeaders MinorBlockHeaders, trackingdata []byte) *RootBlock {
	b := &RootBlock{header: CopyRootBlockHeader(header), td: new(big.Int)}

	if len(mbHeaders) == 0 {
		b.header.MinorHeaderHash = EmptyHash
	} else {
		b.header.MinorHeaderHash = DeriveSha(MinorBlockHeaders(mbHeaders))
		b.minorBlockHeaders = make(MinorBlockHeaders, len(mbHeaders))
		copy(b.minorBlockHeaders, mbHeaders)
	}
	if trackingdata != nil && len(trackingdata) > 0 {
		copy(b.trackingdata, trackingdata)
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
	if cpy.CoinbaseAmount = new(serialize.Uint256); h.CoinbaseAmount != nil && h.CoinbaseAmount.Value != nil {
		cpy.CoinbaseAmount.Value = new(big.Int).Set(h.CoinbaseAmount.Value)
	}
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}
	cpy.Signature = [65]byte{}
	copy(cpy.Signature[:], h.Signature[:])

	return &cpy
}

// Deserialize deserialize the QKC root block
func (b *RootBlock) Deserialize(bb *serialize.ByteBuffer) error {
	var eb extrootblock
	startIndex := bb.GetOffset()
	if err := serialize.Deserialize(bb, &eb); err != nil {
		return err
	}
	b.header, b.minorBlockHeaders, b.trackingdata = eb.Header, eb.MinorBlockHeaders, eb.Trackingdata
	b.size.Store(common.StorageSize(bb.GetOffset() - startIndex))
	return nil
}

// Serialize serialize the QKC root block.
func (b *RootBlock) Serialize(w *[]byte) error {
	offset := len(*w)
	err := serialize.Serialize(w, extrootblock{
		Header:            b.header,
		MinorBlockHeaders: b.minorBlockHeaders,
		Trackingdata:      b.trackingdata,
	})

	b.size.Store(common.StorageSize(len(*w) - offset))
	return err
}

func (b *RootBlock) MinorBlockHeaders() MinorBlockHeaders { return b.minorBlockHeaders }

func (b *RootBlock) MinorBlockHeader(hash common.Hash) *MinorBlockHeader {
	for _, minorBlockHeader := range b.minorBlockHeaders {
		if minorBlockHeader.Hash() == hash {
			return minorBlockHeader
		}
	}

	return nil
}

func (b *RootBlock) TrackingData() []byte { return b.trackingdata }

func (b *RootBlock) Version() uint32              { return b.header.Version }
func (b *RootBlock) Number() uint32               { return b.header.Number }
func (b *RootBlock) NumberU64() uint64            { return uint64(b.header.Number) }
func (b *RootBlock) ParentHash() common.Hash      { return b.header.ParentHash }
func (b *RootBlock) MinorHeaderHash() common.Hash { return b.header.MinorHeaderHash }
func (b *RootBlock) Coinbase() account.Address    { return b.header.Coinbase }
func (b *RootBlock) CoinbaseAmount() *big.Int     { return new(big.Int).Set(b.header.CoinbaseAmount.Value) }
func (b *RootBlock) Time() uint64                 { return b.header.Time }
func (b *RootBlock) Difficulty() *big.Int         { return new(big.Int).Set(b.header.Difficulty) }
func (b *RootBlock) Nonce() uint64                { return b.header.Nonce }
func (b *RootBlock) Extra() []byte                { return common.CopyBytes(b.header.Extra) }
func (b *RootBlock) MixDigest() common.Hash       { return b.header.MixDigest }
func (b *RootBlock) Signature() [65]byte          { return b.header.Signature }

func (b *RootBlock) Header() *RootBlockHeader { return CopyRootBlockHeader(b.header) }

func (b *RootBlock) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}

	bytes, _ := serialize.SerializeToBytes(b)
	b.size.Store(common.StorageSize(len(bytes)))
	return common.StorageSize(len(bytes))
}

// WithSeal returns a new block with the data from b but the header replaced with
// the sealed one.
func (b *RootBlock) WithSeal(header *RootBlockHeader) *RootBlock {
	cpy := *header

	return &RootBlock{
		header:            &cpy,
		minorBlockHeaders: b.minorBlockHeaders,
		trackingdata:      b.trackingdata,
	}
}

// WithBody returns a new block with the given minorBlockHeaders contents.
func (b *RootBlock) WithBody(minorBlockHeaders MinorBlockHeaders, trackingdata []byte) *RootBlock {
	block := &RootBlock{
		header:            CopyRootBlockHeader(b.header),
		minorBlockHeaders: make(MinorBlockHeaders, len(minorBlockHeaders)),
		trackingdata:      make([]byte, len(b.trackingdata)),
	}

	copy(block.minorBlockHeaders, minorBlockHeaders)
	copy(block.trackingdata, trackingdata)
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
