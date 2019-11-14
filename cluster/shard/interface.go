package shard

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

type XshardListTuple struct {
	XshardTxList   []*types.CrossShardTransactionDeposit
	PrevRootHeight uint32
}

type ConnManager interface {
	BroadcastXshardTxList(block *types.MinorBlock, xshardTxList []*types.CrossShardTransactionDeposit, height uint32) error
	SendMinorBlockHeaderToMaster(*rpc.AddMinorBlockHeaderRequest) error
	SendMinorBlockHeaderListToMaster(request *rpc.AddMinorBlockHeaderListRequest) error
	BatchBroadcastXshardTxList(blokHshToXLstAdPrvRotHg map[common.Hash]*XshardListTuple, sorBrch account.Branch) error
	// p2p interface
	BroadcastNewTip(mHeaderLst []*types.MinorBlockHeader, rHeader *types.RootBlockHeader, branch uint32) error
	BroadcastTransactions(peerId string, branch uint32, txs []*types.Transaction) error
	BroadcastMinorBlock(minorBlock *types.MinorBlock) error
	GetMinorBlocks(mHeaderList []common.Hash, peerId string, branch uint32) ([]*types.MinorBlock, error)
	GetMinorBlockHeaderList(gReq *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error)
}
