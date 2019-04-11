// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"bytes"
	"fmt"
	"io"
	"unsafe"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
)

//go:generate gencodec -type Receipt -field-override receiptMarshaling -out gen_receipt_json.go

var (
	receiptStatusFailedRLP     = []byte{}
	receiptStatusSuccessfulRLP = []byte{0x01}
)

const (
	// ReceiptStatusFailed is the status code of a transaction if execution failed.
	ReceiptStatusFailed = uint64(0)

	// ReceiptStatusSuccessful is the status code of a transaction if execution succeeded.
	ReceiptStatusSuccessful = uint64(1)
)

// Receipt represents the results of a transaction.
type Receipt struct {
	// Consensus fields
	PostState         []byte `json:"root"`
	Status            uint64 `json:"status"`
	CumulativeGasUsed uint64 `json:"cumulativeGasUsed" gencodec:"required"`
	Bloom             Bloom  `json:"logsBloom"         gencodec:"required"`
	Logs              []*Log `json:"logs"              gencodec:"required"`

	// Implementation fields (don't reorder!)
	TxHash              common.Hash       `json:"transactionHash" gencodec:"required"`
	ContractAddress     account.Recipient `json:"contractAddress"`
	ContractFullShardId uint32            `json:"contractFullShardId"`
	GasUsed             uint64            `json:"gasUsed" gencodec:"required"`
}

type receiptMarshaling struct {
	PostState         hexutil.Bytes
	Status            hexutil.Uint64
	CumulativeGasUsed hexutil.Uint64
	GasUsed           hexutil.Uint64
}

// receiptSer is the serialize .
type receiptSer struct {
	PostStateOrStatus []byte
	CumulativeGasUsed uint64
	PrevGasUsed       uint64
	Bloom             Bloom
	ContractAddress   account.Address
	Logs              []*Log `bytesizeofslicelen:"4"`
}

// receiptRLP is the consensus encoding of a receipt.
type receiptRLP struct {
	PostStateOrStatus   []byte
	CumulativeGasUsed   uint64
	Bloom               Bloom
	Logs                []*Log
	ContractAddress     account.Recipient
	ContractFullShardId uint32
}

type receiptStorageRLP struct {
	PostStateOrStatus   []byte
	CumulativeGasUsed   uint64
	Bloom               Bloom
	TxHash              common.Hash
	ContractAddress     account.Recipient
	ContractFullShardId uint32
	Logs                []*LogForStorage
	GasUsed             uint64
	Status              uint64
}

// NewReceipt creates a barebone transaction receipt, copying the init fields.
func NewReceipt(root []byte, failed bool, cumulativeGasUsed uint64) *Receipt {
	r := &Receipt{PostState: common.CopyBytes(root), CumulativeGasUsed: cumulativeGasUsed}
	if failed {
		r.Status = ReceiptStatusFailed
	} else {
		r.Status = ReceiptStatusSuccessful
	}
	return r
}

// Deserialize deserialize the QKC minor block
func (r *Receipt) Deserialize(bb *serialize.ByteBuffer) error {
	var rs receiptSer
	if err := serialize.Deserialize(bb, &rs); err != nil {
		return err
	}
	if err := r.setStatus(rs.PostStateOrStatus); err != nil {
		return err
	}

	r.CumulativeGasUsed, r.Bloom, r.Logs, r.GasUsed, r.ContractAddress, r.ContractFullShardId = rs.CumulativeGasUsed, rs.Bloom, rs.Logs, rs.CumulativeGasUsed-rs.PrevGasUsed, rs.ContractAddress.Recipient, rs.ContractAddress.FullShardKey
	return nil
}

// Serialize serialize the QKC minor block.
func (r *Receipt) Serialize(w *[]byte) error {
	return serialize.Serialize(w, receiptSer{
		r.statusEncoding(),
		r.CumulativeGasUsed,
		r.CumulativeGasUsed - r.GasUsed,
		r.Bloom,
		account.Address{Recipient: r.ContractAddress, FullShardKey: r.ContractFullShardId},
		r.Logs,
	})
}

// EncodeRLP implements rlp.Encoder, and flattens the consensus fields of a receipt
// into an RLP stream. If no post state is present, byzantium fork is assumed.
func (r *Receipt) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w,
		&receiptRLP{
			r.statusEncoding(),
			r.CumulativeGasUsed,
			r.Bloom,
			r.Logs,
			r.ContractAddress,
			r.ContractFullShardId})
}

// DecodeRLP implements rlp.Decoder, and loads the consensus fields of a receipt
// from an RLP stream.
func (r *Receipt) DecodeRLP(s *rlp.Stream) error {
	var dec receiptRLP
	if err := s.Decode(&dec); err != nil {
		return err
	}
	if err := r.setStatus(dec.PostStateOrStatus); err != nil {
		return err
	}
	r.CumulativeGasUsed, r.Bloom, r.Logs, r.ContractAddress, r.ContractFullShardId = dec.CumulativeGasUsed, dec.Bloom, dec.Logs, dec.ContractAddress, dec.ContractFullShardId
	return nil
}

func (r *Receipt) setStatus(postStateOrStatus []byte) error {
	switch {
	case bytes.Equal(postStateOrStatus, receiptStatusSuccessfulRLP):
		r.Status = ReceiptStatusSuccessful
	case bytes.Equal(postStateOrStatus, receiptStatusFailedRLP):
		r.Status = ReceiptStatusFailed
	case len(postStateOrStatus) == len(common.Hash{}):
		r.PostState = postStateOrStatus
	default:
		return fmt.Errorf("invalid receipt status %x", postStateOrStatus)
	}
	return nil
}

func (r *Receipt) statusEncoding() []byte {
	if len(r.PostState) == 0 {
		if r.Status == ReceiptStatusFailed {
			return receiptStatusFailedRLP
		}
		return receiptStatusSuccessfulRLP
	}
	return r.PostState
}

// Size returns the approximate memory used by all internal contents. It is used
// to approximate and limit the memory consumption of various caches.
func (r *Receipt) Size() common.StorageSize {
	size := common.StorageSize(unsafe.Sizeof(*r)) + common.StorageSize(len(r.PostState))

	size += common.StorageSize(len(r.Logs)) * common.StorageSize(unsafe.Sizeof(Log{}))
	for _, log := range r.Logs {
		size += common.StorageSize(len(log.Topics)*common.HashLength + len(log.Data))
	}
	return size
}

// ReceiptForStorage is a wrapper around a Receipt that flattens and parses the
// entire content of a receipt, as opposed to only the consensus fields originally.
type ReceiptForStorage Receipt

// EncodeRLP implements rlp.Encoder, and flattens all content fields of a receipt
// into an RLP stream.
func (r *ReceiptForStorage) EncodeRLP(w io.Writer) error {
	enc := &receiptStorageRLP{
		PostStateOrStatus:   (*Receipt)(r).statusEncoding(),
		CumulativeGasUsed:   r.CumulativeGasUsed,
		Bloom:               r.Bloom,
		TxHash:              r.TxHash,
		ContractAddress:     r.ContractAddress,
		ContractFullShardId: r.ContractFullShardId,
		Logs:                make([]*LogForStorage, len(r.Logs)),
		GasUsed:             r.GasUsed,
		Status:              r.Status,
	}
	for i, log := range r.Logs {
		enc.Logs[i] = (*LogForStorage)(log)
	}
	return rlp.Encode(w, enc)
}

// DecodeRLP implements rlp.Decoder, and loads both consensus and implementation
// fields of a receipt from an RLP stream.
func (r *ReceiptForStorage) DecodeRLP(s *rlp.Stream) error {
	var dec receiptStorageRLP
	if err := s.Decode(&dec); err != nil {
		return err
	}
	if err := (*Receipt)(r).setStatus(dec.PostStateOrStatus); err != nil {
		return err
	}
	// Assign the consensus fields
	r.CumulativeGasUsed, r.Bloom = dec.CumulativeGasUsed, dec.Bloom
	r.Logs = make([]*Log, len(dec.Logs))
	for i, log := range dec.Logs {
		r.Logs[i] = (*Log)(log)
	}
	// Assign the implementation fields
	r.TxHash, r.ContractAddress, r.ContractFullShardId, r.GasUsed = dec.TxHash, dec.ContractAddress, dec.ContractFullShardId, dec.GasUsed
	r.Status = dec.Status
	return nil
}

// Receipts is a wrapper around a Receipt array to implement DerivableList.
type Receipts []*Receipt

// Len returns the number of receipts in this list.
func (r Receipts) Len() int { return len(r) }

// GetRlp returns the RLP encoding of one receipt from the list.
func (r Receipts) Bytes(i int) []byte {
	bytes, err := rlp.EncodeToBytes(r[i])
	if err != nil {
		panic(err)
	}
	return bytes
}
