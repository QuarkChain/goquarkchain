package shard

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

func (s *ShardBackend) AddCrossShardTxListByMinorBlockHash(hash common.Hash, txLst rpc.CrossShardTransactionList) bool {
	return false
}

func (s *ShardBackend) AddBlockListForSync(blockLst []*types.MinorBlock) bool {
	return false
}

func (s *ShardBackend) AddTx(tx *types.Transaction) bool {
	return false
}

func (s *ShardBackend) ExecuteTx(tx *types.Transaction, address *account.Address) bool                       { panic("not implemented") }
func (s *ShardBackend) GetTransactionCount(recipient common.Address, hight uint64) int                       { panic("not implemented") }
func (s *ShardBackend) GetBalances(recipient common.Address, hight uint64) *big.Int                          { panic("not implemented") }
func (s *ShardBackend) GetTokenBalance(recipient common.Address) *big.Int                                    { panic("not implemented") }
func (s *ShardBackend) GetCode(recipient common.Address, hight uint64) []byte                                { panic("not implemented") }
func (s *ShardBackend) GetMinorBlockByHash(hash common.Hash, consistencyCheck bool) *types.MinorBlock        { panic("not implemented") }
func (s *ShardBackend) GetMinorBlockByHeight(height uint64) *types.MinorBlock                                { panic("not implemented") }
func (s *ShardBackend) GetTransactionByHash(txHash common.Hash) (*types.MinorBlock, uint32)                  { panic("not implemented") }
func (s *ShardBackend) GetTransactionReceipt(txHash common.Hash) (*types.MinorBlock, uint32, common.Address) { panic("not implemented") }
func (s *ShardBackend) GetTransactionListByAddress(address account.Address, start []byte, limit int)         { panic("not implemented") }
func (s *ShardBackend) GetLogs()                                                                             { panic("not implemented") }
func (s *ShardBackend) EstimateGas(tx *types.Transaction, fromAddress account.Address) int                   { panic("not implemented") }
func (s *ShardBackend) GetStorageAt(recipient common.Address, ) []byte                                       { panic("not implemented") }
func (s *ShardBackend) GasPrice() uint64                                                                     { panic("not implemented") }
func (s *ShardBackend) PoswDiffAdjust(block *types.MinorBlock) *big.Int                                      { panic("not implemented") }
func (s *ShardBackend) GetWork() *consensus.MiningWork                                                       { panic("not implemented") }
func (s *ShardBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) bool            { panic("not implemented") }
