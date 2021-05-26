package qkcapi

import (
	"errors"
	"fmt"

	"github.com/QuarkChain/goquarkchain/account"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/encoder"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)

type MetaMaskEthBlockChainAPI struct {
	CommonAPI
	b Backend
}

func NewMetaMaskEthAPI(b Backend) *MetaMaskEthBlockChainAPI {
	return &MetaMaskEthBlockChainAPI{b: b, CommonAPI: CommonAPI{b}}
}

func (e *MetaMaskEthBlockChainAPI) ChainId() (hexutil.Uint64, error) {
	return hexutil.Uint64(666), nil
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

func (e *MetaMaskEthBlockChainAPI) getHeightFromBlockNumberOrHash(blockNrOrHash rpc.BlockNumberOrHash) (*uint64, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		if blockNr.Int64() == 0 {
			zero := uint64(0)
			return &zero, nil
		} else if blockNr.Int64() == -1 {
			return nil, nil
		} else if blockNr.Int64() == -2 {
			panic("not support yet")
		} else {
			t := blockNr.Uint64()
			return &t, nil
		}
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		block, _, err := e.b.GetMinorBlockByHash(hash, account.Branch{Value: 1}, false)
		if err != nil {
			return nil, err
		}
		tt := block.NumberU64()
		return &tt, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (e *MetaMaskEthBlockChainAPI) GetBalance(address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Big, error) {
	fullShardId := uint32(1)

	addr := account.NewAddress(address, fullShardId)
	height, err := e.getHeightFromBlockNumberOrHash(blockNrOrHash)
	if err != nil {
		return nil, err
	}
	data, err := e.b.GetPrimaryAccountData(&addr, height)
	if err != nil {
		return nil, err
	}
	balance := data.Balance.GetTokenBalance(qcom.TokenIDEncode(DefaultTokenID))
	return (*hexutil.Big)(balance), nil
}

func (e *MetaMaskEthBlockChainAPI) BlockNumber() hexutil.Uint64 {
	stats, err := e.b.GetStats()
	if err != nil {
		panic(err)
	}
	shards := stats["shards"].([]map[string]interface{})
	for _, shard := range shards {
		fullShardId := shard["fullShardId"].(uint32)
		if fullShardId == 1 {
			return hexutil.Uint64(shard["height"].(uint64))
		}
	}
	panic("bug here-")
}

func (e *MetaMaskEthBlockChainAPI) GetBlockByNumber(blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	height := blockNr.Uint64()
	minorBlock, _, err := e.b.GetMinorBlockByHeight(&height, account.Branch{1}, fullTx)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
	}
	return encoder.MinorBlockEncoder(minorBlock, false, nil)
}

func (e *MetaMaskEthBlockChainAPI) GetTransactionCount(address common.Address, blockNr rpc.BlockNumber) (hexutil.Uint64, error) {
	addr := account.NewAddress(address, 1)
	height := new(uint64)
	if blockNr == -1 {
		height = nil
	} else if blockNr == -2 {
		panic("sb")
	} else {
		tmp := blockNr.Uint64()
		height = &tmp
	}

	data, err := e.b.GetPrimaryAccountData(&addr, height)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(data.TransactionCount), nil
}

func (e *MetaMaskEthBlockChainAPI) GetCode(address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	addr := account.NewAddress(address, 1)
	height := blockNr.Uint64()
	return e.b.GetCode(&addr, &height)
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

	if err := s.b.AddTransaction(txQkc); err != nil {
		return common.Hash{}, err
	}
	fmt.Println("????????????????", txQkc.Hash().String())
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
	ans, err := e.CommonAPI.callOrEstimateGas(data, nil, true)
	return ans, err
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

	gas, err := p.CommonAPI.callOrEstimateGas(data, nil, false)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint(4051056), nil
	return hexutil.Uint(qcom.BytesToUint32(gas)), nil
}
