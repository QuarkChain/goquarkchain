package qkcapi

import (
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

type MetaMaskEthBlockChainAPI struct {
	fullShardKey uint32
	c            jsonrpc.RPCClient
}

func NewMetaMaskEthAPI(fullShardKey uint32, client jsonrpc.RPCClient) *MetaMaskEthBlockChainAPI {
	return &MetaMaskEthBlockChainAPI{fullShardKey: fullShardKey, c: client}
}

func (e *MetaMaskEthBlockChainAPI) ChainId() (hexutil.Uint64, error) {
	resp, err := e.c.Call("eth_chainId")
	if err != nil {
		return 0, err
	}
	fmt.Println("chainID", resp.Result)
	panic("ChainID")
}

func (e *MetaMaskEthBlockChainAPI) GasPrice(fullShardKey *hexutil.Uint) (hexutil.Uint64, error) {
	//fullShardId, err := getFullShardId(fullShardKey)
	//if err != nil {
	//	return hexutil.Uint64(0), err
	//}
	//data, err := e.b.GasPrice(account.Branch{Value: fullShardId}, qcom.TokenIDEncode(DefaultTokenID))
	//return hexutil.Uint64(data), nil

	panic("not support")
}

func (e *MetaMaskEthBlockChainAPI) GetBalance(address common.Address, blockNrOrHash rpc.BlockNumber) (*hexutil.Big, error) {
	qkcAddr := account.NewAddress(address, e.fullShardKey)
	resp, err := e.c.Call("eth_getBalances", qkcAddr.ToHex())
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
	resp, err := e.c.Call("eth_getMinorBlockByHeight", hexutil.EncodeUint64(uint64(e.fullShardKey)))
	if err != nil {
		return 0
	}
	fmt.Println("", resp.Result)
	panic("BlockNumber")
}

func (e *MetaMaskEthBlockChainAPI) GetBlockByNumber(blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	//height := blockNr.Uint64()
	//minorBlock, _, err := e.b.GetMinorBlockByHeight(&height, account.Branch{1}, fullTx)
	//if err != nil {
	//	return nil, err
	//}
	//if minorBlock == nil {
	//	return nil, errors.New("minor block is nil")
	//}
	//return encoder.MinorBlockEncoder(minorBlock, false, nil)
	panic("GetBlockByNumber")
}

func (e *MetaMaskEthBlockChainAPI) GetTransactionCount(address common.Address, blockNr rpc.BlockNumber) (hexutil.Uint64, error) {
	qkcAddr := account.NewAddress(address, e.fullShardKey)
	resp, err := e.c.Call("eth_getTransactionCount", qkcAddr.ToHex())
	if err != nil {
		return 0, err
	}
	fmt.Println("GetTransaction", resp.Result)
	panic("GetTransactionCount")
}

func (e *MetaMaskEthBlockChainAPI) GetCode(address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	qkcAddr := account.NewAddress(address, e.fullShardKey)

	resp, err := e.c.Call("eth_getCode", qkcAddr.ToHex())
	if err != nil {
		return nil, err
	}
	fmt.Println("GetCode", resp.Result)
	panic("GetCode")
}

func (s *MetaMaskEthBlockChainAPI) SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(ethTypes.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	evmTx := new(types.EvmTransaction)
	if tx.To() != nil {
		evmTx = types.NewEvmTransaction(tx.Nonce(), *tx.To(), tx.Value(), tx.Gas(), tx.GasPrice(), 1, 1, clusterCfg.Quarkchain.NetworkID, 2, tx.Data(), 35760, 35760)
	} else {
		evmTx = types.NewEvmContractCreation(tx.Nonce(), tx.Value(), tx.Gas(), tx.GasPrice(), 1, 1, clusterCfg.Quarkchain.NetworkID, 2, tx.Data(), 35760, 35760)
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

	resp, err := s.c.Call("eth_sendRawTransaction", rlpTxBytes)
	if err != nil {
		return common.Hash{}, nil
	}
	fmt.Println("resp", resp.Result)
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

	resp, err := e.c.Call("eth_call", data)
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

	resp, err := p.c.Call("eth_estimateGas", data)
	if err != nil {
		panic(err)
	}
	fmt.Println("resp", resp.Result)
	panic("EstimateGas")
}
