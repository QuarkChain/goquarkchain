package qkcapi

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcRPC "github.com/QuarkChain/goquarkchain/cluster/rpc"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"sort"
)

func decodeBlockNumberToUint64(b Backend, blockNumber *rpc.BlockNumber) (*uint64, error) {
	if blockNumber == nil {
		return nil, nil
	}
	if *blockNumber == rpc.PendingBlockNumber {
		return nil, errors.New("is pending block number")
	}
	if *blockNumber == rpc.LatestBlockNumber {
		return nil, nil
	}
	if *blockNumber == rpc.EarliestBlockNumber {
		tBlock := uint64(0)
		return &tBlock, nil
	}

	if *blockNumber < 0 {
		return nil, errors.New("invalid block Num")
	}
	tBlock := uint64(blockNumber.Int64())
	return &tBlock, nil
}

func transHexutilUint64ToUint64(data *hexutil.Uint64) (*uint64, error) {
	if data == nil {
		return nil, nil
	}
	res := uint64(*data)
	return &res, nil
}

// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	clusterConfig *config.ClusterConfig
	b             Backend
}

// NewPublicBlockChainAPI creates a new QuarkChain blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b.GetClusterConfig(), b}
}

// Echoquantity :should use data without leading zero
func (p *PublicBlockChainAPI) Echoquantity(data hexutil.Big) *hexutil.Big {
	return &data

}

// EchoData echo data for test
func (p *PublicBlockChainAPI) EchoData(data hexutil.Big) *hexutil.Big {
	return &data
}

func (p *PublicBlockChainAPI) NetworkInfo() map[string]interface{} {
	config := p.b.GetClusterConfig()

	type ChainIdToShardSize struct {
		chainID   uint32
		shardSize uint32
	}
	ChainIdToShardSizeList := make([]ChainIdToShardSize, 0)
	for _, v := range config.Quarkchain.Chains {
		ChainIdToShardSizeList = append(ChainIdToShardSizeList, ChainIdToShardSize{chainID: v.ChainID, shardSize: v.ShardSize})
	}
	sort.Slice(ChainIdToShardSizeList, func(i, j int) bool { return ChainIdToShardSizeList[i].chainID < ChainIdToShardSizeList[j].chainID }) //Right???
	shardSize := make([]hexutil.Uint, 0)
	for _, v := range ChainIdToShardSizeList {
		shardSize = append(shardSize, hexutil.Uint(v.shardSize))
	}
	return map[string]interface{}{
		"networkId":        hexutil.Uint(config.Quarkchain.NetworkID),
		"chainSize":        hexutil.Uint(config.Quarkchain.ChainSize),
		"shardSizes":       shardSize,
		"syncing":          p.b.IsSyncing(),
		"mining":           p.b.IsMining(),
		"shardServerCount": p.b.GetSlavePoolLen(),
	}

}

func (p *PublicBlockChainAPI) getPrimaryAccountData(address account.Address, blockNr *rpc.BlockNumber) (data *qkcRPC.AccountBranchData, err error) {
	if blockNr == nil {
		data, err = p.b.GetPrimaryAccountData(&address, nil)
		return
	}

	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}

	return p.b.GetPrimaryAccountData(&address, blockNumber)
}

func (p *PublicBlockChainAPI) GetTransactionCount(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Uint64, error) {
	data, err := p.getPrimaryAccountData(address, blockNr)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(data.TransactionCount), nil
}
func (p *PublicBlockChainAPI) GetBalances(address account.Address, blockNr *rpc.BlockNumber) (map[string]interface{}, error) {
	data, err := p.getPrimaryAccountData(address, blockNr)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: data.Branch}
	balances := data.Balance
	fields := map[string]interface{}{
		"branch":      hexutil.Uint64(branch.Value),
		"fullShardId": hexutil.Uint64(branch.GetFullShardID()),
		"shardId":     hexutil.Uint64(branch.GetShardID()),
		"chainId":     hexutil.Uint64(branch.GetChainID()),
		"balances":    (*hexutil.Big)(balances),
	}
	return fields, nil
}
func (p *PublicBlockChainAPI) GetAccountData(address account.Address, blockNr *rpc.BlockNumber, includeShards *bool) (map[string]interface{}, error) {
	if includeShards != nil && blockNr == nil {
		return nil, errors.New("do not allow specify height if client wants info on all shards")
	}
	if includeShards == nil {
		t := false
		includeShards = &t
	}
	if !(*includeShards) {
		accountBranchData, err := p.getPrimaryAccountData(address, blockNr)
		if err != nil {
			return nil, err
		}
		branch := account.Branch{Value: accountBranchData.Branch}
		primary := map[string]interface{}{
			"fullShardId":      hexutil.Uint(branch.GetFullShardID()),
			"shardId":          hexutil.Uint(branch.GetShardID()),
			"chainId":          hexutil.Uint(branch.GetChainID()),
			"balances":         hexutil.Big(*accountBranchData.Balance),
			"transactionCount": hexutil.Uint64(accountBranchData.TransactionCount),
			"isContract":       accountBranchData.IsContract,
		}
		return map[string]interface{}{
			"primary": primary,
		}, nil
	}
	branchToAccountBranchData, err := p.b.GetAccountData(&address, nil)
	if err != nil {
		return nil, err
	}

	shards := make([]map[string]interface{}, 0)
	primary := make(map[string]interface{})
	for branch, accountBranchData := range branchToAccountBranchData {
		branch := account.Branch{Value: branch}
		shardData := map[string]interface{}{
			"fullShardId":      hexutil.Uint(branch.GetFullShardID()),
			"shardId":          hexutil.Uint(branch.GetShardID()),
			"chainId":          hexutil.Uint(branch.GetChainID()),
			"balances":         hexutil.Big(*accountBranchData.Balance),
			"transactionCount": hexutil.Uint(accountBranchData.TransactionCount),
			"isContract":       accountBranchData.IsContract,
		}
		shards = append(shards, shardData)
		fullShardIDByConfig, err := p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
		if err != nil {
			return nil, err
		}
		if branch.GetFullShardID() == fullShardIDByConfig {
			primary = shardData
		}
	}
	return map[string]interface{}{
		"primary": primary,
		"shards":  shards,
	}, nil

}

func (p *PublicBlockChainAPI) SendTransaction(args SendTxArgs) (hexutil.Bytes, error) {
	args.setDefaults()
	tx, err := args.toTransaction(p.b.GetClusterConfig().Quarkchain.NetworkID, true)
	if err != nil {
		return nil, err
	}
	if err := p.b.AddTransaction(tx); err != nil {
		return nil, err
	}
	return IDEncoder(tx.Hash().Bytes(), tx.EvmTx.FromFullShardKey()), nil
}
func (p *PublicBlockChainAPI) SendRawTransaction(encodedTx hexutil.Bytes) (hexutil.Bytes, error) {
	evmTx := new(types.EvmTransaction)
	if err := rlp.DecodeBytes(encodedTx, evmTx); err != nil {
		return nil, err
	}
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	if err := p.b.AddTransaction(tx); err != nil {
		log.Error("sendRawTx err", "err", err)
		return IDEncoder(common.Hash{}.Bytes(), 0), err //TODO need return err?
	}
	return IDEncoder(tx.Hash().Bytes(), tx.EvmTx.FromFullShardKey()), nil
}
func (p *PublicBlockChainAPI) GetRootBlockByHash(hash common.Hash) (map[string]interface{}, error) {
	rootBlock, err := p.b.GetRootBlockByHash(hash)
	if err != nil {
		return nil, err
	}
	return rootBlockEncoder(rootBlock)
}
func (p *PublicBlockChainAPI) GetRootBlockByHeight(heightInput *hexutil.Uint64) (map[string]interface{}, error) {
	blockHeight, err := transHexutilUint64ToUint64(heightInput)
	if err != nil {
		return nil, err
	}
	rootBlock, err := p.b.GetRootBlockByNumber(blockHeight)
	if err != nil {
		return nil, err
	}
	response, err := rootBlockEncoder(rootBlock)
	if err != nil {
		return nil, err
	}
	return response, nil
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
	fullShardIDByConfig, err := p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardIDByConfig}
	minorBlock, err := p.b.GetMinorBlockByHash(blockHash, branch)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
	}
	return minorBlockEncoder(minorBlock, *includeTxs, p.clusterConfig)

}
func (p *PublicBlockChainAPI) GetMinorBlockByHeight(fullShardKeyInput hexutil.Uint, heightInput *hexutil.Uint64, includeTxs *bool) (map[string]interface{}, error) {
	height, err := transHexutilUint64ToUint64(heightInput)
	if err != nil {
		return nil, err
	}
	fullShardKey := uint32(fullShardKeyInput)
	if includeTxs == nil {
		temp := false
		includeTxs = &temp
	}

	fullShardIDByConfig, err := p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)
	branch := account.Branch{Value: fullShardIDByConfig}
	minorBlock, err := p.b.GetMinorBlockByHeight(height, branch)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
	}
	return minorBlockEncoder(minorBlock, *includeTxs, p.clusterConfig)
}
func (p *PublicBlockChainAPI) GetTransactionById(txID hexutil.Bytes) (map[string]interface{}, error) {
	txHash, fullShardKey, err := IDDecoder(txID)
	if err != nil {
		return nil, err
	}
	fullShardIDByConfig, err := p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardIDByConfig}
	minorBlock, index, err := p.b.GetTransactionByHash(txHash, branch)
	if err != nil {
		return nil, err
	}
	if len(minorBlock.Transactions()) <= int(index) {
		return nil, errors.New("index bigger than block's tx")
	}
	return txEncoder(minorBlock, int(index), p.clusterConfig)
}
func (p *PublicBlockChainAPI) Call(data CallArgs, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	if blockNr == nil {
		return p.CallOrEstimateGas(&data, nil, true)
	}
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	return p.CallOrEstimateGas(&data, blockNumber, true)

}
func (p *PublicBlockChainAPI) EstimateGas(data CallArgs) ([]byte, error) {
	return p.CallOrEstimateGas(&data, nil, false)
}
func (p *PublicBlockChainAPI) GetTransactionReceipt(txID hexutil.Bytes) (map[string]interface{}, error) {
	txHash, fullShardKey, err := IDDecoder(txID)
	if err != nil {
		return nil, err
	}

	fullShardIDByConfig, err := p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardIDByConfig}
	minorBlock, index, receipt, err := p.b.GetTransactionReceipt(txHash, branch)
	if err != nil {
		return nil, err
	}
	return receiptEncoder(minorBlock, int(index), receipt)
}
func (p *PublicBlockChainAPI) GetLogs(args *FilterQuery, fullShardKey hexutil.Uint) ([]map[string]interface{}, error) {
	fullShardID, err := p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))
	if err != nil {
		return nil, err
	}
	lastBlockHeight, err := p.b.GetLastMinorBlockByFullShardID(fullShardID)
	if err != nil {
		return nil, err
	}
	if args.FromBlock == nil || args.FromBlock.Int64() == rpc.LatestBlockNumber.Int64() {
		args.FromBlock = new(big.Int).SetUint64(lastBlockHeight)
	}
	if args.ToBlock == nil || args.ToBlock.Int64() == rpc.LatestBlockNumber.Int64() {
		args.ToBlock = new(big.Int).SetUint64(lastBlockHeight)
	}
	if args.FromBlock.Int64() == rpc.PendingBlockNumber.Int64() || args.ToBlock.Int64() == rpc.PendingBlockNumber.Int64() {
		return nil, errors.New("not support pending")
	}

	log, err := p.b.GetLogs(account.Branch{Value: fullShardID}, args.Addresses, args.Topics, args.FromBlock.Uint64(), args.ToBlock.Uint64())
	return logListEncoder(log), nil
}
func (p *PublicBlockChainAPI) GetStorageAt(address account.Address, key common.Hash, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	hash, err := p.b.GetStorageAt(&address, key, blockNumber)
	return hash.Bytes(), err
}
func (p *PublicBlockChainAPI) GetCode(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	return p.b.GetCode(&address, blockNumber)
}
func (p *PublicBlockChainAPI) GetTransactionsByAddress(address account.Address, start *hexutil.Bytes, limit *hexutil.Uint) (map[string]interface{}, error) {
	limitValue := uint32(0)
	if limit != nil {
		limitValue = uint32(*limit)
	}
	if limitValue > 20 {
		limitValue = 20
	}
	startValue := make([]byte, 0)
	if start != nil {
		startValue = *start
	}
	txs, next, err := p.b.GetTransactionsByAddress(&address, startValue, limitValue)
	if err != nil {
		return nil, err
	}

	txsFields := make([]map[string]interface{}, 0)
	for _, tx := range txs {
		to := account.Address{}
		if tx.ToAddress != nil {
			to = *tx.ToAddress
		}
		txField := map[string]interface{}{
			"txId":        IDEncoder(tx.TxHash.Bytes(), tx.FromAddress.FullShardKey),
			"fromAddress": tx.FromAddress,
			"toAddress":   to,
			"value":       (*hexutil.Big)(tx.Value.Value),
			"blockHeight": hexutil.Uint(tx.BlockHeight),
			"timestamp":   hexutil.Uint(tx.Timestamp),
			"success":     tx.Success,
		}
		txsFields = append(txsFields, txField)
	}
	return map[string]interface{}{
		"txList": txsFields,
		"next":   hexutil.Bytes(next),
	}, nil

}
func (p *PublicBlockChainAPI) GasPrice(fullShardKey uint32) (hexutil.Uint64, error) {
	fullShardId, err := p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)
	if err != nil {
		return hexutil.Uint64(0), err
	}
	data, err := p.b.GasPrice(account.Branch{Value: fullShardId})
	return hexutil.Uint64(data), err
}

func (p *PublicBlockChainAPI) SubmitWork(fullShardKey *hexutil.Uint, headHash common.Hash, nonce hexutil.Uint64, mixHash common.Hash) (bool, error) {
	fullShardId := uint32(0)
	var err error
	if fullShardKey != nil {
		fullShardId, err = p.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(uint32(*fullShardKey))
		if err != nil {
			return false, err
		}
	}
	submit, err := p.b.SubmitWork(account.NewBranch(fullShardId), headHash, uint64(nonce), mixHash)
	if err != nil {
		log.Error("Submit remote minered block", "err", err)
		return false, nil
	}
	return submit, nil
}

func (p *PublicBlockChainAPI) GetWork(fullShardKey *hexutil.Uint) ([]common.Hash, error) {
	fullShardId := uint32(0)
	var err error
	if fullShardKey != nil {
		fullShardId, err = p.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(uint32(*fullShardKey))
		if err != nil {
			return nil, err
		}
	}
	work, err := p.b.GetWork(account.NewBranch(fullShardId))
	if err != nil {
		return nil, err
	}
	height := new(big.Int).SetUint64(work.Number)
	var val = make([]common.Hash, 0, 3)
	val = append(val, work.HeaderHash)
	val = append(val, common.BytesToHash(height.Bytes()))
	val = append(val, common.BytesToHash(work.Difficulty.Bytes()))
	return val, nil
}
func (p *PublicBlockChainAPI) NetVersion() hexutil.Uint {
	return hexutil.Uint(p.b.GetClusterConfig().Quarkchain.NetworkID)
}

func (p *PublicBlockChainAPI) GetAccountPermission(addr account.Address) bool {
	status := p.b.CheckAccountPermission(addr)
	if status != nil {
		return false
	}
	return true
}
func (p *PublicBlockChainAPI) QkcQkcGasprice(fullShardKey uint32) (hexutil.Uint64, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) QkcGetblockbynumber(blockNumber rpc.BlockNumber, includeTx bool) (map[string]interface{}, error) {
	panic(-1)
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

func (p *PublicBlockChainAPI) CallOrEstimateGas(args *CallArgs, height *uint64, isCall bool) (hexutil.Bytes, error) {
	if args.To == nil {
		return nil, errors.New("missing to")
	}
	args.setDefaults()
	tx, err := args.toTx(p.b.GetClusterConfig().Quarkchain)
	if err != nil {
		return nil, err
	}
	if isCall {
		res, err := p.b.ExecuteTransaction(tx, args.From, height)
		if err != nil {
			log.Error("call ", "to", tx.EvmTx.To().String(), "err", err)
			return nil, err
		}
		return (hexutil.Bytes)(res), nil
	}
	data, err := p.b.EstimateGas(tx, args.From)
	if err != nil {
		return nil, err
	}
	return qkcCommon.Uint32ToBytes(data), nil
}

type PrivateBlockChainAPI struct {
	b Backend
}

func NewPrivateBlockChainAPI(b Backend) *PrivateBlockChainAPI {
	return &PrivateBlockChainAPI{b}
}

func (p *PrivateBlockChainAPI) GetNextblocktomine() {
	//No need to implement
	panic(-1)
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
	//need to discuss
	panic("not implemented")
}
func (p *PrivateBlockChainAPI) GetStats() (map[string]interface{}, error) {
	return p.b.GetStats()
}
func (p *PrivateBlockChainAPI) GetBlockCount() (map[string]interface{}, error) {
	data, err := p.b.GetBlockCount()
	return map[string]interface{}{
		"rootHeight": hexutil.Uint64(p.b.CurrentBlock().Number()),
		"shardRC":    data,
	}, err
}

//TODO txGenerate implement
func (p *PrivateBlockChainAPI) CreateTransactions(NumTxPreShard hexutil.Uint) error {
	args := CreateTxArgs{
		NumTxPreShard: NumTxPreShard,
		// after that are default values, create tx func will fill.
		XShardPrecent: 0,
		To:            common.Address{},
		Gas:           (*hexutil.Big)(big.NewInt(30000)),
		GasPrice:      (*hexutil.Big)(big.NewInt(1)),
		Value:         (*hexutil.Big)(big.NewInt(0)),
	}
	tx := args.toTx(p.b.GetClusterConfig().Quarkchain)
	return p.b.CreateTransactions(uint32(args.NumTxPreShard), uint32(args.XShardPrecent), tx)
}
func (p *PrivateBlockChainAPI) SetTargetBlockTime(rootBlockTime *uint32, minorBlockTime *uint32) error {
	return p.b.SetTargetBlockTime(rootBlockTime, minorBlockTime)
}
func (p *PrivateBlockChainAPI) SetMining(flag bool) error {
	return p.b.SetMining(flag)
}

//TODO ?? necessary?
func (p *PrivateBlockChainAPI) GetJrpcCalls() { panic("not implemented") }
