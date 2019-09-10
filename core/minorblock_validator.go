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
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"time"
)

// MinorBlockValidator is responsible for validating block Headers, uncles and
// processed state.
//
// MinorBlockValidator implements Validator.
type MinorBlockValidator struct {
	quarkChainConfig *config.QuarkChainConfig // Chain configuration options
	bc               *MinorBlockChain         // Canonical block chain
	engine           consensus.Engine         // Consensus engine used for validating
	branch           account.Branch
	logInfo          string
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(quarkChainConfig *config.QuarkChainConfig, blockchain *MinorBlockChain, engine consensus.Engine, branch account.Branch) *MinorBlockValidator {
	validator := &MinorBlockValidator{
		quarkChainConfig: quarkChainConfig,
		engine:           engine,
		bc:               blockchain,
		branch:           branch,
		logInfo:          fmt.Sprintf("minorBlock validate branch:%v", branch),
	}
	return validator
}

// ValidateBlock validates the given block's uncles and verifies the block
// header's transaction and uncle roots. The Headers are assumed to be already
// validated at this point.
func (v *MinorBlockValidator) ValidateBlock(mBlock types.IBlock, force bool) error {
	if common.IsNil(mBlock) {
		log.Error(v.logInfo, "check block err", ErrMinorBlockIsNil)
		return ErrMinorBlockIsNil
	}
	block, ok := mBlock.(*types.MinorBlock)
	if !ok {
		log.Error(v.logInfo, "check block err", ErrInvalidMinorBlock)
		return ErrInvalidMinorBlock
	}

	if block.Header().Version != 0 {
		return errors.New("incorrect minor block version")
	}

	blockHeight := block.NumberU64()
	if blockHeight < 1 {
		errBlockHeight := errors.New("block.Number <1")
		log.Error(v.logInfo, "err", errBlockHeight, "blockHeight", blockHeight)
		return errBlockHeight
	}
	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(block.Hash()) && !force {
		log.Error(v.logInfo, "already have this block err", ErrKnownBlock, "height", block.NumberU64(), "hash", block.Hash().String())
		return ErrKnownBlock
	}

	if !v.bc.HasBlockAndState(block.IHeader().GetParentHash()) {
		if !v.bc.HasBlock(block.ParentHash()) {
			log.Error(v.logInfo, "parent block do not have", consensus.ErrUnknownAncestor, "parent height", block.Header().Number-1, "hash", block.Header().ParentHash.String())
			return consensus.ErrUnknownAncestor
		}
		log.Warn(v.logInfo, "will insert side chain", ErrPrunedAncestor, "parent height", block.Header().Number-1, "hash", block.Header().ParentHash.String(), "currHash", block.Hash().String())
		return ErrPrunedAncestor
	}

	prevHeader := v.bc.GetHeader(block.IHeader().GetParentHash())
	if common.IsNil(prevHeader) {
		log.Error(v.logInfo, "parent header is not exist", ErrInvalidMinorBlock, "parent height", block.Header().Number-1, "parent hash", block.Header().ParentHash.String())
		return ErrInvalidMinorBlock
	}
	if blockHeight != prevHeader.NumberU64()+1 {
		log.Error(v.logInfo, "err", ErrHeightMismatch, "blockHeight", blockHeight, "prevHeader", prevHeader.NumberU64())
		return ErrHeightMismatch
	}

	if block.Branch().Value != v.branch.Value {
		log.Error(v.logInfo, "err", ErrBranch, "block.branch", block.Branch().Value, "current branch", v.branch.Value)
		return ErrBranch
	}

	if block.Header().Time > uint64(time.Now().Unix())+ALLOWED_FUTURE_BLOCKS_TIME_VALIDATION {
		return fmt.Errorf("block too far into future")
	}

	if block.IHeader().GetTime() <= prevHeader.GetTime() {
		log.Error(v.logInfo, "err", ErrTime, "block.Time", block.IHeader().GetTime(), "prevHeader.Time", prevHeader.GetTime())
		return ErrTime
	}

	if block.Header().MetaHash != block.GetMetaData().Hash() {
		log.Error(v.logInfo, "err", ErrMetaHash, "block.Metahash", block.Header().MetaHash.String(), "block.meta.hash", block.GetMetaData().Hash().String())
		return ErrMetaHash
	}

	if len(block.IHeader().GetExtra()) > int(v.quarkChainConfig.BlockExtraDataSizeLimit) {
		log.Error(v.logInfo, "err", ErrExtraLimit, "len block's extra", len(block.IHeader().GetExtra()), "config's limit", v.quarkChainConfig.BlockExtraDataSizeLimit)
		return ErrExtraLimit
	}

	if block.Header().GasLimit.Value.Cmp(v.bc.gasLimit) != 0 {
		return fmt.Errorf("incorrect gas limit, expected %d, actual %d", v.bc.gasLimit.Uint64(),
			block.Header().GasLimit.Value.Uint64())
	}

	if block.Meta().XShardGasLimit.Value.Cmp(block.Header().GasLimit.Value) >= 0 {
		return fmt.Errorf("xshard_gas_limit %d should not exceed total gas_limit %d",
			block.Meta().XShardGasLimit.Value, block.Header().GasLimit.Value)
	}

	if block.Meta().XShardGasLimit.Value.Cmp(v.bc.xShardGasLimit) != 0 {
		return fmt.Errorf("incorrect xshard gas limit, expected %d, actual %d", v.bc.xShardGasLimit,
			block.Meta().XShardGasLimit.Value)
	}

	txHash := types.CalculateMerkleRoot(block.GetTransactions())
	if txHash != block.GetMetaData().TxHash {
		log.Error(v.logInfo, "txHash is not match err", ErrTxHash, "local", txHash.String(), "block.TxHash", block.GetMetaData().TxHash.String())
		return ErrTxHash
	}

	if !v.branch.IsInBranch(block.IHeader().GetCoinbase().FullShardKey) {
		log.Error(v.logInfo, "err", ErrMinerFullShardKey, "coinbase's fullshardkey", block.IHeader().GetCoinbase().FullShardKey, "current branch", v.branch.Value)
		return ErrMinerFullShardKey
	}

	if !v.quarkChainConfig.SkipMinorDifficultyCheck {
		diff, err := v.engine.CalcDifficulty(v.bc, block.IHeader().GetTime(), prevHeader)
		if err != nil {
			log.Error(v.logInfo, "check diff err", err)
			return err
		}
		if diff.Cmp(block.IHeader().GetDifficulty()) != 0 {
			log.Error(v.logInfo, "check diff err", err, "diff", diff, "block's", block.IHeader().GetDifficulty())
			return ErrDifficulty
		}
	}

	rootBlockHeader := v.bc.getRootBlockHeaderByHash(block.Header().GetPrevRootBlockHash())
	if rootBlockHeader == nil {
		log.Error(v.logInfo, "err", ErrRootBlockIsNil, "height", block.Header().Number, "parentRootBlockHash", block.Header().GetPrevRootBlockHash().String())
		return ErrRootBlockIsNil
	}

	prevRootHeader := v.bc.getRootBlockHeaderByHash(prevHeader.(*types.MinorBlockHeader).GetPrevRootBlockHash())
	if prevRootHeader == nil {
		log.Error(v.logInfo, "err", ErrRootBlockIsNil, "prevHeader's height", prevHeader.NumberU64(), "preHeader's prevRootBlockHash", prevHeader.(*types.MinorBlockHeader).GetPrevRootBlockHash().String())
		return ErrRootBlockIsNil
	}
	if rootBlockHeader.NumberU64() < prevRootHeader.NumberU64() {
		errRootBlockOrder := errors.New("pre root block height must be non-decreasing")
		log.Error(v.logInfo, "err", errRootBlockOrder, "rootBlockHeader's number", rootBlockHeader.Number, "preRootHeader.Number", prevHeader.NumberU64())
		return errRootBlockOrder
	}

	prevConfirmedMinorHeader := v.bc.getLastConfirmedMinorBlockHeaderAtRootBlock(block.Header().PrevRootBlockHash)
	if prevConfirmedMinorHeader != nil && !isSameChain(v.bc.db, prevHeader, prevConfirmedMinorHeader) {
		errMustBeOneMinorChain := errors.New("prev root block's minor block is not in the same chain as the minor block")
		log.Error(v.logInfo, "err", errMustBeOneMinorChain, "prevConfirmedMinor's height", prevConfirmedMinorHeader.Number, "prevConfirmedMinor's hash", prevConfirmedMinorHeader.Hash().String(),
			"preHeader's height", prevHeader.NumberU64(), "preHeader's hash", prevHeader.Hash().String())
		return errMustBeOneMinorChain
	}

	if !v.bc.isSameRootChain(v.bc.getRootBlockHeaderByHash(block.Header().GetPrevRootBlockHash()),
		v.bc.getRootBlockHeaderByHash(prevHeader.(*types.MinorBlockHeader).GetPrevRootBlockHash())) {
		errMustBeOneRootChain := errors.New("prev root blocks are not on the same chain")
		log.Error(v.logInfo, "err", errMustBeOneRootChain, "long", block.Header().GetPrevRootBlockHash().String(), "short", prevHeader.(*types.MinorBlockHeader).GetPrevRootBlockHash().String())
		return errMustBeOneRootChain
	}
	if !v.bc.clusterConfig.Quarkchain.DisablePowCheck {
		if err := v.ValidateSeal(block.Header(), true); err != nil {
			log.Error(v.logInfo, "ValidatorBlockSeal err", err)
			return err
		}
	}
	return nil
}

// ValidatorBlockSeal validate minor block seal when validate block
func (v *MinorBlockValidator) ValidateSeal(mHeader types.IHeader, usePowsDiff bool) error {
	fmt.Println("ValidateSeal", mHeader.NumberU64(), mHeader.GetDifficulty(), usePowsDiff)
	header, ok := mHeader.(*types.MinorBlockHeader)
	if !ok {
		return errors.New("validator minor  seal failed , mBlock is nil")
	}
	if header.NumberU64() == 0 {
		return nil
	}
	adjustedDiff := new(big.Int).Set(header.Difficulty)
	var err error
	if usePowsDiff {
		adjustedDiff, err = v.bc.GetAdjustedDifficulty(header)
		if err != nil {
			return err
		}
	} else {
		shardConfig := v.bc.shardConfig.PoswConfig
		if shardConfig.Enabled {
			adjustedDiff = header.Difficulty.Div(header.Difficulty, new(big.Int).SetUint64(shardConfig.DiffDivider))
		}
	}
	return v.engine.VerifySeal(v.bc, header, adjustedDiff)
}
func compareXshardTxCursor(a, b *types.XShardTxCursorInfo) bool {
	if a == nil || b == nil {
		return false
	}
	if a.XShardDepositIndex != b.XShardDepositIndex {
		return false
	}
	if a.MinorBlockIndex != b.MinorBlockIndex {
		return false
	}
	if a.RootBlockHeight != b.RootBlockHeight {
		return false
	}
	return true
}

func compareCoinbaseAmountMap(a, b map[uint64]*big.Int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		data, ok := b[k]
		if !ok {
			return false
		}
		if data.Cmp(v) != 0 {
			return false
		}
	}
	return true
}

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func (v *MinorBlockValidator) ValidateState(mBlock, parent types.IBlock, statedb *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	if common.IsNil(mBlock) {
		return ErrMinorBlockIsNil
	}
	block, ok := mBlock.(*types.MinorBlock)
	if !ok {
		return ErrInvalidMinorBlock
	}
	mHeader := block.Header()
	if block.GetMetaData().GasUsed.Value.Cmp(statedb.GetGasUsed()) != 0 {
		return fmt.Errorf("invalid gas used (statedb.GetGasUsed: %d usedGas: %d)", block.GetMetaData().GasUsed.Value.Uint64(), statedb.GetGasUsed())
	}
	// Validate the received block's bloom with the one derived from the generated receipts.
	// For valid blocks this should always validate to true.
	bloom := types.CreateBloom(receipts)
	if bloom != mHeader.GetBloom() {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", mHeader.GetBloom(), bloom)
	}

	if !compareXshardTxCursor(statedb.GetTxCursorInfo(), block.Meta().XShardTxCursorInfo) {
		return fmt.Errorf("cross-hard transaction cursor info mismatches! %v %v", statedb.GetTxCursorInfo(), block.Meta().XShardTxCursorInfo)
	}

	receiptSha := types.DeriveSha(receipts)
	fmt.Println("=======height", block.Header().Number)
	for _, tx := range block.Transactions() {
		fmt.Println("txHash", tx.Hash().String())
	}
	fmt.Println("receipte len", len(receipts))
	for _, re := range receipts {
		fmt.Println("root", re.Status)
		fmt.Println("bloom", re.Bloom.Big().String())
		fmt.Println("logs", len(re.Logs))
		fmt.Println("contract_addrsss", re.ContractAddress.String())
		fmt.Println("full_shard_key", re.ContractFullShardKey)
	}
	fmt.Println("========end", block.Header().Number)
	if receiptSha != block.GetMetaData().ReceiptHash {
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", block.GetMetaData().ReceiptHash, receiptSha)
	}
	if statedb.GetGasUsed().Cmp(block.GetMetaData().GasUsed.Value) != 0 {
		return ErrGasUsed
	}
	coinbaseAmount := v.bc.getCoinbaseAmount(block.Header().Number)
	coinbaseAmount.Add(statedb.GetBlockFee())
	if !compareCoinbaseAmountMap(coinbaseAmount.GetBalanceMap(), block.Header().CoinbaseAmount.GetBalanceMap()) {
		return ErrCoinbaseAmount
	}

	if statedb.GetXShardReceiveGasUsed().Cmp(block.GetMetaData().CrossShardGasUsed.Value) != 0 {
		return ErrXShardList
	}
	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if root := statedb.IntermediateRoot(true); block.GetMetaData().Root != root {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", block.GetMetaData().Root, root)
	}
	return nil
}
