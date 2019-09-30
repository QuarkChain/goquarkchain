// Modified from go-ethereum under GNU Lesser General Public License

package core

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*types.Transaction }

// PendingLogsEvent is posted pre mining and notifies of pending logs.
type PendingLogsEvent struct {
	Logs []*types.Log
}

// NewMinedBlockEvent is posted when a block has been imported.
type NewMinedBlockEvent struct{ Block types.IBlock }

// RemovedLogsEvent is posted when a reorg happens
type RemovedLogsEvent struct{ Logs []*types.Log }

type MinorChainEvent struct {
	Block *types.MinorBlock
	Hash  common.Hash
	Logs  []*types.Log
}

type MinorChainSideEvent struct {
	Block types.IBlock
}

type MinorChainHeadEvent struct{ Block *types.MinorBlock }

type RootChainEvent struct {
	Block *types.RootBlock
	Hash  common.Hash
}

type RootChainSideEvent struct {
	Block *types.RootBlock
}

type RootChainHeadEvent struct{ Block *types.RootBlock }

type LoglistEvent struct {
	Logs      []*types.Log
	IsRemoved bool
}
