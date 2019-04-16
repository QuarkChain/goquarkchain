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
	"math/big"
	"reflect"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// MinorBlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// MinorBlockValidator implements Validator.
type MinorBlockValidator struct {
	config               *config.QuarkChainConfig // Chain configuration options
	versionControlConfig *params.ChainConfig
	bc                   *MinorBlockChain // Canonical block chain
	engine               consensus.Engine // Consensus engine used for validating
	branch               account.Branch
	diffCalc             consensus.DifficultyCalculator
	shardConfig          *config.ShardConfig
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewBlockValidator(config *config.QuarkChainConfig, versionConfig *params.ChainConfig, blockchain *MinorBlockChain, engine consensus.Engine, branch account.Branch, diffCalc consensus.DifficultyCalculator, shardConfig *config.ShardConfig) *MinorBlockValidator {
	validator := &MinorBlockValidator{
		config:               config,
		versionControlConfig: versionConfig,
		engine:               engine,
		bc:                   blockchain,
		branch:               branch,
		diffCalc:             diffCalc,
		shardConfig:          shardConfig,
	}
	return validator
}

// ValidateBlock validates the given block's uncles and verifies the block
// header's transaction and uncle roots. The headers are assumed to be already
// validated at this point.
func (v *MinorBlockValidator) ValidateBlock(mBlock types.IBlock) error {
	if common.IsNil(mBlock) {
		return ErrBlockIsNil
	}
	block := mBlock.(*types.MinorBlock)
	if reflect.TypeOf(block) != reflect.TypeOf(new(types.MinorBlock)) {
		return ErrInvalidMinorBlock
	}

	blockHeight := block.NumberU64()
	if blockHeight < 1 {
		return errors.New("block.Number <1")
	}
	// Check whether the block's known, and if not, that it's linkable
	if v.bc.HasBlockAndState(block.Hash()) {
		return ErrKnownBlock
	}

	if !v.bc.HasBlockAndState(block.IHeader().GetParentHash()) {
		return ErrPreBlockNotFound
	}

	prevHeader := v.bc.GetHeader(block.IHeader().GetParentHash()).(*types.MinorBlockHeader)
	if blockHeight != prevHeader.NumberU64()+1 {
		return ErrHeightDisMatch
	}

	if block.Branch().Value != v.branch.Value {
		return ErrBranch
	}

	if block.IHeader().GetTime() <= prevHeader.GetTime() {
		return ErrTime
	}

	if block.Header().MetaHash != block.GetMetaData().Hash() {
		return ErrMetaHash
	}

	if len(block.IHeader().GetExtra()) > int(v.config.BlockExtraDataSizeLimit) {
		return ErrExtraLimit
	}

	if len(block.GetTrackingData()) > int(v.config.BlockExtraDataSizeLimit) {
		return ErrTrackLimit
	}

	if err := v.ValidateGasLimit(block.Header().GetGasLimit().Uint64(), prevHeader.GetGasLimit().Uint64()); err != nil {
		return err
	}

	merkleHash := types.DeriveSha(block.GetTransactions())
	if merkleHash != block.GetMetaData().TxHash {
		return ErrRootHash
	}

	if !v.branch.IsInBranch(block.IHeader().GetCoinbase().FullShardKey) {
		return ErrFullShardKey
	}

	if !v.config.SkipMinorDifficultyCheck {
		diff := v.diffCalc.CalculateDifficulty(prevHeader, block.IHeader().GetTime())
		if diff.Cmp(block.IHeader().GetDifficulty()) != 0 {
			return ErrDifficulty
		}
	}

	rootBlockHeader := v.bc.getRootBlockHeaderByHash(block.Header().GetPrevRootBlockHash())

	if rootBlockHeader == nil {
		return errors.New("get blockHeader errors")
	}

	prevPrevHeader := v.bc.getRootBlockHeaderByHash(prevHeader.GetPrevRootBlockHash())
	if prevPrevHeader == nil {
		return errors.New("get prePreHeader error")
	}
	if rootBlockHeader.NumberU64() < prevPrevHeader.NumberU64() {
		return errors.New("pre root block height must be non-decreasing")
	}

	prevConfirmedMinorHeader := v.bc.getLastConfirmedMinorBlockHeaderAtRootBlock(block.IHeader().GetParentHash())
	if prevConfirmedMinorHeader != nil && !v.bc.isSameMinorChain(prevHeader, prevConfirmedMinorHeader) {
		return errors.New("prev root block's minor block is not in the same chain as the minot block")
	}

	if !v.bc.isSameRootChain(v.bc.getRootBlockHeaderByHash(block.Header().GetPrevRootBlockHash()),
		v.bc.getRootBlockHeaderByHash(prevHeader.GetPrevRootBlockHash())) {
		return errors.New("prev root blocks are not on the same chain")
	}
	return v.ValidatorMinorBlockSeal(block)
}

// ValidateGasLimit validate gasLimit when validateBlock
func (v *MinorBlockValidator) ValidateGasLimit(gasLimit, preGasLimit uint64) error {
	computeGasLimitBounds := func(parentGasLimit uint64) (uint64, uint64) {
		boundaryRange := parentGasLimit / uint64(v.shardConfig.GasLimitAdjustmentFactor)
		upperBound := parentGasLimit + boundaryRange
		lowBound := v.shardConfig.GasLimitMinimum
		if lowBound < parentGasLimit-boundaryRange {
			lowBound = parentGasLimit - boundaryRange
		}
		return lowBound, upperBound
	}
	lowBound, upperBound := computeGasLimitBounds(preGasLimit)
	if gasLimit < lowBound {
		return errors.New("gaslimit < lowBound")
	} else if gasLimit > upperBound {
		return errors.New("gasLimit>upperBound")
	}
	return nil
}

// ValidatorMinorBlockSeal validate minor block seal when validate block
func (v *MinorBlockValidator) ValidatorMinorBlockSeal(mBlock types.IBlock) error {
	block := mBlock.(*types.MinorBlock)
	branch := block.Header().GetBranch()
	fullShardID := (&branch).GetFullShardID()
	genesis := v.config.GetShardConfigByFullShardID(fullShardID)
	consensusType := genesis.ConsensusType
	if !genesis.PoswConfig.Enabled {
		return validateSeal(block.IHeader(), consensusType, nil)
	}
	diff, err := v.bc.POSWDiffAdjust(block)
	if err != nil {
		return err
	}
	return validateSeal(block.IHeader(), consensusType, &diff)
}

func validateSeal(block types.IHeader, consensusType string, diff *uint64) error {
	return nil
}

// ValidateState validates the various changes that happen after a state
// transition, such as amount of used gas, the receipt roots and the state root
// itself. ValidateState returns a database batch if the validation was a success
// otherwise nil and an error is returned.
func (v *MinorBlockValidator) ValidateState(mBlock, parent types.IBlock, statedb *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	if common.IsNil(mBlock) {
		return ErrBlockIsNil
	}
	block := mBlock.(*types.MinorBlock)
	if reflect.TypeOf(block) != reflect.TypeOf(new(types.MinorBlock)) {
		return ErrInvalidMinorBlock
	}
	mHeader := block.Header()
	if mHeader == nil {
		return errors.New("header is nil")
	}
	if block.GetMetaData().GasUsed.Value.Cmp(statedb.GetGasUsed()) != 0 {
		return fmt.Errorf("invalid gas used (statedb.GetGasUsed: %d usedGas: %d)", block.GetMetaData().GasUsed.Value.Uint64(), statedb.GetGasUsed())
	}
	// Validate the received block's bloom with the one derived from the generated receipts.
	// For valid blocks this should always validate to true.
	rbloom := types.CreateBloom(receipts)
	if rbloom != mHeader.GetBloom() {
		return fmt.Errorf("invalid bloom (remote: %x  local: %x)", mHeader.GetBloom(), rbloom)
	}

	receiptSha := types.DeriveSha(receipts)
	if receiptSha != block.GetMetaData().ReceiptHash {
		return fmt.Errorf("invalid receipt root hash (remote: %x local: %x)", block.GetMetaData().ReceiptHash, receiptSha)
	}
	if statedb.GetGasUsed().Cmp(block.GetMetaData().GasUsed.Value) != 0 {
		return ErrGasUsed
	}
	coinBaseAmount := new(big.Int).Add(v.bc.getCoinBaseAmount(), statedb.GetBlockFee())
	if coinBaseAmount.Cmp(block.CoinbaseAmount()) != 0 {
		return ErrCoinBaseAmount
	}

	if statedb.GetXShardReceiveGasUsed().Cmp(block.GetMetaData().CrossShardGasUsed.Value) != 0 {
		return ErrXShardList
	}
	// Validate the state root against the received state root and throw
	// an error if they don't match.
	if root := statedb.IntermediateRoot(false); block.GetMetaData().Root != root {
		return fmt.Errorf("invalid merkle root (remote: %x local: %x)", block.GetMetaData().Root, root)
	}
	return nil
}
