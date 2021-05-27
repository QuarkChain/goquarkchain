package qkcapi

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/ybbus/jsonrpc"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	netWorkID = uint32(3813)
)

type MetaMaskNetApi struct {
	c jsonrpc.RPCClient
}

func NewMetaMaskNetApi(c jsonrpc.RPCClient) *MetaMaskNetApi {
	return &MetaMaskNetApi{c: c}
}

func (e *MetaMaskNetApi) Version() string {
	resp, err := e.c.Call("net_version")
	if err != nil {
		panic(err)
	}
	return resp.Result.(string)
}

type MetaMaskEthBlockChainAPI struct {
	fullShardKey uint32
	c            jsonrpc.RPCClient
}

func NewMetaMaskEthAPI(fullShardKey uint32, client jsonrpc.RPCClient) *MetaMaskEthBlockChainAPI {
	return &MetaMaskEthBlockChainAPI{fullShardKey: fullShardKey, c: client}
}

func (e *MetaMaskEthBlockChainAPI) ChainId() (hexutil.Uint64, error) {
	resp, err := e.c.Call("chainId")
	if err != nil {
		return 0, err
	}
	result, err := hexutil.DecodeUint64(resp.Result.(string))
	return hexutil.Uint64(result), err
}

func (e *MetaMaskEthBlockChainAPI) GasPrice() (hexutil.Uint64, error) {
	resp, err := e.c.Call("gasPrice", hexutil.EncodeUint64(uint64(e.fullShardKey)))
	gasPrice, err := hexutil.DecodeUint64(resp.Result.(string))
	fmt.Println("resp", resp.Result, err, gasPrice)
	return hexutil.Uint64(gasPrice), err
}

func (e *MetaMaskEthBlockChainAPI) GetBalance(address common.Address, blockNrOrHash rpc.BlockNumber) (*hexutil.Big, error) {
	qkcAddr := account.NewAddress(address, e.fullShardKey)
	//fmt.Println("GetBalance", qkcAddr.ToHex())
	resp, err := e.c.Call("getBalances", qkcAddr.ToHex())
	if err != nil {
		return nil, err
	}
	balances := resp.Result.(map[string]interface{})["balances"]
	for _, m := range balances.([]interface{}) {
		bInfo := m.(map[string]interface{})
		if strings.ToUpper((bInfo["tokenStr"]).(string)) == DefaultTokenID {
			b, err := hexutil.DecodeBig(bInfo["balance"].(string))
			if err != nil {
				return nil, err
			}
			return (*hexutil.Big)(b), nil
		}
	}
	return nil, nil
}

func (e *MetaMaskEthBlockChainAPI) BlockNumber() hexutil.Uint64 {
	resp, err := e.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(e.fullShardKey)))
	if err != nil {
		fmt.Println("err", err)
		return 0
	}
	height, _ := hexutil.DecodeUint64(resp.Result.(map[string]interface{})["height"].(string))
	return hexutil.Uint64(height)
}

func (e *MetaMaskEthBlockChainAPI) GetBlockByNumber(blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	resp, err := e.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(e.fullShardKey)), nil, false)
	if err != nil {
		fmt.Println("err", err)
		return nil, err
	}
	return resp.Result.(map[string]interface{}), nil
}

func (e *MetaMaskEthBlockChainAPI) GetTransactionCount(address common.Address, blockNr rpc.BlockNumber) (hexutil.Uint64, error) {
	qkcAddr := account.NewAddress(address, e.fullShardKey)
	resp, err := e.c.Call("getTransactionCount", qkcAddr.ToHex())
	if err != nil {
		return 0, err
	}
	fmt.Println("GetTransaction", resp.Result)
	nonce, err := hexutil.DecodeUint64(resp.Result.(string))
	return hexutil.Uint64(nonce), err
}

func (e *MetaMaskEthBlockChainAPI) GetCode(address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	qkcAddr := account.NewAddress(address, e.fullShardKey)

	resp, err := e.c.Call("getCode", qkcAddr.ToHex())
	if err != nil {
		return nil, err
	}
	fmt.Println("GetCode", "end")
	return hexutil.Decode(resp.Result.(string))
}

func (s *MetaMaskEthBlockChainAPI) SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error) {
	fmt.Println("SSSSSSSSSSSSSSSS")
	tx := new(ethTypes.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		fmt.Println("EEEEEEEEEEEEEEEEEEE", err)
		return common.Hash{}, err
	}
	evmTx := new(types.EvmTransaction)
	if tx.To() != nil {
		evmTx = types.NewEvmTransaction(tx.Nonce(), *tx.To(), tx.Value(), tx.Gas(), tx.GasPrice(), s.fullShardKey, s.fullShardKey, netWorkID, 2, tx.Data(), 35760, 35760)
	} else {
		evmTx = types.NewEvmContractCreation(tx.Nonce(), tx.Value(), tx.Gas(), tx.GasPrice(), s.fullShardKey, s.fullShardKey, netWorkID, 2, tx.Data(), 35760, 35760)
	}
	evmTx.SetVRS(tx.RawSignatureValues())

	txQkc := &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	rlpTxBytes, err := rlp.EncodeToBytes(txQkc)
	if err != nil {
		return common.Hash{}, err
	}
	resp, err := s.c.Call("sendRawTransaction", common.ToHex(rlpTxBytes))
	if err != nil {
		return common.Hash{}, nil
	}
	fmt.Println(" SendRawTransaction resp", resp.Result, txQkc.Hash().String())
	return txQkc.Hash(), nil
}

func u() {

}
func Uint32ToBytes(n uint32) []byte {
	Bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(Bytes, n)
	return Bytes
}

func (c *MetaMaskEthBlockChainAPI) GetTransactionByHash(hash common.Hash) (map[string]interface{}, error) {
	txID := make([]byte, 0)
	txID = append(txID, hash.Bytes()...)
	txID = append(txID, Uint32ToBytes(c.fullShardKey)...)
	fmt.Println("MMMMMM", "GetTransactionByHash", hash.String(), common.ToHex(txID))
	resp, err := c.c.Call("getTransactionById", common.ToHex(txID))
	if err != nil {
		fmt.Println("errrr", err)
		return nil, err
	}

	fmt.Println("GetTransactionByHash end", resp.Result)
	return resp.Result.(map[string]interface{}), nil
}
func (c *MetaMaskEthBlockChainAPI) GetTransactionReceipt(hash common.Hash) (map[string]interface{}, error) {
	txID := make([]byte, 0)
	txID = append(txID, hash.Bytes()...)
	txID = append(txID, Uint32ToBytes(c.fullShardKey)...)
	fmt.Println("GetTransactionReceipt", hash.String())
	resp, err := c.c.Call("getTransactionReceipt", common.ToHex(txID))
	if err != nil {
		fmt.Println("errrr", err)
		return nil, err
	}

	fmt.Println("GetTransactionReceipt end")
	return resp.Result.(map[string]interface{}), nil
}

// MetaCallArgs represents the arguments for a call.
type MetaCallArgs struct {
	From            *account.Recipient `json:"from"`
	To              *account.Recipient `json:"to"`
	Gas             hexutil.Big        `json:"gas"`
	GasPrice        hexutil.Big        `json:"gasPrice"`
	Value           hexutil.Big        `json:"value"`
	Data            hexutil.Bytes      `json:"data"`
	GasTokenID      *hexutil.Uint64    `json:"gasTokenId"`
	TransferTokenID *hexutil.Uint64    `json:"transferTokenId"`
}

func (e *MetaMaskEthBlockChainAPI) Call(mdata MetaCallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	fmt.Println("MMMMMMMMM--call")
	defaultToken := hexutil.Uint64(35760)
	ttFrom := new(account.Address)
	if mdata.From == nil {
		ttFrom = nil
	} else {
		ttFrom.Recipient = *mdata.From
		ttFrom.FullShardKey = e.fullShardKey
	}
	ttTo := new(account.Address)
	if mdata.To == nil {
		ttTo = nil
	} else {
		ttTo.Recipient = *mdata.To
		ttTo.FullShardKey = e.fullShardKey
	}

	data := &CallArgs{
		From:            ttFrom,
		To:              ttTo,
		Gas:             mdata.Gas,
		GasPrice:        mdata.GasPrice,
		Value:           mdata.Value,
		Data:            mdata.Data,
		GasTokenID:      &defaultToken,
		TransferTokenID: &defaultToken,
	}

	fmt.Println("ready call", data.From, data.To)
	resp, err := e.c.Call("call", data)
	fmt.Println("Call-end", err, resp.Result)
	if err != nil {
		panic(err)
	}
	return hexutil.Decode(resp.Result.(string))
}

func (p *MetaMaskEthBlockChainAPI) EstimateGas(mdata MetaCallArgs) (hexutil.Uint, error) {
	defaultToken := hexutil.Uint64(35760)
	tt := account.Recipient{}
	if mdata.To != nil {
		tt = *mdata.To
	}

	data := &CallArgs{
		From: &account.Address{
			Recipient:    *mdata.From,
			FullShardKey: 0,
		},
		To: &account.Address{
			Recipient:    tt,
			FullShardKey: 0,
		},
		Gas:             mdata.Gas,
		GasPrice:        mdata.GasPrice,
		Value:           mdata.Value,
		Data:            mdata.Data,
		GasTokenID:      &defaultToken,
		TransferTokenID: &defaultToken,
	}

	resp, err := p.c.Call("estimateGas", data)
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp.Result)
	fmt.Println("MMMMMMMMM--EstimateGas", resp.Result)
	ans, err := hexutil.DecodeUint64(resp.Result.(string))
	return hexutil.Uint(ans), err
}
