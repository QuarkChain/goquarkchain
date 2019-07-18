package core

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type blockchain interface {
	GetGenesisToken() uint64
	GetBranch() account.Branch
	GetGenesisRootHeight() uint32
	GetRootBlockByHash(hash common.Hash) *types.RootBlock
	GetRootBlockHeaderByHeight(h common.Hash, height uint64) *types.RootBlockHeader
	ReadCrossShardTxList(hash common.Hash) *types.CrossShardTransactionDepositList
	isNeighbor(remoteBranch account.Branch, rootHeight *uint32) bool
}

type XShardTxCursor struct {
	bc                 blockchain
	mBlockHeader       *types.MinorBlockHeader
	maxRootBlockHeader *types.RootBlockHeader
	mBlockIndex        uint64
	xShardDepositIndex uint64
	xTxList            *types.CrossShardTransactionDepositList
	rBlock             *types.RootBlock
}

func NewXShardTxCursor(bc blockchain, mBlockHeader *types.MinorBlockHeader, cursorInfo *types.XShardTxCursorInfo) *XShardTxCursor {
	c := &XShardTxCursor{
		bc:           bc,
		mBlockHeader: mBlockHeader,
	}
	c.maxRootBlockHeader = bc.GetRootBlockByHash(mBlockHeader.PrevRootBlockHash).Header()
	rBlockHeader := bc.GetRootBlockHeaderByHeight(mBlockHeader.PrevRootBlockHash, cursorInfo.RootBlockHeight)
	c.mBlockIndex = cursorInfo.MinorBlockIndex
	c.xShardDepositIndex = cursorInfo.XShardDepositIndex
	c.xTxList = new(types.CrossShardTransactionDepositList)
	if rBlockHeader != nil {
		c.rBlock = bc.GetRootBlockByHash(rBlockHeader.Hash())
		if c.mBlockIndex != 0 {
			c.xTxList = bc.ReadCrossShardTxList(c.rBlock.MinorBlockHeaders()[c.mBlockIndex-1].Hash())
		}
	} else {
		c.rBlock = nil
	}
	return c
}

func (x *XShardTxCursor) getCurrentTx() (*types.CrossShardTransactionDeposit, error) {
	if x.mBlockIndex == 0 {
		if x.xShardDepositIndex != 1 && x.xShardDepositIndex != 2 {
			return nil, errors.New("shardDepositIndex should 1 or 2")
		}
		branch := x.bc.GetBranch()
		if branch.IsInBranch(x.rBlock.Header().Coinbase.FullShardKey) {
			coinbaseAmount := new(big.Int)
			if data, ok := x.rBlock.Header().CoinbaseAmount.BalanceMap[x.bc.GetGenesisToken()]; ok {
				coinbaseAmount = new(big.Int).Set(data)
			}
			return &types.CrossShardTransactionDeposit{
				TxHash:          x.rBlock.Header().Hash(),
				From:            account.CreatEmptyAddress(0),
				To:              x.rBlock.Header().Coinbase,
				Value:           &serialize.Uint256{Value: new(big.Int).Set(coinbaseAmount)},
				GasPrice:        &serialize.Uint256{Value: new(big.Int)},
				GasTokenID:      x.bc.GetGenesisToken(),
				TransferTokenID: x.bc.GetGenesisToken(),
				IsFromRootChain: true,
			}, nil
		}
	} else if x.xShardDepositIndex < uint64(len(x.xTxList.TXList)) {
		return x.xTxList.TXList[x.xShardDepositIndex], nil
	}
	return nil, errors.New("no tx yet")
}

func (x *XShardTxCursor) getNextTx() (*types.CrossShardTransactionDeposit, error) {
	if x.rBlock == nil {
		return nil, nil
	}
	x.xShardDepositIndex += 1
	tx, err := x.getCurrentTx()
	if err != nil {
		return nil, err
	}
	if tx != nil {
		return tx, nil
	}
	x.mBlockIndex += 1
	x.xShardDepositIndex = 0

	for x.mBlockIndex <= uint64(len(x.rBlock.MinorBlockHeaders())) {
		mBlockHeader := x.rBlock.MinorBlockHeaders()[x.mBlockIndex-1]
		if !x.bc.isNeighbor(mBlockHeader.Branch, &x.rBlock.Header().Number) || mBlockHeader.Branch == x.bc.GetBranch() {
			if x.xShardDepositIndex != 0 {
				return nil, errors.New("xShardDepositIndex should 0")
			}
			x.mBlockIndex += 1
		}

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
		if x.xShardDepositIndex != 0 {
			return nil, errors.New("should 0")
		}
		x.mBlockIndex += 1
	}
	rBlockHeader := x.bc.GetRootBlockHeaderByHeight(x.maxRootBlockHeader.Hash(), x.rBlock.Header().NumberU64()+1)
	if rBlockHeader == nil {
		x.rBlock = nil
		x.mBlockIndex = 0
		x.xShardDepositIndex = 0
		return nil, nil
	}
	x.rBlock = x.bc.GetRootBlockByHash(rBlockHeader.Hash())
	x.mBlockIndex = 0
	x.xShardDepositIndex = 1
	return x.getCurrentTx()
}

func (x *XShardTxCursor) getCursorInfo() *types.XShardTxCursorInfo {
	rootBlockHeight := x.maxRootBlockHeader.NumberU64() + 1
	if x.rBlock != nil {
		rootBlockHeight = x.rBlock.Header().NumberU64()
	}
	return &types.XShardTxCursorInfo{
		RootBlockHeight:    rootBlockHeight,
		MinorBlockIndex:    x.mBlockIndex,
		XShardDepositIndex: x.xShardDepositIndex,
	}
}
