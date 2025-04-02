package types

import (
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
)

func bytesToUint64(data string) uint64 {
	data = data[2:]
	n, err := strconv.ParseUint(data, 16, 64)
	if err != nil {
		panic(err)
	}

	n2 := uint64(n)
	return n2
}

func bytesToUint32(data string) uint32 {
	data = data[2:]
	n, err := strconv.ParseUint(data, 16, 32)
	if err != nil {
		panic(err)
	}

	n2 := uint32(n)
	return n2
}
func bytesToBigInt(data string) *big.Int {
	n := new(big.Int)
	n, _ = n.SetString(data[2:], 16)

	return n
}

var (
	rawEvmTx = NewEvmTransaction(bytesToUint64("0x0d"), common.BytesToAddress(common.FromHex("314b2cd22c6d26618ce051a58c65af1253aecbb8")),
		bytesToBigInt("0x056bc75e2d63100000"), bytesToUint64("0x7530"), bytesToBigInt("0x02540be400"), bytesToUint32("0xc47decfd"),
		bytesToUint32("0xc49c1950"), bytesToUint32("0x03"), 1, nil, bytesToUint64("0x0111"), bytesToUint64("0x0222"),
	)
	rawTx = Transaction{
		TxType: EvmTx,
		EvmTx:  rawEvmTx,
	}

	tx = []map[string]string{
		{
			"type":  "uint256",
			"name":  "nonce",
			"value": "0x0d",
		},
		{
			"type":  "uint256",
			"name":  "gasPrice",
			"value": "0x02540be400",
		},
		{
			"type":  "uint256",
			"name":  "gasLimit",
			"value": "0x7530",
		},
		{
			"type":  "uint160",
			"name":  "to",
			"value": "0x314b2cd22c6d26618ce051a58c65af1253aecbb8",
		},
		{
			"type":  "uint256",
			"name":  "value",
			"value": "0x056bc75e2d63100000",
		},
		{
			"type":  "bytes",
			"name":  "data",
			"value": "0x",
		},
		{
			"type":  "uint256",
			"name":  "networkId",
			"value": "0x03",
		},
		{
			"type":  "uint32",
			"name":  "fromFullShardKey",
			"value": "0xc47decfd",
		},
		{
			"type":  "uint32",
			"name":  "toFullShardKey",
			"value": "0xc49c1950",
		},
		{
			"type":  "uint64",
			"name":  "gasTokenId",
			"value": "0x0111",
		},
		{
			"type":  "uint64",
			"name":  "transferTokenId",
			"value": "0x0222",
		},
		{
			"type":  "string",
			"name":  "qkcDomain",
			"value": "bottom-quark",
		},
	}
)

func TestTyped(t *testing.T) {
	assert.Equal(t, tx, evmTxToTypedData(rawEvmTx))
}

func TestSolidityPack(t *testing.T) {
	schema := schema(tx)
	types := types(tx)
	data := data(tx)

	t1 := make([]string, 0)
	for index := 0; index < len(tx); index++ {
		t1 = append(t1, "string")
	}
	h1, err := solidityPack(t1, schema)
	assert.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(h1), "75696e74323536206e6f6e636575696e7432353620676173507269636575696e74323536206761734c696d697475696e7431363020746f75696e743235362076616c75656279746573206461746175696e74323536206e6574776f726b496475696e7433322066726f6d46756c6c53686172644b657975696e74333220746f46756c6c53686172644b657975696e74363420676173546f6b656e496475696e743634207472616e73666572546f6b656e4964737472696e6720716b63446f6d61696e")

	h2, err := solidityPack(types, data)
	assert.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(h2), "000000000000000000000000000000000000000000000000000000000000000d00000000000000000000000000000000000000000000000000000002540be4000000000000000000000000000000000000000000000000000000000000007530314b2cd22c6d26618ce051a58c65af1253aecbb80000000000000000000000000000000000000000000000056bc75e2d631000000000000000000000000000000000000000000000000000000000000000000003c47decfdc49c195000000000000001110000000000000222626f74746f6d2d717561726b")
}

func TestTypedSignatureHash(t *testing.T) {
	h, err := typedSignatureHash(tx)
	assert.NoError(t, err)
	assert.Equal(t, h, "0xe768719d0a211ffb0b7f9c7bc6af9286136b3dd8b6be634a57dc9d6bee35b492")
}

func TestRecover(t *testing.T) {
	rawTx.EvmTx.SetVRS(bytesToBigInt("0x1b"), bytesToBigInt("0xb5145678e43df2b7ea8e0e969e51dbf72c956dd52e234c95393ad68744394855"), bytesToBigInt("0x44515b465dbbf746a484239c11adb98f967e35347e17e71b84d850d8e5c38a6a"))

	sender, err := Sender(NewEIP155Signer(rawTx.EvmTx.NetworkId()), rawTx.EvmTx)
	assert.NoError(t, err)
	assert.Equal(t, strings.ToLower(sender.String()[2:]), "2e6144d0a4786e6f62892eee59c24d1e81e33272")
}
