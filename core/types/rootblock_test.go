package types

import (
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"reflect"
	"testing"
)

func TestRootBlockEncoding(t *testing.T) {
	rootBlockHeaderEnc := common.FromHex("0000000100000002a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000000000000000000000000000000000000000000000000000000000003e800000000009896800227100227100000000000000064000401020304df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28a548f2718797a464bc5f52a8ae18068703edbb2550c78db312ebebb6d8f80c3d1af4416c6b21cf72b5d868fddeb9dbd5d40095c1e74e0b94e9d76915bb0fd1c201")
	//rootBlockHeaderEnc := common.FromHex("0000000100000002a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000000000000000000000000000000000000000000000000000000000003e800000000009896800227100227100000000000000064000401020304df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28ebb1e3be631d473446760c1cec98e9f2922cd5aa849d1e5cdb2b1c55ee77391c627f01155e70f5e55fb998e246cd7733f11ffc83f2c1ae729ed49eb3e1e0262d01")
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
	check("TotalDifficulty", blockHeader.ToTalDifficulty, big.NewInt(10000))
	check("Nonce", blockHeader.Nonce, uint64(100))
	check("Extra", common.Bytes2Hex(blockHeader.Extra), "01020304")
	check("MixDigest", common.Bytes2Hex(blockHeader.MixDigest.Bytes()), "df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28")
	if crypto.CryptoType == "nogm" {
		check("Signature", common.Bytes2Hex(blockHeader.Signature[:]), "a548f2718797a464bc5f52a8ae18068703edbb2550c78db312ebebb6d8f80c3d1af4416c6b21cf72b5d868fddeb9dbd5d40095c1e74e0b94e9d76915bb0fd1c201")
		check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "a6cddb462f647e15298aff364870837904a842c1bc8e8bfd27862f7553da8e0c")
	} else {
		check("Signature", common.Bytes2Hex(blockHeader.Signature[:]), "a548f2718797a464bc5f52a8ae18068703edbb2550c78db312ebebb6d8f80c3d1af4416c6b21cf72b5d868fddeb9dbd5d40095c1e74e0b94e9d76915bb0fd1c201")
		check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "5dc4a399af04427b4834acafd9c69461a52b65a9f81b29ee2164f8f4ba07e076")
	}
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(rootBlockHeaderEnc))

	minorBlockHeadersEnc := common.FromHex("0000000200000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621d32b3e0d00000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000003000000000000000501060000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010003010203000000000000000000000000000000000000000000000000000000000000000400000001000000010000000000000002d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff0000000000000000000000000000000000000000000000000000000000000003a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba4950000000000000000000000000000000000000000000000000000000000000004df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28000000000000000501060000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010003010203df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28")
	var headers MinorBlockHeaders
	bb = serialize.NewByteBuffer(minorBlockHeadersEnc)
	if err := serialize.DeserializeWithTags(bb, &headers, serialize.Tags{ByteSizeOfSliceLen: 4}); err != nil {
		t.Fatal("Deserialize error: ", err)
	}

	bytes = nil
	err = serialize.SerializeWithTags(&bytes, headers, serialize.Tags{ByteSizeOfSliceLen: 4})
	if err != nil {
		t.Fatal("Serialize error: ", err)
	}

	check("len(headers)", len(headers), 2)
	if crypto.CryptoType == "nogm" {
		check("headers[0].Hash", common.Bytes2Hex(headers[0].Hash().Bytes()), "b274edca72087a960cff0dad483392ea36eb87d17dc8da40fe2ceb6616e559e8")
		check("headers[1].Hash", common.Bytes2Hex(headers[1].Hash().Bytes()), "48683020cb768970b700f0fa43f7c6622d0f6b3f45d3c10bf209c7c1272ca9c7")
	} else {
		check("headers[0].Hash", common.Bytes2Hex(headers[0].Hash().Bytes()), "ba27c963b0b2244b29d4c0bfa0d04e5aff647d6a371ec124b58b59550ac9f874")
		check("headers[1].Hash", common.Bytes2Hex(headers[1].Hash().Bytes()), "b2c90714c27fa087fc2b35d7d325a747e6144aad11e51dcb195c6611f768cabe")
	}
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

	block.SignWithPrivateKey(key)
	if crypto.CryptoType == "gm" {
		blockHeader.Signature = block.header.Signature
	}
	check("header", block.header, &blockHeader)
	check("headers", block.minorBlockHeaders.Len(), headers.Len())
	check("headers[0]", block.minorBlockHeaders[0].Hash(), headers[0].Hash())
	check("headers[1]", block.minorBlockHeaders[1].Hash(), headers[1].Hash())
	check("trackingdata", common.Bytes2Hex(block.trackingdata), "0102")
	if crypto.CryptoType == "nogm" {
		check("blockhash", common.Bytes2Hex(block.Hash().Bytes()), "a6cddb462f647e15298aff364870837904a842c1bc8e8bfd27862f7553da8e0c")
	} else {
		check("blockhash", common.Bytes2Hex(block.Hash().Bytes()), "5dc4a399af04427b4834acafd9c69461a52b65a9f81b29ee2164f8f4ba07e076")
	}
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(blockEnc))

}

/*
Py code to generate data:

 header=RootBlockHeader()
        header.version=1
        header.height=2
        header.hash_prev_block=bytes.fromhex("a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97")
        header.hash_merkle_root=bytes.fromhex("297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba495")
        header.coinbase_address=Address.create_from(bytes.fromhex("d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff"))
        header.coinbase_amount=1000
        header.create_time=10000000
        header.difficulty=10000
        header.total_difficulty=10000
        header.nonce=100
        header.extra_data=bytes.fromhex("01020304")
        header.mixhash=bytes.fromhex("df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28")
        privkey = KeyAPI.PrivateKey(
            private_key_bytes=bytes.fromhex("c987d4506fb6824639f9a9e3b8834584f5165e94680501d1b0044071cd36c3b3")
        )

        header.sign_with_private_key(privkey)
        data=header.serialize()
        print("data",len(data),data.hex())
        print("hash",header.get_hash().hex())
        print("sigb",header.signature.hex())
*/
