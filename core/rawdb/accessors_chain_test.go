// Modified from go-ethereum under GNU Lesser General Public License

package rawdb

import (
	"bytes"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	"testing"
)

var (
	limitedSizeByes = []byte{'\x01', '\x02', '\x03'}
	tx1             = types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(1, account.BytesToIdentityRecipient([]byte{0x11}), big.NewInt(111), 1111, big.NewInt(11111), 0, 1, 1, 0, []byte{0x11, 0x11, 0x11}, 0, 0)}
	tx2             = types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(2, account.BytesToIdentityRecipient([]byte{0x22}), big.NewInt(222), 2222, big.NewInt(22222), 0, 1, 1, 0, []byte{0x22, 0x22, 0x22}, 0, 0)}
	tx3             = types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(3, account.BytesToIdentityRecipient([]byte{0x33}), big.NewInt(333), 3333, big.NewInt(33333), 0, 1, 1, 0, []byte{0x33, 0x33, 0x33}, 0, 0)}
	txs             = types.Transactions{&tx1, &tx2, &tx3}

	header1 = &types.MinorBlockHeader{Number: uint64(41)}
	header2 = &types.MinorBlockHeader{Number: uint64(42)}
	header3 = &types.MinorBlockHeader{Number: uint64(43)}
	headers = types.MinorBlockHeaders{header1, header2, header3}
)

// Tests block header storage and retrieval operations.
func TestMinorBlockHeaderStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test header to move around the database and make sure it's really new
	//todo init header and meta
	header := &types.MinorBlockHeader{Number: uint64(42)}
	if entry := ReadMinorBlockHeader(db, header.Hash()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	// Write and verify the header in the database
	WriteMinorBlockHeader(db, header)
	if entry := ReadMinorBlockHeader(db, header.Hash()); entry == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != header.Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, header)
	}
	// Delete the header and verify the execution
	DeleteMinorBlockHeader(db, header.Hash())
	if entry := ReadMinorBlockHeader(db, header.Hash()); entry != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
}

// Tests block header storage and retrieval operations.
func TestRootBlockHeaderStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test header to move around the database and make sure it's really new
	//todo init header and meta
	header := &types.RootBlockHeader{Number: uint32(42)}
	if entry := ReadRootBlockHeader(db, header.Hash()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	// Write and verify the header in the database
	WriteRootBlockHeader(db, header)
	if entry := ReadRootBlockHeader(db, header.Hash()); entry == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != header.Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, header)
	}
	// Delete the header and verify the execution
	DeleteRootBlockHeader(db, header.Hash())
	if entry := ReadRootBlockHeader(db, header.Hash()); entry != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
}

// Tests block storage and retrieval operations.
func TestRootBlockStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test block to move around the database and make sure it's really new
	block := types.NewRootBlockWithHeader(&types.RootBlockHeader{
		Extra:           limitedSizeByes,
		ParentHash:      types.EmptyHash,
		MinorHeaderHash: types.EmptyHash,
	}).WithBody(headers, limitedSizeByes)

	if entry := ReadRootBlock(db, block.Hash()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	if entry := ReadRootBlockHeader(db, block.Hash()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	// Write and verify the block in the database
	WriteRootBlock(db, block)
	if entry := ReadRootBlock(db, block.Hash()); entry == nil {
		t.Fatalf("Stored block not found")
	} else if entry.Hash() != block.Hash() {
		t.Fatalf("Retrieved block mismatch: have %v, want %v", entry, block)
	}
	if entry := ReadRootBlockHeader(db, block.Hash()); entry == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != block.Header().Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, block.Header())
	}
	// Delete the block and verify the execution
	DeleteRootBlock(db, block.Hash())
	if entry := ReadRootBlock(db, block.Hash()); entry != nil {
		t.Fatalf("Deleted block returned: %v", entry)
	}
	if entry := ReadRootBlockHeader(db, block.Hash()); entry != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
}

// Tests block storage and retrieval operations.
func TestMinorBlockStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test block to move around the database and make sure it's really new
	block := types.NewMinorBlockWithHeader(&types.MinorBlockHeader{
		Extra:      limitedSizeByes,
		ParentHash: types.EmptyHash,
	}, &types.MinorBlockMeta{}).WithBody(txs, limitedSizeByes)

	if entry := ReadMinorBlock(db, block.Hash()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	if entry := ReadMinorBlockHeader(db, block.Hash()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	// Write and verify the block in the database
	WriteMinorBlock(db, block)
	if entry := ReadMinorBlock(db, block.Hash()); entry == nil {
		t.Fatalf("Stored block not found")
	} else if entry.Hash() != block.Hash() {
		t.Fatalf("Retrieved block mismatch: have %v, want %v", entry, block)
	}
	if entry := ReadMinorBlockHeader(db, block.Hash()); entry == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != block.Header().Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, block.Header())
	}
	// Delete the block and verify the execution
	DeleteMinorBlock(db, block.Hash())
	if entry := ReadMinorBlock(db, block.Hash()); entry != nil {
		t.Fatalf("Deleted block returned: %v", entry)
	}
	if entry := ReadMinorBlockHeader(db, block.Hash()); entry != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
}

// Tests block total difficulty storage and retrieval operations.
func TestTdStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test TD to move around the database and make sure it's really new
	hash, td := common.Hash{}, big.NewInt(314)
	if entry := ReadTd(db, hash); entry != nil {
		t.Fatalf("Non existent TD returned: %v", entry)
	}
	// Write and verify the TD in the database
	WriteTd(db, hash, td)
	if entry := ReadTd(db, hash); entry == nil {
		t.Fatalf("Stored TD not found")
	} else if entry.Cmp(td) != 0 {
		t.Fatalf("Retrieved TD mismatch: have %v, want %v", entry, td)
	}
	// Delete the TD and verify the execution
	DeleteTd(db, hash)
	if entry := ReadTd(db, hash); entry != nil {
		t.Fatalf("Deleted TD returned: %v", entry)
	}
}

// Tests that canonical numbers can be mapped to hashes and retrieved.
func TestCanonicalMappingStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test canonical number and assinged hash to move around
	hash, number := common.Hash{0: 0xff}, uint64(314)
	if entry := ReadCanonicalHash(db, ChainType(0), number); entry != (common.Hash{}) {
		t.Fatalf("Non existent canonical mapping returned: %v", entry)
	}
	// Write and verify the TD in the database
	WriteCanonicalHash(db, 0, hash, number)
	if entry := ReadCanonicalHash(db, ChainType(0), number); entry == (common.Hash{}) {
		t.Fatalf("Stored canonical mapping not found")
	} else if entry != hash {
		t.Fatalf("Retrieved canonical mapping mismatch: have %v, want %v", entry, hash)
	}
	if entry := ReadCanonicalHash(db, ChainType(1), number); entry != (common.Hash{}) {
		t.Fatalf("Non existent canonical mapping returned: %v", entry)
	}
	// Delete the TD and verify the execution
	DeleteCanonicalHash(db, ChainType(0), number)
	if entry := ReadCanonicalHash(db, ChainType(0), number); entry != (common.Hash{}) {
		t.Fatalf("Deleted canonical mapping returned: %v", entry)
	}
}

// Tests that head headers and head blocks can be assigned, individually.
func TestHeadStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	blockHeadHash := common.BytesToHash([]byte{0x44})
	blockFullHash := common.BytesToHash([]byte{0x55})
	blockFastHash := common.BytesToHash([]byte{0x66})

	// Check that no head entries are in a pristine database
	if entry := ReadHeadHeaderHash(db); entry != (common.Hash{}) {
		t.Fatalf("Non head header entry returned: %v", entry)
	}
	if entry := ReadHeadBlockHash(db); entry != (common.Hash{}) {
		t.Fatalf("Non head block entry returned: %v", entry)
	}
	if entry := ReadHeadFastBlockHash(db); entry != (common.Hash{}) {
		t.Fatalf("Non fast head block entry returned: %v", entry)
	}
	// Assign separate entries for the head header and block
	WriteHeadHeaderHash(db, blockHeadHash)
	WriteHeadBlockHash(db, blockFullHash)
	WriteHeadFastBlockHash(db, blockFastHash)

	// Check that both heads are present, and different (i.e. two heads maintained)
	if entry := ReadHeadHeaderHash(db); entry != blockHeadHash {
		t.Fatalf("Head header hash mismatch: have %v, want %v", entry, blockHeadHash)
	}
	if entry := ReadHeadBlockHash(db); entry != blockFullHash {
		t.Fatalf("Head block hash mismatch: have %v, want %v", entry, blockFullHash)
	}
	if entry := ReadHeadFastBlockHash(db); entry != blockFastHash {
		t.Fatalf("Fast head block hash mismatch: have %v, want %v", entry, blockFastHash)
	}
}

// Tests that receipts associated with a single block can be stored and retrieved.
func TestBlockReceiptStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	receipt1 := &types.Receipt{
		Status:            types.ReceiptStatusFailed,
		CumulativeGasUsed: 1,
		Logs: []*types.Log{
			{Recipient: account.BytesToIdentityRecipient([]byte{0x11})},
			{Recipient: account.BytesToIdentityRecipient([]byte{0x01, 0x11})},
		},
		TxHash:          common.BytesToHash([]byte{0x11, 0x11}),
		ContractAddress: account.BytesToIdentityRecipient([]byte{0x01, 0x11, 0x11}),
		GasUsed:         111111,
	}
	receipt2 := &types.Receipt{
		PostState:         common.Hash{2}.Bytes(),
		CumulativeGasUsed: 2,
		Logs: []*types.Log{
			{Recipient: account.BytesToIdentityRecipient([]byte{0x22})},
			{Recipient: account.BytesToIdentityRecipient([]byte{0x02, 0x22})},
		},
		TxHash:          common.BytesToHash([]byte{0x22, 0x22}),
		ContractAddress: account.BytesToIdentityRecipient([]byte{0x02, 0x22, 0x22}),
		GasUsed:         222222,
	}
	receipts := []*types.Receipt{receipt1, receipt2}

	// Check that no receipt entries are in a pristine database
	hash := common.BytesToHash([]byte{0x03, 0x14})
	if rs := ReadReceipts(db, hash); len(rs) != 0 {
		t.Fatalf("non existent receipts returned: %v", rs)
	}
	// Insert the receipt slice into the database and check presence
	WriteReceipts(db, hash, receipts)
	if rs := ReadReceipts(db, hash); len(rs) == 0 {
		t.Fatalf("no receipts returned")
	} else {
		for i := 0; i < len(receipts); i++ {
			rlpHave, _ := rlp.EncodeToBytes(rs[i])
			rlpWant, _ := rlp.EncodeToBytes(receipts[i])

			if !bytes.Equal(rlpHave, rlpWant) {
				t.Fatalf("receipt #%d: receipt mismatch: have %v, want %v", i, rs[i], receipts[i])
			}
		}
	}
	// Delete the receipt slice and check purge
	DeleteReceipts(db, hash)
	if rs := ReadReceipts(db, hash); len(rs) != 0 {
		t.Fatalf("deleted receipts returned: %v", rs)
	}
}
