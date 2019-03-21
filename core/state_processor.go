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
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

func ApplyTransaction(config *params.ChainConfig, bc ChainContext, gp *core.GasPool, statedb *state.StateDB, header *types.MinorBlockHeader, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, uint64, error) {
	statedb.SetFullShardID(tx.EvmTx.ToFullShardId())

	localFeeRate := float32(1.0)
	if qkcConfig := statedb.GetQuarkChainConfig(); qkcConfig != nil {
		localFeeRate = localFeeRate - qkcConfig.RewardTaxRate
	}
	msg, err := tx.EvmTx.AsMessage(types.MakeSigner(tx.EvmTx.NetworkId()))
	if err != nil {
		return nil, 0, err
	}
	context := NewEVMContext(msg, header, bc)
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	_, gas, failed, err := ApplyMessage(vmenv, msg, gp, localFeeRate)
	if err != nil {
		return nil, 0, err
	}

	var root []byte
	if config.IsByzantium(new(big.Int).SetUint64(header.Number)) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsEIP158(new(big.Int).SetUint64(header.Number))).Bytes()
	}
	*usedGas += gas

	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		//receipt.ContractAddress = account.BytesToIdentityRecipient(crypto.CreateAddress(vmenv.Context.Origin, tx.EvmTx.Nonce()).Bytes())
		receipt.ContractAddress = account.Recipient(vm.CreateAddress(vmenv.Context.Origin, msg.ToFullShardId(), tx.EvmTx.Nonce()))
		receipt.ContractFullShardId = tx.EvmTx.ToFullShardId()
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = statedb.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, gas, err
}
