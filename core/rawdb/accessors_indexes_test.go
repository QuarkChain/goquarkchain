// Modified from go-ethereum under GNU Lesser General Public License

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
	tx1 := types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(1, account.BytesToIdentityRecipient([]byte{0x11}), big.NewInt(111), 1111, big.NewInt(11111), 0, 1, 1, 0, []byte{0x11, 0x11, 0x11}, 0, 0)}
	tx2 := types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(2, account.BytesToIdentityRecipient([]byte{0x22}), big.NewInt(222), 2222, big.NewInt(22222), 0, 1, 1, 0, []byte{0x22, 0x22, 0x22}, 0, 0)}
	tx3 := types.Transaction{TxType: types.EvmTx, EvmTx: types.NewEvmTransaction(3, account.BytesToIdentityRecipient([]byte{0x33}), big.NewInt(333), 3333, big.NewInt(33333), 0, 1, 1, 0, []byte{0x33, 0x33, 0x33}, 0, 0)}
	txs := []*types.Transaction{&tx1, &tx2, &tx3}

	block := types.NewMinorBlock(&types.MinorBlockHeader{Number: uint64(314)}, &types.MinorBlockMeta{}, txs, nil, nil)

	// Check that no transactions entries are in a pristine database
	for i, tx := range txs {
		if txn, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: non existent transaction returned: %v", i, tx.Hash(), txn)
		}
	}
	// Insert all the transactions into the database, and verify contents
	WriteMinorBlock(db, block)
	WriteBlockContentLookupEntries(db, block)

	for i, tx := range txs {
		if txn, hash, index := ReadTransaction(db, tx.Hash()); txn == nil {
			t.Fatalf("tx #%d [%x]: transaction not found", i, tx.Hash())
		} else {
			if hash != block.Hash() || index != uint32(i) {
				t.Fatalf("tx #%d [%x]: positional metadata mismatch: have %x/%d, want %x/%v", i, tx.Hash(), hash, index, block.Hash(), i)
			}
			if tx.Hash() != txn.Hash() {
				t.Fatalf("tx #%d [%x]: transaction mismatch: have %v, want %v", i, tx.Hash(), txn, tx)
			}
		}
	}
	// Delete the transactions and check purge
	for i, tx := range txs {
		DeleteBlockContentLookupEntry(db, tx.Hash())
		if txn, _, _ := ReadTransaction(db, tx.Hash()); txn != nil {
			t.Fatalf("tx #%d [%x]: deleted transaction returned: %v", i, tx.Hash(), txn)
		}
	}
}
