package master

import (
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
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

	GetMinorBlocks(request *p2p.GetMinorBlockListRequest) (*p2p.GetMinorBlockListResponse, error)

	GetMinorBlockHeaders(request *p2p.GetMinorBlockHeaderListRequest) (*p2p.GetMinorBlockHeaderListResponse, error)

	HandleNewTip(request *p2p.Tip) (bool, error)

	AddMinorBlock(request *p2p.NewBlockMinor) (bool, error)
}

type ISlaveConn interface {
	ShardConnForP2P
	GetSlaveID()string
	GetShardMaskList()[]*types.ChainMask
	hasShard(fullShardID uint32) bool
	SendPing(rootBlock *types.RootBlock, initializeShardSize bool) ([]byte, []*types.ChainMask, error)
	HeartBeat() bool
	GetUnconfirmedHeaders() (*rpc.GetUnconfirmedHeadersResponse, error)
	GetAccountData(address account.Address, height *uint64) (*rpc.GetAccountDataResponse, error)
	AddRootBlock(rootBlock *types.RootBlock, expectSwitch bool) error
	GenTx(numTxPerShard, xShardPercent uint32, tx *types.Transaction) error
	SendMiningConfigToSlaves(artificialTxConfig *rpc.ArtificialTxConfig, mining bool) error
	AddTransaction(tx *types.Transaction) error
	ExecuteTransaction(tx *types.Transaction, fromAddress account.Address, height *uint64) ([]byte, error)
	GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) (*types.MinorBlock, error)
	GetMinorBlockByHeight(height uint64, branch account.Branch) (*types.MinorBlock, error)
	GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error)
	GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error)
	GetTransactionsByAddress(address account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error)
	GetLogs(branch account.Branch, address []account.Address, topics []*rpc.Topic, startBlock, endBlock uint64) ([]*types.Log, error)
	EstimateGas(tx *types.Transaction, fromAddress account.Address) (uint32, error)
	GetStorageAt(address account.Address, key common.Hash, height *uint64) (common.Hash, error)
	GetCode(address account.Address, height *uint64) ([]byte, error)
	GasPrice(branch account.Branch) (uint64, error)
}