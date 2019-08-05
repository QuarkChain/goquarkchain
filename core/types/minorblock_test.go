package types

import (
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/QuarkChain/goquarkchain/serialize"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

var (
	//	reciept, _ = account.BytesToIdentityRecipient(common.Hex2Bytes("b94f5374fce5edbc8e2a8697c15331677e6ebf0b"))
	evmTx1 = NewEvmTransaction(
		0,
		reciept,
		big.NewInt(0), 0, big.NewInt(0),
		0, 0, 1, 0, nil,
	)
	tx1 = Transaction{TxType: 1, EvmTx: evmTx1}
	//nonce , to , amount , gasLimit , gasPrice, fromFullShardKey , toFullShardKey , networkId , version , data
	evmTx2 = NewEvmTransaction(
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
	tx2 = Transaction{TxType: 1, EvmTx: evmTx2}
)

// from bcValidBlockTest.json, "SimpleTx"
func TestMinorBlockHeaderSerializing(t *testing.T) {
	blocHeaderEnc := common.FromHex("00000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621d32b3e0d000000000000000000000000000000000000000000000000000000000000000300000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000030000000000000005010600000000000000070000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100030102030000000000000000000000000000000000000000000000000000000000000004")
	var blockHeader MinorBlockHeader
	bb := serialize.NewByteBuffer(blocHeaderEnc)
	if err := serialize.Deserialize(bb, &blockHeader); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err := serialize.SerializeToBytes(&blockHeader)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("Version", blockHeader.Version, uint32(1))
	check("Height", blockHeader.Number, uint64(2))
	check("coinbase_Recipient", blockHeader.Coinbase.Recipient[:], common.FromHex("d3f86deb4a2bbf85048b3e790460c40dbab1f621"))
	check("coinbase_FullShardKey", uint32(blockHeader.Coinbase.FullShardKey), uint32(0xd32b3e0d))
	check("CoinbaseAmount", blockHeader.CoinbaseAmount.Value, big.NewInt(3))
	check("ParentHash", blockHeader.ParentHash, common.HexToHash("0000000000000000000000000000000000000000000000000000000000000001"))
	check("PrevRootBlockHash", blockHeader.PrevRootBlockHash, common.HexToHash("0000000000000000000000000000000000000000000000000000000000000002"))
	check("GasLimit", blockHeader.GasLimit.Value.Uint64(), uint64(4))
	check("MetaHash", blockHeader.MetaHash, common.HexToHash("0000000000000000000000000000000000000000000000000000000000000003"))
	check("Time", blockHeader.Time, uint64(5))
	check("Difficulty", blockHeader.Difficulty, big.NewInt(6))
	check("Nonce", blockHeader.Nonce, uint64(7))
	check("Bloom", common.Bytes2Hex(blockHeader.Bloom[:]), "00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001")
	check("Extra", common.Bytes2Hex(blockHeader.Extra), "010203")
	check("MixDigest", common.Bytes2Hex(blockHeader.MixDigest.Bytes()), "0000000000000000000000000000000000000000000000000000000000000004")
	if crypto.CryptoType == "gm" {
		check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "b274edca72087a960cff0dad483392ea36eb87d17dc8da40fe2ceb6616e559e8")
	} else {
		check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "b274edca72087a960cff0dad483392ea36eb87d17dc8da40fe2ceb6616e559e8")
	}

	check("serialize", bytes, blocHeaderEnc)

	blocMetaEnc := common.FromHex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a280000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000012c")
	var blockMeta MinorBlockMeta
	bb = serialize.NewByteBuffer(blocMetaEnc)
	if err := serialize.Deserialize(bb, &blockMeta); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err = serialize.SerializeToBytes(blockMeta)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	check("TxHash", blockMeta.TxHash[:], common.FromHex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97"))
	check("Root", blockMeta.Root[:], common.FromHex("297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495"))
	check("ReceiptHash", blockMeta.ReceiptHash[:], common.FromHex("df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28"))
	check("GasUsed", *blockMeta.GasUsed, serialize.Uint256{Value: big.NewInt(100)})
	check("CrossShardGasUsed", *blockMeta.CrossShardGasUsed, serialize.Uint256{Value: big.NewInt(300)})
	check("bmserialize", bytes, blocMetaEnc)

	signer := NewEIP155Signer(1)
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	transactionsEnc := common.FromHex("000000020100000063f86180808094b94f5374fce5edbc8e2a8697c15331677e6ebf0b8080018080801ca043e4eece5c63faa6eb5c5efd1a52c2c3d00991439d0e6dbd86d4a79ce4d99945a006367e73feb8149166ebf5db5d38723fa059abe8967335eee981a31d980e0c260100000069f86703018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a8435353434018080801ca03bffe222c359fcc09a9860e1e6c1f0652bb1bf65c78c05eaa5b895fd29c90698a01976cd9b54c1476388d9a7132da1fe31eeb28fdd840fc3bb349970774f8b0aa8")
	var trans Transactions
	bb = serialize.NewByteBuffer(transactionsEnc)
	if err := serialize.DeserializeWithTags(bb, &trans, serialize.Tags{ByteSizeOfSliceLen: 4}); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes = nil
	err = serialize.SerializeWithTags(&bytes, trans, serialize.Tags{ByteSizeOfSliceLen: 4})
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	tx1.EvmTx, _ = SignTx(evmTx1, signer, key)
	tx2.EvmTx, _ = SignTx(evmTx2, signer, key)
	check("len(Transactions)", len(trans), 2)
	check("Transactions[0].Hash", common.Bytes2Hex(trans[0].Hash().Bytes()), common.Bytes2Hex(tx1.Hash().Bytes()))
	check("Transactions[1].Hash", common.Bytes2Hex(trans[1].Hash().Bytes()), common.Bytes2Hex(tx2.Hash().Bytes()))
	check("txserialize", common.Bytes2Hex(bytes), common.Bytes2Hex(transactionsEnc))

	blockEnc := append(blocHeaderEnc, append(blocMetaEnc, append(transactionsEnc, common.Hex2Bytes("00020102")...)...)...)
	var block MinorBlock
	bb = serialize.NewByteBuffer(blockEnc)
	if err := serialize.Deserialize(bb, &block); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err = serialize.SerializeToBytes(&block)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	check("header", block.header, &blockHeader)
	check("meta", block.meta, &blockMeta)
	check("transactions", block.transactions.Len(), trans.Len())
	check("transactions[0]", block.transactions[0].Hash(), trans[0].Hash())
	check("transactions[1]", block.transactions[1].Hash(), trans[1].Hash())
	check("trackingdata", common.Bytes2Hex(block.trackingdata), "0102")
	check("blockhash", common.Bytes2Hex(block.Hash().Bytes()), "b274edca72087a960cff0dad483392ea36eb87d17dc8da40fe2ceb6616e559e8")
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(blockEnc))

}

func TestCalculateMerkleRoot(t *testing.T) {
	encList := [][]byte{
		common.FromHex("00000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff0000000000000000000000000000000000000000000000000000000000000003a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba4950000000000000000000000000000000000000000000000000000000000000004df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28000000000000000501060000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010003010203df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28"),
		common.FromHex("00000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff0000000000000000000000000000000000000000000000000000000000000003b40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba4950000000000000000000000000000000000000000000000000000000000000004df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28000000000000000501060000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010003010203df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28"),
	}
	list := make([]*MinorBlockHeader, 0)
	for _, bytes := range encList {
		var blockHeader MinorBlockHeader
		bb := serialize.NewByteBuffer(bytes)
		if err := serialize.Deserialize(bb, &blockHeader); err != nil {
			t.Fatal("Deserialize error: ", err)
		}
		list = append(list, &blockHeader)
	}
	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	check("header", list[0].Hash().Hex(), "0x48683020cb768970b700f0fa43f7c6622d0f6b3f45d3c10bf209c7c1272ca9c7")
	check("header", list[1].Hash().Hex(), "0x3f1c7abb6f6ae734a0c4ecd7b1610a543c67d67349254c3784794785d7d3e02e")
	check("header", CalculateMerkleRoot(list).Hex(), "0xbf3becc8ee28d3602634c6f5ca1989cbc34e1809b4fc6b5e258c0f3557e84119")
}
