package core

import (
	"errors"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
)

type blockchain interface {
	GetGenesisToken() uint64
	GetBranch() account.Branch
	GetGenesisRootHeight() uint32
	GetRootBlockByHash(hash common.Hash) *types.RootBlock
	GetRootBlockByHeight(h common.Hash, height uint64) *types.RootBlock
	ReadCrossShardTxList(hash common.Hash) *types.CrossShardTransactionDepositList
	isNeighbor(remoteBranch account.Branch, rootHeight *uint32) bool
}

type XShardTxCursor struct {
	bc                 blockchain
	minorBlock         *types.MinorBlock
	maxRootBlockHeader *types.RootBlock
	mBlockIndex        uint64
	xShardDepositIndex uint64
	xTxList            *types.CrossShardTransactionDepositList
	rBlock             *types.RootBlock
}

/*
   # Cursor definitions (root_block_height, mblock_index, deposit_index)
   # (x, 0, 0): EOF
   # (x, 0, z), z > 0: Root-block coinbase tx (always exist)
   # (x, y, z), y > 0: Minor-block x-shard tx (may not exist if not neighbor or no xshard)
   #
   # Note that: the cursor must be
   # - EOF
   # - A valid x-shard transaction deposit
*/
func NewXShardTxCursor(bc blockchain, minorBlock *types.MinorBlock, cursorInfo *types.XShardTxCursorInfo) *XShardTxCursor {
	c := &XShardTxCursor{
		bc:         bc,
		minorBlock: minorBlock,
	}
	// Recover cursor
	c.maxRootBlockHeader = bc.GetRootBlockByHash(minorBlock.PrevRootBlockHash())
	rBlockHeader := bc.GetRootBlockByHeight(minorBlock.PrevRootBlockHash(), cursorInfo.RootBlockHeight)
	c.mBlockIndex = cursorInfo.MinorBlockIndex
	c.xShardDepositIndex = cursorInfo.XShardDepositIndex
	// Recover rblock and xtx_list if it is processing tx from peer-shard
	c.xTxList = new(types.CrossShardTransactionDepositList)
	if rBlockHeader != nil {
		c.rBlock = bc.GetRootBlockByHash(rBlockHeader.Hash())
		if c.mBlockIndex != 0 {
			c.xTxList = bc.ReadCrossShardTxList(c.rBlock.MinorBlockHeaders()[c.mBlockIndex-1].Hash())
		}
	} else {
		// EOF
		c.rBlock = nil
	}
	return c
}

func (x *XShardTxCursor) getCurrentTx() (*types.CrossShardTransactionDeposit, error) {
	if x.mBlockIndex == 0 {
		// 0 is reserved for EOF
		if x.xShardDepositIndex != 1 && x.xShardDepositIndex != 2 {
			return nil, errors.New("shardDepositIndex should be 1 or 2")
		}
		if x.xShardDepositIndex == 1 {
			branch := x.bc.GetBranch()
			coinbaseAmount := new(big.Int)
			if branch.IsInBranch(x.rBlock.Coinbase().FullShardKey) {
				coinbaseAmount = x.rBlock.CoinbaseAmount().GetTokenBalance(x.bc.GetGenesisToken())
			}
			genesisToken := x.bc.GetGenesisToken()
			// Perform x-shard from root chain coinbase
			return &types.CrossShardTransactionDeposit{
				TxHash:          x.rBlock.Hash(),
				From:            x.rBlock.Coinbase(),
				To:              x.rBlock.Coinbase(),
				Value:           &serialize.Uint256{Value: new(big.Int).Set(coinbaseAmount)},
				GasPrice:        &serialize.Uint256{Value: new(big.Int)},
				GasRemained:     &serialize.Uint256{Value: new(big.Int)},
				GasTokenID:      genesisToken,
				TransferTokenID: genesisToken,
				IsFromRootChain: true,
			}, nil
		}

		return nil, nil
	} else if x.xShardDepositIndex < uint64(len(x.xTxList.TXList)) {
		return x.xTxList.TXList[x.xShardDepositIndex], nil
	} else {
		return nil, nil
	}
}

func (x *XShardTxCursor) getNextTx() (*types.CrossShardTransactionDeposit, error) {
	// Check if reach EOF
	if x.rBlock == nil {
		return nil, nil
	}
	x.xShardDepositIndex += 1
	tx, err := x.getCurrentTx()
	if err != nil {
		return nil, err
	}
	// Reach the EOF of the mblock or rblock x-shard txs
	if tx != nil {
		return tx, nil
	}
	x.mBlockIndex += 1
	x.xShardDepositIndex = 0

	// Iterate minor blocks' cross-shard transactions
	for x.mBlockIndex <= uint64(len(x.rBlock.MinorBlockHeaders())) {
		// If it is not neighbor, move to next minor block
		mBlockHeader := x.rBlock.MinorBlockHeaders()[x.mBlockIndex-1]
		t := x.rBlock.Number()
		if mBlockHeader.Branch == x.bc.GetBranch() || !x.bc.isNeighbor(mBlockHeader.Branch, &t) {
			if x.xShardDepositIndex != 0 {
				return nil, errors.New("xShardDepositIndex should 0")
			}
			x.mBlockIndex += 1
			continue
		}
		// Check if the neighbor has the permission to send tx to local shard
		prevRootHeader := x.bc.GetRootBlockByHash(mBlockHeader.PrevRootBlockHash)
		if prevRootHeader.Number() <= x.bc.GetGenesisRootHeight() {
			if x.xShardDepositIndex != 0 {
				return nil, errors.New("should 0")
			}
			if x.bc.ReadCrossShardTxList(mBlockHeader.Hash()) != nil {
				return nil, errors.New("should nil")
			}
			x.mBlockIndex += 1
			continue
		}
		x.xTxList = x.bc.ReadCrossShardTxList(mBlockHeader.Hash())
		tx, err := x.getCurrentTx()
		if err != nil {
			return nil, err
		}
		if tx != nil {
			return tx, nil
		}
		// Move to next minor block
		if x.xShardDepositIndex != 0 {
			return nil, errors.New("should 0")
		}
		x.mBlockIndex += 1
	}

	// Move to next root block
	rBlockHeader := x.bc.GetRootBlockByHeight(x.maxRootBlockHeader.Hash(), x.rBlock.NumberU64()+1)
	if rBlockHeader == nil {
		// EOF
		x.rBlock = nil
		x.mBlockIndex = 0
		x.xShardDepositIndex = 0
		return nil, nil
	}
	// Root-block coinbase (always exist)
	x.rBlock = x.bc.GetRootBlockByHash(rBlockHeader.Hash())
	x.mBlockIndex = 0
	x.xShardDepositIndex = 1
	return x.getCurrentTx()
}

func (x *XShardTxCursor) getCursorInfo() *types.XShardTxCursorInfo {
	rootBlockHeight := x.maxRootBlockHeader.NumberU64() + 1
	if x.rBlock != nil {
		rootBlockHeight = x.rBlock.NumberU64()
	}
	return &types.XShardTxCursorInfo{
		RootBlockHeight:    rootBlockHeight,
		MinorBlockIndex:    x.mBlockIndex,
		XShardDepositIndex: x.xShardDepositIndex,
	}
}
