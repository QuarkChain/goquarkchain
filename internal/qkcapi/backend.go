package qkcapi

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcRPC "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
)

type Backend interface {
	AddTransaction(tx *types.Transaction) error
	ExecuteTransaction(tx *types.Transaction, address *account.Address, height *uint64) ([]byte, error)
	GetMinorBlockByHash(blockHash common.Hash, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *qkcRPC.PoSWInfo, error)
	GetMinorBlockByHeight(height *uint64, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *qkcRPC.PoSWInfo, error)
	GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error)
	GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error)
	GetTransactionsByAddress(address *account.Address, start []byte, limit uint32, transferTokenID *uint64) ([]*qkcRPC.TransactionDetail, []byte, error)
	GetAllTx(branch account.Branch, start []byte, limit uint32) ([]*qkcRPC.TransactionDetail, []byte, error)
	GetLogs(branch account.Branch, address []account.Address, topics [][]common.Hash, startBlock, endBlock uint64) ([]*types.Log, error)
	EstimateGas(tx *types.Transaction, address *account.Address) (uint32, error)
	GetStorageAt(address *account.Address, key common.Hash, height *uint64) (common.Hash, error)
	GetCode(address *account.Address, height *uint64) ([]byte, error)
	GasPrice(branch account.Branch, tokenID uint64) (uint64, error)
	GetWork(branch account.Branch) (*consensus.MiningWork, error)
	SubmitWork(branch account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash, signature *[65]byte) (bool, error)
	GetRootBlockByNumber(blockNr *uint64) (*types.RootBlock, error)
	GetRootBlockByHash(hash common.Hash) (*types.RootBlock, error)
	NetWorkInfo() map[string]interface{}
	GetPrimaryAccountData(address *account.Address, blockHeight *uint64) (*qkcRPC.AccountBranchData, error)
	CurrentBlock() *types.RootBlock
	GetAccountData(address *account.Address, height *uint64) (map[uint32]*qkcRPC.AccountBranchData, error)
	GetClusterConfig() *config.ClusterConfig
	GetPeerInfolist() []qkcRPC.PeerInfoForDisPlay
	GetStats() (map[string]interface{}, error)
	GetBlockCount() (map[uint32]map[account.Recipient]uint32, error)
	SetTargetBlockTime(rootBlockTime *uint32, minorBlockTime *uint32) error
	SetMining(mining bool)
	CreateTransactions(numTxPerShard, xShardPercent uint32, tx *types.Transaction) error
	IsSyncing() bool
	IsMining() bool
	GetSlavePoolLen() int
	GetLastMinorBlockByFullShardID(fullShardId uint32) (uint64, error)
	GetRootHashConfirmingMinorBlock(mBlockID []byte) common.Hash
	// p2p discovery healty nodes
	GetKadRoutingTable() ([]string, error)
}

func GetAPIs(apiBackend Backend) []rpc.API {
	return []rpc.API{
		{
			Namespace: "qkc",
			Version:   "1.0",
			Service:   NewPublicBlockChainAPI(apiBackend),
			Public:    true,
		},
		{
			Namespace: "qkc",
			Version:   "1.0",
			Service:   NewPrivateBlockChainAPI(apiBackend),
			Public:    false,
		},
	}
}
