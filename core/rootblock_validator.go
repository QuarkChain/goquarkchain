package core

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/ethereum/go-ethereum/common"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
)

// RootBlockValidator implements Validator.
type RootBlockValidator struct {
	config     *config.QuarkChainConfig // config configuration options
	blockChain *RootBlockChain          // root block chain
	engine     consensus.Engine         // engine engine used for validating
}

// NewRootBlockValidator returns a new root block validator which is safe for re-use
func NewRootBlockValidator(config *config.QuarkChainConfig, blockchain *RootBlockChain, engine consensus.Engine) *RootBlockValidator {
	validator := &RootBlockValidator{
		config:     config,
		engine:     engine,
		blockChain: blockchain,
	}
	return validator
}

// ValidateBlock validates the given block and verifies the block header's roots.
func (v *RootBlockValidator) ValidateBlock(block types.IBlock) error {
	// Check whether the block's known, and if not, that it's linkable
	if block == nil {
		panic("input block for ValidateBlock is nil")
	}
	if reflect.TypeOf(block) != reflect.TypeOf(new(types.RootBlock)) {
		panic("invalid type of block")
	}

	rootBlock := block.(*types.RootBlock)

	if rootBlock.NumberU64() < 1 {
		return errors.New("unexpected height")
	}
	if v.blockChain.HasBlock(block.Hash()) {
		return ErrKnownBlock
	}
	// Header validity is known at this point, check the uncles and transactions
	header := rootBlock.Header()

	if err := v.engine.VerifyHeader(v.blockChain, header, true); err != nil {
		return err
	}

	if uint32(len(rootBlock.TrackingData())) > v.config.BlockExtraDataSizeLimit {
		return errors.New("tracking data in block is too large")
	}

	mheaderHash := types.DeriveSha(rootBlock.MinorBlockHeaders())
	if mheaderHash != rootBlock.Header().MinorHeaderHash {
		return fmt.Errorf("incorrect merkle root %v - %v ",
			rootBlock.Header().MinorHeaderHash.String(),
			mheaderHash.String())
	}

	if !v.config.SkipRootCoinbaseCheck {
		coinbaseAmount := CalculateRootBlockCoinbase(v.config, rootBlock)
		if coinbaseAmount.Cmp(rootBlock.CoinbaseAmount()) != 0 {
			return fmt.Errorf("bad coinbase amount for root block %v. expect %d but got %d.",
				rootBlock.Hash().String(),
				coinbaseAmount,
				rootBlock.CoinbaseAmount())
		}
	}

	var fullShardId uint32 = 0
	var parentHeader *types.MinorBlockHeader
	var prevRootBlockHashList map[common.Hash]bool
	var shardIdToMinorHeadersMap = make(map[uint32][]*types.MinorBlockHeader)
	for _, mheader := range rootBlock.MinorBlockHeaders() {
		if !v.blockChain.containMinorBlock(mheader.Hash()) {
			return fmt.Errorf("minor block is not validated. %v-%d",
				mheader.Coinbase.FullShardKey, mheader.Number)
		}
		if mheader.Time > rootBlock.Header().Time {
			return fmt.Errorf("minor block create time is larger than root block %d-%d",
				mheader.Time, mheader.Time)
		}
		if mheader.Branch.GetFullShardID() < fullShardId {
			return errors.New("shard id must be ordered")
		}
		if mheader.Branch.GetFullShardID() > fullShardId {
			fullShardId = mheader.Branch.GetFullShardID()
			shardIdToMinorHeadersMap[fullShardId] = make([]*types.MinorBlockHeader, 0, rootBlock.MinorBlockHeaders().Len())
		} else if mheader.Number != parentHeader.Number+1 {
			return fmt.Errorf("mheader.Number must equal to prev header + 1, header number %d, prev number %d", mheader.Number, parentHeader.Number)
		} else if mheader.ParentHash != parentHeader.Hash() {
			return fmt.Errorf("minor block %v does not link to previous block %v",
				mheader.Hash().String(), parentHeader.Hash().String())
		}

		prevRootBlockHashList[mheader.PrevRootBlockHash] = true
		parentHeader = mheader
		shardIdToMinorHeadersMap[fullShardId] = append(shardIdToMinorHeadersMap[fullShardId], mheader)
	}

	for key, _ := range prevRootBlockHashList {
		if v.blockChain.isSameChain(rootBlock.Header(), v.blockChain.GetHeader(key).(*types.RootBlockHeader)) {
			return errors.New("minor block's prev root block must be in the same chain")
		}
	}

	if rootBlock.Header().Number < 1 {
		return nil
	}

	latestMinorBlockHeaders := v.blockChain.GetLatestMinorBlockHeaders(header.ParentHash)
	fullShardIdList := v.config.GetInitializedShardIdsBeforeRootHeight(header.Number)
	for fullShardId, minorHeaders := range shardIdToMinorHeadersMap {
		if uint32(len(minorHeaders)) > v.config.GetShardConfigByFullShardID(fullShardId).MaxBlocksPerShardInOneRootBlock() {
			return fmt.Errorf("too many minor blocks in the root block for shard %d", fullShardId)
		}
		inList := false
		for _, id := range fullShardIdList {
			if id == fullShardId {
				inList = true
				break
			}
		}
		if !inList {
			return fmt.Errorf("found minor block header in root block %v for uninitialized shard %d",
				block.Hash().String(), fullShardId)
		}

		prevHeader, ok := latestMinorBlockHeaders[fullShardId]
		if !ok && minorHeaders[0].Number != 0 {
			//todo double check when adding chain
			//todo reshard will not be 0, this check will be wrong
			return fmt.Errorf("genesis block height is not 0 for shard %d", fullShardId)
		}

		if prevHeader != nil && (prevHeader.Number+1 != minorHeaders[0].Number || prevHeader.Hash() != minorHeaders[0].ParentHash) {
			return fmt.Errorf("minor block %v does not link to previous block %v",
				minorHeaders[0].Hash().String(), prevHeader.Hash().String())
		}

		latestMinorBlockHeaders[fullShardId] = minorHeaders[len(minorHeaders)-1]
	}

	v.blockChain.SetLatestMinorBlockHeaders(block.Hash(), latestMinorBlockHeaders)
	return nil
}

func (v *RootBlockValidator) ValidateState(block, parent types.IBlock, statedb *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	panic(errors.New("not implement"))
}

// ValidateHeader calls underlying engine's header verification method.
func (v *RootBlockValidator) ValidateHeader(header types.IHeader) error {
	return v.engine.VerifyHeader(v.blockChain, header, true)
}

type fakeRootBlockValidator struct {
	Err error
}

func (v *fakeRootBlockValidator) ValidateBlock(block types.IBlock) error {
	return v.Err
}

func (v *fakeRootBlockValidator) ValidateHeader(header types.IHeader) error {
	return v.Err
}

func (v *fakeRootBlockValidator) ValidateState(block, parent types.IBlock, statedb *state.StateDB, receipts types.Receipts, usedGas uint64) error {
	panic(errors.New("not implement"))
}
