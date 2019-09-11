package types

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"strconv"
	"testing"
)

func bytesToUint64(data string) uint64 {
	n, err := strconv.ParseUint(data, 16, 64)
	if err != nil {
		panic(err)
	}

	n2 := uint64(n)
	return n2
}

func bytesToUint32(data string) uint32 {
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

func TestTyped(t *testing.T) {
	fmt.Println("!!!!!!!", bytesToUint64("0xd"))
	rawEvmTx := NewEvmTransaction(bytesToUint64("0x0d"), common.BytesToAddress(common.FromHex("314b2cd22c6d26618ce051a58c65af1253aecbb8")),
		bytesToBigInt("0x056bc75e2d63100000"), bytesToUint64("0x7530"), bytesToBigInt("0x02540be400"), bytesToUint32("0xc47decfd"),
		bytesToUint32("0xc49c1950"), bytesToUint32("0x03"), 1, nil, bytesToUint64("0x0111"), bytesToUint64("0x0222"),
	)
	//rawTx = Transaction{
	//	TxType: EvmTx,
	//	EvmTx:  rawEvmTx,
	//}
	//
	//tx = []map[string]string{
	//	map[string]string{
	//		"type":  "uint256",
	//		"name":  "nonce",
	//		"value": "0x0d",
	//	},
	//	map[string]string{
	//		"type":  "uint256",
	//		"name":  "gasPrice",
	//		"value": "0x02540be400",
	//	},
	//	map[string]string{
	//		"type":  "uint256",
	//		"name":  "gasLimit",
	//		"value": "0x7530",
	//	},
	//	map[string]string{
	//		"type":  "uint160",
	//		"name":  "to",
	//		"value": "0x314b2cd22c6d26618ce051a58c65af1253aecbb8",
	//	},
	//	map[string]string{
	//		"type":  "uint256",
	//		"name":  "value",
	//		"value": "0x056bc75e2d63100000",
	//	},
	//	map[string]string{
	//		"type":  "bytes",
	//		"name":  "data",
	//		"value": "0x",
	//	},
	//	map[string]string{
	//		"type":  "uint256",
	//		"name":  "networkId",
	//		"value": "0x03",
	//	},
	//	map[string]string{
	//		"type":  "uint32",
	//		"name":  "fromFullShardKey",
	//		"value": "0xc47decfd",
	//	},
	//	map[string]string{
	//		"type":  "uint32",
	//		"name":  "toFullShardKey",
	//		"value": "0xc49c1950",
	//	},
	//	map[string]string{
	//		"type":  "uint64",
	//		"name":  "gasTokenId",
	//		"value": "0x0111",
	//	},
	//	map[string]string{
	//		"type":  "uint64",
	//		"name":  "transferTokenId",
	//		"value": "0x0222",
	//	},
	//	map[string]string{
	//		"type":  "string",
	//		"name":  "qkcDomain",
	//		"value": "bottom-quark",
	//	},
	//}

	fmt.Println("flag", rawEvmTx.data.AccountNonce)
}
