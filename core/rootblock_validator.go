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
	"fmt"
	"github.com/syndtr/goleveldb/leveldb/errors"
	"reflect"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
)

// BlockValidator is responsible for validating block headers, uncles and
// processed state.
//
// BlockValidator implements Validator.
type RootBlockValidator struct {
	config *config.QuarkChainConfig // Chain configuration options
	bc     *RootBlockChain          // Canonical block chain
	engine consensus.Engine         // Consensus engine used for validating
}

// NewBlockValidator returns a new block validator which is safe for re-use
func NewRootBlockValidator(config *config.QuarkChainConfig, blockchain *RootBlockChain, engine consensus.Engine) *RootBlockValidator {
	validator := &RootBlockValidator{
		config: config,
		engine: engine,
		bc:     blockchain,
	}
	return validator
}

// ValidateBody validates the given block and verifies the block
// header's transaction roots. The headers are assumed to be already
// validated at this point.
func (v *RootBlockValidator) ValidateBlock(block types.IBlock) error {
	// Check whether the block's known, and if not, that it's linkable
	if block == nil {
		return errors.New("input block for ValidateBlock is nil")
	}
	if reflect.TypeOf(block) != reflect.TypeOf(new(types.RootBlock)) {
		return errors.New("invalid type of block")
	}

	rootBlock := block.(*types.RootBlock)

	if rootBlock.NumberU64() < 1 {
		return errors.New("unexpected height")
	}
	if v.bc.HasBlock(block.Hash()) {
		return ErrKnownBlock
	}
	// Header validity is known at this point, check the uncles and transactions
	header := rootBlock.Header()

	if err := v.engine.VerifyHeader(v.bc, header, true); err != nil {
		return err
	}

	if uint32(len(rootBlock.TrackingData())) > v.config.BlockExtraDataSizeLimit {
		return errors.New("tracking data in block is too large")
	}

	mheaderHash := types.DeriveSha(rootBlock.MinorBlockHeaders())
	if mheaderHash != rootBlock.Header().MinorHeaderHash {
		return fmt.Errorf("incorrect merkle root %v - %v ", rootBlock.Header().MinorHeaderHash, mheaderHash)
	}

	var fullShardId uint32 = 0
	var number uint64 = 0
	var shardIdToMinNumberMap = make(map[uint32]uint64)
	for _, mheader := range rootBlock.MinorBlockHeaders() {
		if !v.bc.HasHeader(mheader.Hash()) {
			return fmt.Errorf("minor block is not validated. %v-%d",
				mheader.Coinbase.FullShardKey, mheader.Number)
		}
		if mheader.Time > rootBlock.Header().Time {
			return fmt.Errorf("minor block create time is larger than root block %d-%d",
				mheader.Time, mheader.Time)
		}
		if mheader.PrevRootBlockHash != rootBlock.Header().ParentHash {
			return errors.New("minor block's prev root block must be in the same chain")
		}
		if mheader.Branch.GetFullShardID() < fullShardId {
			return errors.New("shard id must be ordered")
		} else if mheader.Branch.GetFullShardID() > fullShardId {
			fullShardId = mheader.Branch.GetFullShardID()
			number = 0
		} else if mheader.Number < number {
			return errors.New("mheader.Number must be ordered")
		} else {
			if number == 0 {
				shardIdToMinNumberMap[fullShardId] = mheader.Number
			}
			number = mheader.Number
		}
	}

	if rootBlock.Header().Number <= 1 {
		return nil
	}

	preBlock := v.bc.GetBlock(header.ParentHash)
	if preBlock == nil {
		return errors.New("parent block is missing")
	}
	fullShardId = 0
	number = 0 // max number in prev root block
	var numberToCheck uint64 = 0
	for _, mheader := range preBlock.(*types.RootBlock).MinorBlockHeaders() {
		// prev block minor block headers should be in order
		if mheader.Branch.GetFullShardID() > fullShardId && fullShardId != 0 {
			numberToCheck = shardIdToMinNumberMap[fullShardId]
			if numberToCheck != 0 && number != numberToCheck-1 {
				return fmt.Errorf("minor block header with number %d is missing in root block", number)
			}
		}
		if mheader.Branch.GetFullShardID() > fullShardId {
			fullShardId = mheader.Branch.GetFullShardID()
			number = 0
		} else {
			// minor block number in root block should be in order for same shard
			// get the max number in prev root block
			number = mheader.Number
		}
	}

	return nil
}

type FackRootBlockValidator struct {
	Err error
}

func (v *FackRootBlockValidator) ValidateBlock(block types.IBlock) error {
	return v.Err
}
