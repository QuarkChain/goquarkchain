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

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	qkcParam "github.com/QuarkChain/goquarkchain/params"
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
	statedb.SetBlockCoinbase(block.IHeader().GetCoinbase().Recipient)
	statedb.SetGasLimit(block.GasLimit())
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.IHeader()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.Header().GetGasLimit().Uint64())
		xGas     = block.Meta().XShardGasLimit.Value.Uint64()
	)

	// Iterate over and process the individual transactions
	for i, tx := range block.GetTransactions() {
		evmTx, err := p.bc.validateTx(tx, statedb, nil, nil, &xGas)
		if err != nil {
			return nil, nil, 0, err
		}
		statedb.Prepare(tx.Hash(), block.Hash(), i)
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
		statedb.AddBalance(block.IHeader().GetCoinbase().Recipient, v, k)
	}
	statedb.Finalise(true)
	return receipts, allLogs, *usedGas, nil
}

// ValidateTransaction validateTx before applyTx
func ValidateTransaction(state vm.StateDB, tx *types.Transaction, fromAddress *account.Address) error {
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

	allowTransferTokens := state.GetQuarkChainConfig().AllowedTransferTokenIDs()
	allowGasTokens := state.GetQuarkChainConfig().AllowedGasTokenIDs()
	if _, ok := allowTransferTokens[tx.EvmTx.TransferTokenID()]; !ok {
		return fmt.Errorf("token %v is not allowed transferToken list %v", tx.EvmTx.TransferTokenID(), allowTransferTokens)
	}

	if _, ok := allowGasTokens[tx.EvmTx.GasTokenID()]; !ok {
		return fmt.Errorf("token %v is not allowed gasToken list %v", tx.EvmTx.GasTokenID(), allowGasTokens)
	}

	if tx.EvmTx.TransferTokenID() == tx.EvmTx.GasTokenID() {
		totalCost := new(big.Int).Mul(tx.EvmTx.GasPrice(), new(big.Int).SetUint64(tx.EvmTx.Gas()))
		totalCost = new(big.Int).Add(totalCost, tx.EvmTx.Value())
		if state.GetBalance(*from, tx.EvmTx.TransferTokenID()).Cmp(totalCost) < 0 {
			return fmt.Errorf("money is low: token:%v balance %v,totalCost %v", tx.EvmTx.TransferTokenID(), state.GetBalance(*from, tx.EvmTx.TransferTokenID()), totalCost)
		}
	} else {
		if state.GetBalance(*from, tx.EvmTx.TransferTokenID()).Cmp(tx.EvmTx.Value()) < 0 {
			return fmt.Errorf("money is low: token:%v balance %v, value:%v", tx.EvmTx.TransferTokenID(), state.GetBalance(*from, tx.EvmTx.TransferTokenID()), tx.EvmTx.Value())
		}
		gasCost := new(big.Int).Mul(tx.EvmTx.GasPrice(), new(big.Int).SetUint64(tx.EvmTx.Gas()))
		if state.GetBalance(*from, tx.EvmTx.GasTokenID()).Cmp(gasCost) < 0 {
			return fmt.Errorf("money is low: token %v balance %v value %v", tx.EvmTx.GasTokenID(), state.GetBalance(*from, tx.EvmTx.GasTokenID()), gasCost)
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
	statedb.SetFullShardKey(tx.EvmTx.ToFullShardKey())
	localFeeRate := big.NewRat(1, 1)
	if qkcConfig := statedb.GetQuarkChainConfig(); qkcConfig != nil {
		num := qkcConfig.RewardTaxRate.Num().Int64()
		denom := qkcConfig.RewardTaxRate.Denom().Int64()
		localFeeRate = big.NewRat(denom-num, denom)

	}
	msg, err := tx.EvmTx.AsMessage(types.MakeSigner(tx.EvmTx.NetworkId()), tx.Hash())
	if err != nil {
		return nil, nil, 0, err
	}
	context := NewEVMContext(msg, header, bc)
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	ret, gas, failed, err := ApplyMessage(vmenv, msg, gp, localFeeRate)
	if err != nil {
		return nil, nil, 0, err
	}

	var root []byte
	statedb.Finalise(true)
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = account.Recipient(vm.CreateAddress(vmenv.Context.Origin, msg.ToFullShardKey(), tx.EvmTx.Nonce()))
		receipt.ContractFullShardId = tx.EvmTx.ToFullShardId()
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
	receipt.ContractFullShardId = tx.EvmTx.ToFullShardKey()

	return ret, receipt, gas, err
}

func ApplyCrossShardDeposit(config *params.ChainConfig, bc ChainContext, header types.IHeader, cfg vm.Config,
	evmState *state.StateDB, tx *types.CrossShardTransactionDeposit, usedGas *uint64, localFeeRate *big.Rat,
	checkIsFromRootChain bool) (*types.Receipt, error) {

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
	evmState.SetFullShardKey(tx.To.FullShardKey)
	evmState.AddBalance(tx.From.Recipient, tx.Value.Value, tx.TransferTokenID)
	msg := types.NewMessage(tx.From.Recipient, &tx.To.Recipient, 0, tx.Value.Value,
		tx.GasRemained.Value.Uint64(), tx.GasPrice.Value, tx.MessageData, false,
		tx.From.FullShardKey, tx.To.FullShardKey, tx.TransferTokenID, tx.GasTokenID)
	context := NewEVMContext(msg, header, bc)
	context.IsApplyXShard = true
	context.XShardGasUsedStart = gasUsedStart
	vmenv := vm.NewEVM(context, evmState, config, cfg)
	gp := new(GasPool).AddGas(evmState.GetGasLimit().Uint64())
	_, gas, fail, err = ApplyMessage(vmenv, msg, gp, localFeeRate)
	if err != nil {
		//fmt.Println("111")
		return nil, err
	}
	*usedGas += gas
	if evmState.GetQuarkChainConfig().XShardAddReceiptTimestamp != 0 {
		//fmt.Println("?????")
		var root []byte
		receipt := types.NewReceipt(root, fail, *usedGas)
		receipt.TxHash = tx.TxHash
		receipt.GasUsed = gas
		receipt.Logs = evmState.GetLogs(tx.TxHash)
		receipt.Bloom = types.CreateBloom(types.Receipts{receipt})
		receipt.ContractFullShardId = tx.To.FullShardKey
		return receipt, nil
	}
	//fmt.Println("@@@@@@@@@@@@@@@@@@@@@", evmState.GetQuarkChainConfig().XShardAddReceiptTimestamp)
	return nil, nil
}
