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
	fullShardID uint32
	chainID     uint32
	c           jsonrpc.RPCClient
}

func NewMetaMaskEthAPI(fullShardKey uint32, chainID uint32, client jsonrpc.RPCClient) *MetaMaskEthBlockChainAPI {
	return &MetaMaskEthBlockChainAPI{fullShardID: fullShardKey, chainID: chainID, c: client}
}

func (e *MetaMaskEthBlockChainAPI) ChainId() hexutil.Uint64 {
	return hexutil.Uint64(e.chainID)
}

func (e *MetaMaskEthBlockChainAPI) GasPrice() (hexutil.Uint64, error) {
	resp, err := e.c.Call("gasPrice", hexutil.EncodeUint64(uint64(e.fullShardID)))
	gasPrice, err := hexutil.DecodeUint64(resp.Result.(string))
	return hexutil.Uint64(gasPrice), err
}

func (e *MetaMaskEthBlockChainAPI) GetBalance(address common.Address, blockNrOrHash rpc.BlockNumber) (*hexutil.Big, error) {
	qkcAddr := account.NewAddress(address, e.fullShardID)
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
	fmt.Println("Start:", "getMinorBlockByHeight")
	resp, err := e.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(e.fullShardID)))
	if err != nil {
		return 0
	}
	height, _ := hexutil.DecodeUint64(resp.Result.(map[string]interface{})["height"].(string))
	fmt.Println("End: getMinorBlockByHeight", height)
	return hexutil.Uint64(height)
}

func (e *MetaMaskEthBlockChainAPI) GetBlockByNumber(blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	resp, err := e.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(e.fullShardID)), nil, false)
	if err != nil {
		return nil, err
	}
	return resp.Result.(map[string]interface{}), nil
}

func (e *MetaMaskEthBlockChainAPI) GetTransactionCount(address common.Address, blockNr rpc.BlockNumber) (hexutil.Uint64, error) {
	qkcAddr := account.NewAddress(address, e.fullShardID)
	resp, err := e.c.Call("getTransactionCount", qkcAddr.ToHex())
	if err != nil {
		return 0, err
	}
	nonce, err := hexutil.DecodeUint64(resp.Result.(string))
	return hexutil.Uint64(nonce), err
}

func (e *MetaMaskEthBlockChainAPI) GetCode(address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	qkcAddr := account.NewAddress(address, e.fullShardID)

	resp, err := e.c.Call("getCode", qkcAddr.ToHex())
	if err != nil {
		return nil, err
	}
	return hexutil.Decode(resp.Result.(string))
}

func (s *MetaMaskEthBlockChainAPI) SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(ethTypes.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	evmTx := new(types.EvmTransaction)
	if tx.To() != nil {
		evmTx = types.NewEvmTransaction(tx.Nonce(), *tx.To(), tx.Value(), tx.Gas(), tx.GasPrice(), s.fullShardID, s.fullShardID, netWorkID, 2, tx.Data(), 35760, 35760)
	} else {
		evmTx = types.NewEvmContractCreation(tx.Nonce(), tx.Value(), tx.Gas(), tx.GasPrice(), s.fullShardID, s.fullShardID, netWorkID, 2, tx.Data(), 35760, 35760)
	}
	evmTx.SetVRS(tx.RawSignatureValues())
	rlpTxBytes, err := rlp.EncodeToBytes(evmTx)
	if err != nil {
		return common.Hash{}, err
	}
	_, err = s.c.Call("sendRawTransaction", common.ToHex(rlpTxBytes))
	if err != nil {
		return common.Hash{}, nil
	}

	txQkc := &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	fmt.Println("????????????????????????SSSSSSSSSS", txQkc.Hash().String())
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
	txID = append(txID, Uint32ToBytes(c.fullShardID)...)
	resp, err := c.c.Call("getTransactionById", common.ToHex(txID))
	if err != nil {
		return nil, err
	}
	return resp.Result.(map[string]interface{}), nil
}
func (c *MetaMaskEthBlockChainAPI) GetTransactionReceipt(hash common.Hash) (map[string]interface{}, error) {
	txID := make([]byte, 0)
	txID = append(txID, hash.Bytes()...)
	txID = append(txID, Uint32ToBytes(c.fullShardID)...)
	resp, err := c.c.Call("getTransactionReceipt", common.ToHex(txID))
	fmt.Println("resp", resp, err)
	if err != nil {
		return nil, err
	}
	ans := resp.Result.(map[string]interface{})

	if len(ans["contractAddress"].(string)) != 0 {
		ans["contractAddress"] = ans["contractAddress"].(string)[:42]
	}
	return ans, nil
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

func (e *MetaMaskEthBlockChainAPI) toCallJsonArg(iscall bool, mdata MetaCallArgs) interface{} {
	defaultToken := hexutil.Uint64(35760)
	ttFrom := new(account.Address)
	if mdata.From == nil {
	} else {
		ttFrom.Recipient = *mdata.From
		ttFrom.FullShardKey = e.fullShardID
	}
	ttTo := new(account.Address)
	if mdata.To == nil {
	} else {
		ttTo.Recipient = *mdata.To
		ttTo.FullShardKey = e.fullShardID
	}

	arg := make(map[string]interface{})
	if mdata.From != nil {
		arg["from"] = account.Address{
			Recipient:    *mdata.From,
			FullShardKey: e.fullShardID,
		}.ToHex()
	}
	if mdata.To != nil {
		arg["to"] = account.Address{
			Recipient:    *mdata.To,
			FullShardKey: e.fullShardID,
		}.ToHex()
	}

	arg["gas"] = mdata.Gas
	arg["gasPrice"] = mdata.GasPrice
	arg["value"] = mdata.Value
	arg["data"] = mdata.Data
	arg["gas_token_id"] = &defaultToken
	arg["transfer_token_id"] = &defaultToken
	if iscall {
		return arg
	}

	estimates := make([]map[string]interface{}, 0)
	estimates = append(estimates, arg)
	return estimates
}

func (e *MetaMaskEthBlockChainAPI) Call(mdata MetaCallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	resp, err := e.c.Call("call", e.toCallJsonArg(true, mdata), hexutil.Uint64(blockNr.Uint64()))
	fmt.Println("call-----", mdata, blockNr.Uint64(), e.toCallJsonArg(true, mdata))
	if err != nil {
		panic(err)
	}
	return hexutil.Decode(resp.Result.(string))
}
func (p *MetaMaskEthBlockChainAPI) EstimateGas(mdata MetaCallArgs) (hexutil.Uint, error) {
	resp, err := p.c.Call("estimateGas", p.toCallJsonArg(false, mdata))
	fmt.Println("estimate err", err, p.toCallJsonArg(false, mdata))
	if err != nil {
		panic(err)
	}
	ans, err := hexutil.DecodeUint64(resp.Result.(string))
	return hexutil.Uint(ans), err
}
