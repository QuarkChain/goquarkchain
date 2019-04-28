package qkcapi

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	qkcRPC "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
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
	rootBlock, err := p.b.GetRootBlockByNumber(blockHeight)
	if err == nil {
		response, err := rootBlockEncoder(rootBlock)
		if err != nil {
			return nil, err
		}
		return response, nil
	}
	return nil, err
}
func (p *PublicBlockChainAPI) GetMinorBlockById(blockID hexutil.Bytes, includeTxs *bool) (map[string]interface{}, error) {
	if includeTxs == nil {
		temp := false
		includeTxs = &temp
	}
	blockHash, fullShardKey, err := IDDecoder(blockID)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))}
	minorBlock, err := p.b.GetMinorBlockByHash(blockHash, branch)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, err
	}
	return minorBlockEncoder(minorBlock, *includeTxs)

}
func (p *PublicBlockChainAPI) GetMinorBlockByHeight(fullShardKey uint32, height *uint64, includeTxs *bool) (map[string]interface{}, error) {
	if includeTxs == nil {
		temp := false
		includeTxs = &temp
	}
	branch := account.Branch{Value: p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)}
	minorBlock, err := p.b.GetMinorBlockByHeight(height, branch)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, err
	}
	return minorBlockEncoder(minorBlock, *includeTxs)
}
func (p *PublicBlockChainAPI) GetTransactionById(txID hexutil.Bytes) (map[string]interface{}, error) {
	txHash, fullShardKey, err := IDDecoder(txID)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))}
	minorBlock, index, err := p.b.GetTransactionByHash(txHash, branch)
	if err != nil {
		return nil, err
	}
	if len(minorBlock.Transactions()) <= int(index) {
		return nil, errors.New("re err")
	}
	return txEncoder(minorBlock, int(index))

}
func (p *PublicBlockChainAPI) Call(data CallArgs, blockNr *rpc.BlockNumber) ([]byte, error) {
	if blockNr == nil {
		return p.CallOrEstimateGas(&data, nil, true)
	}
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	return p.CallOrEstimateGas(&data, &blockNumber, true)

}
func (p *PublicBlockChainAPI) EstimateGas(data CallArgs) ([]byte, error) {
	return p.CallOrEstimateGas(&data, nil, false)
}
func (p *PublicBlockChainAPI) GetTransactionReceipt(txID hexutil.Bytes) (map[string]interface{}, error) {
	txHash, fullShardKey, err := IDDecoder(txID)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)}
	minorBlock, index, receipt, err := p.b.GetTransactionReceipt(txHash, branch)
	if err != nil {
		return nil, err
	}
	return receiptEncoder(minorBlock, int(index), receipt), nil
}
func (p *PublicBlockChainAPI) GetLogs() { panic("not implemented") }
func (p *PublicBlockChainAPI) GetStorageAt(address account.Address, key common.Hash, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	var (
		hash common.Hash
		err  error
	)
	if blockNr == nil {
		hash, err = p.b.GetStorageAt(address, key, nil)
	} else {
		blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
		if err != nil {
			return nil, err
		}
		hash, err = p.b.GetStorageAt(address, key, &blockNumber)
	}
	if err != nil {
		return nil, err
	}
	return DataEncoder(hash.Bytes()), nil
}
func (p *PublicBlockChainAPI) GetCode(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	var (
		bytes []byte
		err   error
	)
	if blockNr == nil {
		bytes, err = p.b.GetCode(address, nil)
	} else {
		blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
		if err != nil {
			return nil, err
		}
		bytes, err = p.b.GetCode(address, &blockNumber)
	}

	if err != nil {
		return nil, err
	}
	return DataEncoder(bytes), nil
}
func (p *PublicBlockChainAPI) GetTransactionsByAddress(fullShardKey uint32) (hexutil.Uint64, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) GasPrice(fullShardKey uint32) (hexutil.Uint64, error) {
	branch := account.Branch{Value: p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)}
	res, err := p.b.GasPrice(branch)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(res), nil
}
func (p *PublicBlockChainAPI) SubmitWork(fullShardKey hexutil.Uint, headHash common.Hash, nonce hexutil.Uint64, mixHash common.Hash) bool {
	branch := new(account.Branch)
	branch.Value = p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))
	flag := p.b.SubmitWork(branch, headHash, uint64(nonce), mixHash)
	return flag
}
func (p *PublicBlockChainAPI) GetWork(fullShardKey hexutil.Uint) []*hexutil.Big {
	branch := new(account.Branch)
	branch.Value = p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))
	res := p.b.GetWork(branch)
	fileds := make([]*hexutil.Big, 0)
	fileds = append(fileds, (*hexutil.Big)(res.HeaderHash.Big()))
	fileds = append(fileds, (*hexutil.Big)(new(big.Int).SetUint64(res.Number)))
	fileds = append(fileds, (*hexutil.Big)(res.Difficulty))
	return fileds
}
func (p *PublicBlockChainAPI) NetVersion() hexutil.Uint64 {
	return hexutil.Uint64(p.b.GetClusterConfig().Quarkchain.NetworkID)
}
func (p *PublicBlockChainAPI) QkcQkcGasprice(fullShardKey uint32) (hexutil.Uint64, error) {
	return p.GasPrice(fullShardKey)
}
func (p *PublicBlockChainAPI) QkcGetblockbynumber(blockNumber rpc.BlockNumber, includeTx bool) (map[string]interface{}, error) {
	panic("not implemented")
}
func (p *PublicBlockChainAPI) QkcGetbalance()            { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGettransactioncount()   { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetcode()               { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcCall()                  { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcSendrawtransaction()    { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGettransactionreceipt() { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcEstimategas()           { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetlogs()               { panic("not implemented") }
func (p *PublicBlockChainAPI) QkcGetstorageat()          { panic("not implemented") }

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *account.Address `json:"from"`
	To       *account.Address `json:"to"`
	Gas      hexutil.Uint64   `json:"gas"`
	GasPrice hexutil.Big      `json:"gasPrice"`
	Value    hexutil.Big      `json:"value"`
	Data     hexutil.Bytes    `json:"data"`
}

func (c *CallArgs) setDefaults() {
	if c.From == nil {
		temp := account.CreatEmptyAddress(c.To.FullShardKey)
		c.From = &temp
	}
}
func (c *CallArgs) toTx(networkid uint32) *types.Transaction {
	nonce := uint64(0)
	evmTx := types.NewEvmTransaction(nonce, c.To.Recipient, c.Value.ToInt(), uint64(c.Gas), c.GasPrice.ToInt(), c.From.FullShardKey, c.To.FullShardKey, networkid, 0, c.Data)
	return &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
}
func (p *PublicBlockChainAPI) CallOrEstimateGas(args *CallArgs, height *uint64, isCall bool) ([]byte, error) {
	if args.To == nil {
		return nil, errors.New("missing to")
	}
	args.setDefaults()
	tx := args.toTx(p.b.GetClusterConfig().Quarkchain.NetworkID)
	if isCall {
		res, err := p.b.ExecuteTransaction(tx, *args.From, height)
		if err != nil {
			return nil, err
		}
		return DataEncoder(res), nil
	}
	res, err := p.b.EstimateGas(tx, *args.From)
	if err != nil {
		return nil, err
	}
	return []byte(hexutil.Uint(res).String()), nil // TODO ? check?
}

type PrivateBlockChainAPI struct {
	b Backend
}

func NewPrivateBlockChainAPI(b Backend) *PrivateBlockChainAPI {
	return &PrivateBlockChainAPI{b}
}

func (p *PrivateBlockChainAPI) Getnextblocktomine() {
	fmt.Println("Getnextblocktomine func response.")
}
func (p *PrivateBlockChainAPI) AddBlock(branch hexutil.Uint, blockData hexutil.Bytes) (bool, error) {
	if branch == 0 {
		rootBlock := new(types.RootBlock)
		if err := serialize.DeserializeFromBytes(blockData, rootBlock); err != nil {
			return false, err
		}
		if err := p.b.AddRootBlockFromMine(rootBlock); err != nil {
			return false, err
		}
		return true, nil
	}
	if err := p.b.AddRawMinorBlock(account.Branch{Value: uint32(branch)}, blockData); err != nil {
		return false, err
	}
	return true, nil
}
func (p *PrivateBlockChainAPI) GetPeers() map[string]interface{} {
	fields := make(map[string]interface{})

	list := make([]map[string]interface{}, 0)
	peerList := p.b.GetPeers()
	for _, v := range peerList {
		list = append(list, map[string]interface{}{
			"id":   hexutil.Bytes(v.ID),
			"ip":   hexutil.Uint(v.IP),
			"port": hexutil.Uint(v.Port),
		})
	}
	fields["peers"] = list
	return fields
}
func (p *PrivateBlockChainAPI) GetSyncStats() {
	panic("not implemented")
}
func (p *PrivateBlockChainAPI) GetStats() map[string]interface{} {
	return p.b.GetStats()
}
func (p *PrivateBlockChainAPI) GetBlockCount() map[string]interface{} {
	return p.b.GetBlockCount()
}
func (p *PrivateBlockChainAPI) CreateTransactions() { panic("not implemented") }
func (p *PrivateBlockChainAPI) SetTargetBlockTime(rootBlockTime *uint32, minorBlockTime *uint32) error {
	return p.b.SetTargetBlockTime(rootBlockTime, minorBlockTime)
}
func (p *PrivateBlockChainAPI) SetMining(flag bool) (bool, error) {
	err := p.b.SetMining(flag)
	if err != nil {
		return false, err
	}
	return true, nil
}
func (p *PrivateBlockChainAPI) GetJrpcCalls() { panic("not implemented") }
