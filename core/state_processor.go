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
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	qkcParams "github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
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
func (p *StateProcessor) Process(block *types.MinorBlock, statedb *state.StateDB, cfg vm.Config, evmTxIncluded []*types.Transaction, xShardReceiveTxList []*types.CrossShardTransactionDeposit) (types.Receipts, []*types.Log, uint64, error) {
	statedb.SetQuarkChainConfig(p.bc.clusterConfig.Quarkchain)
	statedb.SetBlockCoinbase(block.IHeader().GetCoinbase().Recipient)
	statedb.SetGasLimit(block.GasLimit())
	if evmTxIncluded == nil {
		evmTxIncluded = make([]*types.Transaction, 0)
	}
	if xShardReceiveTxList == nil {
		xShardReceiveTxList = make([]*types.CrossShardTransactionDeposit, 0)
	}

	rootBlockHeader := p.bc.getRootBlockHeaderByHash(block.Header().GetPrevRootBlockHash())
	preBlock := p.bc.GetMinorBlock(block.IHeader().GetParentHash())
	if preBlock == nil {
		return nil, nil, 0, errors.New("preBlock is nil")
	}

	preRootHeader := p.bc.getRootBlockHeaderByHash(preBlock.Header().GetPrevRootBlockHash())

	txList, err := p.bc.runCrossShardTxList(statedb, rootBlockHeader, preRootHeader)
	if err != nil {
		return nil, nil, 0, err
	}
	xShardReceiveTxList = append(xShardReceiveTxList, txList...)
	var (
		receipts types.Receipts
		usedGas  = new(uint64)
		header   = block.IHeader()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.Header().GetGasLimit().Uint64())
	)

	// Iterate over and process the individual transactions
	for i, tx := range block.GetTransactions() {
		evmTx, err := p.bc.validateTx(tx, statedb, nil, nil)
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
	coinbaseAmount := p.bc.getCoinbaseAmount()
	statedb.AddBalance(block.IHeader().GetCoinbase().Recipient, coinbaseAmount)
	statedb.Finalise()
	return receipts, allLogs, *usedGas, nil
}

func CheckSuperAccount(state vm.StateDB, from account.Recipient, to *account.Recipient) error {
	if !qkcParams.IsSuperAccount(from) {
		if state.GetAccountStatus(from) == false {
			return ErrAuthFromAccount
		}

		if to != nil && state.GetAccountStatus(*to) == false {
			return ErrAuthToAccount
		}
		return nil
	}
	if to == nil { //TODO need?
		return errors.New("super account need set to")
	}
	return nil
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

	if err := CheckSuperAccount(state, *from, tx.EvmTx.To()); err != nil {
		return err
	}
	reqNonce := state.GetNonce(*from)
	if reqNonce > tx.EvmTx.Nonce() {
		return ErrNonceTooLow
	}

	if state.GetBalance(*from).Cmp(tx.EvmTx.Cost()) < 0 {
		fmt.Println("?>???????????????", (*from).String(), state.GetBalance(*from).String(), tx.EvmTx.Cost())
		return ErrInsufficientFunds
	}

	totalGas, err := IntrinsicGas(tx.EvmTx.Data(), tx.EvmTx.To() == nil, tx.EvmTx.ToFullShardId() != tx.EvmTx.FromFullShardId())
	if err != nil {
		return err
	}
	if tx.EvmTx.Gas() < totalGas {
		return ErrIntrinsicGas
	}

	blockLimit := new(big.Int).Add(state.GetGasUsed(), new(big.Int).SetUint64(tx.EvmTx.Gas()))
	if blockLimit.Cmp(state.GetGasLimit()) > 0 {
		return errors.New("gasLimit is too low")
	}
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
	msg, err := tx.EvmTx.AsMessage(types.MakeSigner(tx.EvmTx.NetworkId()))
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
	statedb.Finalise()
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
