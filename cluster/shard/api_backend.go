package shard

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
)

func (s *ShardBackend) GetUnconfirmedHeaders() []*types.MinorBlockHeader {
	return nil
}

func (s *ShardBackend) AddMinorBlock(block *types.MinorBlock) bool {
	// TODO judge if the block is the heightest block
	return false
}

func (s *ShardBackend) AddRootBlock(block *types.RootBlock) bool { panic("not implemented") }

func (s *ShardBackend) AddCrossShardTxListByMinorBlockHash(hash common.Hash, txLst *rpc.CrossShardTransactionList) bool {
	return false
}

func (s *ShardBackend) AddBlockListForSync(blockLst []*types.MinorBlock) error {
	return nil
}

func (s *ShardBackend) AddTx(tx *types.Transaction) error { return nil }

func (s *ShardBackend) ExecuteTx(tx *types.Transaction, address *account.Address) ([]byte, error)                                           { panic("not implemented") }
func (s *ShardBackend) GetTransactionCount(recipient account.Recipient, hight *uint64) uint64                                               { panic("not implemented") }
func (s *ShardBackend) GetBalances(recipient account.Recipient, hight *uint64) *big.Int                                                     { panic("not implemented") }
func (s *ShardBackend) GetTokenBalance(recipient account.Recipient) *big.Int                                                                { panic("not implemented") }
func (s *ShardBackend) GetCode(recipient account.Recipient, hight *uint64) []byte                                                           { panic("not implemented") }
func (s *ShardBackend) GetMinorBlockByHash(hash common.Hash, consistencyCheck bool) *types.MinorBlock                                       { panic("not implemented") }
func (s *ShardBackend) GetMinorBlockByHeight(height uint64) *types.MinorBlock                                                               { panic("not implemented") }
func (s *ShardBackend) GetTransactionByHash(txHash common.Hash) (*types.MinorBlock, uint32)                                                 { panic("not implemented") }
func (s *ShardBackend) GetTransactionReceipt(txHash common.Hash) (*types.MinorBlock, uint32, *types.Receipt)                                { panic("not implemented") }
func (s *ShardBackend) GetTransactionListByAddress(address *account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte) { panic("not implemented") }
func (s *ShardBackend) GetLogs() []*types.Log                                                                                               { panic("not implemented") }
func (s *ShardBackend) EstimateGas(tx *types.Transaction, fromAddress *account.Address) uint32                                              { panic("not implemented") }
func (s *ShardBackend) GetStorageAt(recipient account.Recipient) []byte                                                                     { panic("not implemented") }
func (s *ShardBackend) GasPrice() uint64                                                                                                    { panic("not implemented") }
func (s *ShardBackend) PoswDiffAdjust(block *types.MinorBlock) *big.Int                                                                     { panic("not implemented") }
func (s *ShardBackend) GetWork() *consensus.MiningWork                                                                                      { panic("not implemented") }
func (s *ShardBackend) SubmitWork(headerHash common.Hash, nonce uint64, mixHash common.Hash) error                                          { panic("not implemented") }

func (s *ShardBackend) HandleNewTip(tip *p2p.Tip) error {
	if len(tip.MinorBlockHeaderList) != 1 {
		log.Error("minor block header list must have only one header", "shard id", s.config.ShardID)
	}
	mHeader := tip.MinorBlockHeaderList[0]
	if mHeader.Branch != s.shardState.Branch() {

	}
	return nil
}
