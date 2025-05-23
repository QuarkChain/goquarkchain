package qkcapi

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/QuarkChain/goquarkchain/account"
	qCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ybbus/jsonrpc"
)

type NetApi struct {
	c jsonrpc.RPCClient
}

func NewNetApi(c jsonrpc.RPCClient) *NetApi {
	return &NetApi{c: c}
}

func (e *NetApi) Version() string {
	resp, err := e.c.Call("net_version")
	if err != nil {
		return err.Error()
	}
	return resp.Result.(string)
}

type Web3Api struct {
}

func NewWeb3Api(c jsonrpc.RPCClient) *Web3Api {
	return &Web3Api{}
}

func (e *Web3Api) ClientVersion() string {
	return "GoQuarkChain/release:mainnet1.5.2"
}

type ShardAPI struct {
	fullShardID uint32
	chainID     uint32
	hashMap     map[common.Hash]common.Hash

	c jsonrpc.RPCClient
}

func NewShardAPI(fullShardID uint32, chainID uint32, client jsonrpc.RPCClient) *ShardAPI {
	return &ShardAPI{fullShardID: fullShardID, chainID: chainID, hashMap: make(map[common.Hash]common.Hash), c: client}
}

func (s *ShardAPI) ChainId() hexutil.Uint64 {
	return hexutil.Uint64(s.chainID)
}

func (s *ShardAPI) GasPrice() (*hexutil.Big, error) {
	resp, err := s.c.Call("gasPrice", hexutil.EncodeUint64(uint64(s.fullShardID)))
	if err != nil {
		return nil, err
	}
	gasPrice, err := hexutil.DecodeBig(resp.Result.(string))
	if gasPrice.Cmp(new(big.Int).SetUint64(0)) == 0 {
		gasPrice = new(big.Int).SetUint64(1)
	}
	return (*hexutil.Big)(gasPrice), nil
}

func (s *ShardAPI) GetBalance(address common.Address, blockNrOrHash string) (*hexutil.Big, error) {
	resp, err := s.c.Call("getBalances", account.NewAddress(address, s.fullShardID).ToHex(), blockNrOrHash)
	if err != nil {
		return nil, err
	}
	balances := resp.Result.(map[string]interface{})["balances"]
	for _, b := range balances.([]interface{}) {
		bInfo := b.(map[string]interface{})
		if strings.ToUpper((bInfo["tokenStr"]).(string)) == DefaultTokenID {
			b, err := hexutil.DecodeBig(bInfo["balance"].(string))
			if err != nil {
				return nil, err
			}
			return (*hexutil.Big)(b), nil
		}
	}
	return &hexutil.Big{}, nil
}

func (s *ShardAPI) BlockNumber() (hexutil.Uint64, error) {
	resp, err := s.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(s.fullShardID)))
	if err != nil {
		return 0, err
	}
	height, err := hexutil.DecodeUint64(resp.Result.(map[string]interface{})["height"].(string))
	return hexutil.Uint64(height), err
}

func reWriteBlockResult(block map[string]interface{}) map[string]interface{} {
	if block == nil {
		return nil
	}
	// Truncate fields which contain full shard id
	if block["id"] != nil {
		block["id"] = block["id"].(string)[:66]
	}
	if block["idPrevMinorBlock"] != nil {
		block["idPrevMinorBlock"] = block["idPrevMinorBlock"].(string)[:66]
	}
	if block["miner"] != nil {
		block["miner"] = block["miner"].(string)[:42]
	}
	// rename fields
	if block["height"] != nil {
		block["number"] = block["height"]
	}
	if block["hashPrevMinorBlock"] != nil {
		block["parentHash"] = block["hashPrevMinorBlock"]
	}
	if block["nonce"] != nil {
		nonce, _ := strconv.ParseUint(block["nonce"].(string)[2:], 16, 32)
		block["nonce"] = fmt.Sprintf("0x%016x", nonce)
	}
	if block["sha3Uncles"] == nil {
		block["sha3Uncles"] = types.EmptyUncleHash
	}
	if block["hashMerkleRoot"] != nil {
		block["transactionsRoot"] = block["hashMerkleRoot"]
	}
	if block["hashEvmStateRoot"] != nil {
		block["stateRoot"] = block["hashEvmStateRoot"]
	}
	if block["receiptsRoot"] == nil {
		block["receiptsRoot"] = common.ToHex(common.Hash{}.Bytes())
	}
	if block["logsBloom"] == nil {
		block["logsBloom"] = common.ToHex(make([]byte, types.BloomByteLength))
	}
	return block
}

func (s *ShardAPI) GetBlockByNumber(blockNr string, fullTx bool) (map[string]interface{}, error) {
	var (
		resp *jsonrpc.RPCResponse
		err  error
	)
	if blockNr == "latest" {
		resp, err = s.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(s.fullShardID)), nil, fullTx)
	} else {
		resp, err = s.c.Call("getMinorBlockByHeight", hexutil.EncodeUint64(uint64(s.fullShardID)), blockNr, fullTx)
	}

	if err != nil {
		return nil, err
	}
	block, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, errors.New("GetBlockByNumber failed")
	}
	return reWriteBlockResult(block), nil
}

func (s *ShardAPI) GetTransactionCount(address common.Address, blockNr string) (hexutil.Uint64, error) {
	var (
		resp *jsonrpc.RPCResponse
		err  error
	)
	if blockNr == "pending" {
		resp, err = s.c.Call("getTransactionCount", account.NewAddress(address, s.fullShardID).ToHex(), nil)
	} else {
		resp, err = s.c.Call("getTransactionCount", account.NewAddress(address, s.fullShardID).ToHex(), blockNr)
	}

	if err != nil {
		return 0, err
	}
	nonce, err := hexutil.DecodeUint64(resp.Result.(string))
	return hexutil.Uint64(nonce), err
}

func (s *ShardAPI) GetCode(address common.Address, blockNr string) (hexutil.Bytes, error) {
	var (
		resp *jsonrpc.RPCResponse
		err  error
	)
	if blockNr == "pending" {
		resp, err = s.c.Call("getCode", account.NewAddress(address, s.fullShardID).ToHex(), nil)
	} else {
		resp, err = s.c.Call("getCode", account.NewAddress(address, s.fullShardID).ToHex(), blockNr)
	}
	if err != nil {
		return nil, err
	}
	return hexutil.Decode(resp.Result.(string))
}

func (s *ShardAPI) GetBlockByHash(hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	resp, err := s.c.Call("getMinorBlockById", common.ToHex(append(hash.Bytes(), qCommon.Uint32ToBytes(s.fullShardID)...)), fullTx)
	if err != nil {
		return nil, err
	}
	block, ok := resp.Result.(map[string]interface{})
	if !ok {
		return nil, errors.New("GetBlockByHash failed")
	}
	return reWriteBlockResult(block), nil
}

func (s *ShardAPI) SendRawTransaction(encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(ethTypes.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	evmTx := new(types.EvmTransaction)
	if tx.To() != nil {
		evmTx = types.NewEvmTransaction(tx.Nonce(), *tx.To(), tx.Value(), tx.Gas(), tx.GasPrice(), s.fullShardID, s.fullShardID, s.chainID, 2, tx.Data(), 35760, 35760)
	} else {
		evmTx = types.NewEvmContractCreation(tx.Nonce(), tx.Value(), tx.Gas(), tx.GasPrice(), s.fullShardID, s.fullShardID, s.chainID, 2, tx.Data(), 35760, 35760)
	}
	evmTx.SetVRS(tx.RawSignatureValues())
	rlpTxBytes, err := rlp.EncodeToBytes(evmTx)
	if err != nil {
		return common.Hash{}, err
	}
	_, err = s.c.Call("sendRawTransaction", common.ToHex(rlpTxBytes))
	if err != nil {
		return common.Hash{}, err
	}

	txQkc := &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	s.hashMap[tx.Hash()] = txQkc.Hash()
	return txQkc.Hash(), nil
}

func (s *ShardAPI) getTxIDInShard(h common.Hash) []byte {
	fullShardIDBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(fullShardIDBytes, s.fullShardID)

	txID := make([]byte, 0)
	txID = append(txID, h.Bytes()...)
	txID = append(txID, fullShardIDBytes...)
	return txID
}

func (s *ShardAPI) GetTransactionByHash(ethhash common.Hash) (map[string]interface{}, error) {
	hash, ok := s.hashMap[ethhash]
	if !ok {
		hash = ethhash
	}
	resp, err := s.c.Call("getTransactionById", common.ToHex(s.getTxIDInShard(hash)))
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, nil
	}
	ans := resp.Result.(map[string]interface{})
	if ans["from"] != nil {
		ans["from"] = ans["from"].(string)[:42]
	}
	if ans["to"] != nil {
		to := ans["to"].(string)
		if len(to) > 42 {
			ans["to"] = to[:42]
		} else if len(to) <= 2 { // remove to if it is empty (0x)
			delete(ans, "to")
		}
	}
	if ans["data"] != nil {
		ans["input"] = ans["data"]
	}
	if ans["blockId"] != nil {
		ans["blockHash"] = ans["blockId"].(string)[:66]
	}
	if ans["blockHeight"] != nil {
		ans["blockNumber"] = ans["blockHeight"]
	}
	return ans, nil
}

func (s *ShardAPI) GetTransactionReceipt(ethhash common.Hash) (map[string]interface{}, error) {
	hash, ok := s.hashMap[ethhash]
	if !ok {
		hash = ethhash
	}
	resp, err := s.c.Call("getTransactionReceipt", common.ToHex(s.getTxIDInShard(hash)))
	if err != nil {
		return nil, err
	}
	if resp.Result == nil {
		return nil, nil
	}
	ans := resp.Result.(map[string]interface{})
	if ans["contractAddress"] != nil {
		contractAddress := ans["contractAddress"].(string)
		if len(contractAddress) > 42 {
			ans["contractAddress"] = contractAddress[:42]
		}
	}
	if ans["logsBloom"] == nil {
		ans["logsBloom"] = common.ToHex(make([]byte, types.BloomByteLength))
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

func (s *ShardAPI) toCallJsonArg(isCall bool, mdata MetaCallArgs) interface{} {
	defaultToken := hexutil.Uint64(35760)
	arg := make(map[string]interface{})

	if mdata.From != nil {
		arg["from"] = account.Address{
			Recipient:    *mdata.From,
			FullShardKey: s.fullShardID,
		}.ToHex()
	}
	if mdata.To != nil {
		arg["to"] = account.Address{
			Recipient:    *mdata.To,
			FullShardKey: s.fullShardID,
		}.ToHex()
	}

	arg["gas"] = mdata.Gas
	arg["gasPrice"] = mdata.GasPrice
	arg["value"] = mdata.Value
	arg["data"] = mdata.Data
	arg["gas_token_id"] = &defaultToken
	arg["transfer_token_id"] = &defaultToken
	if isCall {
		return arg
	}

	estimates := make([]map[string]interface{}, 0)
	estimates = append(estimates, arg)
	return estimates
}

func (s *ShardAPI) Call(mdata MetaCallArgs, blockNr string) (hexutil.Bytes, error) {
	resp, err := s.c.Call("call", s.toCallJsonArg(true, mdata), blockNr)
	if err != nil {
		return nil, err
	}

	_, ok := resp.Result.(string)
	if !ok {
		return nil, errors.New("call failed")
	}
	return hexutil.Decode(resp.Result.(string))
}

func (s *ShardAPI) EstimateGas(mdata MetaCallArgs) (hexutil.Uint, error) {
	resp, err := s.c.Call("estimateGas", s.toCallJsonArg(false, mdata))
	if err != nil {
		return hexutil.Uint(21000), err
	}
	gasLimit, ok := resp.Result.(string)
	if !ok {
		return hexutil.Uint(21000), errors.New("estimateGas failed")
	}
	ans, err := hexutil.DecodeUint64(gasLimit)
	return hexutil.Uint(ans), err
}
