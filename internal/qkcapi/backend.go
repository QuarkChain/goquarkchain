package qkcapi

import (
	"github.com/QuarkChain/goquarkchain/account"
	qkcRPC "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type Backend interface {
	AddTransaction(tx *types.Transaction) error
	ExecuteTransaction(tx *types.Transaction, address account.Address, height *uint64) ([]byte, error)
	GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) (*types.MinorBlock, error)
	GetMinorBlockByHeight(height uint64, branch account.Branch) (*types.MinorBlock, error)
	GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error)
	GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error)
	GetTransactionsByAddress(address account.Address, start []byte, limit uint32) ([]*qkcRPC.TransactionDetail, []byte, error)
	GetLogs(branch account.Branch, address []account.Address, topics []*qkcRPC.Topic, startBlock, endBlock *qkcRPC.BlockHeight) ([]*types.Log, error)
	EstimateGas(tx *types.Transaction, address account.Address) (uint32, error)
	GetStorageAt(address account.Address, key *serialize.Uint256, height uint64) (*serialize.Uint256, error)
	GetCode(address account.Address, height uint64) ([]byte, error)
	GasPrice(branch account.Branch) (uint64, error)
	GetWork(branch account.Branch) consensus.MiningWork
	SubmitWork(branch account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) bool

	RootBlockByNumber(blockNr *rpc.BlockNumber) (*types.RootBlock, error)
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
