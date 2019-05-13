package qkcapi

import (
	"github.com/QuarkChain/goquarkchain/account"
	qkcRPC "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

func decodeBlockNumberToUint64(b Backend, blockNumber *rpc.BlockNumber) (uint64, error) {
	panic(-1)
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

func (p *PublicBlockChainAPI) getPrimaryAccountData(address account.Address, blockNr *rpc.BlockNumber) (*qkcRPC.AccountBranchData, error) {
	panic(-1)

}

func (p *PublicBlockChainAPI) GetTransactionCount(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Uint64, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) GetBalances(address account.Address, blockNr *rpc.BlockNumber) (map[string]interface{}, error) {
	panic(-1)
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
func (p *PublicBlockChainAPI) GetRootBlockById(hash common.Hash) (map[string]interface{}, error) {
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
		return nil, err
	}
	return minorBlockEncoder(minorBlock, *includeTxs)
}
func (p *PublicBlockChainAPI) GetTransactionById(txID hexutil.Bytes) (map[string]interface{}, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) Call(data CallArgs, blockNr *rpc.BlockNumber) ([]byte, error) {
	panic(-1)

}
func (p *PublicBlockChainAPI) EstimateGas(data CallArgs) ([]byte, error) {
	panic(-1)
}
func (p *PublicBlockChainAPI) GetTransactionReceipt(txID hexutil.Bytes) (map[string]interface{}, error) {
	panic(-1)
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
	panic(-1)
}
func (c *CallArgs) toTx(networkid uint32) *types.Transaction {
	panic(-1)
}
func (p *PublicBlockChainAPI) CallOrEstimateGas(args *CallArgs, height *uint64, isCall bool) ([]byte, error) {
	panic(-1)
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
func (p *PrivateBlockChainAPI) GetBlockCount() map[string]interface{} {
	panic(-1)
}
func (p *PrivateBlockChainAPI) CreateTransactions() { panic("not implemented") }
func (p *PrivateBlockChainAPI) SetTargetBlockTime(rootBlockTime *uint32, minorBlockTime *uint32) error {
	panic(-1)
}
func (p *PrivateBlockChainAPI) SetMining(flag bool) (bool, error) {
	panic(-1)
}
func (p *PrivateBlockChainAPI) GetJrpcCalls() { panic("not implemented") }
