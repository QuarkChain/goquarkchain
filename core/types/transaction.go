// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"container/heap"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
	"math/big"
	"sync/atomic"
)

const (
	InvalidTxType = iota
	EvmTx
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

type EvmTransaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

type txdata struct {
	AccountNonce    uint64             `json:"nonce"              gencodec:"required"`
	Price           *big.Int           `json:"gasPrice"           gencodec:"required"`
	GasLimit        uint64             `json:"gas"                gencodec:"required"`
	Recipient       *account.Recipient `json:"to"                 rlp:"nil"` // nil means contract creation
	Amount          *big.Int           `json:"value"              gencodec:"required"`
	Payload         []byte             `json:"input"              gencodec:"required"`
	FromFullShardId uint32             `json:"fromfullshardid"    gencodec:"required"`
	ToFullShardId   uint32             `json:"tofullshardid"      gencodec:"required"`
	NetworkId       uint32             `json:"networkId"          gencodec:"required"`
	Version         uint32             `json:"version"            gencodec:"required"`
	// Signature values
	V *big.Int `json:"v"             gencodec:"required"`
	R *big.Int `json:"r"             gencodec:"required"`
	S *big.Int `json:"s"             gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash"              rlp:"-"`
}

func NewEvmTransaction(nonce uint64, to account.Recipient, amount *big.Int, gasLimit uint64, gasPrice *big.Int, fromFullShardId uint32, toFullShardId uint32, networkId uint32, version uint32, data []byte) *EvmTransaction {
	return newEvmTransaction(nonce, &to, amount, gasLimit, gasPrice, fromFullShardId, toFullShardId, networkId, version, data)
}

func NewEvmContractCreation(nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, fromFullShardId uint32, toFullShardId uint32, networkId uint32, version uint32, data []byte) *EvmTransaction {
	return newEvmTransaction(nonce, nil, amount, gasLimit, gasPrice, fromFullShardId, toFullShardId, networkId, version, data)
}

func newEvmTransaction(nonce uint64, to *account.Recipient, amount *big.Int, gasLimit uint64, gasPrice *big.Int, fromFullShardId uint32, toFullShardId uint32, networkId uint32, version uint32, data []byte) *EvmTransaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce:    nonce,
		Recipient:       to,
		Payload:         data,
		Amount:          new(big.Int),
		GasLimit:        gasLimit,
		Price:           new(big.Int),
		FromFullShardId: fromFullShardId,
		ToFullShardId:   toFullShardId,
		NetworkId:       networkId,
		Version:         version,
		V:               new(big.Int),
		R:               new(big.Int),
		S:               new(big.Int),
	}
	if amount != nil {
		d.Amount.Set(amount)
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}

	return &EvmTransaction{data: d}
}

// EncodeRLP implements rlp.Encoder
func (tx *EvmTransaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *EvmTransaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

type txdataUnsigned struct {
	AccountNonce    uint64             `json:"nonce"              gencodec:"required"`
	Price           *big.Int           `json:"gasPrice"           gencodec:"required"`
	GasLimit        uint64             `json:"gas"                gencodec:"required"`
	Recipient       *account.Recipient `json:"to"                 rlp:"nil"` // nil means contract creation
	Amount          *big.Int           `json:"value"              gencodec:"required"`
	Payload         []byte             `json:"input"              gencodec:"required"`
	FromFullShardId uint32             `json:"fromfullshardid"    gencodec:"required"`
	ToFullShardId   uint32             `json:"tofullshardid"      gencodec:"required"`
	NetworkId       uint32             `json:"networkid"          gencodec:"required"`
}

func (tx *EvmTransaction) getUnsignedHash() common.Hash {
	unsigntx := txdataUnsigned{
		AccountNonce:    tx.data.AccountNonce,
		Price:           tx.data.Price,
		GasLimit:        tx.data.GasLimit,
		Recipient:       tx.data.Recipient,
		Amount:          tx.data.Amount,
		Payload:         tx.data.Payload,
		FromFullShardId: tx.data.FromFullShardId,
		ToFullShardId:   tx.data.ToFullShardId,
		NetworkId:       tx.data.NetworkId,
	}

	return rlpHash(unsigntx)
}

func (tx *EvmTransaction) Data() []byte            { return common.CopyBytes(tx.data.Payload) }
func (tx *EvmTransaction) Gas() uint64             { return tx.data.GasLimit }
func (tx *EvmTransaction) GasPrice() *big.Int      { return new(big.Int).Set(tx.data.Price) }
func (tx *EvmTransaction) Value() *big.Int         { return new(big.Int).Set(tx.data.Amount) }
func (tx *EvmTransaction) Nonce() uint64           { return tx.data.AccountNonce }
func (tx *EvmTransaction) CheckNonce() bool        { return true }
func (tx *EvmTransaction) FromFullShardId() uint32 { return tx.data.FromFullShardId }
func (tx *EvmTransaction) ToFullShardId() uint32   { return tx.data.ToFullShardId }
func (tx *EvmTransaction) NetworkId() uint32       { return tx.data.NetworkId }

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *EvmTransaction) To() *account.Recipient {
	if tx.data.Recipient == nil {
		return nil
	}

	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *EvmTransaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *EvmTransaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// AsMessage returns the transaction as a core.Message.
// AsMessage requires a signer to derive the sender.
// XXX Rename message to something less arbitrary?
func (tx *EvmTransaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:           tx.data.AccountNonce,
		gasLimit:        tx.data.GasLimit,
		gasPrice:        new(big.Int).Set(tx.data.Price),
		to:              tx.data.Recipient,
		amount:          tx.data.Amount,
		data:            tx.data.Payload,
		checkNonce:      true,
		fromFullShardId: tx.data.FromFullShardId,
		toFullShardId:   tx.data.ToFullShardId,
	}

	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}

// WithSignature returns a new transaction with the given signature.
// This signature needs to be formatted as described in the yellow paper (v+27).
func (tx *EvmTransaction) WithSignature(signer Signer, sig []byte) (*EvmTransaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &EvmTransaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

// Cost returns amount + gasprice * gaslimit.
func (tx *EvmTransaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total
}

func (tx *EvmTransaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

type Transaction struct {
	TxType uint8
	EvmTx  *EvmTransaction

	hash atomic.Value
}

func (tx *Transaction) Serialize(w *[]byte) error {
	*w = append(*w, tx.TxType)

	switch tx.TxType {
	case EvmTx:
		bytes, err := rlp.EncodeToBytes(tx.EvmTx)
		if err != nil {
			return err
		}
		serialize.Serialize(w, uint32(len(bytes)))
		*w = append(*w, bytes...)
		return nil
	default:
		return fmt.Errorf("ser: Transacton type %d is not supported", tx.TxType)
	}
}

func (tx *Transaction) Deserialize(bb *serialize.ByteBuffer) error {
	txType, err := bb.GetUInt8()
	if err != nil {
		return err
	}

	switch txType {
	case EvmTx:
		tx.TxType = txType
		bytes, err := bb.GetVarBytes(4)
		if err != nil {
			return err
		}

		if tx.EvmTx == nil {
			tx.EvmTx = new(EvmTransaction)
		}
		return rlp.DecodeBytes(bytes, tx.EvmTx)
	default:
		return fmt.Errorf("deser: Transacton type %d is not supported", tx.TxType)
	}
}

// Hash return the hash of the transaction it contained
func (tx *Transaction) Hash() (h common.Hash) {
	if tx.TxType == EvmTx {
		return tx.EvmTx.Hash()
	}

	log.Error(fmt.Sprintf("do not support tx type %d", tx.TxType))
	return *new(common.Hash)
}

func (tx *Transaction) getNonce() uint64 {
	if tx.TxType == EvmTx {
		return tx.EvmTx.data.AccountNonce
	}

	//todo verify the default value when have more type of tx
	return 0
}

func (tx *Transaction) getPrice() *big.Int {
	if tx.TxType == EvmTx {
		return tx.EvmTx.data.Price
	}

	//todo verify the default value when have more type of tx
	return big.NewInt(0)
}

func (tx *Transaction) Sender(signer Signer) account.Recipient {
	if tx.TxType == EvmTx {
		addr, err := Sender(signer, tx.EvmTx)
		if err != nil {
			log.Error(err.Error(), tx)
			return *new(account.Recipient)
		}

		return addr
	} else {
		log.Error(fmt.Sprintf("do not support tx type %d", tx.TxType))
		return *new(account.Recipient)
	}
}

// Transactions is a EvmTransaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

func (s Transactions) Bytes(i int) []byte {
	enc, _ := serialize.SerializeToBytes(s[i]) //todo error handle?
	return enc
}

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].getNonce() < s[j].getNonce() }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice Transactions

func (s TxByPrice) Len() int           { return len(s) }
func (s TxByPrice) Less(i, j int) bool { return s[i].getPrice().Cmp(s[j].getPrice()) > 0 }
func (s TxByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *TxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

func (s *TxByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// TransactionsByPriceAndNonce represents a set of transactions that can return
// transactions in a profit-maximizing sorted order, while supporting removing
// entire batches of transactions for non-executable accounts.
type TransactionsByPriceAndNonce struct {
	txs    map[account.Recipient]Transactions // Per account nonce-sorted list of transactions
	heads  TxByPrice                          // Next transaction for each unique account (price heap)
	signer Signer                             // Signer for the set of transactions
}

// NewTransactionsByPriceAndNonce creates a transaction set that can retrieve
// price sorted transactions in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// if after providing it to the constructor.
func NewTransactionsByPriceAndNonce(signer Signer, txs map[account.Recipient]Transactions) *TransactionsByPriceAndNonce {
	// Initialize a price based heap with the head transactions
	heads := make(TxByPrice, 0, len(txs))
	for from, accTxs := range txs {
		heads = append(heads, accTxs[0])
		// Ensure the sender address is from the signer
		acc := accTxs[0].Sender(signer)
		txs[acc] = accTxs[1:]
		if from != acc {
			delete(txs, from)
		}
	}
	heap.Init(&heads)

	// Assemble and return the transaction set
	return &TransactionsByPriceAndNonce{
		txs:    txs,
		heads:  heads,
		signer: signer,
	}
}

// Peek returns the next transaction by price.
func (t *TransactionsByPriceAndNonce) Peek() *Transaction {
	if len(t.heads) == 0 {
		return nil
	}
	return t.heads[0]
}

// Shift replaces the current best head with the next one from the same account.
func (t *TransactionsByPriceAndNonce) Shift() {
	acc := t.heads[0].Sender(t.signer)
	if txs, ok := t.txs[acc]; ok && len(txs) > 0 {
		t.heads[0], t.txs[acc] = txs[0], txs[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}
}

// Pop removes the best transaction, *not* replacing it with the next one from
// the same account. This should be used when a transaction cannot be executed
// and hence all subsequent ones should be discarded from the same account.
func (t *TransactionsByPriceAndNonce) Pop() {
	heap.Pop(&t.heads)
}

type CrossShardTransactionDeposit struct {
	TxHash   common.Hash
	From     account.Address
	To       account.Address
	Value    *serialize.Uint256
	GasPrice *serialize.Uint256
}

// Message is a fully derived transaction and implements core.Message
//
// NOTE: In a future PR this will be removed.
type Message struct {
	to              *account.Recipient
	from            account.Recipient
	nonce           uint64
	amount          *big.Int
	gasLimit        uint64
	gasPrice        *big.Int
	data            []byte
	checkNonce      bool
	fromFullShardId uint32
	toFullShardId   uint32
}

func NewMessage(from account.Recipient, to *account.Recipient, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool, fromShardId, toShardId uint32) Message {
	return Message{
		from:            from,
		to:              to,
		nonce:           nonce,
		amount:          amount,
		gasLimit:        gasLimit,
		gasPrice:        gasPrice,
		data:            data,
		checkNonce:      checkNonce,
		fromFullShardId: fromShardId,
		toFullShardId:   toShardId,
	}
}

func (m Message) From() account.Recipient { return m.from }
func (m Message) To() *account.Recipient  { return m.to }
func (m Message) GasPrice() *big.Int      { return m.gasPrice }
func (m Message) Value() *big.Int         { return m.amount }
func (m Message) Gas() uint64             { return m.gasLimit }
func (m Message) Nonce() uint64           { return m.nonce }
func (m Message) Data() []byte            { return m.data }
func (m Message) CheckNonce() bool        { return m.checkNonce }
func (m Message) IsCrosShard() bool       { return m.fromFullShardId != m.toFullShardId }
func (m Message) FromFullShardId() uint32 { return m.fromFullShardId }
func (m Message) ToFullShardId() uint32   { return m.toFullShardId }
