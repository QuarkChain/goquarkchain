package miner

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

type MinerAPI interface {
	GetDefaultCoinbaseAddress() account.Address
	CreateBlockToMine(addr *account.Address) (types.IBlock, *big.Int, uint64, error)
	InsertMinedBlock(types.IBlock) error
	IsSyncing() bool
	GetTip() uint64
}
