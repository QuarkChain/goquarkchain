package qkcapi

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	qkcRPC "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
)

func decodeBlockNumberToUint64(b Backend, blockNumber *rpc.BlockNumber) (uint64, error) {
	if blockNumber == nil {
		return b.CurrentBlock().NumberU64(), nil
	}
	if *blockNumber == rpc.PendingBlockNumber {
		return 0, errors.New("is pending block number")
	}
	if *blockNumber == rpc.LatestBlockNumber {
		return b.CurrentBlock().NumberU64(), nil
	}
	if *blockNumber == rpc.EarliestBlockNumber {
		return 0, nil
	}

	if *blockNumber < 0 {
		return 0, errors.New("invalid block Num")
	}
	return uint64(blockNumber.Int64()), nil
}

// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	b Backend
}

// NewPublicBlockChainAPI creates a new QuarkChain blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}

// Echoquantity :should use data without leading zero
func (p *PublicBlockChainAPI) Echoquantity(data hexutil.Big) *hexutil.Big {
	return &data
}

// EchoData echo data for test
func (p *PublicBlockChainAPI) EchoData(data rpc.BlockNumber) *hexutil.Big {
	fmt.Println("data", data.Int64())
	return nil
}

func (p *PublicBlockChainAPI) NetworkInfo() map[string]interface{} {
	return p.b.NetWorkInfo()
}

func (p *PublicBlockChainAPI) getPrimaryAccountData(address account.Address, blockNr *rpc.BlockNumber) (*qkcRPC.AccountBranchData, error) {
	var err error
	var data *qkcRPC.AccountBranchData

	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}

	if blockNr == nil {
		data, err = p.b.GetPrimaryAccountData(address, nil)
	} else {
		data, err = p.b.GetPrimaryAccountData(address, &blockNumber)
	}

	if err != nil {
		return nil, err
	}
	return data, err
}

func (p *PublicBlockChainAPI) GetTransactionCount(address account.Address, blockNr *rpc.BlockNumber) (*hexutil.Big, error) {
	data, err := p.getPrimaryAccountData(address, blockNr)
	if err != nil {
		return nil, err
	}
	return (*hexutil.Big)(data.TransactionCount.Value), nil
}
func (p *PublicBlockChainAPI) GetBalances(address account.Address, blockNr *rpc.BlockNumber) (map[string]interface{}, error) {
	data, err := p.getPrimaryAccountData(address, blockNr)
	if err != nil {
		return nil, err
	}
	branch := data.Branch
	balances := data.TokenBalances
	fields := map[string]interface{}{
		"branch":      hexutil.Uint64(branch.Value),
		"fullShardId": hexutil.Uint64(branch.GetFullShardID()),
		"shardId":     hexutil.Uint64(branch.GetShardID()),
		"chainId":     hexutil.Uint64(branch.GetChainID()),
		"balances":    balancesEncoder(balances),
	}
	return fields, nil
}
func (p *PublicBlockChainAPI) GetAccountData(address account.Address, blockNr *rpc.BlockNumber, includeShards *bool) (map[string]interface{}, error) {
	if includeShards == nil && blockNr == nil {
		return nil, nil
	}
	if includeShards == nil {
		temp := false
		includeShards = &temp
	}
	if !*includeShards {
		data, err := p.getPrimaryAccountData(address, blockNr)
		if err != nil {
			return nil, err
		}
		branch := data.Branch

		primary := map[string]interface{}{
			"fullShardId":      hexutil.Uint64(branch.GetFullShardID()),
			"shardId":          hexutil.Uint64(branch.GetShardID()),
			"chainId":          hexutil.Uint64(branch.GetChainID()),
			"balances":         balancesEncoder(data.TokenBalances),
			"transactionCount": (*hexutil.Big)(data.TransactionCount.Value),
			"isContract":       data.IsContract,
		}
		return map[string]interface{}{
			"primary": primary,
		}, nil
	}
	branchToAccountBranchData, err := p.b.GetAccountData(address)
	if err != nil {
		return nil, err
	}
	shards := make([]map[string]interface{}, 0)
	primary := make(map[string]interface{})
	for branch, data := range branchToAccountBranchData {
		shard := map[string]interface{}{
			"fullShardId":      hexutil.Uint64(branch.GetFullShardID()),
			"shardId":          hexutil.Uint64(branch.GetShardID()),
			"chainId":          hexutil.Uint64(branch.GetChainID()),
			"balances":         balancesEncoder(data.TokenBalances),
			"transactionCount": (*hexutil.Big)(data.TransactionCount.Value),
			"isContract":       data.IsContract,
		}
		shards = append(shards, shard)
		if branch.GetFullShardID() == p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey) {
			primary = shard
		}
	}
	return map[string]interface{}{
		"primary": primary,
		"shards":  shards,
	}, nil
}
func (p *PublicBlockChainAPI) SendUnsigedTransaction(args SendTxArgs) (map[string]interface{}, error) {
	// Set some sanity defaults and terminate on failure
	args.setDefaults()
	tx, err := args.toTransaction(p.b.GetClusterConfig().Quarkchain.NetworkID)
	if err != nil {
		return nil, err
	}

	fields := map[string]interface{}{
		"txHashUnsigned":  tx.EvmTx.Hash(),
		"nonce":           (hexutil.Uint64)(tx.EvmTx.Nonce()),
		"to":              DataEncoder(tx.EvmTx.To().Bytes()),
		"fromFullShardId": FullShardKeyEncoder(tx.EvmTx.FromFullShardId()), //TODO fullshardKey
		"toFullShardId":   FullShardKeyEncoder(tx.EvmTx.ToFullShardId()),   //TODO fullShardKey
		"value":           (*hexutil.Big)(tx.EvmTx.Value()),
		"gasPrice":        (*hexutil.Big)(tx.EvmTx.GasPrice()),
		"gas":             (hexutil.Uint64)(tx.EvmTx.Gas()),
		"data":            hexutil.Bytes(tx.EvmTx.Data()),
		"networkId":       hexutil.Uint64(p.b.GetClusterConfig().Quarkchain.NetworkID),
	}
	return fields, nil
}
func (p *PublicBlockChainAPI) SendTransaction() {
	//TODO support new tx v,r,s
	panic("not implemented")
}
func (p *PublicBlockChainAPI) SendRawTransaction(encodedTx hexutil.Bytes) (hexutil.Bytes, error) {
	evmTx := new(types.EvmTransaction)
	if err := rlp.DecodeBytes(encodedTx, evmTx); err != nil {
		return nil, err
	}
	tx := &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	err := p.b.AddTransaction(tx)
	if err != nil {
		return IDEncoder(common.Hash{}.Bytes(), 0), err
	}
	return IDEncoder(tx.Hash().Bytes(), evmTx.FromFullShardId()), nil //TODO FullSHardID

}
func (p *PublicBlockChainAPI) GetRootBlockById(hash common.Hash) (map[string]interface{}, error) {
	rootBlock, err := p.b.GetRootBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	return rootBlockEncoder(rootBlock)
}
func (p *PublicBlockChainAPI) GetRootBlockByHeight(blockHeight *uint64) (map[string]interface{}, error) {

	if blockHeight==nil{
		temp:=p.b.GetCu
	}
	rootBlock, err := p.b.GetRootBlockByNumber(blockNumber)
	if err == nil {
		response, err := rootBlockEncoder(rootBlock)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
	return nil, err
}
func (p *PublicBlockChainAPI) GetMinorBlockById()        { panic("not implemented") }
func (p *PublicBlockChainAPI) GetMinorBlockByHeight()    { panic("not implemented") }
func (p *PublicBlockChainAPI) GetTransactionById()       { panic("not implemented") }
func (p *PublicBlockChainAPI) Call()                     { panic("not implemented") }
func (p *PublicBlockChainAPI) EstimateGas()              { panic("not implemented") }
func (p *PublicBlockChainAPI) GetTransactionReceipt()    { panic("not implemented") }
func (p *PublicBlockChainAPI) GetLogs()                  { panic("not implemented") }
func (p *PublicBlockChainAPI) GetStorageAt()             { panic("not implemented") }
func (p *PublicBlockChainAPI) GetCode()                  { panic("not implemented") }
func (p *PublicBlockChainAPI) GetTransactionsByAddress() { panic("not implemented") }
func (p *PublicBlockChainAPI) GasPrice()                 { panic("not implemented") }
func (p *PublicBlockChainAPI) SubmitWork()               { panic("not implemented") }
func (p *PublicBlockChainAPI) GetWork()                  { panic("not implemented") }
func (p *PublicBlockChainAPI) NetVersion()               { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcQkcGasprice()           { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetblockbynumber()      { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetbalance()            { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGettransactioncount()   { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetcode()               { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcCall()                  { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcSendrawtransaction()    { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGettransactionreceipt() { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcEstimategas()           { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetlogs()               { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetstorageat()          { panic("not implemented") }

type PrivateBlockChainAPI struct {
	b Backend
}

func NewPrivateBlockChainAPI(b Backend) *PrivateBlockChainAPI {
	return &PrivateBlockChainAPI{b}
}

func (p *PrivateBlockChainAPI) Getnextblocktomine() {
	fmt.Println("Getnextblocktomine func response.")
}
func (p *PrivateBlockChainAPI) AddBlock()           { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetPeers()           { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetSyncStats()       { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetStats()           { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetBlockCount()      { panic("not implemented") }
func (p *PrivateBlockChainAPI) CreateTransactions() { panic("not implemented") }
func (p *PrivateBlockChainAPI) SetTargetBlockTime() { panic("not implemented") }
func (p *PrivateBlockChainAPI) SetMining()          { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetJrpcCalls()       { panic("not implemented") }
