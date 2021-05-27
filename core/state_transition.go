// Copyright 2014 The go-ethereum Authors
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
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	qkcParam "github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
)

var (
	errInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
}

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	//FromFrontier() (common.Address, error)
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
	IsCrossShard() bool
	FromFullShardKey() uint32
	ToFullShardKey() *uint32
	TxHash() common.Hash
	GasTokenID() uint64
	TransferTokenID() uint64
	RefundRate() uint8
}

// IntrinsicGas computes the 'intrinsic gas' for a message with the given data.
func IntrinsicGas(data []byte, contractCreation, isCrossShard bool) (uint64, error) {
	// Set the starting gas for the raw transaction
	var gas uint64
	if contractCreation {
		gas = params.TxGasContractCreation
	} else {
		gas = params.TxGas
	}
	// Bump the required gas by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		if (math.MaxUint64-gas)/params.TxDataNonZeroGas < nz {
			return 0, vm.ErrOutOfGas
		}
		gas += nz * params.TxDataNonZeroGas
		z := uint64(len(data)) - nz
		if (math.MaxUint64-gas)/params.TxDataZeroGas < z {
			return 0, vm.ErrOutOfGas
		}
		gas += z * params.TxDataZeroGas
	}

	// GTXXSHARDCOST
	if isCrossShard {
		gas += qkcParam.GtxxShardCost.Uint64()
	}
	return gas, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm *vm.EVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(evm *vm.EVM, msg Message, gp *GasPool) ([]byte, uint64, bool, error) {
	return NewStateTransition(evm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return common.Address{}
	}
	return *st.msg.To()
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return vm.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.evm.GasPriceInGasToken)
	if st.state.GetBalance(st.msg.From(), st.evm.GasTokenID).Cmp(mgval) < 0 {
		return errInsufficientBalanceForGas
	}
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	st.state.SubBalance(st.msg.From(), mgval, st.evm.GasTokenID)
	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(st.msg.From())
		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the used gas. It returns an error if failed.
// An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, failed bool, err error) {
	var (
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error.
		vmerr            error
		gas              uint64
		msg              = st.msg
		evm              = st.evm
		contractCreation = msg.To() == nil
	)
	if evm.IsApplyXShard {
		st.preFill()
		gas = evm.XShardGasUsedStart
	} else {
		// Pay intrinsic gas
		if err = st.preCheck(); err != nil {
			return
		}
		gas, err = IntrinsicGas(st.data, contractCreation, msg.IsCrossShard())
		if err != nil {
			return nil, 0, false, err
		}
	}
	if err = st.useGas(gas); err != nil {
		return nil, 0, false, err
	}

	sender := vm.AccountRef(msg.From())
	if msg.IsCrossShard() {
		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		return st.AddCrossShardTxDeposit(gas)
	}
	if contractCreation || evm.ContractAddress != nil {
		ret, _, st.gas, vmerr = evm.Create(sender, st.data, st.gas, st.value, evm.ContractAddress)
		fmt.Println("236--err", vmerr)
	} else {
		// Increment the nonce for the next transaction
		if !st.evm.IsApplyXShard {
			st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		}

		if st.transferFailureByPoSWBalanceCheck() {
			ret, st.gas, vmerr = nil, 0, vm.ErrPoSWSenderNotAllowed
			fmt.Println("245--err", vmerr)
		} else {
			ret, st.gas, vmerr = evm.Call(sender, st.to(), st.data, st.gas, st.value)
			fmt.Println("248--err", vmerr)
		}
	}

	if vmerr != nil {
		log.Debug("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		if vmerr == vm.ErrInsufficientBalance {
			return nil, 0, false, vmerr
		}
	}
	st.refundGas(vmerr)
	st.chargeFee(st.gasUsed())
	fmt.Println("vmerr", hex.EncodeToString(ret), vmerr, err)
	if vmerr == vm.ErrPoSWSenderNotAllowed {

		return nil, st.gasUsed(), true, nil
	}
	return ret, st.gasUsed(), vmerr != nil, err
}

func (st *StateTransition) refund(total *big.Int) {
	gasTokenId := st.evm.StateDB.GetQuarkChainConfig().GetDefaultChainTokenID()
	if st.msg.RefundRate() == 100 {
		st.state.AddBalance(st.msg.From(), total, gasTokenId)
		return
	}

	bigIntMulUint8 := func(data *big.Int, u uint8) *big.Int {
		return new(big.Int).Mul(data, new(big.Int).SetUint64(uint64(u)))
	}
	bigIntDivUint8 := func(data *big.Int, u uint8) *big.Int {
		return new(big.Int).Div(data, new(big.Int).SetUint64(uint64(u)))
	}

	toRefund := bigIntMulUint8(total, st.msg.RefundRate())
	toRefund = bigIntDivUint8(toRefund, 100)

	toburn := new(big.Int).Sub(total, toRefund)

	st.state.AddBalance(st.msg.From(), toRefund, gasTokenId)
	if toburn.Cmp(common.Big0) >= 0 {
		st.state.AddBalance(common.Address{}, toburn, gasTokenId)
	}
}

func (st *StateTransition) refundGas(vmerr error) {

	// Apply refund counter, capped to half of the used gas.
	if vmerr == nil {
		refund := st.gasUsed() / 2
		if refund > st.state.GetRefund() {
			refund = st.state.GetRefund()
		}
		st.gas += refund
	}
	st.state.SubRefund(st.state.GetRefund())

	// Return ETH for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice)
	st.refund(remaining)
	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}

func (st *StateTransition) preFill() {
	st.gas += st.msg.Gas() + st.evm.XShardGasUsedStart
	st.initialGas = st.gas
}

func (st *StateTransition) AddCrossShardTxDeposit(intrinsicGas uint64) (ret []byte, usedGas uint64,
	failed bool, err error) {
	evm := st.evm
	msg := st.msg
	state := evm.StateDB
	gensisToken := state.GetQuarkChainConfig().GetDefaultChainTokenID()
	if !evm.CanTransfer(state, msg.From(), st.value, st.msg.TransferTokenID()) {
		return nil, st.gas, false, vm.ErrInsufficientBalance
	}
	remoteGasReserved := uint64(0)
	if st.transferFailureByPoSWBalanceCheck() {
		//Currently, burn all gas
		st.gas = 0
		failed = true
		err = vm.ErrPoSWSenderNotAllowed
	} else if msg.To() == nil {
		state.SubBalance(msg.From(), st.value, msg.TransferTokenID())
		remoteGasReserved = msg.Gas() - intrinsicGas
		crossShardValue := new(serialize.Uint256)
		crossShardValue.Value = new(big.Int).Set(msg.Value())
		crossShardGasPrice := new(serialize.Uint256)
		crossShardGasPrice.Value = new(big.Int).Set(msg.GasPrice())
		crossShardGas := new(serialize.Uint256)
		crossShardGas.Value = new(big.Int).SetUint64(remoteGasReserved)

		fromFullShardKey := msg.FromFullShardKey()
		crossShardData := &types.CrossShardTransactionDeposit{
			CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
				TxHash: msg.TxHash(),
				From: account.Address{
					Recipient:    account.Recipient(msg.From()),
					FullShardKey: msg.FromFullShardKey(),
				},
				To: account.Address{
					Recipient:    vm.CreateAddress(msg.From(), &fromFullShardKey, state.GetNonce(msg.From())),
					FullShardKey: *msg.ToFullShardKey(),
				},
				Value:           crossShardValue,
				GasTokenID:      gensisToken,
				TransferTokenID: msg.TransferTokenID(),
				GasRemained:     crossShardGas,
				GasPrice:        crossShardGasPrice,
				MessageData:     msg.Data(),
				CreateContract:  true,
			},
			RefundRate: st.msg.RefundRate(),
		}
		state.AppendXShardList(crossShardData)
		failed = false

	} else {
		state.SubBalance(msg.From(), st.value, msg.TransferTokenID())
		crossShardValue := new(serialize.Uint256)
		crossShardValue.Value = new(big.Int).Set(msg.Value())
		crossShardGasPrice := new(serialize.Uint256)
		crossShardGasPrice.Value = new(big.Int).Set(msg.GasPrice())
		crossShardGas := new(serialize.Uint256)
		crossShardGas.Value = new(big.Int)

		if state.GetTimeStamp() >= state.GetQuarkChainConfig().EnableEvmTimeStamp {
			remoteGasReserved = msg.Gas() - intrinsicGas
			crossShardGas.Value = new(big.Int).SetUint64(remoteGasReserved)
		}

		crossShardData := &types.CrossShardTransactionDeposit{
			CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
				TxHash: msg.TxHash(),
				From: account.Address{
					Recipient:    account.Recipient(msg.From()),
					FullShardKey: msg.FromFullShardKey(),
				},
				To: account.Address{
					Recipient:    account.Recipient(*msg.To()),
					FullShardKey: *msg.ToFullShardKey(),
				},
				Value: crossShardValue,
				//convert to genesis token and use converted gas price
				GasTokenID:      gensisToken,
				TransferTokenID: msg.TransferTokenID(),
				GasRemained:     crossShardGas,
				GasPrice:        crossShardGasPrice,
				MessageData:     msg.Data(),
				CreateContract:  false,
			},
			RefundRate: st.msg.RefundRate(),
		}
		state.AppendXShardList(crossShardData)
		failed = false
	}
	localGasUsed := st.gasUsed()
	//refund: gasRemained is always 0?
	gasRemained := msg.Gas() - localGasUsed - remoteGasReserved
	fund := new(big.Int).Mul(new(big.Int).SetUint64(gasRemained), st.gasPrice)
	st.refund(fund)
	if !failed {
		//reserve part of the gas for the target shard miner for fee
		localGasUsed -= qkcParam.GtxxShardCost.Uint64()
	}
	st.chargeFee(localGasUsed)
	return nil, state.GetGasUsed().Uint64(), failed, nil
}

func (st *StateTransition) chargeFee(gasUsed uint64) {
	fee := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), st.gasPrice)
	rateFee := qkcCommon.BigIntMulBigRat(fee, st.state.GetQuarkChainConfig().LocalFeeRate)
	gasTokenId := st.evm.StateDB.GetQuarkChainConfig().GetDefaultChainTokenID()
	st.state.AddBalance(st.evm.Coinbase, rateFee, gasTokenId)
	blockFee := make(map[uint64]*big.Int)
	blockFee[gasTokenId] = rateFee
	st.state.AddBlockFee(blockFee)
	if st.state.GetTimeStamp() >= st.state.GetQuarkChainConfig().EnableEvmTimeStamp {
		st.state.AddGasUsed(new(big.Int).SetUint64(gasUsed))
		return
	}
	st.state.AddGasUsed(new(big.Int).SetUint64(st.gasUsed()))
}

func (st *StateTransition) transferFailureByPoSWBalanceCheck() bool {
	if v, ok := st.state.GetSenderDisallowMap()[st.msg.From()]; ok {
		fmt.Println("????????", st.msg.From().String(), st.state.GetSenderDisallowMap())

		tt := st.state.GetSenderDisallowMap()
		for _, v := range tt {
			fmt.Println("origin", v.String())
		}

		fmt.Println("????", st.msg.Value(), v, st.state.GetBalance(st.msg.From(), st.evm.StateDB.GetQuarkChainConfig().GetDefaultChainTokenID()))
		if new(big.Int).Add(st.msg.Value(), v).Cmp(st.state.GetBalance(st.msg.From(), st.evm.StateDB.GetQuarkChainConfig().GetDefaultChainTokenID())) == 1 {
			return true
		}
	}
	return false
}
