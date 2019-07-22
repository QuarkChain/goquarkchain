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
	rootBlockHeaderEnc := common.FromHex("0000000100000002a40920ae6f758f88c61b405f9fc39fdd6274666462b14e3887522166e6537a97297d6ae9803346cdb059a671dea7e37b684dcabfa767f2d872026ad0a3aba4950000000000000000000000000000000000000000000000000000000000000000d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000002010101010102010200000000009896800227100227100000000000000064000401020304df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28c758a15769202219b1fce50049eeac1af1dddb28bc282c1fb79a2208fa24f763308b1b191d656a5123ac979067a6c941867f3000d978a5d34810fe6c194dc38101")
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
	check("CoinbaseAmount", blockHeader.CoinbaseAmount.BalanceMap[1], new(big.Int).SetUint64(1))
	check("CoinbaseAmount", blockHeader.CoinbaseAmount.BalanceMap[2], new(big.Int).SetUint64(2))
	check("Time", blockHeader.Time, uint64(10000000))
	check("Difficulty", blockHeader.Difficulty, big.NewInt(10000))
	check("TotalDifficulty", blockHeader.ToTalDifficulty, big.NewInt(10000))
	check("Nonce", blockHeader.Nonce, uint64(100))
	check("Extra", common.Bytes2Hex(blockHeader.Extra), "01020304")
	check("MixDigest", common.Bytes2Hex(blockHeader.MixDigest.Bytes()), "df227f34313c2bc4a4a986817ea46437f049873f2fca8e2b89b1ecd0f9e67a28")
	check("Signature", common.Bytes2Hex(blockHeader.Signature[:]), "c758a15769202219b1fce50049eeac1af1dddb28bc282c1fb79a2208fa24f763308b1b191d656a5123ac979067a6c941867f3000d978a5d34810fe6c194dc38101")
	check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "725576c58f70f22166767d41d50fd1e22d2913524f967bf1a7fc020cb0e19b10")
	check("Hash", common.Bytes2Hex(blockHeader.Hash().Bytes()), "725576c58f70f22166767d41d50fd1e22d2913524f967bf1a7fc020cb0e19b10")
	check("serialize", common.Bytes2Hex(bytes), common.Bytes2Hex(rootBlockHeaderEnc))

	minorBlockHeadersEnc := common.FromHex("0000000200000457000000010000000000002b67d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff0000000201010101010201020000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000003000000000000000501060000000000000007000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010003010203000000000000000000000000000000000000000000000000000000000000000400000457000000010000000000a98ac7d3f86deb4a2bbf85048b3e790460c40dbab1f621000003ff00000002010101010102010200000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000002000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000030000000000000005010600000000000000070000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000100030102030000000000000000000000000000000000000000000000000000000000000004")
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
	check("headers[0].Hash", common.Bytes2Hex(headers[0].Hash().Bytes()), "cfe6b217b566f12e7568d46c47de85d13193902eafb8f39d9d56ae725cf11f7f")
	check("headers[1].Hash", common.Bytes2Hex(headers[1].Hash().Bytes()), "1245f631e4ce43188fd9412d1fcab34db8c62f5728d0d54550d1a0dc67617f01")
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
	check("blockhash", common.Bytes2Hex(block.Hash().Bytes()), "725576c58f70f22166767d41d50fd1e22d2913524f967bf1a7fc020cb0e19b10")
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
