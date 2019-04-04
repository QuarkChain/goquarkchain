// Modified from go-ethereum under GNU Lesser General Public License

package rawdb

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

// ReadBlockContentLookupEntry retrieves the positional metadata associated with a transaction
// hash to allow retrieving the transaction or receipt by hash.
func ReadBlockContentLookupEntry(db DatabaseReader, hash common.Hash) (common.Hash, uint64) {
	data, _ := db.Get(lookupKey(hash))
	if len(data) == 0 {
		return common.Hash{}, 0
	}
	var entry LookupEntry
	if err := serialize.Deserialize(serialize.NewByteBuffer(data), &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", "hash", hash, "err", err)
		return common.Hash{}, 0
	}
	return entry.BlockHash, entry.Index
}

// WriteBlockContentLookupEntries stores a positional metadata for every transaction from
// a block, enabling hash based transaction and receipt lookups.
func WriteBlockContentLookupEntries(db DatabaseWriter, block types.IBlock) {
	hash := block.Hash()
	for i, item := range block.Content() {
		entry := LookupEntry{
			BlockHash: hash,
			Index:     uint64(i),
		}
		data, err := serialize.SerializeToBytes(entry)
		if err != nil {
			log.Crit("Failed to encode content lookup entry", "err", err)
		}
		if err := db.Put(lookupKey(item.Hash()), data); err != nil {
			log.Crit("Failed to store content lookup entry", "err", err)
		}
	}
}

// DeleteBlockContentLookupEntry removes all transaction data associated with a hash.
func DeleteBlockContentLookupEntry(db DatabaseDeleter, hash common.Hash) {
	db.Delete(lookupKey(hash))
}

// ReadMinorHeader retrieves a specific MinorHeader from the database, along with
// its added positional metadata.
func ReadMinorHeaderFromRootBlock(db DatabaseReader, hash common.Hash) (*types.MinorBlockHeader, common.Hash, uint64) {
	blockHash, headerIndex := ReadBlockContentLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0
	}
	block := ReadRootBlock(db, blockHash)
	if block == nil || len(block.MinorBlockHeaders()) <= int(headerIndex) {
		log.Error("Minor Block header referenced missing", "hash", blockHash, "index", headerIndex)
		return nil, common.Hash{}, 0
	}
	return block.MinorBlockHeaders()[headerIndex], blockHash, headerIndex
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransaction(db DatabaseReader, hash common.Hash) (*types.Transaction, common.Hash, uint64) {
	blockHash, txIndex := ReadBlockContentLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0
	}
	block := ReadMinorBlock(db, blockHash)
	if block == nil || len(block.Transactions()) <= int(txIndex) {
		log.Error("Transaction referenced missing", "hash", blockHash, "index", txIndex)
		return nil, common.Hash{}, 0
	}
	return block.Transactions()[txIndex], blockHash, txIndex
}

// ReadReceipt retrieves a specific transaction receipt from the database, along with
// its added positional metadata.
func ReadReceipt(db DatabaseReader, hash common.Hash) (*types.Receipt, common.Hash, uint64) {
	blockHash, receiptIndex := ReadBlockContentLookupEntry(db, hash)
	if blockHash == (common.Hash{}) {
		return nil, common.Hash{}, 0
	}
	receipts := ReadReceipts(db, blockHash)
	if len(receipts) <= int(receiptIndex) {
		log.Error("Receipt refereced missing", "hash", blockHash, "index", receiptIndex)
		return nil, common.Hash{}, 0
	}
	return receipts[receiptIndex], blockHash, receiptIndex
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func ReadBloomBits(db DatabaseReader, bit uint, section uint64, head common.Hash) ([]byte, error) {
	return db.Get(bloomBitsKey(bit, section, head))
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func WriteBloomBits(db DatabaseWriter, bit uint, section uint64, head common.Hash, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section, head), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}
