// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"crypto/ecdsa"
	"math/big"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

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
	Extra           []byte             `json:"extraData"        gencodec:"required"   bytesizeofslicelen:"2"`
	MixDigest       common.Hash        `json:"mixHash"`
	Signature       [65]byte           `json:"signature"        gencodec:"required"`
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// Serialize encoding.
func (h *RootBlockHeader) Hash() common.Hash {
	return serHash(*h, map[string]bool{"Signature": true})
}

// SealHash returns the block hash of the header, which is keccak256 hash of its
// Serialize encoding for Seal.
func (h *RootBlockHeader) SealHash() common.Hash {
	return serHash(*h, map[string]bool{"Signature": true, "MixDigest": true, "Nonce": true})
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

func (h *RootBlockHeader) GetParentHash() common.Hash   { return h.ParentHash }
func (h *RootBlockHeader) GetCoinbase() account.Address { return h.Coinbase }
func (h *RootBlockHeader) GetCoinbaseAmount() *big.Int  { return h.CoinbaseAmount.Value }
func (h *RootBlockHeader) GetTime() uint64              { return h.Time }
func (h *RootBlockHeader) GetDifficulty() *big.Int      { return new(big.Int).Set(h.Difficulty) }
func (h *RootBlockHeader) GetNonce() uint64             { return h.Nonce }
func (h *RootBlockHeader) GetExtra() []byte {
	if h.Extra != nil {
		return common.CopyBytes(h.Extra)
	}
	return nil
}
func (h *RootBlockHeader) GetMixDigest() common.Hash { return h.MixDigest }

func (h *RootBlockHeader) NumberU64() uint64 { return uint64(h.Number) }

func (h *RootBlockHeader) SetExtra(data []byte) {
	h.Extra = common.CopyBytes(data)
}

func (h *RootBlockHeader) SetDifficulty(difficulty *big.Int) {
	h.Difficulty = difficulty
}

func (h *RootBlockHeader) SetNonce(nonce uint64) {
	h.Nonce = nonce
}

func (h *RootBlockHeader) SetCoinbase(addr account.Address) {
	h.Coinbase = addr
}

func (h *RootBlockHeader) CreateBlockToAppend(createTime *uint64, difficulty *big.Int, address *account.Address, nonce *uint64, extraData []byte) *RootBlock {
	if createTime == nil {
		preTime := h.Time + 1
		createTime = &preTime
	}

	if difficulty == nil {
		difficulty = h.Difficulty
	}

	if address == nil {
		empty := account.CreatEmptyAddress(0)
		address = &empty
	}

	if nonce == nil {
		zeroNonce := uint64(0)
		nonce = &zeroNonce
	}

	if extraData == nil {
		extraData = make([]byte, 0)
	}

	header := &RootBlockHeader{
		Version:         h.Version,
		Number:          h.Number + 1,
		ParentHash:      h.Hash(),
		MinorHeaderHash: common.Hash{},
		Coinbase:        *address,
		CoinbaseAmount:  &serialize.Uint256{Value: new(big.Int)},
		Time:            *createTime,
		Difficulty:      difficulty,
		Nonce:           *nonce,
		Extra:           extraData,
	}
	return &RootBlock{
		header:            header,
		minorBlockHeaders: make(MinorBlockHeaders, 0),
		trackingdata:      []byte{},
	}
}

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

func (b *RootBlock) IHeader() IHeader {
	return b.header
}

// "external" block encoding. used for eth protocol, etc.
type extrootblock struct {
	Header            *RootBlockHeader
	MinorBlockHeaders MinorBlockHeaders `bytesizeofslicelen:"4"`
	Trackingdata      []byte            `bytesizeofslicelen:"2"`
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
		b.header.MinorHeaderHash = CalculateMerkleRoot(MinorBlockHeaders(mbHeaders))
		b.minorBlockHeaders = make(MinorBlockHeaders, len(mbHeaders))
		copy(b.minorBlockHeaders, mbHeaders)
	}
	if trackingdata != nil && len(trackingdata) > 0 {
		b.trackingdata = make([]byte, len(trackingdata))
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
func (b *RootBlock) CoinbaseAmount() *big.Int {
	if b.header.CoinbaseAmount != nil && b.header.CoinbaseAmount.Value != nil {
		return new(big.Int).Set(b.header.CoinbaseAmount.Value)
	}
	return new(big.Int)
}
func (b *RootBlock) Time() uint64           { return b.header.Time }
func (b *RootBlock) Difficulty() *big.Int   { return new(big.Int).Set(b.header.Difficulty) }
func (b *RootBlock) Nonce() uint64          { return b.header.Nonce }
func (b *RootBlock) Extra() []byte          { return common.CopyBytes(b.header.Extra) }
func (b *RootBlock) MixDigest() common.Hash { return b.header.MixDigest }
func (b *RootBlock) Signature() [65]byte    { return b.header.Signature }

func (b *RootBlock) Header() *RootBlockHeader { return CopyRootBlockHeader(b.header) }
func (b *RootBlock) Content() []IHashable {
	items := make([]IHashable, len(b.minorBlockHeaders), len(b.minorBlockHeaders))
	for i, item := range b.minorBlockHeaders {
		items[i] = item
	}
	return items
}

func (b *RootBlock) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}

	bytes, _ := serialize.SerializeToBytes(b)
	b.size.Store(common.StorageSize(len(bytes)))
	return common.StorageSize(len(bytes))
}

// WithMingResult returns a new block with the data from b and update nonce and mixDigest
func (b *RootBlock) WithMingResult(nonce uint64, mixDigest common.Hash) IBlock {
	cpy := CopyRootBlockHeader(b.header)
	cpy.Nonce = nonce
	cpy.MixDigest = mixDigest

	return b.WithSeal(cpy)
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

func (b *RootBlock) GetTrackingData() []byte {
	return b.trackingdata
}

func (b *RootBlock) GetSize() common.StorageSize {
	return b.Size()
}

func (b *RootBlock) Finalize(coinbaseAmount *big.Int, coinbaseAddress *account.Address) *RootBlock {
	if coinbaseAmount == nil {
		coinbaseAmount = new(big.Int)
	}

	if coinbaseAddress == nil {
		a := account.CreatEmptyAddress(0)
		coinbaseAddress = &a
	}
	b.header.MinorHeaderHash = CalculateMerkleRoot(b.minorBlockHeaders)
	b.header.CoinbaseAmount = &serialize.Uint256{Value: coinbaseAmount}
	b.header.Coinbase = *coinbaseAddress
	return b
}
func (b *RootBlock) AddMinorBlockHeader(header *MinorBlockHeader) {
	b.minorBlockHeaders = append(b.minorBlockHeaders, header)
}
func (b *RootBlock) ExtendMinorBlockHeaderList(headers []*MinorBlockHeader) {
	b.minorBlockHeaders = append(b.minorBlockHeaders, headers...)
}
