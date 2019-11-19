package types

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"math"
	"math/big"
	"reflect"
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
		0, 0, 1, 0, nil, 0, 0,
	)
	emptyTx = Transaction{TxType: 0, EvmTx: emptyEvmTx}
	//nonce , to , amount , gasLimit , gasPrice, fromFullShardKey , toFullShardKey , networkId , version , data
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
		nil, 0, 0,
	)
	signTx, _ = rightvrsTx.WithSignature(
		NewEIP155Signer(1),
		common.Hex2Bytes("98ff921201554726367d2be8c804a7ff89ccf285ebc57dff8ae4c44b9c19ac4a8887321be575c8095f789dd4c743dfe42c1820f9231f98a962b210e3ac2452a301"),
	)
)

func TestTransactionSigHash(t *testing.T) {
	var signer = NewEIP155Signer(1)
	//hash unsigned
	if signer.Hash(emptyEvmTx) != common.HexToHash("15e523e4a18884f01753358af140664007e19b2c67cfa6618cadb85de14f3bd0") {
		t.Errorf("empty transaction unsigned hash mismatch, got %x, expect %x", signer.Hash(emptyEvmTx), common.HexToHash("297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495"))
	}
	if emptyEvmTx.Hash() != common.HexToHash("a04873d41928c8acc76d4d6495fec31fb58afc7d5a5782d9ba4bb30fdbf1b147") {
		t.Errorf("empty transaction hash mismatch, got %x, expect %x", emptyTx.Hash(), common.HexToHash("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97"))
	}

	//hash unsigned
	if signer.Hash(rightvrsTx) != common.HexToHash("a8915d9a38bacbdc640ab287d4beb9b06ea1af52da8568c298739c9d7514e87b") {
		t.Errorf("RightVRS transaction unsigned hash mismatch, got %x, expect %x", signer.Hash(rightvrsTx), common.HexToHash("e4f3c1dd000045bf26006df7eb7cb0a882f70a6ab81723d93638151f6418f78a"))
	}
	if rightvrsTx.Hash() != common.HexToHash("4bf87b2a5b39b7894b4b4b197ffe1ef7e67085bbc60d599ed3d4d587aa72af76") {
		t.Errorf("RightVRS transaction hash mismatch, got %x, expect %x", rightvrsTx.Hash(), common.HexToHash("df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28"))
	}
}

func TestTransactionEncode(t *testing.T) {
	txb, err := rlp.EncodeToBytes(rightvrsTx)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	should := common.FromHex("ed03018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a800184000000008400000000808080808080")
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
	tx, err := decodeTx(common.Hex2Bytes("f86b80808094b94f5374fce5edbc8e2a8697c15331677e6ebf0b808001840000000084000000008080801ba0d7265f92d763da5e2ea5016b837bf56f5bf42d22aead9ad5e7be2ddf01efcc68a07159634972d77349a76108c6db0634ea7b65768881b152c656deca190df6e427"))
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

	tx, err := decodeTx(common.Hex2Bytes("f86b80808094b94f5374fce5edbc8e2a8697c15331677e6ebf0b808001840000000084000000008080801ba0d7265f92d763da5e2ea5016b837bf56f5bf42d22aead9ad5e7be2ddf01efcc68a07159634972d77349a76108c6db0634ea7b65768881b152c656deca190df6e427"))
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
			tx, _ := SignTx(NewEvmTransaction(uint64(start+i), account.Recipient{}, big.NewInt(100), 100, big.NewInt(int64(start+i)), 0, 0, 1, 0, nil, 0, 0), signer, key)
			groups[recipient] = append(groups[recipient], &Transaction{TxType: EvmTx, EvmTx: tx})
		}
	}
	// Sort the transactions and cross check the nonce ordering
	txset, err := NewTransactionsByPriceAndNonce(signer, groups)
	if err != nil {
		t.Errorf("NewTransactionsByPriceAndNonce err %v", err)
	}

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
func TestTxSize(t *testing.T) {

	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatal("CreatIdentityFromKey error: ", err)
	}
	defaultFullShardKey, err := id1.GetDefaultFullShardKey()
	if err != nil {
		t.Fatal("GetDefaultFullShardKey error: ", err)
	}
	acc1 := account.CreatAddressFromIdentity(id1, defaultFullShardKey)
	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	evmTx := NewEvmTransaction(
		0,
		acc1.Recipient,
		big.NewInt(0),
		30000,
		big.NewInt(0),
		0xFFFF,
		0xFFFF,
		1,
		0,
		nil,
		12345,
		1234,
	)
	signer := NewEIP155Signer(1)
	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		t.Fatal("prvKey error: ", err)
	}
	evmTx, err = SignTx(evmTx, signer, prvKey)
	if err != nil {
		t.Fatal("SignTx error: ", err)
	}
	tx := &Transaction{
		EvmTx:  evmTx,
		TxType: EvmTx,
	}
	txBytes, err := serialize.SerializeToBytes(&tx)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}
	evmTx2 := NewEvmTransaction(
		math.MaxUint64-1,
		acc1.Recipient,
		big.NewInt(math.MaxInt64-1),
		math.MaxUint64-1,
		big.NewInt(math.MaxInt64-1),
		uint32(math.Pow(256, 4)-1),
		uint32(math.Pow(256, 4)-1),
		1,
		0,
		[]byte{},
		4873763662273663091,
		4873763662273663091,
	)

	evmTx2, err = SignTx(evmTx2, signer, prvKey)
	if err != nil {
		t.Fatal("SignTx error: ", err)
	}
	tx2 := &Transaction{
		EvmTx:  evmTx2,
		TxType: EvmTx,
	}
	txBytes2, err := serialize.SerializeToBytes(&tx2)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	check("EvmTransaction min len", len(txBytes), 120)
	check("EvmTransaction max len", len(txBytes2), 162)
}
