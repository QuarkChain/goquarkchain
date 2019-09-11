package types

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
	"regexp"
	"strconv"
	"strings"
)

func makeHexWith0x(data string) string {
	ans := "0x"
	if len(data)%2 == 1 {
		ans += "0"
	}
	ans += data
	return ans
}

func bigIntToHexWith0x(data *big.Int) string {
	t := fmt.Sprintf("%x", data)
	return makeHexWith0x(t)
}

func uint64ToHexWith0x(data uint64) string {
	t := fmt.Sprintf("%x", data)
	return makeHexWith0x(t)
}

func uint32ToHexWith0x(data uint32) string {
	t := fmt.Sprintf("%x", data)
	return makeHexWith0x(t)
}

func receiptToHexWith0x(data *account.Recipient) string {
	t := make([]byte, 0)
	if data == nil {
		return makeHexWith0x(hex.EncodeToString(t))
	}
	return strings.ToLower(data.String())
}

func strRJust(initStr []byte, fill byte, width int) []byte {
	if len(initStr) >= width {
		return initStr
	}
	data := make([]byte, 0)
	for index := 0; index < width-len(initStr); index++ {
		data = append(data, fill)
	}
	data = append(data, initStr...)
	return data
}

func evmTxToTypedData(evmTx *EvmTransaction) []map[string]string {
	typedTxData := make([]map[string]string, 0)
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "nonce",
		"value": uint64ToHexWith0x(evmTx.data.AccountNonce),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "gasPrice",
		"value": bigIntToHexWith0x(evmTx.data.Price),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "gasLimit",
		"value": uint64ToHexWith0x(evmTx.data.GasLimit),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint160",
		"name":  "to",
		"value": receiptToHexWith0x(evmTx.To()),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "value",
		"value": bigIntToHexWith0x(evmTx.data.Amount),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "bytes",
		"name":  "data",
		"value": "0x" + hex.EncodeToString(evmTx.data.Payload),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "networkId",
		"value": uint32ToHexWith0x(evmTx.data.NetworkId),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint32",
		"name":  "fromFullShardKey",
		"value": uint32ToHexWith0x(evmTx.data.FromFullShardKey.GetValue()),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint32",
		"name":  "toFullShardKey",
		"value": uint32ToHexWith0x(evmTx.data.ToFullShardKey.GetValue()),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint64",
		"name":  "gasTokenId",
		"value": uint64ToHexWith0x(evmTx.data.GasTokenID),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint64",
		"name":  "transferTokenId",
		"value": uint64ToHexWith0x(evmTx.data.TransferTokenID),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "string",
		"name":  "qkcDomain",
		"value": "bottom-quark",
	})
	return typedTxData
}

func calSize(str string) (int, error) {
	reg := regexp.MustCompile(`\d+`)
	indexList := reg.FindAllStringIndex(str, -1)
	if len(indexList) != 1 {
		return 0, errors.New("len(indexList) should equal 1")
	}
	data := str[indexList[0][0]:indexList[0][1]]
	size, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0, err
	}
	return int(size), nil
}

func solidityPack(types, values []string) ([]byte, error) {
	if len(types) != len(values) {
		return nil, errors.New("types's len should equal values's len")
	}
	retv := make([]byte, 0)
	for index, t := range types {
		value := values[index]
		if t == "bytes" {
			if value == "0x" {
				continue
			}
			d, err := hex.DecodeString(value)
			if err != nil {
				return nil, err
			}
			retv = append(retv, d...)

		} else if t == "string" {
			retv = append(retv, []byte(value)...)

		} else if t == "bool" || t == "address" {
			return nil, errors.New("not support bool and address")

		} else if strings.HasPrefix(t, "bytes") {
			size, err := calSize(t)
			if err != nil {
				return nil, err
			}
			if size < 1 || size > 32 {
				return nil, errors.New("unsupported byte size")
			}
			v, err := hex.DecodeString(value[2:])
			if len(v) > size {
				return nil, errors.New("data is large than size")
			}
			retv = append(retv, []byte(strRJust(v, byte(0), size))...)

		} else if strings.HasPrefix(t, "int") || strings.HasPrefix(t, "uint") {
			size, err := calSize(t)
			if err != nil {
				return nil, err
			}
			if size%8 != 0 || size < 8 || size > 256 {
				return nil, errors.New("unsupported int size")
			}
			v, err := hex.DecodeString(value[2:])
			if err != nil {
				return nil, err
			}
			if len(v) > int(size)/8 {
				return nil, errors.New("data is larger than size")
			}
			retv = append(retv, []byte(strRJust(v, byte(0), int(size)/8))...)
		} else {
			return nil, fmt.Errorf("unsupported or invalid type %v", t)
		}
	}
	return retv, nil
}

func schema(tx []map[string]string) []string {
	t := make([]string, 0)
	for _, v := range tx {
		t = append(t, fmt.Sprintf("%s %s", v["type"], v["name"]))
	}
	return t
}

func types(tx []map[string]string) []string {
	t := make([]string, 0)
	for _, v := range tx {
		t = append(t, v["type"])
	}
	return t
}

func data(tx []map[string]string) []string {
	t := make([]string, 0)
	for _, v := range tx {
		if v["types"] == "bytes" {
			t = append(t, v["value"][2:])
		} else {
			t = append(t, v["value"])
		}
	}
	return t
}

func typedSignatureHash(tx []map[string]string) (string, error) {
	schema := schema(tx)
	types := types(tx)
	data := data(tx)

	string1 := make([]string, 0)
	for index := 0; index < len(tx); index++ {
		string1 = append(string1, "string")
	}

	s1, err := soliditySha3(string1, schema)
	if err != nil {
		return "", err
	}
	s2, err := soliditySha3(types, data)
	if err != nil {
		return "", err
	}
	return soliditySha3([]string{"bytes32", "bytes32"}, []string{s1, s2})
}

func soliditySha3(types, value []string) (string, error) {
	packData, err := solidityPack(types, value)
	if err != nil {
		return common.Hash{}.String(), err
	}
	return sha3_256(packData).String(), nil
}
