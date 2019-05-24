package miner

import "github.com/QuarkChain/goquarkchain/core/types"

type MinerAPI interface {
	CreateBlockAsyncFunc() (types.IBlock, error)
	AddBlockAsyncFunc(types.IBlock) error
}
