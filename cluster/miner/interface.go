package miner

import (
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
)

type MinerAPI interface {
	GetDefaultCoinbaseAddress() account.Address
	CreateBlockToMine(addr *account.Address) (types.IBlock, *big.Int, uint64, error)
	InsertMinedBlock(types.IBlock) error
	IsSyncing() bool
	IsRemoteMining() bool
	GetTip() uint64
}
