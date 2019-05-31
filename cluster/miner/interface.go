package miner

import "github.com/QuarkChain/goquarkchain/core/types"

type MinerAPI interface {
	CreateBlockToMine() (types.IBlock, error)
	InsertMinedBlock(types.IBlock) error
}
