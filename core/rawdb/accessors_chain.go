// Modified from go-ethereum under GNU Lesser General Public License
package rawdb

import (
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

const DBLOG = "db-operation"

type minorBlockHeaders struct {
	Headers []*types.MinorBlockHeader `bytesizeofslicelen:"4"`
}

type HashList struct {
	HList []common.Hash `bytesizeofslicelen:"4"`
}

// ReadCanonicalHash retrieves the hash assigned to a canonical block number.
func ReadCanonicalHash(db DatabaseReader, chainType ChainType, number uint64) common.Hash {
	data, _ := db.Get(headerHashKey(chainType, number))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// WriteCanonicalHash stores the hash assigned to a canonical block number.
func WriteCanonicalHash(db DatabaseWriter, chainType ChainType, hash common.Hash, number uint64) {
	if err := db.Put(headerHashKey(chainType, number), hash.Bytes()); err != nil {
		log.Crit("Failed to store number to hash mapping", "err", err)
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db DatabaseDeleter, chainType ChainType, number uint64) {
	if err := db.Delete(headerHashKey(chainType, number)); err != nil {
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
func HasHeader(db DatabaseReader, hash common.Hash) bool {
	if has, err := db.Has(headerKey(hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadMinorBlockHeader retrieves the block header corresponding to the hash.
func ReadMinorBlockHeader(db DatabaseReader, hash common.Hash) *types.MinorBlockHeader {
	data, _ := db.Get(headerKey(hash))
	if len(data) == 0 {
		return nil
	}

	header := new(types.MinorBlockHeader)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), header); err != nil {
		log.Error("Invalid block header Deserialize", "hash", hash, "err", err)
		return nil
	}
	return header
}

func WriteMinorBlockHeader(db DatabaseWriter, header *types.MinorBlockHeader) {
	// Write the hash -> number mapping
	var (
		hash    = header.Hash()
		encoded = encodeBlockNumber(header.Number)
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
	key = headerKey(hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// DeleteMinorBlockHeader removes all block header data associated with a hash.
func DeleteMinorBlockHeader(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(headerKey(hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerNumberKey(hash)); err != nil {
		log.Crit("Failed to delete hash to number mapping", "err", err)
	}
}

func ReadLatestMinorBlockHeaders(db DatabaseReader, hash common.Hash) []*types.MinorBlockHeader {
	data, _ := db.Get(latestMHeaderKey(hash))
	if len(data) == 0 {
		return nil
	}
	headers := new(minorBlockHeaders)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), headers); err != nil {
		log.Error("Invalid block header Deserialize", "hash", hash, "err", err)
		return nil
	}
	return headers.Headers
}

func WriteLatestMinorBlockHeaders(db DatabaseWriter, hash common.Hash, headers []*types.MinorBlockHeader) {
	// Write the encoded header
	hs := minorBlockHeaders{headers}
	data, err := serialize.SerializeToBytes(hs)
	if err != nil {
		log.Crit("Failed to Serialize header", "err", err)
	}
	key := latestMHeaderKey(hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

func DeleteLatestMinorBlockHeaders(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(latestMHeaderKey(hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
}

// ReadRootBlockHeader retrieves the block header corresponding to the hash.
func ReadRootBlockHeader(db DatabaseReader, hash common.Hash) *types.RootBlockHeader {
	data, _ := db.Get(headerKey(hash))
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
		encoded = encodeBlockNumber(header.NumberU64())
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
	key = headerKey(hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}

// DeleteRootBlockHeader removes all block header data associated with a hash.
func DeleteRootBlockHeader(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(headerKey(hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}
	if err := db.Delete(headerNumberKey(hash)); err != nil {
		log.Crit("Failed to delete hash to number mapping", "err", err)
	}
}

// HasBlock verifies the existence of a block body corresponding to the hash.
func HasBlock(db DatabaseReader, hash common.Hash) bool {
	if has, err := db.Has(blockKey(hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadMinorBlock retrieves the block body corresponding to the hash.
func ReadMinorBlock(db DatabaseReader, hash common.Hash) *types.MinorBlock {
	data, _ := db.Get(blockKey(hash))
	if len(data) == 0 {
		return nil
	}
	block := new(types.MinorBlock)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), block); err != nil {
		log.Error("Invalid block body Deserialize", "hash", hash, "err", err)
		return nil
	}
	return block
}

// WriteMinorBlock storea a block body into the database.
func WriteMinorBlock(db DatabaseWriter, block *types.MinorBlock) {
	data, err := serialize.SerializeToBytes(block)
	if err != nil {
		log.Crit("Failed to serialize body", "err", err)
	}
	log.Info(DBLOG+" Write MinorBlock", "branch", fmt.Sprintf("%x", block.Branch().Value), "height", block.NumberU64(), "hash", block.Hash().TerminalString(), "len(tx)", len(block.Transactions()))
	if err := db.Put(blockKey(block.Hash()), data); err != nil {
		log.Crit("Failed to store minor block body", "err", err)
	}
}

// ReadRootBlock retrieves the block rootBlockBody corresponding to the hash.
func ReadRootBlock(db DatabaseReader, hash common.Hash) *types.RootBlock {
	data, _ := db.Get(blockKey(hash))
	if len(data) == 0 {
		return nil
	}
	block := new(types.RootBlock)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), block); err != nil {
		log.Error("Invalid block rootBlockBody Deserialize", "hash", hash, "err", err)
		return nil
	}
	return block
}

// WriteRootBlock storea a block rootBlockBody into the database.
func WriteRootBlock(db DatabaseWriter, block *types.RootBlock) {
	data, err := serialize.SerializeToBytes(block)
	if err != nil {
		log.Crit("Failed to serialize RootBlock", "err", err)
	}
	log.Info(DBLOG+" Write RootBlock", "height", block.NumberU64(), "hash", block.Hash())
	if err := db.Put(blockKey(block.Hash()), data); err != nil {
		log.Crit("Failed to store RootBlock", "err", err)
	}
}

// DeleteBlock removes block data associated with a hash.
func DeleteBlock(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(blockKey(hash)); err != nil {
		log.Crit("Failed to delete block", "err", err)
	}
}

// HasReceipts verifies the existence of all the transaction receipts belonging
// to a block.
func HasReceipts(db DatabaseReader, hash common.Hash) bool {
	if has, err := db.Has(blockReceiptsKey(hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func ReadReceipts(db DatabaseReader, hash common.Hash) types.Receipts {
	// Retrieve the flattened receipt slice
	data, _ := db.Get(blockReceiptsKey(hash))
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
func WriteReceipts(db DatabaseWriter, hash common.Hash, receipts types.Receipts) {
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
	if err := db.Put(blockReceiptsKey(hash), bytes); err != nil {
		log.Crit("Failed to store block receipts", "err", err)
	}
}

// DeleteReceipts removes all receipt data associated with a block hash.
func DeleteReceipts(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(blockReceiptsKey(hash)); err != nil {
		log.Crit("Failed to delete block receipts", "err", err)
	}
}

// DeleteBlock removes all block data associated with a hash.
func DeleteMinorBlock(db DatabaseDeleter, hash common.Hash) {
	DeleteReceipts(db, hash)
	DeleteBlock(db, hash)
	DeleteMinorBlockCommitStatus(db, hash)
}

// DeleteRootBlock removes all block data associated with a hash.
func DeleteRootBlock(db DatabaseDeleter, hash common.Hash) {
	DeleteLatestMinorBlockHeaders(db, hash)
	DeleteBlock(db, hash)
}

func WriteRootBlockCommittingHash(db DatabaseWriter, hash common.Hash) {
	//  use write-ahead log so if crashed the root block can be re-broadcasted
	if err := db.Put(rbCommittingKey, hash.Bytes()); err != nil {
		log.Crit("Failed to store rb committing block's hash", "err", err)
	}
}

func ReadRbCommittingHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(rbCommittingKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}
func DeleteRbCommittingHash(db DatabaseDeleter) {
	if err := db.Delete(rbCommittingKey); err != nil {
		log.Crit("Failed to delete block receipts", "err", err)
	}
}

// FindCommonMinorAncestor returns the last common ancestor of two block headers
func FindCommonMinorAncestor(db DatabaseReader, a, b *types.MinorBlockHeader) *types.MinorBlockHeader {
	for bn := b.Number; a.Number > bn; {
		a = ReadMinorBlockHeader(db, a.ParentHash)
		if a == nil {
			return nil
		}
	}
	for an := a.Number; an < b.Number; {
		b = ReadMinorBlockHeader(db, b.ParentHash)
		if b == nil {
			return nil
		}
	}
	for a.Hash() != b.Hash() {
		a = ReadMinorBlockHeader(db, a.ParentHash)
		if a == nil {
			return nil
		}
		b = ReadMinorBlockHeader(db, b.ParentHash)
		if b == nil {
			return nil
		}
	}
	return a
}

// FindCommonRootAncestor returns the last common ancestor of two block headers
func FindCommonRootAncestor(db DatabaseReader, a, b *types.RootBlockHeader) *types.RootBlockHeader {
	for bn := b.NumberU64(); a.NumberU64() > bn; {
		a = ReadRootBlockHeader(db, a.ParentHash)
		if a == nil {
			return nil
		}
	}
	for an := a.NumberU64(); an < b.NumberU64(); {
		b = ReadRootBlockHeader(db, b.ParentHash)
		if b == nil {
			return nil
		}
	}
	for a.Hash() != b.Hash() {
		a = ReadRootBlockHeader(db, a.ParentHash)
		if a == nil {
			return nil
		}
		b = ReadRootBlockHeader(db, b.ParentHash)
		if b == nil {
			return nil
		}
	}
	return a
}

func WriteTotalTx(db DatabaseWriter, hash common.Hash, txCount uint32) {
	data := encodeUint32(txCount)
	if err := db.Put(totalTxCountKey(hash), data); err != nil {
		log.Crit("Failed to store total Tx", "err", err)
	}
}
func ReadTotalTx(db DatabaseReader, hash common.Hash) *uint32 {
	data, _ := db.Get(totalTxCountKey(hash))
	if len(data) == 0 {
		return nil
	}
	number := binary.BigEndian.Uint32(data)
	return &number

}

func WriteGenesisBlock(db DatabaseWriter, rHash common.Hash, block *types.MinorBlock) {
	data, err := serialize.SerializeToBytes(block)
	if err != nil {
		log.Crit("can not serilalize Minor block")
	}
	key := makeGenesisKey(rHash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store genesis", "err", err)
	}
}
func ReadGenesis(db DatabaseReader, hash common.Hash) *types.MinorBlock {
	data, _ := db.Get(makeGenesisKey(hash))
	if len(data) == 0 {
		return nil
	}
	res := new(types.MinorBlock)
	if err := serialize.DeserializeFromBytes(data, res); err != nil {
		return nil
	}
	return res
}

func WriteConfirmedCrossShardTxList(db DatabaseWriter, hash common.Hash, list *types.CrossShardTransactionDepositList) {
	data, err := serialize.SerializeToBytes(list)
	if err != nil {
		log.Crit("can not serialize CrossShardTransactionDepositList")
	}
	key := makeConfirmedXShardKey(hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}
func ReadConfirmedCrossShardTxList(db DatabaseReader, hash common.Hash) *types.CrossShardTransactionDepositList {
	data, _ := db.Get(makeConfirmedXShardKey(hash))
	if len(data) == 0 {
		return nil
	}
	list := new(types.CrossShardTransactionDepositList)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), list); err != nil {
		log.Error("Invalid block header Deserialize", "hash", hash, "err", err)
		return nil
	}
	return list
}

func WriteCrossShardTxList(db DatabaseWriter, hash common.Hash, list *types.CrossShardTransactionDepositList) {
	data, err := serialize.SerializeToBytes(list)
	if err != nil {
		log.Crit("can not serialize CrossShardTransactionDepositList")
	}
	key := makeXShardTxList(hash)
	if err := db.Put(key, data); err != nil {
		log.Crit("Failed to store header", "err", err)
	}
}
func ReadCrossShardTxList(db DatabaseReader, hash common.Hash) *types.CrossShardTransactionDepositList {
	data, _ := db.Get(makeXShardTxList(hash))
	if len(data) == 0 {
		return nil
	}
	list := new(types.CrossShardTransactionDepositList)
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), list); err != nil {
		log.Error("Invalid block header Deserialize", "hash", hash, "err", err)
		return nil
	}
	return list
}

func WriteLastConfirmedMinorBlockHeaderAtRootBlock(db DatabaseWriter, rHash common.Hash, mHash common.Hash) {
	if err := db.Put(makeRLastMHash(rHash), mHash.Bytes()); err != nil {
		log.Crit("failed to store last confirmed  minot block at root block")
	}
}

func ReadLastConfirmedMinorBlockHeaderAtRootBlock(db DatabaseReader, rHash common.Hash) common.Hash {
	data, _ := db.Get(makeRLastMHash(rHash))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

func WriteMinorBlockCnt(db DatabaseWriter, fullShardID uint32, height uint32, data []byte) {
	if err := db.Put(makeMinorCount(fullShardID, height), data); err != nil {
		log.Crit("failed to store minor block cnt")
	}
}

func GetMinorBlockCnt(db DatabaseReader, fullShardID uint32, height uint32) []byte {
	data, _ := db.Get(makeMinorCount(fullShardID, height))
	if len(data) == 0 {
		return []byte{}
	}
	return data
}

func WriteMinorBlockCoinbase(db DatabaseWriter, mHash common.Hash, coinbaseToken *types.TokenBalances) {
	tokenBytes, err := serialize.SerializeToBytes(coinbaseToken)
	if err != nil {
		log.Crit("failed to put minor block coinbase", "serialize err", err)
	}
	if err := db.Put(makeMinorBlockCoinbase(mHash), tokenBytes); err != nil {
		log.Crit("failed to put minor block coinbase ", "put err", err)

	}
}

func GetMinorBlockCoinbaseToken(db DatabaseReader, hash common.Hash) *types.TokenBalances {
	data, _ := db.Get(makeMinorBlockCoinbase(hash))
	if len(data) == 0 {
		return nil
	}
	tokens := new(types.TokenBalances)
	if err := serialize.DeserializeFromBytes(data, tokens); err != nil {
		log.Error("GetMinorBlockCoinbaseToken", "deserialize err", err)
		return nil
	}
	return tokens
}

func ContainMinorBlockByHash(db DatabaseReader, hash common.Hash) bool {
	if has, err := db.Has(makeMinorBlockCoinbase(hash)); !has || err != nil {
		return false
	}
	return true
}

func PutRootBlockConfirmingMinorBlock(db DatabaseWriter, mBlockID []byte, rHash common.Hash) {
	if err := db.Put(makeRootBlockConfirmingMinorBlock(mBlockID), rHash.Bytes()); err != nil {
		log.Crit("failed to put rootBlock confirming minor block", "err", err)
	}
}

func GetRootBlockConfirmingMinorBlock(db DatabaseReader, mBlockID []byte) common.Hash {
	data, _ := db.Get(makeRootBlockConfirmingMinorBlock(mBlockID))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}
func PutXShardDepositHashList(db DatabaseWriter, h common.Hash, hList *HashList) {
	bytes, err := serialize.SerializeToBytes(hList)
	if err != nil {
		log.Crit("can not serialize HashList")
	}
	if err := db.Put(makeXShardDepositHashList(h), bytes); err != nil {
		log.Crit("failed to put xshard deposit hash list err", err)
	}
}

func GetXShardDepositHashList(db DatabaseReader, h common.Hash) *HashList {
	data, _ := db.Get(makeXShardDepositHashList(h))
	hList := new(HashList)
	if err := serialize.DeserializeFromBytes(data, hList); err != nil {
		log.Error("GetXShardDepositHashList", "DeserializeFromBytes err", err)
		return nil
	}
	return hList
}

func WriteCommitMinorBlock(db DatabaseWriter, h common.Hash) {
	if err := db.Put(makeCommitMinorBlock(h), []byte{1}); err != nil { // value must not empty
		log.Crit("failed to write commit minor block", "err", err)
	}
}

func HasCommitMinorBlock(db DatabaseReader, h common.Hash) bool {
	if has, err := db.Has(makeCommitMinorBlock(h)); !has || err != nil {
		return false
	}
	return true
}

func DeleteMinorBlockCommitStatus(db DatabaseDeleter, h common.Hash) {
	if err := db.Delete(makeCommitMinorBlock(h)); err != nil {
		log.Crit("Failed to delete commit minor block", "err", err)
	}
}
