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

// tx's format Examples of transaction formats
var (
	// in shard tx
	inShard = `{
  "Nonce": 0,
  "GasPrice": 21000,
  "Gas": 900000,
  "Value":10000000,
  "FromFullShardKey": "0x00000000",
  "ToFullShardKey": "0x00000000",
  "NetWorkID": 3,
  "To": "0x0000000000000000000000000000000000000011",
  "Key": "0x8cfc088e66867b9796731e9752beec1ce1bf65f600096b9bba10923b01c5db56"
}`
	// in shard contract tx
	contract = `{
  "Nonce": 1,
  "GasPrice": 21000,
  "Gas": 900000,
  "FromFullShardKey": "0x00000000",
  "ToFullShardKey": "0x00000000",
  "NetWorkID": 3,
  "Data":"0x608060405234801561001057600080fd5b5060405161045938038061045983398101806040528101908080518201929190505050336000806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055508060019080519060200190610089929190610090565b5050610135565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106100d157805160ff19168380011785556100ff565b828001600101855582156100ff579182015b828111156100fe5782518255916020019190600101906100e3565b5b50905061010c9190610110565b5090565b61013291905b8082111561012e576000816000905550600101610116565b5090565b90565b610315806101446000396000f300608060405260043610610057576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff16806342cbb15c1461005c578063a413686214610087578063cfae3217146100f0575b600080fd5b34801561006857600080fd5b50610071610180565b6040518082815260200191505060405180910390f35b34801561009357600080fd5b506100ee600480360381019080803590602001908201803590602001908080601f0160208091040260200160405190810160405280939291908181526020018383808284378201915050505050509192919290505050610188565b005b3480156100fc57600080fd5b506101056101a2565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561014557808201518184015260208101905061012a565b50505050905090810190601f1680156101725780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b600043905090565b806001908051906020019061019e929190610244565b5050565b606060018054600181600116156101000203166002900480601f01602080910402602001604051908101604052809291908181526020018280546001816001161561010002031660029004801561023a5780601f1061020f5761010080835404028352916020019161023a565b820191906000526020600020905b81548152906001019060200180831161021d57829003601f168201915b5050505050905090565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f1061028557805160ff19168380011785556102b3565b828001600101855582156102b3579182015b828111156102b2578251825591602001919060010190610297565b5b5090506102c091906102c4565b5090565b6102e691905b808211156102e25760008160009055506001016102ca565b5090565b905600a165627a7a72305820b65144ba1d967908bb1a2d47f6e1c39f81b666ce2776d5ee9791692038a1b0b30029",
  "Key": "0x8cfc088e66867b9796731e9752beec1ce1bf65f600096b9bba10923b01c5db56"
}`

	// xShard tx
	xshard = `{
  "Nonce": 2,
  "GasPrice": 21000,
  "Gas": 900000,
  "Value":10000000,
  "FromFullShardKey": "0x00000000",
  "ToFullShardKey": "0x00000000",
  "NetWorkID": 3,
  "To": "0x0000000000000000000000000000000000000011",
  "Key": "0x8cfc088e66867b9796731e9752beec1ce1bf65f600096b9bba10923b01c5db56"
}`
)

// To generate tx_data
// And send json'rpc sendRawTransaction
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
		fmt.Println("normal tx")
		evmTx = types.NewEvmTransaction(txFromFile.Nonce, *txFromFile.To, txFromFile.Value, txFromFile.Gas,
			txFromFile.GasPrice, txFromFile.FromFullShardKey, txFromFile.ToFullShardKey, txFromFile.NetWorkID,
			0, txFromFile.Data)
	} else {
		fmt.Println("contract")
		evmTx = types.NewEvmContractCreation(txFromFile.Nonce, txFromFile.Value, txFromFile.Gas, txFromFile.GasPrice,
			txFromFile.FromFullShardKey, txFromFile.ToFullShardKey, txFromFile.NetWorkID, 0, txFromFile.Data)

	}

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
	s := new(types.EvmTransaction)
	err = rlp.DecodeBytes(data, s)
	fmt.Println("data_to_json_rpc", hex.EncodeToString(data))
	vv, rr, ss := tx.RawSignatureValues()
	fmt.Printf("%x\n", vv)
	fmt.Printf("%x\n", rr)
	fmt.Printf("%x\n", ss)
}
