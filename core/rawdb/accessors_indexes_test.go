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
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"math/big"
	"testing"
)

// Tests that positional lookup metadata can be stored and retrieved.
func TestLookupStorage(t *testing.T) {
	db := ethdb.NewMemDatabase()

	//nonce uint64, to account.Recipient, amount *big.Int, gasLimit uint64, gasPrice *big.Int, fromFullShardId uint32, toFullShardId uint32, networkId uint32, version uint32, data []byte) *EvmTransaction {
	tx1 := types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(1, account.BytesToIdentityRecipient([]byte{0x11}), big.NewInt(111), 1111, big.NewInt(11111), 0, 1, 1, 0, []byte{0x11, 0x11, 0x11})}
	tx2 := types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(2, account.BytesToIdentityRecipient([]byte{0x22}), big.NewInt(222), 2222, big.NewInt(22222), 0, 1, 1, 0, []byte{0x22, 0x22, 0x22})}
	tx3 := types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(3, account.BytesToIdentityRecipient([]byte{0x33}), big.NewInt(333), 3333, big.NewInt(33333), 0, 1, 1, 0, []byte{0x33, 0x33, 0x33})}
	txs := []*types.Transaction{&tx1, &tx2, &tx3}

	block := types.NewMinorBlock(&types.MinorBlockHeader{Number: uint64(314)}, &types.MinorBlockMeta{}, txs, nil, nil)

	// Check that no transactions entries are in a pristine database
	for i, tx := range txs {
		if txn, _, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: non existent transaction returned: %v", i, tx.Hash(), txn)
		}
	}
	// Insert all the transactions into the database, and verify contents
	WriteMinorBlock(db, block)
	WriteTxLookupEntries(db, block)

	for i, tx := range txs {
		if txn, hash, number, index := ReadTransaction(db, tx.Hash()); txn == nil {
			t.Fatalf("tx #%d [%x]: transaction not found", i, tx.Hash())
		} else {
			if hash != block.Hash() || number != block.Number() || index != uint64(i) {
				t.Fatalf("tx #%d [%x]: positional metadata mismatch: have %x/%d/%d, want %x/%v/%v", i, tx.Hash(), hash, number, index, block.Hash(), block.Number(), i)
			}
			if tx.Hash() != txn.Hash() {
				t.Fatalf("tx #%d [%x]: transaction mismatch: have %v, want %v", i, tx.Hash(), txn, tx)
			}
		}
	}
	// Delete the transactions and check purge
	for i, tx := range txs {
		DeleteTxLookupEntry(db, tx.Hash())
		if txn, _, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: deleted transaction returned: %v", i, tx.Hash(), txn)
		}
	}
}
