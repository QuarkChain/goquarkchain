// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package tests

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/serialize"
	"math/big"
	"strings"

	qkcConfig "github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	qkcCore "github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/core"
	ethTypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// StateTest checks transaction processing without block context.
// See https://github.com/ethereum/EIPs/issues/176 for the test format specification.
type StateTest struct {
	json stJSON
}

// StateSubtest selects a specific configuration of a General State Test.
type StateSubtest struct {
	Fork   string
	Index  int
	Path   string
	PyData map[string]map[string]string
}

func (t *StateTest) UnmarshalJSON(in []byte) error {
	return json.Unmarshal(in, &t.json)
}

type stJSON struct {
	Env  stEnv                    `json:"env"`
	Pre  GenesisAlloc             `json:"pre"`
	Tx   stTransaction            `json:"transaction"`
	Out  hexutil.Bytes            `json:"out"`
	Post map[string][]stPostState `json:"post"`
}

type stPostState struct {
	Root    common.UnprefixedHash `json:"hash"`
	Logs    common.UnprefixedHash `json:"logs"`
	Indexes struct {
		Data            int `json:"data"`
		Gas             int `json:"gas"`
		Value           int `json:"value"`
		TransferTokenID int `json:"transferTokenId"`
	}
}

//go:generate gencodec -type stEnv -field-override stEnvMarshaling -out gen_stenv.go

type stEnv struct {
	Coinbase   common.Address `json:"currentCoinbase"   gencodec:"required"`
	Difficulty *big.Int       `json:"currentDifficulty" gencodec:"required"`
	GasLimit   uint64         `json:"currentGasLimit"   gencodec:"required"`
	Number     uint64         `json:"currentNumber"     gencodec:"required"`
	Timestamp  uint64         `json:"currentTimestamp"  gencodec:"required"`
}

type stEnvMarshaling struct {
	Coinbase   common.UnprefixedAddress
	Difficulty *math.HexOrDecimal256
	GasLimit   math.HexOrDecimal64
	Number     math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
}

//go:generate gencodec -type stTransaction -field-override stTransactionMarshaling -out gen_sttransaction.go

type stTransaction struct {
	GasPrice        *big.Int `json:"gasPrice"`
	Nonce           uint64   `json:"nonce"`
	To              string   `json:"to"`
	Data            []string `json:"data"`
	GasLimit        []uint64 `json:"gasLimit"`
	Value           []string `json:"value"`
	PrivateKey      []byte   `json:"secretKey"`
	TransferTokenID []uint64 `json:"transferTokenId"`
}

type stTransactionMarshaling struct {
	GasPrice   *math.HexOrDecimal256
	Nonce      math.HexOrDecimal64
	GasLimit   []math.HexOrDecimal64
	PrivateKey hexutil.Bytes
}

// Subtests returns all valid subtests of the test.
func (t *StateTest) Subtests() []StateSubtest {
	var sub []StateSubtest
	for fork, pss := range t.json.Post {
		for i := range pss {
			sub = append(sub, StateSubtest{fork, i, "", nil})
		}
	}
	return sub
}

func TransFromBlock(block *ethTypes.Block) *types.MinorBlockHeader {
	blockHeader := block.Header()
	coinbase := account.NewAddress(account.Recipient(blockHeader.Coinbase), 0)

	gasLimit := new(serialize.Uint256)
	gasLimit.Value = big.NewInt(int64(blockHeader.GasLimit))
	return &types.MinorBlockHeader{
		Number:     blockHeader.Number.Uint64(),
		Version:    1,
		Branch:     account.Branch{Value: 1},
		Coinbase:   coinbase,
		GasLimit:   gasLimit,
		Difficulty: big.NewInt(blockHeader.Difficulty.Int64()),
		Time:       blockHeader.Time.Uint64(),
	}
}

var (
	testQkcConfig = qkcConfig.NewQuarkChainConfig()
)

// Run executes a specific subtest.
func (t *StateTest) Run(subtest StateSubtest, vmconfig vm.Config) (*state.StateDB, error) {
	config, ok := Forks[subtest.Fork]
	if !ok {
		return nil, errors.New("not support")
	}
	block := t.genesis(config).ToBlock(nil)
	header := TransFromBlock(block)
	statedb := MakePreState(ethdb.NewMemDatabase(), t.json.Pre)

	//rootDis, _ := statedb.Commit(true)
	post := t.json.Post[subtest.Fork][subtest.Index]
	msg, err := t.json.Tx.toMessage(post)
	if err != nil {
		return nil, err
	}
	context := qkcCore.NewEVMContext(*msg, header, nil)
	evm := vm.NewEVM(context, statedb, config, vmconfig)

	gaspool := new(qkcCore.GasPool)
	gaspool.AddGas(block.GasLimit())
	snapshot := statedb.Snapshot()

	localFee := big.NewRat(1, 1)
	//fmt.Println("apply_tx")
	if _, _, _, err := qkcCore.ApplyMessage(evm, msg, gaspool, localFee); err != nil {
		statedb.RevertToSnapshot(snapshot)
	}
	//fmt.Println("apply_tx-end")

	root, err := statedb.Commit(config.IsEIP158(block.Number()))
	// Add 0-value mining reward. This only makes a difference in the cases
	// where
	// - the coinbase suicided, or
	// - there are only 'bad' transactions, which aren't executed. In those cases,
	//   the coinbase gets no txfee, so isn't created, and thus needs to be touched
	statedb.AddBalance(block.Coinbase(), new(big.Int), 0)
	// And _now_ get the state root
	// N.B: We need to do this in a two-step process, because the first Commit takes care
	// of suicides, and we need to touch the coinbase _after_ it has potentially suicided.
	if root != common.Hash(post.Root) {
		return statedb, fmt.Errorf("post state root mismatch: got %x, want %x", root, post.Root)
	}
	//if logs := rlpHash(statedb.Logs()); logs != common.Hash(post.Logs) {
	//	return statedb, fmt.Errorf("post state logs hash mismatch: got %x, want %x", logs, post.Logs)
	//}
	return statedb, nil
}

func (t *StateTest) gasLimit(subtest StateSubtest) uint64 {
	return t.json.Tx.GasLimit[t.json.Post[subtest.Fork][subtest.Index].Indexes.Gas]
}

func MakePreState(db ethdb.Database, accounts GenesisAlloc) *state.StateDB {
	sdb := state.NewDatabase(db)
	statedb, _ := state.New(common.Hash{}, sdb)
	statedb.SetQuarkChainConfig(testQkcConfig)
	for addr, a := range accounts {
		statedb.SetCode(addr, a.Code)
		statedb.SetNonce(addr, a.Nonce)
		if a.Balance != nil {
			statedb.SetBalance(addr, a.Balance, 0)
		} else {
			for tokenID, value := range a.Balances {
				statedb.SetBalance(addr, value, tokenID)
			}
		}

		for k, v := range a.Storage {
			statedb.SetState(addr, k, v)
		}
	}
	// Commit and re-open to start with a clean state.
	root, _ := statedb.Commit(false)
	statedb, _ = state.New(root, sdb)
	statedb.SetQuarkChainConfig(testQkcConfig)
	return statedb
}

func (t *StateTest) genesis(config *params.ChainConfig) *core.Genesis {
	return &core.Genesis{
		Config:     config,
		Coinbase:   t.json.Env.Coinbase,
		Difficulty: t.json.Env.Difficulty,
		GasLimit:   t.json.Env.GasLimit,
		Number:     t.json.Env.Number,
		Timestamp:  t.json.Env.Timestamp,
	}
}

func (tx *stTransaction) toMessage(ps stPostState) (*types.Message, error) {
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

	// Get values specific to this post state.
	if ps.Indexes.Data > len(tx.Data) {
		return nil, fmt.Errorf("tx data index %d out of bounds", ps.Indexes.Data)
	}
	if ps.Indexes.Value > len(tx.Value) {
		return nil, fmt.Errorf("tx value index %d out of bounds", ps.Indexes.Value)
	}
	if ps.Indexes.Gas > len(tx.GasLimit) {
		return nil, fmt.Errorf("tx gas limit index %d out of bounds", ps.Indexes.Gas)
	}
	if ps.Indexes.TransferTokenID > len(tx.TransferTokenID) {
		return nil, fmt.Errorf("tx TransferTokenID index %d out of bounds", ps.Indexes.TransferTokenID)
	}
	dataHex := tx.Data[ps.Indexes.Data]
	valueHex := tx.Value[ps.Indexes.Value]
	gasLimit := tx.GasLimit[ps.Indexes.Gas]
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

	fromRecipient := from
	toRecipient := &common.Address{}
	if to != nil {
		toRecipient.SetBytes((*to).Bytes())
	} else {

		toRecipient = nil
	}
	testQKCID := qkcCommon.TokenIDEncode("QKC")
	transferTokenID := testQKCID
	if len(tx.TransferTokenID) != 0 {
		transferTokenID = tx.TransferTokenID[ps.Indexes.TransferTokenID]
	}

	msg := types.NewMessage(fromRecipient, toRecipient, tx.Nonce, value, gasLimit, tx.GasPrice, data, true, 0, 0, transferTokenID, testQKCID)
	return &msg, nil
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}

func CheckPyData(root common.Hash, subtest StateSubtest, postRoot common.Hash) error {
	key := fmt.Sprintf("../../../fixtures/GeneralStateTests/%s", subtest.Path)

	if _, ok := subtest.PyData[key]; ok == false {

		return errors.New("data is not enouh")
	}
	if _, ok := subtest.PyData[key][postRoot.String()]; !ok {
		return errors.New("not this key")
	}

	if root.String() != subtest.PyData[key][postRoot.String()] {
		return errors.New("data is not match")
	}
	return nil

}
