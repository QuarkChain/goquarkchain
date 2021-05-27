package qkcapi

import (
	"encoding/hex"
	"fmt"
	"reflect"
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
	fmt.Println("call-vaerison", err)
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp.Result)
	v, err := hexutil.DecodeUint64(resp.Result.(string))
	fmt.Println("vvv", v, err, reflect.TypeOf(resp.Result))
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
				//fmt.Println("GetBalance balance err", err)
				return nil, err
			}
			//fmt.Println("GetBalance balance", b)
			return (*hexutil.Big)(b), nil
		}
	}
	//fmt.Println("GetBalance balance", "nnnnnnnnnnull")
	return nil, nil
}

func (e *MetaMaskEthBlockChainAPI) BlockNumber() hexutil.Uint64 {
	//fmt.Println("BBBBBBBBBBBBBBBBBBBBBBBBBBBlockNumber----")
	resp, err := e.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(e.fullShardKey)))
	if err != nil {
		fmt.Println("err", err)
		return 0
	}
	//fmt.Println("", resp.Result.(map[string]interface{})["height"])
	height, _ := hexutil.DecodeUint64(resp.Result.(map[string]interface{})["height"].(string))
	fmt.Println("BLockNumber", height)
	return hexutil.Uint64(height)
}

func (e *MetaMaskEthBlockChainAPI) GetBlockByNumber(blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	resp, err := e.c.Call("eth_getBlockByNumber", hexutil.EncodeUint64(uint64(e.fullShardKey)), false)
	if err != nil {
		fmt.Println("err", err)
		return nil, err
	}
	fmt.Println("resp", resp.Result)
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

	//panic("GetTransactionCount")
}

func (e *MetaMaskEthBlockChainAPI) GetCode(address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	qkcAddr := account.NewAddress(address, e.fullShardKey)

	resp, err := e.c.Call("getCode", qkcAddr.ToHex())
	if err != nil {
		return nil, err
	}
	fmt.Println("GetCode", resp.Result)
	return hexutil.Decode(resp.Result.(string))
}

func (s *MetaMaskEthBlockChainAPI) SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error) {
	fmt.Println("SSSSSSSSSSSSSSSS", encodedTx.String())
	tx := new(ethTypes.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		fmt.Println("EEEEEEEEEEEEEEEEEEE", err)
		return common.Hash{}, err
	}
	fmt.Println("ssss-1", tx.To(), tx.To() != nil)
	fmt.Println("ssss-1", tx.To() != nil)
	fmt.Println("ssss-1", *tx.To())
	evmTx := new(types.EvmTransaction)
	if tx.To() != nil {
		evmTx = types.NewEvmTransaction(tx.Nonce(), *tx.To(), tx.Value(), tx.Gas(), tx.GasPrice(), 1, 1, netWorkID, 2, tx.Data(), 35760, 35760)
		fmt.Println("??????")
	} else {
		evmTx = types.NewEvmContractCreation(tx.Nonce(), tx.Value(), tx.Gas(), tx.GasPrice(), 1, 1, netWorkID, 2, tx.Data(), 35760, 35760)
	}
	fmt.Println("ssss-2")
	evmTx.SetVRS(tx.RawSignatureValues())

	txQkc := &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	fmt.Println("ssss-3")
	rlpTxBytes, err := rlp.EncodeToBytes(txQkc)
	if err != nil {
		fmt.Println("dasdasdsadsa", err)
		return common.Hash{}, err
	}
	fmt.Println("ssss-4")
	resp, err := s.c.Call("qkc_sendRawTransaction", hex.EncodeToString(rlpTxBytes))
	if err != nil {
		fmt.Println("1611111111", err)
		return common.Hash{}, nil
	}
	fmt.Println("ssss-5")
	fmt.Println(" SendRawTransaction resp", resp.Result)
	panic("SendRawTransaction")
	return txQkc.Hash(), nil
}
func (e *MetaMaskEthBlockChainAPI) Call(mdata MetaCallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	defaultToken := hexutil.Uint64(35760)
	ttFrom := new(account.Address)
	if mdata.From == nil {
		ttFrom = nil
	} else {
		ttFrom.Recipient = *mdata.From
		ttFrom.FullShardKey = 1
	}
	ttTo := new(account.Address)
	if mdata.To == nil {
		ttTo = nil
	} else {
		ttTo.Recipient = *mdata.To
		ttTo.FullShardKey = 1
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

	resp, err := e.c.Call("call", data)
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp.Result)
	panic("CALLL")
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
	panic("EstimateGas")
}
