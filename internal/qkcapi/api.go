package qkcapi

import "fmt"

// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	b Backend
}

// NewPublicBlockChainAPI creates a new QuarkChain blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}

func (p *PublicBlockChainAPI) Echoquantity() {
	fmt.Println("Echoquantity func response.")
}
func (p *PublicBlockChainAPI) EchoData()                 { panic("not implemented") }
func (p *PublicBlockChainAPI) NetworkInfo()              { panic("not implemented") }
func (p *PublicBlockChainAPI) GetBalances()              { panic("not implemented") }
func (p *PublicBlockChainAPI) GetAccountData()           { panic("not implemented") }
func (p *PublicBlockChainAPI) SendUnsigedTransaction()   { panic("not implemented") }
func (p *PublicBlockChainAPI) SendTransaction()          { panic("not implemented") }
func (p *PublicBlockChainAPI) SendRawTransaction()       { panic("not implemented") }
func (p *PublicBlockChainAPI) GetRootBlockById()         { panic("not implemented") }
func (p *PublicBlockChainAPI) GetRootBlockByHeight()     { panic("not implemented") }
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
func (p *PublicBlockChainAPI) EthGasprice()              { panic("not implemented") }
func (p *PublicBlockChainAPI) EthGetblockbynumber()      { panic("not implemented") }
func (p *PublicBlockChainAPI) EthGetbalance()            { panic("not implemented") }
func (p *PublicBlockChainAPI) EthGettransactioncount()   { panic("not implemented") }
func (p *PublicBlockChainAPI) EthGetcode()               { panic("not implemented") }
func (p *PublicBlockChainAPI) EthCall()                  { panic("not implemented") }
func (p *PublicBlockChainAPI) EthSendrawtransaction()    { panic("not implemented") }
func (p *PublicBlockChainAPI) EthGettransactionreceipt() { panic("not implemented") }
func (p *PublicBlockChainAPI) EthEstimategas()           { panic("not implemented") }
func (p *PublicBlockChainAPI) EthGetlogs()               { panic("not implemented") }
func (p *PublicBlockChainAPI) EthGetstorageat()          { panic("not implemented") }

type PrivateBlockChainAPI struct {
	b Backend
}

func NewPrivateBlockChainAPI(b Backend) *PrivateBlockChainAPI {
	return &PrivateBlockChainAPI{b}
}

func (p *PrivateBlockChainAPI) Getnextblocktomine() { panic("not implemented") }
func (p *PrivateBlockChainAPI) AddBlock()           { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetPeers()           { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetSyncStats()       { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetStats()           { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetBlockCount()      { panic("not implemented") }
func (p *PrivateBlockChainAPI) CreateTransactions() { panic("not implemented") }
func (p *PrivateBlockChainAPI) SetTargetBlockTime() { panic("not implemented") }
func (p *PrivateBlockChainAPI) SetMining()          { panic("not implemented") }
func (p *PrivateBlockChainAPI) GetJrpcCalls()       { panic("not implemented") }
