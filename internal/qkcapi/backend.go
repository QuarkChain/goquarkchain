package qkcapi

import (
	"fmt"
	"reflect"

	"github.com/QuarkChain/goquarkchain/common/hexutil"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
)

type Backend interface {
	AddTransaction(tx *types.Transaction) error
	ExecuteTransaction(tx *types.Transaction, address *account.Address, height *uint64) ([]byte, error)
	GetMinorBlockByHash(blockHash common.Hash, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *qrpc.PoSWInfo, error)
	GetMinorBlockByHeight(height *uint64, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *qrpc.PoSWInfo, error)
	GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error)
	GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error)
	GetTransactionsByAddress(address *account.Address, start []byte, limit uint32, transferTokenID *uint64) ([]*qrpc.TransactionDetail, []byte, error)
	GetAllTx(branch account.Branch, start []byte, limit uint32) ([]*qrpc.TransactionDetail, []byte, error)
	GetLogs(args *rpc.FilterQuery) ([]*types.Log, error)
	EstimateGas(tx *types.Transaction, address *account.Address) (uint32, error)
	GetStorageAt(address *account.Address, key common.Hash, height *uint64) (common.Hash, error)
	GetCode(address *account.Address, height *uint64) ([]byte, error)
	GasPrice(branch account.Branch, tokenID uint64) (uint64, error)
	GetWork(fullShardId *uint32, address *common.Address) (*consensus.MiningWork, error)
	SubmitWork(fullShardId *uint32, headerHash common.Hash, nonce uint64, mixHash common.Hash, signature *[65]byte) (bool, error)
	GetRootBlockByNumber(blockNr *uint64, needExtraInfo bool) (*types.RootBlock, *qrpc.PoSWInfo, error)
	GetRootBlockByHash(hash common.Hash, needExtraInfo bool) (*types.RootBlock, *qrpc.PoSWInfo, error)
	NetWorkInfo() map[string]interface{}
	GetPrimaryAccountData(address *account.Address, blockHeight *uint64) (*qrpc.AccountBranchData, error)
	CurrentBlock() *types.RootBlock
	GetAccountData(address *account.Address, height *uint64) (map[uint32]*qrpc.AccountBranchData, error)
	GetClusterConfig() *config.ClusterConfig
	GetPeerInfolist() []qrpc.PeerInfoForDisPlay
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
	fmt.Println("app", apiBackend.NetWorkInfo()["networkId"], reflect.TypeOf(apiBackend.NetWorkInfo()["networkId"]))
	networkId := apiBackend.NetWorkInfo()["networkId"].(hexutil.Uint)
	once.Do(func() {
		clusterCfg = apiBackend.GetClusterConfig()
	})
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
		{
			Namespace: "eth",
			Version:   "1.0",
			Service:   NewEthAPI(apiBackend),
			Public:    true,
		},
		{
			Namespace: "net",
			Version:   "1.0",
			Service:   NewPublicNetAPI(uint(networkId)),
			Public:    true,
		},
	}
}
