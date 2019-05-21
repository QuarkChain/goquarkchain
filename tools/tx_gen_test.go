package test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"io/ioutil"
	"math/big"
	"testing"
)

type JsonStruct struct {
}

func NewJsonStruct() *JsonStruct {
	return &JsonStruct{}
}

func (jst *JsonStruct) Load(filename string, v interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, v)
	if err != nil {
		return err
	}
	return nil
}

type TxInFile struct {
	Nonce            uint64          `json:"Nonce"`
	GasPrice         *big.Int        `json:"GasPrice"`
	Gas              uint64          `json:"Gas"`
	Value            *big.Int        `json:"Value"`
	Data             []byte          `json:"Data"`
	FromFullShardKey uint32          `json:"FromFullShardKey"`
	ToFullShardKey   uint32          `json:"ToFullShardKey"`
	NetWorkID        uint32          `json:"NetWorkID"`
	To               *common.Address `json:"To"`
	Key              common.Hash     `json:"Key"`
}

func (t *TxInFile) UnmarshalJSON(input []byte) error {
	type TXUnmarshal struct {
		Nonce            uint64               `json:"Nonce"`
		GasPrice         *big.Int             `json:"GasPrice"`
		Gas              uint64               `json:"Gas"`
		Value            *big.Int             `json:"Value"`
		Data             *hexutil.Bytes       `json:"Data"`
		FromFullShardKey *math.HexOrDecimal64 `json:"FromFullShardKey"`
		ToFullShardKey   *math.HexOrDecimal64 `json:"ToFullShardKey"`
		NetWorkID        uint32               `json:"NetWorkID"`
		To               *common.Address      `json:"To"`
		Key              *common.Hash         `json:"Key"`
	}
	var dec TXUnmarshal
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}

	t.Nonce = dec.Nonce

	if dec.GasPrice != nil {
		t.GasPrice = dec.GasPrice
	}

	t.Gas = dec.Gas

	if dec.Value != nil {
		t.Value = dec.Value
	}
	if dec.Data != nil {
		t.Data = *dec.Data
	}
	if dec.FromFullShardKey != nil {
		t.FromFullShardKey = uint32(*dec.FromFullShardKey)
	}
	if dec.ToFullShardKey != nil {
		t.ToFullShardKey = uint32(*dec.ToFullShardKey)
	}

	t.NetWorkID = dec.NetWorkID

	if dec.To != nil {
		t.To = dec.To
	}
	if dec.Key != nil {
		t.Key = *dec.Key
	}
	return nil
}

func TestReadTxFromFile(t *testing.T) {
	jsonParse := NewJsonStruct()
	txFromFile := new(TxInFile)
	err := jsonParse.Load("./tx.json", txFromFile)
	if err != nil {
		panic(err)
	}
	var evmTx *types.EvmTransaction
	if txFromFile.To != nil {
		evmTx = types.NewEvmTransaction(txFromFile.Nonce, *txFromFile.To, txFromFile.Value, txFromFile.Gas, txFromFile.GasPrice, txFromFile.FromFullShardKey, txFromFile.ToFullShardKey, txFromFile.NetWorkID, 0, txFromFile.Data)
	} else {
		evmTx = types.NewEvmContractCreation(txFromFile.Nonce, txFromFile.Value, txFromFile.Gas, txFromFile.GasPrice, txFromFile.FromFullShardKey, txFromFile.ToFullShardKey, txFromFile.NetWorkID, 0, txFromFile.Data)

	}

	fmt.Println("fromFullShardKey", evmTx.FromFullShardKey())
	fmt.Println(evmTx.ToFullShardKey())
	fmt.Println("nonce", evmTx.Nonce())
	fmt.Println("data", hex.EncodeToString(evmTx.Data()))
	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(txFromFile.Key.Bytes()))
	if err != nil {
		panic(err)
	}
	tx, err := types.SignTx(evmTx, types.MakeSigner(evmTx.NetworkId()), prvKey)
	if err != nil {
		panic(err)
	}

	from, err := types.Sender(types.MakeSigner(tx.NetworkId()), tx)
	if err != nil {
		t.Error(err)
		t.FailNow()
	}
	fmt.Println("from", from.String())
	data, err := rlp.EncodeToBytes(tx)
	fmt.Println("data", hex.EncodeToString(data))
	s := new(types.EvmTransaction)
	err = rlp.DecodeBytes(data, s)
	fmt.Println("err", err)
}
