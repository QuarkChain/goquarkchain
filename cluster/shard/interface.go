package shard

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type ShardAPI interface {
	GetUnconfirmedHeaders() []*types.MinorBlockHeader
	AddMinorBlock(block *types.MinorBlock) bool
	AddRootBlock(block *types.RootBlock) bool
	AddCrossShardTxListByMinorBlockHash(hash common.Hash, txLst *rpc.CrossShardTransactionList) bool
	AddBlockListForSync(blockLst []*types.MinorBlock) error
	AddTx(tx *types.Transaction) error
	ExecuteTx(tx *types.Transaction, address *account.Address) ([]byte, error)
	GetTransactionCount(recipient account.Recipient, hight *uint64) uint64
	GetBalances(recipient account.Recipient, hight *uint64) *big.Int
	GetTokenBalance(recipient account.Recipient) *big.Int
	GetCode(recipient account.Recipient, hight *uint64) []byte
	GetMinorBlockByHash(hash common.Hash, consistencyCheck bool) *types.MinorBlock
	GetMinorBlockByHeight(height uint64) *types.MinorBlock
	GetTransactionByHash(txHash common.Hash) (*types.MinorBlock, uint32)
	GetTransactionReceipt(txHash common.Hash) (*types.MinorBlock, uint32, *types.Receipt)
	GetTransactionListByAddress(address *account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte)
	GetLogs() []*types.Log
	EstimateGas(tx *types.Transaction, fromAddress *account.Address) uint32
	GetStorageAt(recipient account.Recipient) []byte
	GasPrice() uint64
	PoswDiffAdjust(block *types.MinorBlock) *big.Int
	GetWork() *consensus.MiningWork
	SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) error
	HandleNewTip(tip *p2p.Tip) error
}
