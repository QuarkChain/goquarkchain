package types

import "fmt"

func evmTxToTypedData(evmTx *EvmTransaction) []map[string]string {
	typedTxData := make([]map[string]string, 0)
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "nonce",
		"value": fmt.Sprintf("%v", evmTx.data.AccountNonce),
	})
	fmt.Println("????", evmTx.data.AccountNonce, fmt.Sprintf("%v", evmTx.data.AccountNonce))
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "gasPrice",
		"value": fmt.Sprintf("%v", evmTx.data.Price),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "gasLimit",
		"value": fmt.Sprintf("%v", evmTx.data.GasLimit),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint256",
		"name":  "networkId",
		"value": fmt.Sprintf("%v", evmTx.data.NetworkId),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint32",
		"name":  "fromFullShardKey",
		"value": fmt.Sprintf("%v", evmTx.data.FromFullShardKey),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint32",
		"name":  "toFullShardKey",
		"value": fmt.Sprintf("%v", evmTx.data.ToFullShardKey),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint64",
		"name":  "gasTokenId",
		"value": fmt.Sprintf("%v", evmTx.data.GasTokenID),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "uint64",
		"name":  "transferTokenId",
		"value": fmt.Sprintf("%v", evmTx.data.TransferTokenID),
	})
	typedTxData = append(typedTxData, map[string]string{
		"type":  "string",
		"name":  "qkcDomain",
		"value": "bottom-quark",
	})
	return typedTxData
}
