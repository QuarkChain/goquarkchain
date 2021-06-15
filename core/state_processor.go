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

package core

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/QuarkChain/goquarkchain/account"
	qkcCmn "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	qkcParam "github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	//TODO delete eth minorBlockChain config
	config *params.ChainConfig // Chain configuration options
	bc     *MinorBlockChain    // Canonical block chain
	engine consensus.Engine    // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(config *params.ChainConfig, bc *MinorBlockChain, engine consensus.Engine) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
func (p *StateProcessor) Process(block *types.MinorBlock, statedb *state.StateDB, cfg vm.Config) (types.Receipts, []*types.Log, uint64, error) {
	statedb.SetQuarkChainConfig(p.bc.clusterConfig.Quarkchain)
	statedb.SetBlockCoinbase(block.Coinbase().Recipient)
	statedb.SetGasLimit(block.GasLimit())
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.IHeader()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit().Uint64())
		xGas     = block.GetXShardGasLimit().Uint64()
	)

	// Iterate over and process the individual transactions
	for i, tx := range block.GetTransactions() {
		evmTx, err := p.bc.validateTx(tx, statedb, nil, nil, &xGas)
		if err != nil {
			return nil, nil, 0, err
		}
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		if err := evmTx.EvmTx.SetQuarkChainConfig(p.bc.clusterConfig.Quarkchain); err != nil {
			return nil, nil, 0, err
		}
		_, receipt, _, err := ApplyTransaction(p.config, p.bc, gp, statedb, header, evmTx, usedGas, cfg)
		if err != nil {
			return nil, nil, 0, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	coinbaseAmount := p.bc.getCoinbaseAmount(block.Number())
	bMap := coinbaseAmount.GetBalanceMap()
	for k, v := range bMap {
		statedb.AddBalance(block.Coinbase().Recipient, v, k)
	}
	statedb.Finalise(true)
	return receipts, allLogs, *usedGas, nil
}

// ValidateTransaction validateTx before applyTx
func ValidateTransaction(state vm.StateDB, chainConfig *params.ChainConfig, tx *types.Transaction, fromAddress *account.Address) error {
	from := new(account.Recipient)
	if fromAddress == nil {
		tempFrom, err := tx.Sender(types.MakeSigner(tx.EvmTx.NetworkId()))
		if err != nil {
			return err
		}
		from = &tempFrom
	} else {
		from = &fromAddress.Recipient
	}

	// not need to check gas(uint64)
	if qkcCmn.BiggerThanUint128Max(tx.EvmTx.GasPrice()) || tx.EvmTx.GasTokenID() > qkcCmn.TOKENIDMAX ||
		tx.EvmTx.TransferTokenID() > qkcCmn.TOKENIDMAX {
		return fmt.Errorf("startgas, gasprice, and token_id must <= UINT128_MAX")
	}
	reqNonce := state.GetNonce(*from)
	if bytes.Equal(from.Bytes(), account.Recipient{}.Bytes()) {
		reqNonce = 0
	}
	if reqNonce > tx.EvmTx.Nonce() {
		return ErrNonceTooLow
	}

	totalGas, err := IntrinsicGas(tx.EvmTx.Data(), tx.EvmTx.To() == nil, tx.EvmTx.ToFullShardId() != tx.EvmTx.FromFullShardId())
	if err != nil {
		return err
	}
	if tx.EvmTx.Gas() < totalGas {
		return ErrIntrinsicGas
	}

	defaultChainToken := state.GetQuarkChainConfig().GetDefaultChainTokenID()
	bal := make(map[uint64]*big.Int)
	bal[tx.EvmTx.TransferTokenID()] = state.GetBalance(*from, tx.EvmTx.TransferTokenID())
	if tx.EvmTx.TransferTokenID() != tx.EvmTx.GasTokenID() {
		bal[tx.EvmTx.GasTokenID()] = state.GetBalance(*from, tx.EvmTx.GasTokenID())
	}

	for _, tokenID := range []uint64{tx.EvmTx.TransferTokenID(), tx.EvmTx.GasTokenID()} {
		if tokenID != defaultChainToken && bal[tokenID].Cmp(common.Big0) == 0 {
			return fmt.Errorf("{%v}: non-default token {%v} has zero balance", tx.Hash().String(), tokenID)
		}
	}

	cost := make(map[uint64]*big.Int)
	cost[tx.EvmTx.TransferTokenID()] = tx.EvmTx.Value()
	gasCost := new(big.Int).Mul(tx.EvmTx.GasPrice(), new(big.Int).SetUint64(tx.EvmTx.Gas()))
	if tx.EvmTx.TransferTokenID() == tx.EvmTx.GasTokenID() {
		cost[tx.EvmTx.TransferTokenID()] = new(big.Int).Add(gasCost, cost[tx.EvmTx.TransferTokenID()])
	} else {
		cost[tx.EvmTx.GasTokenID()] = gasCost
	}

	for tokenID, value := range bal {
		if value.Cmp(cost[tokenID]) < 0 {
			return fmt.Errorf("InsufficientBalance tokenID:%v,value:%v,costValue:%v", tokenID, value, cost[tokenID])
		}
	}

	if tx.EvmTx.GasPrice().Cmp(common.Big0) != 0 && tx.EvmTx.GasTokenID() != defaultChainToken {
		snapshot := state.Snapshot()
		_, gengisTokenGasPrice, err := PayNativeTokenAsGas(state, chainConfig, tx.EvmTx.GasTokenID(), tx.EvmTx.Gas(), tx.EvmTx.GasPrice())
		if err != nil {
			return err
		}
		state.RevertToSnapshot(snapshot)

		if gengisTokenGasPrice.Cmp(common.Big0) == 0 {
			return fmt.Errorf("{%v}: non-default gas token {%v} not ready for being used to pay gas", tx.Hash().String(), tx.EvmTx.GasTokenID())
		}
		balGasReserve := state.GetBalance(vm.SystemContracts[vm.GENERAL_NATIVE_TOKEN].Address(), defaultChainToken)

		if balGasReserve.Cmp(new(big.Int).Mul(gengisTokenGasPrice, new(big.Int).SetUint64(tx.EvmTx.Gas()))) < 0 {
			return fmt.Errorf("{%v}: non-default gas token {%v} not enough reserve balance for conversion", tx.Hash().String(), tx.EvmTx.GasTokenID())
		}

	}

	blockLimit := new(big.Int).Add(state.GetGasUsed(), new(big.Int).SetUint64(tx.EvmTx.Gas()))
	if blockLimit.Cmp(state.GetGasLimit()) > 0 {
		return errors.New("gasLimit is too low")
	}

	//TODO EIP86-specific restrictions?
	return nil
}

// ApplyTransaction apply tx
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, gp *GasPool, statedb *state.StateDB, header types.IHeader, tx *types.Transaction, usedGas *uint64, cfg vm.Config) ([]byte, *types.Receipt, uint64, error) {
	if !reflect.ValueOf(bc).IsNil() { //chain_makers.go
		if err := tx.EvmTx.SetQuarkChainConfig(bc.Config()); err != nil {
			return nil, nil, 0, err
		}
	}
	statedbGensisToken := statedb.GetQuarkChainConfig().GetDefaultChainTokenID()
	gasPrice, refundRate := tx.EvmTx.GasPrice(), uint8(100)
	convertedGenesisTokenGasPrice := new(big.Int)

	var err error

	if tx.EvmTx.GasTokenID() != statedbGensisToken {
		refundRate, convertedGenesisTokenGasPrice, err = PayNativeTokenAsGas(statedb, config, tx.EvmTx.GasTokenID(), tx.EvmTx.Gas(), tx.EvmTx.GasPrice())
		if convertedGenesisTokenGasPrice == nil || convertedGenesisTokenGasPrice.Cmp(common.Big0) <= 0 {
			return nil, nil, 0, fmt.Errorf("convertedGenesisTokenGasPeice %v shoud >0", convertedGenesisTokenGasPrice)
		}
		gasPrice = convertedGenesisTokenGasPrice
		contractAddr := vm.SystemContracts[vm.GENERAL_NATIVE_TOKEN].Address()

		contractBal := statedb.GetBalance(contractAddr, statedbGensisToken)
		txGasLimit := new(big.Int).SetUint64(tx.EvmTx.Gas())
		txGasBal := new(big.Int).Mul(txGasLimit, convertedGenesisTokenGasPrice)
		if contractBal.Cmp(txGasBal) < 0 {
			return nil, nil, 0, fmt.Errorf("contract balance:%v < tx gas balance:%v", contractBal, txGasBal)
		}

		statedb.SubBalance(contractAddr, new(big.Int).Mul(txGasLimit, convertedGenesisTokenGasPrice), statedbGensisToken)
		statedb.AddBalance(contractAddr, new(big.Int).Mul(txGasLimit, tx.EvmTx.GasPrice()), tx.EvmTx.GasTokenID())
	}
	statedb.SetFullShardKey(tx.EvmTx.ToFullShardKey())
	msg, err := tx.EvmTx.AsMessage(types.NewEIP155Signer(tx.EvmTx.NetworkId()), tx.Hash(), gasPrice, tx.EvmTx.GasTokenID(), refundRate)
	if err != nil {
		return nil, nil, 0, err
	}

	context := NewEVMContext(msg, header, bc, tx.EvmTx.GasPrice())
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	ret, gas, failed, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, nil, 0, err
	}

	var root []byte
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, statedb.GetGasUsed().Uint64())
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil && !msg.IsCrossShard() && !failed {
		receipt.ContractAddress = account.Recipient(vm.CreateAddress(vmenv.Context.Origin, msg.ToFullShardKey(), tx.EvmTx.Nonce()))
	}
	receipt.ContractFullShardKey = tx.EvmTx.ToFullShardKey()
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return ret, receipt, gas, err
}

func ApplyCrossShardDeposit(config *params.ChainConfig, bc ChainContext, header types.IHeader, cfg vm.Config,
	evmState *state.StateDB, tx *types.CrossShardTransactionDeposit, usedGas *uint64,
	checkIsFromRootChain bool, txIndex int) (*types.Receipt, error) {

	var (
		gas  uint64
		fail bool
		err  error
	)
	gasUsedStart := qkcParam.GtxxShardCost.Uint64()
	if checkIsFromRootChain {
		if tx.IsFromRootChain {
			gasUsedStart = 0
		}
	} else {
		if tx.GasPrice.Value.Cmp(big.NewInt(0)) == 0 {
			gasUsedStart = 0
		}
	}

	quarkChainConfig := evmState.GetQuarkChainConfig()
	if evmState.GetTimeStamp() < quarkChainConfig.EnableEvmTimeStamp {
		//TODO:FIXME:full_shard_key is not set
		evmState.AddBalance(tx.To.Recipient, tx.Value.Value, tx.TransferTokenID)
		evmState.AddGasUsed(new(big.Int).SetUint64(gasUsedStart))
		*usedGas += gasUsedStart

		xShardFee := new(big.Int).Mul(tx.GasPrice.Value, qkcParam.GtxxShardCost)
		xShardFee = qkcCmn.BigIntMulBigRat(xShardFee, quarkChainConfig.LocalFeeRate)

		evmState.AddBlockFee(map[uint64]*big.Int{
			tx.GasTokenID: xShardFee,
		})
		evmState.AddBalance(evmState.GetBlockCoinbase(), xShardFee, tx.GasTokenID)
		return nil, nil
	}

	evmState.SetFullShardKey(tx.To.FullShardKey)

	evmState.AddBalance(tx.From.Recipient, tx.Value.Value, tx.TransferTokenID)
	msg := types.NewMessage(tx.From.Recipient, &tx.To.Recipient, 0, tx.Value.Value,
		tx.GasRemained.Value.Uint64(), tx.GasPrice.Value, tx.MessageData, false,
		tx.From.FullShardKey, &tx.To.FullShardKey, tx.TransferTokenID, tx.GasTokenID, tx.RefundRate)
	context := NewEVMContext(msg, header, bc, msg.GasPrice())
	context.IsApplyXShard = true
	context.XShardGasUsedStart = gasUsedStart
	if tx.CreateContract {
		context.ContractAddress = &tx.To.Recipient
	}
	vmenv := vm.NewEVM(context, evmState, config, cfg)
	gp := new(GasPool).AddGas(evmState.GetGasLimit().Uint64())
	evmState.Prepare(tx.TxHash, header.Hash(), txIndex)
	_, gas, fail, err = ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, err
	}
	*usedGas += gas
	if evmState.GetTimeStamp() >= quarkChainConfig.EnableEvmTimeStamp {
		var root []byte
		receipt := types.NewReceipt(root, fail, *usedGas)
		receipt.TxHash = tx.TxHash
		receipt.GasUsed = gas
		receipt.Logs = evmState.GetLogs(tx.TxHash)
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
		if tx.CreateContract && !fail {
			receipt.ContractAddress = tx.To.Recipient
		}
		receipt.ContractFullShardKey = tx.To.FullShardKey
		return receipt, nil
	}
	return nil, nil
}

func PayNativeTokenAsGas(evmState vm.StateDB, config *params.ChainConfig, tokenID, gas uint64,
	gasPriceInNativeToken *big.Int) (uint8, *big.Int, error) {
	// not need to check gas(uint64)
	if tokenID > qkcCmn.TOKENIDMAX || qkcCmn.BiggerThanUint128Max(gasPriceInNativeToken) {
		return 0, nil, fmt.Errorf("PayNativeTokenAsGas : tokenid %v > TOKENIDMAX %v gasPriceInNativeToken %v > Uint128Max ", tokenID, qkcCmn.TOKENIDMAX, gasPriceInNativeToken)
	}

	//# Call the `payAsGas` function
	data := common.Hex2Bytes("5ae8f7f1")
	data = append(data, qkcCmn.EncodeToByte32(tokenID)...)
	data = append(data, qkcCmn.EncodeToByte32(gas)...)
	data = append(data, qkcCmn.BigToByte32(gasPriceInNativeToken)...)
	return callGeneralNativeTokenManager(evmState, config, data)
}

func GetGasUtilityInfo(evmState vm.StateDB, config *params.ChainConfig, tokenID uint64,
	gasPriceInNativeToken *big.Int) (uint8, *big.Int, error) {

	//# Call the `calculateGasPrice` function
	data := common.Hex2Bytes("ce9e8c47")
	data = append(data, qkcCmn.EncodeToByte32(tokenID)...)
	data = append(data, qkcCmn.EncodeToByte32(gasPriceInNativeToken.Uint64())...)
	return callGeneralNativeTokenManager(evmState, config, data)
}

func callGeneralNativeTokenManager(evmState vm.StateDB, config *params.ChainConfig, data []byte) (uint8, *big.Int, error) {
	contractAddr := vm.SystemContracts[vm.GENERAL_NATIVE_TOKEN].Address()
	code := evmState.GetCode(contractAddr)
	if len(code) == 0 {
		return 0, nil, ErrContractNotFound
	}
	ctx := vm.Context{
		CanTransfer:                       CanTransfer,
		Transfer:                          Transfer,
		TransferFailureByPoswBalanceCheck: TransferFailureByPoswBalanceCheck,
		BlockNumber:                       new(big.Int).SetUint64(evmState.GetBlockNumber()),
	}
	evm := vm.NewEVM(ctx, evmState, config, vm.Config{})
	//# Only contract itself can invoke payment
	sender := vm.AccountRef(contractAddr)
	ret, _, err := evm.Call(&sender, contractAddr, data, 1000000, new(big.Int))
	if err != nil {
		return 0, nil, err
	}
	refundRate := int(ret[31])
	convertedGasPrice := new(big.Int).SetBytes(ret[32:64])
	return uint8(refundRate), convertedGasPrice, nil
}

func ConvertToDefaultChainTokenGasPrice(state vm.StateDB, paramConfig *params.ChainConfig, tokenID uint64, gasprice *big.Int) (*big.Int, error) {
	if tokenID == state.GetQuarkChainConfig().GetDefaultChainTokenID() {
		return gasprice, nil
	}
	snapshot := state.Snapshot()
	_, data, err := GetGasUtilityInfo(state, paramConfig, tokenID, gasprice)
	state.RevertToSnapshot(snapshot)
	return data, err
}
