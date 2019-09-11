package miner

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

type MinerAPI interface {
	GetDefaultCoinbaseAddress() account.Address
	CreateBlockToMine(addr *account.Address) (types.IBlock, *big.Int, error)
	InsertMinedBlock(types.IBlock) error
	IsSyncIng() bool
	GetTip() uint64
}
