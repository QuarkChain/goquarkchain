// Copyright 2018 The go-ethereum Authors
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

package rawdb

import (
	"bytes"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	"testing"
)

var (
	limitedSizeByes = serialize.LimitedSizeByteSlice2{'\x01', '\x02', '\x03'}
	tx1             = types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(1, account.BytesToIdentityRecipient([]byte{0x11}), big.NewInt(111), 1111, big.NewInt(11111), 0, 1, 1, 0, []byte{0x11, 0x11, 0x11})}
	tx2             = types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(2, account.BytesToIdentityRecipient([]byte{0x22}), big.NewInt(222), 2222, big.NewInt(22222), 0, 1, 1, 0, []byte{0x22, 0x22, 0x22})}
	tx3             = types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(3, account.BytesToIdentityRecipient([]byte{0x33}), big.NewInt(333), 3333, big.NewInt(33333), 0, 1, 1, 0, []byte{0x33, 0x33, 0x33})}
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
	meta := &types.MinorBlockMeta{}
	if entry, m := ReadMinorBlockHeader(db, header.Hash(), header.Number); entry != nil || m != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	// Write and verify the header in the database
	WriteMinorBlockHeader(db, header, meta)
	if entry, m := ReadMinorBlockHeader(db, header.Hash(), header.Number); entry == nil || m == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != header.Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, header)
	}
	// Delete the header and verify the execution
	DeleteMinorBlockHeader(db, header.Hash(), header.Number)
	if entry, m := ReadMinorBlockHeader(db, header.Hash(), header.Number); entry != nil || m != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
}

// Tests block header storage and retrieval operations.
func TestRootBlockHeaderStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test header to move around the database and make sure it's really new
	//todo init header and meta
	header := &types.RootBlockHeader{Number: uint32(42)}
	if entry := ReadRootBlockHeader(db, header.Hash(), header.NumberU64()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	// Write and verify the header in the database
	WriteRootBlockHeader(db, header)
	if entry := ReadRootBlockHeader(db, header.Hash(), header.NumberU64()); entry == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != header.Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, header)
	}
	// Delete the header and verify the execution
	DeleteRootBlockHeader(db, header.Hash(), header.NumberU64())
	if entry := ReadRootBlockHeader(db, header.Hash(), header.NumberU64()); entry != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
}

// Tests block body storage and retrieval operations.
func TestMinorBlockBodyStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	hash := common.BytesToHash(common.FromHex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97"))

	if entry, _ := ReadMinorBlockBody(db, hash, 0); entry != nil {
		t.Fatalf("Non existent body returned: %v", entry)
	}
	// Write and verify the body in the database
	WriteMinorBlockBody(db, hash, 0, txs, limitedSizeByes)
	if trans, trackingData := ReadMinorBlockBody(db, hash, 0); trans == nil {
		t.Fatalf("Stored body not found")
	} else if types.DeriveSha(trans) != types.DeriveSha(txs) {
		t.Fatalf("Retrieved body mismatch: have %v, want %v", trans, txs)
	} else if common.Bytes2Hex(trackingData) != common.Bytes2Hex(limitedSizeByes) {
		t.Fatalf("Retrieved body mismatch: have %v, want %v", trackingData, limitedSizeByes)
	}
	// Delete the body and verify the execution
	DeleteBody(db, hash, 0)
	if entry, _ := ReadMinorBlockBody(db, hash, 0); entry != nil {
		t.Fatalf("Deleted body returned: %v", entry)
	}
}

func TestRootBlockBodyStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	hash := common.BytesToHash(common.FromHex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97"))

	if entry, _ := ReadRootBlockBody(db, hash, 0); entry != nil {
		t.Fatalf("Non existent body returned: %v", entry)
	}
	// Write and verify the body in the database
	WriteRootBlockBody(db, hash, 0, headers, common.FromHex("010203"))
	if hs, _ := ReadRootBlockBody(db, hash, 0); hs == nil {
		t.Fatalf("Stored body not found")
	} else if types.DeriveSha(hs) != types.DeriveSha(headers) {
		t.Fatalf("Retrieved body mismatch: have %v, want %v", hs, headers)
	}
	// Delete the body and verify the execution
	DeleteBody(db, hash, 0)
	if entry, _ := ReadRootBlockBody(db, hash, 0); entry != nil {
		t.Fatalf("Deleted body returned: %v", entry)
	}
}

// Tests block storage and retrieval operations.
func TestRootBlockStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test block to move around the database and make sure it's really new
	block := types.NewRootBlockWithHeader(&types.RootBlockHeader{
		Extra:           &limitedSizeByes,
		ParentHash:      types.EmptyHash,
		MinorHeaderHash: types.EmptyHash,
	}).WithBody(headers, limitedSizeByes)

	if entry := ReadRootBlock(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	if entry := ReadRootBlockHeader(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	if entry, _ := ReadRootBlockBody(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Non existent body returned: %v", entry)
	}
	// Write and verify the block in the database
	WriteRootBlock(db, block)
	if entry := ReadRootBlock(db, block.Hash(), block.NumberU64()); entry == nil {
		t.Fatalf("Stored block not found")
	} else if entry.Hash() != block.Hash() {
		t.Fatalf("Retrieved block mismatch: have %v, want %v", entry, block)
	}
	if entry := ReadRootBlockHeader(db, block.Hash(), block.NumberU64()); entry == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != block.Header().Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, block.Header())
	}
	if entry, _ := ReadRootBlockBody(db, block.Hash(), block.NumberU64()); entry == nil {
		t.Fatalf("Stored body not found")
	} else if types.DeriveSha(headers) != types.DeriveSha(entry) {
		t.Fatalf("Retrieved body mismatch: have %v, want %v", entry, headers)
	}
	// Delete the block and verify the execution
	DeleteRootBlock(db, block.Hash(), block.NumberU64())
	if entry := ReadRootBlock(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Deleted block returned: %v", entry)
	}
	if entry := ReadRootBlockHeader(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
	if entry, _ := ReadRootBlockBody(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Deleted body returned: %v", entry)
	}
}

// Tests block storage and retrieval operations.
func TestMinorBlockStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test block to move around the database and make sure it's really new
	block := types.NewMinorBlockWithHeader(&types.MinorBlockHeader{
		Extra:      &limitedSizeByes,
		ParentHash: types.EmptyHash,
	}, &types.MinorBlockMeta{}).WithBody(txs, limitedSizeByes)

	if entry := ReadMinorBlock(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	if entry, _ := ReadMinorBlockHeader(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Non existent header returned: %v", entry)
	}
	if entry, _ := ReadMinorBlockBody(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Non existent body returned: %v", entry)
	}
	// Write and verify the block in the database
	WriteMinorBlock(db, block)
	if entry := ReadMinorBlock(db, block.Hash(), block.Number()); entry == nil {
		t.Fatalf("Stored block not found")
	} else if entry.Hash() != block.Hash() {
		t.Fatalf("Retrieved block mismatch: have %v, want %v", entry, block)
	}
	if entry, _ := ReadMinorBlockHeader(db, block.Hash(), block.Number()); entry == nil {
		t.Fatalf("Stored header not found")
	} else if entry.Hash() != block.Header().Hash() {
		t.Fatalf("Retrieved header mismatch: have %v, want %v", entry, block.Header())
	}
	if entry, _ := ReadMinorBlockBody(db, block.Hash(), block.Number()); entry == nil {
		t.Fatalf("Stored body not found")
	} else if types.DeriveSha(txs) != types.DeriveSha(entry) {
		t.Fatalf("Retrieved body mismatch: have %v, want %v", entry, txs)
	}
	// Delete the block and verify the execution
	DeleteMinorBlock(db, block.Hash(), block.Number())
	if entry := ReadMinorBlock(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Deleted block returned: %v", entry)
	}
	if entry, _ := ReadMinorBlockHeader(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Deleted header returned: %v", entry)
	}
	if entry, _ := ReadMinorBlockBody(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Deleted body returned: %v", entry)
	}
}

// Tests that partial block contents don't get reassembled into full blocks.
func TestPartialRootBlockStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()
	block := types.NewRootBlockWithHeader(&types.RootBlockHeader{
		Extra:           &limitedSizeByes,
		MinorHeaderHash: types.EmptyHash,
		ParentHash:      types.EmptyHash,
	})
	// Store a header and check that it's not recognized as a block
	WriteRootBlockHeader(db, block.Header())
	if entry := ReadRootBlock(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	DeleteRootBlockHeader(db, block.Hash(), block.NumberU64())

	// Store a body and check that it's not recognized as a block
	WriteRootBlockBody(db, block.Hash(), block.NumberU64(), block.MinorBlockHeaders(), block.TrackingData())
	if entry := ReadRootBlock(db, block.Hash(), block.NumberU64()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	DeleteBody(db, block.Hash(), block.NumberU64())

	// Store a header and a body separately and check reassembly
	WriteRootBlockHeader(db, block.Header())
	WriteRootBlockBody(db, block.Hash(), block.NumberU64(), block.MinorBlockHeaders(), block.TrackingData())

	if entry := ReadRootBlock(db, block.Hash(), block.NumberU64()); entry == nil {
		t.Fatalf("Stored block not found")
	} else if entry.Hash() != block.Hash() {
		t.Fatalf("Retrieved block mismatch: have %v, want %v", entry, block)
	}
}

// Tests that partial block contents don't get reassembled into full blocks.
func TestPartialMinorBlockStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()
	block := types.NewMinorBlockWithHeader(&types.MinorBlockHeader{
		Extra:      &limitedSizeByes,
		MetaHash:   types.EmptyHash,
		ParentHash: types.EmptyHash,
	}, &types.MinorBlockMeta{Root: types.EmptyHash})

	// Store a header and check that it's not recognized as a block
	WriteMinorBlockHeader(db, block.Header(), block.Meta())
	if entry := ReadMinorBlock(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	DeleteMinorBlockHeader(db, block.Hash(), block.Number())

	// Store a body and check that it's not recognized as a block
	WriteMinorBlockBody(db, block.Hash(), block.Number(), block.Transactions(), block.TrackingData())
	if entry := ReadMinorBlock(db, block.Hash(), block.Number()); entry != nil {
		t.Fatalf("Non existent block returned: %v", entry)
	}
	DeleteBody(db, block.Hash(), block.Number())

	// Store a header and a body separately and check reassembly
	WriteMinorBlockHeader(db, block.Header(), block.Meta())
	WriteMinorBlockBody(db, block.Hash(), block.Number(), block.Transactions(), block.TrackingData())

	if entry := ReadMinorBlock(db, block.Hash(), block.Number()); entry == nil {
		t.Fatalf("Stored block not found")
	} else if entry.Hash() != block.Hash() {
		t.Fatalf("Retrieved block mismatch: have %v, want %v", entry, block)
	}
}

// Tests block total difficulty storage and retrieval operations.
func TestTdStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test TD to move around the database and make sure it's really new
	hash, td := common.Hash{}, big.NewInt(314)
	if entry := ReadTd(db, hash, 0); entry != nil {
		t.Fatalf("Non existent TD returned: %v", entry)
	}
	// Write and verify the TD in the database
	WriteTd(db, hash, 0, td)
	if entry := ReadTd(db, hash, 0); entry == nil {
		t.Fatalf("Stored TD not found")
	} else if entry.Cmp(td) != 0 {
		t.Fatalf("Retrieved TD mismatch: have %v, want %v", entry, td)
	}
	// Delete the TD and verify the execution
	DeleteTd(db, hash, 0)
	if entry := ReadTd(db, hash, 0); entry != nil {
		t.Fatalf("Deleted TD returned: %v", entry)
	}
}

// Tests that canonical numbers can be mapped to hashes and retrieved.
func TestCanonicalMappingStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	// Create a test canonical number and assinged hash to move around
	hash, number := common.Hash{0: 0xff}, uint64(314)
	if entry := ReadCanonicalHash(db, number); entry != (common.Hash{}) {
		t.Fatalf("Non existent canonical mapping returned: %v", entry)
	}
	// Write and verify the TD in the database
	WriteCanonicalHash(db, hash, number)
	if entry := ReadCanonicalHash(db, number); entry == (common.Hash{}) {
		t.Fatalf("Stored canonical mapping not found")
	} else if entry != hash {
		t.Fatalf("Retrieved canonical mapping mismatch: have %v, want %v", entry, hash)
	}
	// Delete the TD and verify the execution
	DeleteCanonicalHash(db, number)
	if entry := ReadCanonicalHash(db, number); entry != (common.Hash{}) {
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
	if rs := ReadReceipts(db, hash, 0); len(rs) != 0 {
		t.Fatalf("non existent receipts returned: %v", rs)
	}
	// Insert the receipt slice into the database and check presence
	WriteReceipts(db, hash, 0, receipts)
	if rs := ReadReceipts(db, hash, 0); len(rs) == 0 {
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
	DeleteReceipts(db, hash, 0)
	if rs := ReadReceipts(db, hash, 0); len(rs) != 0 {
		t.Fatalf("deleted receipts returned: %v", rs)
	}
}
