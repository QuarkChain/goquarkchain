package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	gomath "math"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

type stEnv struct {
	Coinbase   common.Address `json:"currentCoinbase"   gencodec:"required"`
	Difficulty *big.Int       `json:"currentDifficulty" gencodec:"required"`
	GasLimit   uint64         `json:"currentGasLimit"   gencodec:"required"`
	Number     uint64         `json:"currentNumber"     gencodec:"required"`
	Timestamp  uint64         `json:"currentTimestamp"  gencodec:"required"`
	ParentHash string         `json:"previousHash"  gencodec:"required"`
}

type stEnvMarshaling struct {
	Coinbase   common.UnprefixedAddress
	Difficulty *math.HexOrDecimal256
	GasLimit   math.HexOrDecimal64
	Number     math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
}

type stTransaction struct {
	GasPrice   *big.Int `json:"gasPrice"`
	Nonce      uint64   `json:"nonce"`
	To         string   `json:"to"`
	Data       []string `json:"data"`
	GasLimit   []uint64 `json:"gasLimit"`
	Value      []string `json:"value"`
	PrivateKey []byte   `json:"secretKey"`
}

type stTransactionMarshaling struct {
	GasPrice   *math.HexOrDecimal256
	Nonce      math.HexOrDecimal64
	GasLimit   []math.HexOrDecimal64
	PrivateKey hexutil.Bytes
}

type stJSON struct {
	Env stEnv           `json:"env"`
	Txs []stTransaction `json:"transactions"`
}

type Testdata struct {
	json stJSON
}

func (t *Testdata) UnmarshalJSON(in []byte) error {
	return json.Unmarshal(in, &t.json)
}

func (tx *stTransaction) toMessage() (core.Message, error) {
	// Derive sender from private key if present.
	var from common.Address
	if len(tx.PrivateKey) > 0 {
		key, err := crypto.ToECDSA(tx.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %v", err)
		}
		from = crypto.PubkeyToAddress(key.PublicKey)
	}
	// Parse recipient if present.
	var to *common.Address
	if tx.To != "" {
		to = new(common.Address)
		if err := to.UnmarshalText([]byte(tx.To)); err != nil {
			return nil, fmt.Errorf("invalid to address: %v", err)
		}
	}

	dataHex := tx.Data[0]
	valueHex := tx.Value[0]
	gasLimit := tx.GasLimit[0]
	// Value, Data hex encoding is messy: https://github.com/ethereum/tests/issues/203
	value := new(big.Int)
	if valueHex != "0x" {
		v, ok := math.ParseBig256(valueHex)
		if !ok {
			return nil, fmt.Errorf("invalid tx value %q", valueHex)
		}
		value = v
	}
	data, err := hex.DecodeString(strings.TrimPrefix(dataHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid tx data %q", dataHex)
	}

	msg := types.NewMessage(from, to, tx.Nonce, value, gasLimit, tx.GasPrice, data, true)
	return msg, nil
}

func (tx *stTransaction) toTransaction() (*types.Transaction, error) {
	signer := new(types.HomesteadSigner)
	key, err := crypto.ToECDSA(tx.PrivateKey)
	if err != nil {
		panic(err)
	}
	msg, err := tx.toMessage()
	if err != nil {
		return nil, err
	}
	ret := types.NewTransaction(tx.Nonce, *msg.To(), msg.Value(), msg.Gas(), msg.GasPrice(), msg.Data())
	ret, err = types.SignTx(ret, signer, key)
	if err != nil {
		panic(err)
	}
	return ret, nil
}

func TestBlockEncodingDecoding(t *testing.T) {
	bytes, err := ioutil.ReadFile("../BlockData.json")
	if err != nil {
		panic(err)
	}
	var td Testdata
	if err = json.Unmarshal(bytes, &td); err != nil {
		panic(err)
	}

	h := &types.Header{
		Coinbase:   td.json.Env.Coinbase,
		Difficulty: td.json.Env.Difficulty,
		GasLimit:   td.json.Env.GasLimit,
		Number:     big.NewInt(1),
		Time:       big.NewInt(int64(td.json.Env.Timestamp)),
		ParentHash: common.HexToHash(td.json.Env.ParentHash),
	}

	txs := []*types.Transaction{}
	for _, stTx := range td.json.Txs {
		tx, err := stTx.toTransaction()
		if err != nil {
			panic(err)
		}
		txs = append(txs, tx)
	}

	b := types.NewBlock(h, txs, nil, nil)
	b.ReceivedAt = time.Now()
	bs, err := rlp.EncodeToBytes(b)
	if err != nil {
		panic(err)
	}
	t.Logf("byte length %d\n", len(bs))

	warmupRuns, measureRuns := 500, 10

	// Serialization performance.
	for i := 0; i < warmupRuns; i++ {
		_, err := rlp.EncodeToBytes(b)
		if err != nil {
			panic(err)
		}
	}
	recorded := []int64{}
	for i := 0; i < measureRuns; i++ {
		start := time.Now()
		rlp.EncodeToBytes(b)
		used := int64(time.Now().Sub(start) / time.Microsecond)
		recorded = append(recorded, used)
	}
	mean, std := calculateMeanAndStd(recorded)
	t.Logf("profiling serialization, mean: %f, stdev: %f\n", mean, std)

	var recoveredBlock types.Block
	// Deserialization performance.
	for i := 0; i < warmupRuns; i++ {
		if err := rlp.DecodeBytes(bs, &recoveredBlock); err != nil {
			panic(err)
		}
		if len(recoveredBlock.Transactions()) != 500 {
			panic("recover failure")
		}
	}
	recorded = []int64{}
	for i := 0; i < measureRuns; i++ {
		// var recoveredBlock types.Block
		start := time.Now()
		rlp.DecodeBytes(bs, &recoveredBlock)
		used := int64(time.Now().Sub(start) / time.Microsecond)
		recorded = append(recorded, used)
	}
	mean, std = calculateMeanAndStd(recorded)
	t.Logf("profiling deserialization, mean: %f, stdev: %f\n", mean, std)
}

func calculateMeanAndStd(arr []int64) (mean, std float64) {
	sum := int64(0)
	for _, n := range arr {
		sum += n
	}
	mean = float64(sum) / float64(len(arr))
	for _, n := range arr {
		std += gomath.Pow(float64(n)-mean, 2)
	}
	std = gomath.Sqrt(std / float64(len(arr)))
	return
}
