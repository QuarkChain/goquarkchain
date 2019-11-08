package rpc

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type NetworkError struct {
	Msg string
}

func (e *NetworkError) Error() string {
	return e.Msg
}

type ConnManager interface {
	GetOneConnById(fullShardId uint32) ISlaveConn
	GetSlaveConnsById(fullShardId uint32) []ISlaveConn
	GetSlaveConns() []ISlaveConn
	ConnCount() int
}

type ISlaveConn interface {
	// AddTransactions will add the tx to shard tx pool, and return the tx hash
	// which have been added to tx pool. so tx which cannot pass verification
	// or existed in tx pool will not be included in return hash list
	AddTransactions(request *NewTransactionList) (*HashList, error)
	GetMinorBlockByHash(blockHash common.Hash, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *PoSWInfo, error)
	GetMinorBlockByHeight(height *uint64, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *PoSWInfo, error)
	GetMinorBlocks(request *GetMinorBlockListRequest) (*p2p.GetMinorBlockListResponse, error)
	GetMinorBlockHeaderList(req *p2p.GetMinorBlockHeaderListWithSkipRequest) (*p2p.GetMinorBlockHeaderListResponse, error)
	HandleNewTip(request *HandleRawMinorTip) error
	HandleNewMinorBlock(request *p2p.NewBlockMinor) (bool, error)
	AddBlockListForSync(request *AddBlockListForSyncRequest) (*ShardStatus, error)
	GetSlaveID() string
	GetShardMaskList() []*types.ChainMask
	MasterInfo(ip string, port uint16, rootTip *types.RootBlock) error
	HasShard(fullShardID uint32) bool
	SendPing() ([]byte, []*types.ChainMask, error)
	HeartBeat() bool
	GetUnconfirmedHeaders() (*GetUnconfirmedHeadersResponse, error)
	GetAccountData(address *account.Address, height *uint64) (*GetAccountDataResponse, error)
	AddRootBlock(rootBlock *types.RootBlock, expectSwitch bool) error
	GenTx(numTxPerShard, xShardPercent uint32, tx *types.Transaction) error
	SendMiningConfigToSlaves(artificialTxConfig *ArtificialTxConfig, mining bool) error
	AddTransaction(tx *types.Transaction) error
	ExecuteTransaction(tx *types.Transaction, fromAddress *account.Address, height *uint64) ([]byte, error)
	GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error)
	GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error)
	GetTransactionsByAddress(address *account.Address, start []byte, limit uint32, transferTokenID *uint64) ([]*TransactionDetail, []byte, error)
	GetAllTx(branch account.Branch, start []byte, limit uint32) ([]*TransactionDetail, []byte, error)
	GetLogs(args *rpc.FilterQuery) ([]*types.Log, error)
	EstimateGas(tx *types.Transaction, fromAddress *account.Address) (uint32, error)
	GetStorageAt(address *account.Address, key common.Hash, height *uint64) (common.Hash, error)
	GetCode(address *account.Address, height *uint64) ([]byte, error)
	GasPrice(branch account.Branch, tokenID uint64) (uint64, error)
	GetWork(branch account.Branch, address *account.Address) (*consensus.MiningWork, error)
	SubmitWork(work *SubmitWorkRequest) (success bool, err error)
	SetMining(mining bool) error
	GetRootChainStakes(address account.Address, lastMinor common.Hash) (*big.Int, *account.Recipient, error)
	CheckMinorBlocksInRoot(rootBlock *types.RootBlock) error
}
