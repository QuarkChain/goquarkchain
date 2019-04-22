package shard

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type ShardAPI interface {
	AddCrossShardTxListByMinorBlockHash(hash common.Hash, txLst rpc.CrossShardTransactionList) bool
	AddBlockListForSync(blockLst []*types.MinorBlock) bool
	AddTx(tx *types.Transaction) bool
	ExecuteTx(tx *types.Transaction, address *account.Address) bool
	GetTransactionCount(recipient common.Address, hight uint64) int
	GetBalances(recipient common.Address, hight uint64) *big.Int
	GetTokenBalance(recipient common.Address) *big.Int
	GetCode(recipient common.Address, hight uint64) []byte
	GetMinorBlockByHash(hash common.Hash, consistencyCheck bool) *types.MinorBlock
	GetMinorBlockByHeight(height uint64) *types.MinorBlock
	GetTransactionByHash(txHash common.Hash) (*types.MinorBlock, uint32)
	GetTransactionReceipt(txHash common.Hash) (*types.MinorBlock, uint32, common.Address)
	GetTransactionListByAddress(address account.Address, start []byte, limit int)
	GetLogs()
	EstimateGas(tx *types.Transaction, fromAddress account.Address) int
	GetStorageAt(recipient common.Address, ) []byte
	GasPrice() uint64
	PoswDiffAdjust(block *types.MinorBlock) *big.Int
	GetWork() *consensus.MiningWork
	SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) bool
}
