// Modified from go-ethereum under GNU Lesser General Public License
package types

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"sync/atomic"
	"time"
	"unsafe"
)

//go:generate gencodec -type Header -field-override headerMarshaling -out gen_header_json.go

// MinorBlockHeader represents a minor block header in the QuarkChain.
type MinorBlockHeader struct {
	Version           uint32             `json:"version"                    gencodec:"required"`
	Branch            account.Branch     `json:"branch"                     gencodec:"required"`
	Number            uint64             `json:"number"                     gencodec:"required"`
	Coinbase          account.Address    `json:"miner"                      gencodec:"required"`
	CoinbaseAmount    *serialize.Uint256 `json:"coinbaseAmount"             gencodec:"required"`
	ParentHash        common.Hash        `json:"parentHash"                 gencodec:"required"`
	PrevRootBlockHash common.Hash        `json:"prevRootBlockHash"          gencodec:"required"`
	GasLimit          *serialize.Uint256 `json:"gasLimit"                   gencodec:"required"`
	MetaHash          common.Hash        `json:"metaHash"                   gencodec:"required"`
	Time              uint64             `json:"timestamp"                  gencodec:"required"`
	Difficulty        *big.Int           `json:"difficulty"                 gencodec:"required"`
	Nonce             uint64             `json:"nonce"`
	Bloom             Bloom              `json:"logsBloom"                  gencodec:"required"`
	Extra             []byte             `json:"extraData"                  gencodec:"required"   bytesizeofslicelen:"2"`
	MixDigest         common.Hash        `json:"mixHash"`
}

type MinorBlockMeta struct {
	Root              common.Hash        `json:"stateRoot"                  gencodec:"required"`
	TxHash            common.Hash        `json:"transactionsRoot"           gencodec:"required"`
	ReceiptHash       common.Hash        `json:"receiptsRoot"               gencodec:"required"`
	GasUsed           *serialize.Uint256 `json:"gasUsed"                    gencodec:"required"`
	CrossShardGasUsed *serialize.Uint256 `json:"crossShardGasUsed"          gencodec:"required"`
}

// Hash returns the block hash of the header, which is simply the keccak256 hash of its
// Serialize encoding.
func (h *MinorBlockHeader) Hash() common.Hash {
	return serHash(h)
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (h *MinorBlockHeader) Size() common.StorageSize {
	return common.StorageSize(unsafe.Sizeof(*h)) +
		common.StorageSize(len(h.Extra)+(h.Difficulty.BitLen())/8)
}

// MinorBlockHeaders is a MinorBlockHeader slice type for basic sorting.
type MinorBlockHeaders []*MinorBlockHeader

// Len returns the length of s.
func (s MinorBlockHeaders) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s MinorBlockHeaders) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Bytes implements DerivableList and returns the i'th element of s in serialize.
func (s MinorBlockHeaders) Bytes(i int) []byte {
	enc, _ := serialize.SerializeToBytes(s[i])
	return enc
}

// MinorBlock represents an entire block in the Ethereum blockchain.
type MinorBlock struct {
	header       *MinorBlockHeader
	meta         *MinorBlockMeta
	transactions Transactions
	trackingdata []byte

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

// "external" block encoding. used for qkc protocol, etc.
type extminorblock struct {
	Header       *MinorBlockHeader
	Meta         *MinorBlockMeta
	Txs          Transactions `bytesizeofslicelen:"4"`
	Trackingdata []byte       `bytesizeofslicelen:"2"`
}

// NewBlock creates a new block. The input data is copied,
// changes to header and to the field values will not affect the
// block.
//
// The values of MinorHeaderHash, ReceiptHash and Bloom in header
// are ignored and set to values derived from the given txs and receipts.
func NewMinorBlock(header *MinorBlockHeader, meta *MinorBlockMeta, txs []*Transaction, receipts []*Receipt, trackingdata []byte) *MinorBlock {
	b := &MinorBlock{header: CopyMinorBlockHeader(header), meta: CopyMinorBlockMeta(meta), td: new(big.Int)}

	// TODO: panic if len(txs) != len(receipts)
	if len(txs) == 0 {
		b.meta.TxHash = EmptyHash
	} else {
		b.meta.TxHash = DeriveSha(Transactions(txs))
		b.transactions = make(Transactions, len(txs))
		copy(b.transactions, txs)
	}

	if len(receipts) == 0 {
		b.meta.ReceiptHash = EmptyHash
	} else {
		b.meta.ReceiptHash = DeriveSha(Receipts(receipts))
		b.header.Bloom = CreateBloom(receipts)
	}

	if trackingdata != nil && len(trackingdata) > 0 {
		copy(b.trackingdata, trackingdata)
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
	if cpy.Difficulty = new(big.Int); h.Difficulty != nil {
		cpy.Difficulty.Set(h.Difficulty)
	}
	if cpy.CoinbaseAmount = new(serialize.Uint256); h.CoinbaseAmount != nil && h.CoinbaseAmount.Value != nil {
		cpy.CoinbaseAmount.Value = new(big.Int).Set(h.CoinbaseAmount.Value)
	}
	if cpy.GasLimit = new(serialize.Uint256); h.GasLimit != nil && h.GasLimit.Value != nil {
		cpy.GasLimit.Value = new(big.Int).Set(h.GasLimit.Value)
	}
	if h.Extra != nil && len(h.Extra) > 0 {
		cpy.Extra = make([]byte, len(h.Extra))
		copy(cpy.Extra, h.Extra)
	}

	return &cpy //todo verify the copy for struct
}

func CopyMinorBlockMeta(m *MinorBlockMeta) *MinorBlockMeta {
	cpy := *m
	if cpy.GasUsed = new(serialize.Uint256); m.GasUsed != nil && m.GasUsed.Value != nil {
		cpy.GasUsed.Value = new(big.Int).Set(m.GasUsed.Value)
	}
	if cpy.CrossShardGasUsed = new(serialize.Uint256); m.CrossShardGasUsed != nil && m.CrossShardGasUsed.Value != nil {
		cpy.CrossShardGasUsed.Value = new(big.Int).Set(m.CrossShardGasUsed.Value)
	}
	return &cpy
}

// Deserialize deserialize the QKC minor block
func (b *MinorBlock) Deserialize(bb *serialize.ByteBuffer) error {
	var eb extminorblock
	startIndex := bb.GetOffset()
	if err := serialize.Deserialize(bb, &eb); err != nil {
		return err
	}
	b.header, b.meta, b.transactions, b.trackingdata = eb.Header, eb.Meta, eb.Txs, eb.Trackingdata
	b.size.Store(common.StorageSize(bb.GetOffset() - startIndex))
	return nil
}

// Serialize serialize the QKC minor block.
func (b *MinorBlock) Serialize(w *[]byte) error {
	offset := len(*w)
	err := serialize.Serialize(w, extminorblock{
		Header:       b.header,
		Txs:          b.transactions,
		Meta:         b.meta,
		Trackingdata: b.trackingdata,
	})

	b.size.Store(common.StorageSize(len(*w) - offset))
	return err
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

func (b *MinorBlock) TrackingData() []byte { return b.trackingdata }

//header properties
func (b *MinorBlock) Version() uint32                { return b.header.Version }
func (b *MinorBlock) Branch() account.Branch         { return b.header.Branch }
func (b *MinorBlock) Number() uint64                 { return b.header.Number }
func (b *MinorBlock) Coinbase() account.Address      { return b.header.Coinbase }
func (b *MinorBlock) CoinbaseAmount() *big.Int       { return new(big.Int).Set(b.header.CoinbaseAmount.Value) }
func (b *MinorBlock) ParentHash() common.Hash        { return b.header.ParentHash }
func (b *MinorBlock) PrevRootBlockHash() common.Hash { return b.header.PrevRootBlockHash }
func (b *MinorBlock) GasLimit() *big.Int             { return new(big.Int).Set(b.header.GasLimit.Value) }
func (b *MinorBlock) MetaHash() common.Hash          { return b.header.MetaHash }
func (b *MinorBlock) Time() uint64                   { return b.header.Time }
func (b *MinorBlock) Difficulty() *big.Int           { return new(big.Int).Set(b.header.Difficulty) }
func (b *MinorBlock) Nonce() uint64                  { return b.header.Nonce }
func (b *MinorBlock) Extra() []byte                  { return common.CopyBytes(b.header.Extra) }
func (b *MinorBlock) Bloom() Bloom                   { return b.header.Bloom }
func (b *MinorBlock) MixDigest() common.Hash         { return b.header.MixDigest }

//meta properties
func (b *MinorBlock) Root() common.Hash        { return b.meta.Root }
func (b *MinorBlock) TxHash() common.Hash      { return b.meta.TxHash }
func (b *MinorBlock) ReceiptHash() common.Hash { return b.meta.ReceiptHash }
func (b *MinorBlock) GasUsed() *big.Int        { return new(big.Int).Set(b.meta.GasUsed.Value) }
func (b *MinorBlock) CrossShardGasUsed() *big.Int {
	return new(big.Int).Set(b.meta.CrossShardGasUsed.Value)
}

func (b *MinorBlock) Header() *MinorBlockHeader { return CopyMinorBlockHeader(b.header) }
func (b *MinorBlock) Meta() *MinorBlockMeta     { return CopyMinorBlockMeta(b.meta) }

// Size returns the true RLP encoded storage size of the block, either by encoding
// and returning it, or returning a previsouly cached value.
func (b *MinorBlock) Size() common.StorageSize {
	if size := b.size.Load(); size != nil {
		return size.(common.StorageSize)
	}

	bytes, _ := serialize.SerializeToBytes(b)
	b.size.Store(common.StorageSize(len(bytes)))
	return common.StorageSize(len(bytes))
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
		trackingdata: b.trackingdata,
	}
}

// WithBody returns a new block with the given transaction and uncle contents.
func (b *MinorBlock) WithBody(transactions []*Transaction, trackingData []byte) *MinorBlock {
	block := &MinorBlock{
		header:       CopyMinorBlockHeader(b.header),
		meta:         CopyMinorBlockMeta(b.meta),
		transactions: make([]*Transaction, len(transactions)),
		trackingdata: make([]byte, len(trackingData)),
	}
	copy(block.transactions, transactions)
	copy(block.trackingdata, trackingData)
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
