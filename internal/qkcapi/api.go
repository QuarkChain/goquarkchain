package qkcapi

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	qkcRPC "github.com/QuarkChain/goquarkchain/cluster/rpc"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
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

func transHexutilUint64ToUint64(data *hexutil.Uint64) (*uint64, error) {
	if data == nil {
		return nil, nil
	}
	res := uint64(*data)
	return &res, nil
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
	panic(-1)

}

// EchoData echo data for test
func (p *PublicBlockChainAPI) EchoData(data rpc.BlockNumber) *hexutil.Big {
	panic(-1)
}

func (p *PublicBlockChainAPI) NetworkInfo() map[string]interface{} {
	panic(-1)

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

	data, err = p.b.GetPrimaryAccountData(&address, &blockNumber)
	return
}

func (p *PublicBlockChainAPI) GetTransactionCount(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Uint64, error) {
	panic(-1)
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
	panic(-1)

}
func (p *PublicBlockChainAPI) SendUnsigedTransaction(args SendTxArgs) (map[string]interface{}, error) {
	panic(-1)

}
func (p *PublicBlockChainAPI) SendTransaction() {
	//TODO support new tx v,r,s
	panic("not implemented")
}
func (p *PublicBlockChainAPI) SendRawTransaction(encodedTx hexutil.Bytes) (hexutil.Bytes, error) {
	panic(-1)

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
	branch := account.Branch{Value: p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))}
	minorBlock, err := p.b.GetMinorBlockByHash(blockHash, branch)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
	}
	return minorBlockEncoder(minorBlock, *includeTxs)

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
	branch := account.Branch{Value: p.b.GetClusterConfig().Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)}
	minorBlock, err := p.b.GetMinorBlockByHeight(height, branch)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
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
		return nil, errors.New("index bigger than block's tx")
	}
	return txEncoder(minorBlock, int(index))
}
func (p *PublicBlockChainAPI) Call(data CallArgs, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
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
	panic(-1)
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
func (p *PublicBlockChainAPI) GetLogs() { panic("-1") }
func (p *PublicBlockChainAPI) GetStorageAt(address account.Address, key common.Hash, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) GetCode(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) GetTransactionsByAddress(fullShardKey uint32) (hexutil.Uint64, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) GasPrice(fullShardKey uint32) (hexutil.Uint64, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) SubmitWork(fullShardKey hexutil.Uint, headHash common.Hash, nonce hexutil.Uint64, mixHash common.Hash) bool {
	panic(-1)
}
func (p *PublicBlockChainAPI) GetWork(fullShardKey hexutil.Uint) []*hexutil.Big {
	panic(-1)
}
func (p *PublicBlockChainAPI) NetVersion() hexutil.Uint64 {
	panic(-1)
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

func (p *PrivateBlockChainAPI) Getnextblocktomine() {
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
	panic(-1)
}
func (p *PrivateBlockChainAPI) GetBlockCount() (map[uint32]map[account.Recipient]uint32, error) {
	fmt.Println("GGGGGGGGGGGGGGGGGGG")
	return p.b.GetBlockCount()
}

//TODO txGenerate implement
func (p *PrivateBlockChainAPI) CreateTransactions(args *CreateTxArgs) error {
	args.setDefaults()
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
