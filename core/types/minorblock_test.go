package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/crypto"
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
	//nonce , to , amount , gasLimit , gasPrice, fromFullShardId , toFullShardId , networkId , version , data
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
	if err := serialize.Deserialize(&bb, &blockHeader); err != nil {
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
	check("Extra", common.Bytes2Hex(*blockHeader.Extra), "010203")
	check("MixDigest", common.Bytes2Hex(blockHeader.MixDigest.Bytes()), "0000000000000000000000000000000000000000000000000000000000000004")
	check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "b274edca72087a960cff0dad483392ea36eb87d17dc8da40fe2ceb6616e559e8")
	check("serialize", bytes, blocHeaderEnc)

	blocMetaEnc := common.FromHex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a280000000000000000000000000000000000000000000000000000000000000064000000000000000000000000000000000000000000000000000000000000012c")
	var blockMeta MinorBlockMeta
	bb = serialize.NewByteBuffer(blocMetaEnc)
	if err := serialize.Deserialize(&bb, &blockMeta); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err = serialize.SerializeToBytes(blockMeta)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	check("Root", blockMeta.Root[:], common.FromHex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97"))
	check("TxHash", blockMeta.TxHash[:], common.FromHex("297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495"))
	check("ReceiptHash", blockMeta.ReceiptHash[:], common.FromHex("df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28"))
	check("GasUsed", *blockMeta.GasUsed, serialize.Uint256{Value: big.NewInt(100)})
	check("CrossShardGasUsed", *blockMeta.CrossShardGasUsed, serialize.Uint256{Value: big.NewInt(300)})
	check("bmserialize", bytes, blocMetaEnc)

	signer := NewEIP155Signer(1)
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	transactionsEnc := common.FromHex("000000020100000063f86180808094b94f5374fce5edbc8e2a8697c15331677e6ebf0b8080808001801ca0fab2cc481eb33edacc4016fe35f30051014109cf0d227679003dd36534247845a019c29e2b33a1a8adf95dabf603cc62758775ce73c3d78098160818dcd7c0970d0100000069f86703018207d094b94f5374fce5edbc8e2a8697c15331677e6ebf0b0a8435353434808001801ca03ba243b74816362081890b8b680d303a5cce7803a12d8ce863723bcd1be94efba0399e91eb5b20c258a77f7045da3f2b84bc1c1f40e0c23bcc3df7bce05bea2ed8")
	var trans Transactions
	bb = serialize.NewByteBuffer(transactionsEnc)
	if err := serialize.Deserialize(&bb, &trans); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err = serialize.SerializeToBytes(trans)
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
	if err := serialize.Deserialize(&bb, &block); err != nil {
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
