package types

import (
	"bytes"
	"crypto/ecdsa"
	"github.com/QuarkChain/goquarkchain/account"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

// The values in those tests are from the EvmTransaction Tests
var (
	reciept    = account.BytesToIdentityRecipient(common.Hex2Bytes("b94f5374fce5edbc8e2a8697c15331677e6ebf0b"))
	emptyEvmTx = NewEvmTransaction(
		0,
		reciept,
		big.NewInt(0), 0, big.NewInt(0),
		0, 0, 1, 0, nil,
	)
	emptyTx = Transaction{TxType: 1, EvmTx: emptyEvmTx}
	//nonce , to , amount , gasLimit , gasPrice, fromFullShardId , toFullShardId , networkId , version , data
	rightvrsTx = NewEvmTransaction(
		3,
		reciept,
		big.NewInt(10),
		2000,
		big.NewInt(1),
		0,
		0,
		1,
		0,
		common.FromHex("35353434"),
	)
	signTx, _ = rightvrsTx.WithSignature(
		NewEIP155Signer(1),
		common.Hex2Bytes("98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"),
	)
)

func TestTransactionSigHash(t *testing.T) {
	var signer = NewEIP155Signer(1)
	//hash unsigned
	if signer.Hash(emptyEvmTx) != common.HexToHash("297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495") {
		t.Errorf("empty transaction unsigned hash mismatch, got %x, expect %x", signer.Hash(emptyEvmTx), common.HexToHash("297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495"))
	}
	if emptyEvmTx.Hash() != common.HexToHash("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97") {
		t.Errorf("empty transaction hash mismatch, got %x, expect %x", emptyTx.Hash(), common.HexToHash("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97"))
	}

	//hash unsigned
	if signer.Hash(rightvrsTx) != common.HexToHash("e4f3c1dd000045bf26006df7eb7cb0a882f70a6ab81723d93638151f6418f78a") {
		t.Errorf("RightVRS transaction unsigned hash mismatch, got %x, expect %x", signer.Hash(rightvrsTx), common.HexToHash("e4f3c1dd000045bf26006df7eb7cb0a882f70a6ab81723d93638151f6418f78a"))
	}
	if rightvrsTx.Hash() != common.HexToHash("df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28") {
		t.Errorf("RightVRS transaction hash mismatch, got %x, expect %x", rightvrsTx.Hash(), common.HexToHash("df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28"))
	}
}

func TestTransactionEncode(t *testing.T) {
	txb, err := rlp.EncodeToBytes(rightvrsTx)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	should := common.FromHex("e703018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a843535343480800180808080")
	if !bytes.Equal(txb, should) {
		t.Errorf("encoded RLP mismatch, got %x", txb)
	}
}

func decodeTx(data []byte) (*EvmTransaction, error) {
	var tx EvmTransaction
	t, err := &tx, rlp.Decode(bytes.NewReader(data), &tx)

	return t, err
}

func publicKey2Recipient(pk *ecdsa.PublicKey) account.Recipient {
	pubBytes := crypto.FromECDSAPub(pk)
	recipient := account.BytesToIdentityRecipient(crypto.Keccak256(pubBytes[1:])[12:])
	return recipient
}

func defaultTestKey() (*ecdsa.PrivateKey, account.Recipient) {
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	recipient := publicKey2Recipient(&key.PublicKey)
	return key, recipient
}

func TestRecipientEmpty(t *testing.T) {
	_, addr := defaultTestKey()
	tx, err := decodeTx(common.Hex2Bytes("f86180808094b94f5374fce5edbc8e2a8697c15331677e6ebf0b8080808001801ca0fab2cc481eb33edacc4016fe35f30051014109cf0d227679003dd36534247845a019c29e2b33a1a8adf95dabf603cc62758775ce73c3d78098160818dcd7c0970d"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	from, err := Sender(NewEIP155Signer(tx.NetworkId()), tx)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	if addr != from {
		t.Errorf("derived address doesn't match addr %x, from %x", addr, from)
	}
}

func TestRecipientNormal(t *testing.T) {
	_, addr := defaultTestKey()

	tx, err := decodeTx(common.Hex2Bytes("f86703018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a8435353434808001801ca03ba243b74816362081890b8b680d303a5cce7803a12d8ce863723bcd1be94efba0399e91eb5b20c258a77f7045da3f2b84bc1c1f40e0c23bcc3df7bce05bea2ed8"))
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	from, err := Sender(NewEIP155Signer(1), tx)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}

	if addr != from {
		t.Error("derived address doesn't match")
	}
}

// Tests that transactions can be correctly sorted according to their price in
// decreasing order, but at the same time with increasing nonces when issued by
// the same account.
func TestTransactionPriceNonceSort(t *testing.T) {
	// Generate a batch of accounts to start with
	keys := make([]*ecdsa.PrivateKey, 25)
	for i := 0; i < len(keys); i++ {
		keys[i], _ = crypto.GenerateKey()
	}

	signer := NewEIP155Signer(1)
	// Generate a batch of transactions with overlapping values, but shifted nonces
	groups := map[account.Recipient]Transactions{}
	for start, key := range keys {
		recipient := publicKey2Recipient(&key.PublicKey)
		for i := 0; i < 25; i++ {
			tx, _ := SignTx(NewEvmTransaction(uint64(start+i), account.Recipient{}, big.NewInt(100), 100, big.NewInt(int64(start+i)), 0, 0, 1, 0, nil), signer, key)
			groups[recipient] = append(groups[recipient], &Transaction{TxType: EvmTx, EvmTx: tx})
		}
	}
	// Sort the transactions and cross check the nonce ordering
	txset := NewTransactionsByPriceAndNonce(signer, groups)

	txs := Transactions{}
	for tx := txset.Peek(); tx != nil; tx = txset.Peek() {
		txs = append(txs, tx)
		txset.Shift()
	}
	if len(txs) != 25*25 {
		t.Errorf("expected %d transactions, found %d", 25*25, len(txs))
	}
	for i, txi := range txs {
		fromi, _ := Sender(signer, txi.EvmTx)

		// Make sure the nonce order is valid
		for j, txj := range txs[i+1:] {
			fromj, _ := Sender(signer, txj.EvmTx)

			if fromi == fromj && txi.EvmTx.Nonce() > txj.EvmTx.Nonce() {
				t.Errorf("invalid nonce ordering: tx #%d (A=%x N=%v) < tx #%d (A=%x N=%v)", i, fromi[:4], txi.EvmTx.Nonce(), i+j, fromj[:4], txj.EvmTx.Nonce())
			}
		}

		// If the next tx has different from account, the price must be lower than the current one
		if i+1 < len(txs) {
			next := txs[i+1]
			fromNext, _ := Sender(signer, next.EvmTx)
			if fromi != fromNext && txi.EvmTx.GasPrice().Cmp(next.EvmTx.GasPrice()) < 0 {
				t.Errorf("invalid gasprice ordering: tx #%d (A=%x P=%v) < tx #%d (A=%x P=%v)", i, fromi[:4], txi.EvmTx.GasPrice(), i+1, fromNext[:4], next.EvmTx.GasPrice())
			}
		}
	}
}
