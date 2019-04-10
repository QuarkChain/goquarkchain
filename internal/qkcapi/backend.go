package qkcapi

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type Backend interface {
	SendConnectToSlaves() bool
	AddTransaction(tx types.Transaction) bool
	ExecuteTransaction(tx types.Transaction, address account.Address, height uint64)
	GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) types.MinorBlock
	GetMinorBlockByHeight(height uint64, branch account.Branch) types.MinorBlock
	GetTransactionByHash(txHash common.Hash, branch account.Branch) (types.MinorBlock, uint32)
	GetTransactionReceipt(txHash common.Hash, branch account.Branch) (types.MinorBlock, uint32, types.Receipt)
	GetTransactionsByAddress(address account.Address, start uint64, limit uint32) ([]types.Transactions, uint64)
	GetLogs()
	EstimateGas(tx types.Transaction, address account.Address) uint32
	GetStorageAt(address account.Address, key serialize.Uint256, height uint64) []byte
	GetCode(address account.Address, height uint64) []byte
	GasPrice(branch account.Branch) uint64
	GetWork(branch account.Branch) consensus.MiningWork
	SubmitWork(branch account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) bool
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
