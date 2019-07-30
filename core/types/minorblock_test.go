package types

import (
	"encoding/hex"
	"github.com/QuarkChain/goquarkchain/serialize"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	//	reciept, _ = account.BytesToIdentityRecipient(common.Hex2Bytes("b94f5374fce5edbc8e2a8697c15331677e6ebf0b"))
	evmTx1 = NewEvmTransaction(
		0,
		reciept,
		big.NewInt(0), 0, big.NewInt(0),
		0, 0, 1, 0, nil,
	)
	tx1 = Transaction{TxType: 0, EvmTx: evmTx1}
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
		nil,
	)
	tx2 = Transaction{TxType: 0, EvmTx: evmTx2}
)

// from bcValidBlockTest.json, "SimpleTx"
func TestMinorBlockHeaderSerializing(t *testing.T) {
	blocHeaderEnc := common.FromHex("00000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000002010101010102010200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000030000000000000005010600000000000000070000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100030102030000000000000000000000000000000000000000000000000000000000000004")
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
	check("Branch", blockHeader.Branch.Value, uint32(1))
	check("coinbase_Recipient", blockHeader.Coinbase.Recipient[:], common.FromHex("d3f86deb4a2bbf85048b3e790460c40dbab1f621"))
	check("coinbase_FullShardKey", uint32(blockHeader.Coinbase.FullShardKey), uint32(0x000003ff))
	check("CoinbaseAmount", blockHeader.CoinbaseAmount.BalanceMap[1], common.Big1)
	check("CoinbaseAmount", blockHeader.CoinbaseAmount.BalanceMap[2], common.Big2)
	check("CoinbaseAmount", len(blockHeader.CoinbaseAmount.BalanceMap), 2)
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
	check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "b0b7dfab9a8f485ea97a4642cdd380182ede101a64ecb3e73eb211496153d869")
	check("serialize", hex.EncodeToString(bytes), hex.EncodeToString(blocHeaderEnc))

	blocMetaEnc := common.FromHex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a280000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000012c0000000000000001000000000000000200000000000000030000000000000000000000000000000000000000000000000000000000000190")
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
	check("xshard_tx_cursor_info", blockMeta.XShardTxCursorInfo.RootBlockHeight, uint64(1))
	check("xshard_tx_cursor_info", blockMeta.XShardTxCursorInfo.MinorBlockIndex, uint64(2))
	check("xshard_tx_cursor_info", blockMeta.XShardTxCursorInfo.XShardDepositIndex, uint64(3))
	check("evm_xshard_gas_limit", blockMeta.XShardGasLimit.Value.Uint64(), uint64(400))
	check("bmserialize", bytes, blocMetaEnc)

	signer := NewEIP155Signer(1)
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	transactionsEnc := common.FromHex("00000002000000006df86b80808094b94f5374fce5edbc8e2a8697c15331677e6ebf0b808001840000000084000000008080801ba0d7265f92d763da5e2ea5016b837bf56f5bf42d22aead9ad5e7be2ddf01efcc68a07159634972d77349a76108c6db0634ea7b65768881b152c656deca190df6e427000000006ff86d03018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a8001840000000084000000008080801ba01e681d99a80f28640faa7e224823dd133ffbd59731e3c7009f4375134a4bd58ea0089addb6d4ca918d12471682a9e5f9d03f0738358a72e493a075519cb07cf34f")
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
	check("Transactions[1]", common.Bytes2Hex(trans[1].Hash().Bytes()), common.Bytes2Hex(tx2.Hash().Bytes()))
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
	check("blockhash", common.Bytes2Hex(block.Hash().Bytes()), "b0b7dfab9a8f485ea97a4642cdd380182ede101a64ecb3e73eb211496153d869")
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(blockEnc))

}

func TestCalculateMerkleRoot(t *testing.T) {
	encList := [][]byte{
		common.FromHex("00000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000002010101010102010200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000030000000000000005010600000000000000070000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100030102030000000000000000000000000000000000000000000000000000000000000004"),
		common.FromHex("0000000100000001000000000000006fd3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000002010101010102010200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000030000000000000005010600000000000000070000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100030102030000000000000000000000000000000000000000000000000000000000000004"),
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

	check("header", list[0].Hash().Hex(), "0xb0b7dfab9a8f485ea97a4642cdd380182ede101a64ecb3e73eb211496153d869")
	check("header", list[1].Hash().Hex(), "0xc1eaf394ed0b62b881e163c5399ad6342e753e72a6f585cc75a18b06dd45a59c")
	check("merkleRootHash", CalculateMerkleRoot(list).Hex(), "0xf175a1f35419972b352b2e2a7bbba6a6ade1c5a59da57114b23438bd3dbf82f2")
}
