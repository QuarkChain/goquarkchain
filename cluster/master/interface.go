package master

import (
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/p2p"
)

type NetworkError struct {
	Msg string
}

func (e *NetworkError) Error() string {
	return e.Msg
}

type ShardConnForP2P interface {
	// AddTransactions will add the tx to shard tx pool, and return the tx hash
	// which have been added to tx pool. so tx which cannot pass verification
	// or existed in tx pool will not be included in return hash list
	AddTransactions(request *p2p.NewTransactionList) (*rpc.HashList, error)

	GetMinorBlockList(request *rpc.GetMinorBlockListRequest) (*rpc.GetMinorBlockListResponse, error)

	GetMinorBlockHeaderList(request *p2p.GetMinorBlockHeaderListRequest) (*p2p.GetMinorBlockHeaderListResponse, error)

	HandleNewTip(request *p2p.Tip) (bool, error)

	AddMinorBlock(request *p2p.NewBlockMinor) (bool, error)
}
