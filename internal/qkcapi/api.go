package qkcapi

import (
	"errors"
	"fmt"
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
func (p *PublicBlockChainAPI) GetBalances(address common.Address, blockNr *rpc.BlockNumber) (*hexutil.Big, error) {
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	if blockNr == nil {
		p.b.GetPrimaryAccountData(address, nil)
	} else {
		p.b.GetPrimaryAccountData(address)
	}

}
func (p *PublicBlockChainAPI) GetAccountData()         { panic("not implemented") }
func (p *PublicBlockChainAPI) SendUnsigedTransaction() { panic("not implemented") }
func (p *PublicBlockChainAPI) SendTransaction()        { panic("not implemented") }
func (p *PublicBlockChainAPI) SendRawTransaction()     { panic("not implemented") }
func (p *PublicBlockChainAPI) GetRootBlockById()       { panic("not implemented") }
func (p *PublicBlockChainAPI) GetRootBlockByHeight(blockNr *rpc.BlockNumber) (map[string]interface{}, error) {
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	rootBlock, err := p.b.RootBlockByNumber(blockNumber)
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
