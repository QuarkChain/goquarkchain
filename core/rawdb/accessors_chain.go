// Modified from go-ethereum under GNU Lesser General Public License
package rawdb

import (
	"encoding/binary"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
)

type minorBlockBody struct {
	Transactions types.Transactions `bytesize:"4"`
	Trackingdata []byte             `bytesize:"2"`
}

type rootBlockBody struct {
	MinorBlockHeaders types.MinorBlockHeaders `bytesize:"4"`
	Trackingdata      []byte                  `bytesize:"2"`
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block number.
func ReadCanonicalHash(db DatabaseReader, number uint64) common.Hash {
	data, _ := db.Get(headerHashKey(number))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteCanonicalHash stores the hash assigned to a canonical block number.
func WriteCanonicalHash(db DatabaseWriter, hash common.Hash, number uint64) {
	if err := db.Put(headerHashKey(number), hash.Bytes()); err != nil {
		log.Crit("Failed to store number to hash mapping", "err", err)
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db DatabaseDeleter, number uint64) {
	if err := db.Delete(headerHashKey(number)); err != nil {
		log.Crit("Failed to delete number to hash mapping", "err", err)
	}
}

// ReadHeaderNumber returns the header number assigned to a hash.
func ReadHeaderNumber(db DatabaseReader, hash common.Hash) *uint64 {
	data, _ := db.Get(headerNumberKey(hash))
	if len(data) != 8 {
		return nil
	}
	number := binary.BigEndian.Uint64(data)
	return &number
}

// ReadHeadHeaderHash retrieves the hash of the current canonical head header.
func ReadHeadHeaderHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headHeaderKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadHeaderHash stores the hash of the current canonical head header.
func WriteHeadHeaderHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headHeaderKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last header's hash", "err", err)
	}
}

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func ReadHeadBlockHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadBlockHash stores the head block's hash.
func WriteHeadBlockHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headBlockKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// ReadHeadFastBlockHash retrieves the hash of the current fast-sync head block.
func ReadHeadFastBlockHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headFastBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteHeadFastBlockHash stores the hash of the current fast-sync head block.
func WriteHeadFastBlockHash(db DatabaseWriter, hash common.Hash) {
	if err := db.Put(headFastBlockKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store last fast block's hash", "err", err)
	}
}

// ReadFastTrieProgress retrieves the number of tries nodes fast synced to allow
// reporting correct numbers across restarts.
func ReadFastTrieProgress(db DatabaseReader) uint64 {
	data, _ := db.Get(fastTrieProgressKey)
	if len(data) == 0 {
		return 0
	}
	return new(big.Int).SetBytes(data).Uint64()
}

// WriteFastTrieProgress stores the fast sync trie process counter to support
// retrieving it across restarts.
func WriteFastTrieProgress(db DatabaseWriter, count uint64) {
	if err := db.Put(fastTrieProgressKey, new(big.Int).SetUint64(count).Bytes()); err != nil {
		log.Crit("Failed to store fast sync trie progress", "err", err)
	}
}

// HasHeader verifies the existence of a block header corresponding to the hash.
func HasHeader(db DatabaseReader, hash common.Hash, number uint64) bool {
	if has, err := db.Has(headerKey(number, hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadHeader retrieves the block header corresponding to the hash.
func ReadMinorBlockHeader(db DatabaseReader, hash common.Hash, number uint64) (*types.MinorBlockHeader, *types.MinorBlockMeta) {
	data, _ := db.Get(headerKey(number, hash))
	if len(data) == 0 {
		return nil, nil
	}
	header := new(types.MinorBlockHeader)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), header); err != nil {
		log.Error("Invalid block header Deserialize", "hash", hash, "err", err)
		return nil, nil
	}
	data, _ = db.Get(metaKey(number, hash))
	if len(data) == 0 {
		return nil, nil
	}
	meta := new(types.MinorBlockMeta)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), meta); err != nil {
		log.Error("Invalid block header Deserialize", "hash", hash, "err", err)
		return nil, nil
	}
	return header, meta
}

func WriteMinorBlockHeader(db DatabaseWriter, header *types.MinorBlockHeader) {
	// Write the hash -> number mapping
	var (
		hash    = header.Hash()
		number  = header.Number
		encoded = encodeBlockNumber(number)
	)
	key := headerNumberKey(hash)
	if err := db.Put(key, encoded); err != nil {
		log.Crit("Failed to store hash to number mapping", "err", err)
	}
	// Write the encoded header
	data, err := serialize.SerializeToBytes(header)
	if err != nil {
		log.Crit("Failed to Serialize header", "err", err)
	}
	key = headerKey(number, hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

func WriteMinorBlockMeta(db DatabaseWriter, header *types.MinorBlockHeader, meta *types.MinorBlockMeta) {
	// Write the hash -> number mapping
	var (
		hash   = header.Hash()
		number = header.Number
	)
	// Write the encoded meta
	data, err := serialize.SerializeToBytes(meta)
	if err != nil {
		log.Crit("Failed to Serialize header", "err", err)
	}
	key := metaKey(number, hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// DeleteHeader removes all block header data associated with a hash.
func DeleteMinorBlockHeader(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(headerKey(number, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(metaKey(number, hash)); err != nil {
		log.Crit("Failed to delete meta", "err", err)
	}
	if err := db.Delete(headerNumberKey(hash)); err != nil {
		log.Crit("Failed to delete hash to number mapping", "err", err)
	}
}

// ReadHeader retrieves the block header corresponding to the hash.
func ReadRootBlockHeader(db DatabaseReader, hash common.Hash, number uint64) *types.RootBlockHeader {
	data, _ := db.Get(headerKey(number, hash))
	if len(data) == 0 {
		return nil
	}
	header := new(types.RootBlockHeader)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), header); err != nil {
		log.Error("Invalid block header Deserialize", "hash", hash, "err", err)
		return nil
	}
	return header
}

// WriteRootBlockHeader stores a block header into the database and also stores the hash-
// to-number mapping.
func WriteRootBlockHeader(db DatabaseWriter, header *types.RootBlockHeader) {
	// Write the hash -> number mapping
	var (
		hash    = header.Hash()
		number  = header.NumberU64()
		encoded = encodeBlockNumber(number)
	)
	key := headerNumberKey(hash)
	if err := db.Put(key, encoded); err != nil {
		log.Crit("Failed to store hash to number mapping", "err", err)
	}
	// Write the encoded header
	data, err := serialize.SerializeToBytes(header)
	if err != nil {
		log.Crit("Failed to Serialize header", "err", err)
	}
	key = headerKey(number, hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// DeleteHeader removes all block header data associated with a hash.
func DeleteRootBlockHeader(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(headerKey(number, hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerNumberKey(hash)); err != nil {
		log.Crit("Failed to delete hash to number mapping", "err", err)
	}
}

// HasBody verifies the existence of a block body corresponding to the hash.
func HasBody(db DatabaseReader, hash common.Hash, number uint64) bool {
	if has, err := db.Has(blockBodyKey(number, hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadBody retrieves the block body corresponding to the hash.
func ReadMinorBlockBody(db DatabaseReader, hash common.Hash, number uint64) (types.Transactions, []byte) {
	data, _ := db.Get(blockBodyKey(number, hash))
	if len(data) == 0 {
		return nil, nil
	}
	body := new(minorBlockBody)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), body); err != nil {
		log.Error("Invalid block body Deserialize", "hash", hash, "err", err)
		return nil, nil
	}
	return body.Transactions, body.Trackingdata
}

// WriteBody storea a block body into the database.
func WriteMinorBlockBody(db DatabaseWriter, hash common.Hash, number uint64, transactions types.Transactions, trackingData []byte) {
	data, err := serialize.SerializeToBytes(minorBlockBody{Transactions: transactions, Trackingdata: trackingData})
	if err != nil {
		log.Crit("Failed to serialize body", "err", err)
	}
	if err := db.Put(blockBodyKey(number, hash), data); err != nil {
		log.Crit("Failed to store minor block body", "err", err)
	}
}

// ReadRootBlockBody retrieves the block rootBlockBody corresponding to the hash.
func ReadRootBlockBody(db DatabaseReader, hash common.Hash, number uint64) (types.MinorBlockHeaders, []byte) {
	data, _ := db.Get(blockBodyKey(number, hash))
	if len(data) == 0 {
		return nil, nil
	}
	body := new(rootBlockBody)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), body); err != nil {
		log.Error("Invalid block rootBlockBody Deserialize", "hash", hash, "err", err)
		return nil, nil
	}
	return body.MinorBlockHeaders, body.Trackingdata
}

// WriteRootBlockBody storea a block rootBlockBody into the database.
func WriteRootBlockBody(db DatabaseWriter, hash common.Hash, number uint64, minorHeaders types.MinorBlockHeaders, trackingData []byte) {
	data, err := serialize.SerializeToBytes(rootBlockBody{MinorBlockHeaders: minorHeaders, Trackingdata: trackingData})
	if err != nil {
		log.Crit("Failed to serialize rootBlockBody", "err", err)
	}
	if err := db.Put(blockBodyKey(number, hash), data); err != nil {
		log.Crit("Failed to store block rootBlockBody", "err", err)
	}
}

// DeleteBody removes all block body data associated with a hash.
func DeleteBody(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(blockBodyKey(number, hash)); err != nil {
		log.Crit("Failed to delete block body", "err", err)
	}
}

// ReadTd retrieves a block's total difficulty corresponding to the hash.
func ReadTd(db DatabaseReader, hash common.Hash, number uint64) *big.Int {
	data, _ := db.Get(headerTDKey(number, hash))
	if len(data) == 0 {
		return nil
	}
	td := new(big.Int)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), td); err != nil {
		log.Error("Invalid block total difficulty", "hash", hash, "err", err)
		return nil
	}
	return td
}

// WriteTd stores the total difficulty of a block into the database.
func WriteTd(db DatabaseWriter, hash common.Hash, number uint64, td *big.Int) {
	data, err := serialize.SerializeToBytes(td)
	if err != nil {
		log.Crit("Failed to Serialize block total difficulty", "err", err)
	}
	if err := db.Put(headerTDKey(number, hash), data); err != nil {
		log.Crit("Failed to store block total difficulty", "err", err)
	}
}

// DeleteTd removes all block total difficulty data associated with a hash.
func DeleteTd(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(headerTDKey(number, hash)); err != nil {
		log.Crit("Failed to delete block total difficulty", "err", err)
	}
}

// HasReceipts verifies the existence of all the transaction receipts belonging
// to a block.
func HasReceipts(db DatabaseReader, hash common.Hash, number uint64) bool {
	if has, err := db.Has(blockReceiptsKey(number, hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func ReadReceipts(db DatabaseReader, hash common.Hash, number uint64) types.Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(number, hash))
	if len(data) == 0 {
		return nil
	}
	// Convert the receipts from their storage form to their internal representation
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(receipt)
	}
	return receipts
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func WriteReceipts(db DatabaseWriter, hash common.Hash, number uint64, receipts types.Receipts) {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*types.ReceiptForStorage)(receipt)
	}
	bytes, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		log.Crit("Failed to encode block receipts", "err", err)
	}
	// Store the flattened receipt slice
	if err := db.Put(blockReceiptsKey(number, hash), bytes); err != nil {
		log.Crit("Failed to store block receipts", "err", err)
	}
}

// DeleteReceipts removes all receipt data associated with a block hash.
func DeleteReceipts(db DatabaseDeleter, hash common.Hash, number uint64) {
	if err := db.Delete(blockReceiptsKey(number, hash)); err != nil {
		log.Crit("Failed to delete block receipts", "err", err)
	}
}

// ReadBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body. If either the header or body could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block body the header and thus
// canonical hash can be stored in the database but the body data not (yet).
func ReadMinorBlock(db DatabaseReader, hash common.Hash, number uint64) *types.MinorBlock {
	header, meta := ReadMinorBlockHeader(db, hash, number)
	if header == nil {
		return nil
	}
	transactions, trackingData := ReadMinorBlockBody(db, hash, number)
	if transactions == nil {
		return nil
	}
	return types.NewMinorBlockWithHeader(header, meta).WithBody(transactions, trackingData)
}

// WriteBlock serializes a block into the database, header and body separately.
func WriteMinorBlock(db DatabaseWriter, block *types.MinorBlock) {
	WriteMinorBlockBody(db, block.Hash(), block.Number(), block.Transactions(), block.TrackingData())
	WriteMinorBlockHeader(db, block.Header())
	WriteMinorBlockMeta(db, block.Header(), block.Meta())
}

// DeleteBlock removes all block data associated with a hash.
func DeleteMinorBlock(db DatabaseDeleter, hash common.Hash, number uint64) {
	DeleteReceipts(db, hash, number)
	DeleteMinorBlockHeader(db, hash, number)
	DeleteBody(db, hash, number)
	DeleteTd(db, hash, number)
}

// ReadRootBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and rootBlockBody. If either the header or rootBlockBody could not
// be retrieved nil is returned.
//
// Note, due to concurrent download of header and block rootBlockBody the header and thus
// canonical hash can be stored in the database but the rootBlockBody data not (yet).
func ReadRootBlock(db DatabaseReader, hash common.Hash, number uint64) *types.RootBlock {
	header := ReadRootBlockHeader(db, hash, number)
	if header == nil {
		return nil
	}
	minorHeaders, trackingData := ReadRootBlockBody(db, hash, number)
	if minorHeaders == nil {
		return nil
	}
	return types.NewRootBlockWithHeader(header).WithBody(minorHeaders, trackingData)
}

// WriteRootBlock serializes a block into the database, header and rootBlockBody separately.
func WriteRootBlock(db DatabaseWriter, block *types.RootBlock) {
	WriteRootBlockBody(db, block.Hash(), block.NumberU64(), block.MinorBlockHeaders(), block.TrackingData())
	WriteRootBlockHeader(db, block.Header())
}

// DeleteRootBlock removes all block data associated with a hash.
func DeleteRootBlock(db DatabaseDeleter, hash common.Hash, number uint64) {
	DeleteRootBlockHeader(db, hash, number)
	DeleteBody(db, hash, number)
	DeleteTd(db, hash, number)
}

// FindCommonAncestor returns the last common ancestor of two block headers
func FindCommonAncestor(db DatabaseReader, a, b *types.MinorBlockHeader) *types.MinorBlockHeader {
	for bn := b.Number; a.Number > bn; {
		a, _ = ReadMinorBlockHeader(db, a.ParentHash, a.Number-1)
		if a == nil {
			return nil
		}
	}
	for an := a.Number; an < b.Number; {
		b, _ = ReadMinorBlockHeader(db, b.ParentHash, b.Number-1)
		if b == nil {
			return nil
		}
	}
	for a.Hash() != b.Hash() {
		a, _ = ReadMinorBlockHeader(db, a.ParentHash, a.Number-1)
		if a == nil {
			return nil
		}
		b, _ = ReadMinorBlockHeader(db, b.ParentHash, b.Number-1)
		if b == nil {
			return nil
		}
	}
	return a
}

// FindCommonRootAncestor returns the last common ancestor of two block headers
func FindCommonRootAncestor(db DatabaseReader, a, b *types.RootBlockHeader) *types.RootBlockHeader {
	for bn := b.NumberU64(); a.NumberU64() > bn; {
		a = ReadRootBlockHeader(db, a.ParentHash, a.NumberU64()-1)
		if a == nil {
			return nil
		}
	}
	for an := a.NumberU64(); an < b.NumberU64(); {
		b = ReadRootBlockHeader(db, b.ParentHash, b.NumberU64()-1)
		if b == nil {
			return nil
		}
	}
	for a.Hash() != b.Hash() {
		a = ReadRootBlockHeader(db, a.ParentHash, a.NumberU64()-1)
		if a == nil {
			return nil
		}
		b = ReadRootBlockHeader(db, b.ParentHash, b.NumberU64()-1)
		if b == nil {
			return nil
		}
	}
	return a
}
