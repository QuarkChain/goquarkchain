package miner

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

type MinerAPI interface {
	CreateBlockToMine() (types.IBlock, *big.Int, error)
	InsertMinedBlock(types.IBlock) error
	IsSyncIng() bool
	GetTip() uint64
}
