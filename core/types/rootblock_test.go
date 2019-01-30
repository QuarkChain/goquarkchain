package types

import (
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"math/big"
	"reflect"
	"testing"
)

func TestRootBlockEncoding(t *testing.T) {
	rootBlockHeaderEnc := common.FromHex("0000000100000002a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000000000000000000000000000000000000000000000000000000000003e800000000009896800227100000000000000064000401020304df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a280000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
	var blockHeader RootBlockHeader
	bb := serialize.NewByteBuffer(rootBlockHeaderEnc)
	if err := serialize.Deserialize(bb, &blockHeader); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err := serialize.SerializeToBytes(&blockHeader)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	key, _ := crypto.HexToECDSA("c987d4506fb6824639f9a9e3b8834584f5165e94680501d1b0044071cd36c3b3")
	blockHeader.SignWithPrivateKey(key)

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	check("Version", blockHeader.Version, uint32(1))
	check("Number", blockHeader.Number, uint32(2))
	check("ParentHash", common.Bytes2Hex(blockHeader.ParentHash.Bytes()), "a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97")
	check("MinorHeaderHash", common.Bytes2Hex(blockHeader.MinorHeaderHash.Bytes()), "297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495")
	check("coinbase_Recipient", common.Bytes2Hex(blockHeader.Coinbase.Recipient[:]), "d3f86deb4a2bbf85048b3e790460c40dbab1f621")
	check("coinbase_FullShardKey", uint32(blockHeader.Coinbase.FullShardKey), uint32(1023))
	check("CoinbaseAmount", blockHeader.CoinbaseAmount.Value, big.NewInt(1000))
	check("Time", blockHeader.Time, uint64(10000000))
	check("Difficulty", blockHeader.Difficulty, big.NewInt(10000))
	check("Nonce", blockHeader.Nonce, uint64(100))
	check("Extra", common.Bytes2Hex(*blockHeader.Extra), "01020304")
	check("MixDigest", common.Bytes2Hex(blockHeader.MixDigest.Bytes()), "df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28")
	check("Signature", common.Bytes2Hex(blockHeader.Signature[:]), "67998790744e85e3343830c1463b6ce04c204ad0e9a9069644aff2e8d45ff58b1ab4e1098244ffad3d80748e728aa6298abc989c2f2076f246d5885586b12f0201")
	check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "b499cb555a385c537f92c2633c21ac7a9a1c4a84a9c79c5c3d6bebc1138c9835")
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(rootBlockHeaderEnc))

	minorBlockHeadersEnc := common.FromHex("0000000200000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621d32b3e0d00000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000003000000000000000501060000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010003010203000000000000000000000000000000000000000000000000000000000000000400000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff0000000000000000000000000000000000000000000000000000000000000003a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba4950000000000000000000000000000000000000000000000000000000000000004df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28000000000000000501060000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010003010203df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28")
	var headers MinorBlockHeaders
	bb = serialize.NewByteBuffer(minorBlockHeadersEnc)
	if err := serialize.Deserialize(bb, &headers); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err = serialize.SerializeToBytes(headers)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	check("len(headers)", len(headers), 2)
	check("headers[0].Hash", common.Bytes2Hex(headers[0].Hash().Bytes()), "b274edca72087a960cff0dad483392ea36eb87d17dc8da40fe2ceb6616e559e8")
	check("headers[1].Hash", common.Bytes2Hex(headers[1].Hash().Bytes()), "48683020cb768970b700f0fa43f7c6622d0f6b3f45d3c10bf209c7c1272ca9c7")
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(minorBlockHeadersEnc))

	blockEnc := append(rootBlockHeaderEnc, append(minorBlockHeadersEnc, common.Hex2Bytes("00020102")...)...)
	var block RootBlock
	bb = serialize.NewByteBuffer(blockEnc)
	if err := serialize.Deserialize(bb, &block); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes, err = serialize.SerializeToBytes(&block)
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	block.header.SignWithPrivateKey(key)
	check("header", block.header, &blockHeader)
	check("headers", block.minorBlockHeaders.Len(), headers.Len())
	check("headers[0]", block.minorBlockHeaders[0].Hash(), headers[0].Hash())
	check("headers[1]", block.minorBlockHeaders[1].Hash(), headers[1].Hash())
	check("trackingdata", common.Bytes2Hex(block.trackingdata), "0102")
	check("blockhash", common.Bytes2Hex(block.Hash().Bytes()), "b499cb555a385c537f92c2633c21ac7a9a1c4a84a9c79c5c3d6bebc1138c9835")
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(blockEnc))

}
