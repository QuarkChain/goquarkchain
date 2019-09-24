package qkcapi

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
	"sync"
)

var (
	EmptyTxID = IDEncoder(common.Hash{}.Bytes(), 0)

	once           sync.Once
	clusterCfg     *config.ClusterConfig
	DefaultTokenID = "QKC"
)

func getFullShardId(fullShardKey *hexutil.Uint) (fullShardId uint32, err error) {
	if fullShardKey != nil {
		fullShardId, err = clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(uint32(*fullShardKey))
		if err != nil {
			return
		}
	}
	return 0, nil
}

func callOrEstimateGas(b Backend, args *CallArgs, height *uint64, isCall bool) (hexutil.Bytes, error) {
	if args.To == nil {
		return nil, errors.New("missing to")
	}
	args.setDefaults()
	tx, err := args.toTx(b.GetClusterConfig().Quarkchain)
	if err != nil {
		return nil, err
	}
	if isCall {
		res, err := b.ExecuteTransaction(tx, args.From, height)
		if err != nil {
			return nil, err
		}
		return (hexutil.Bytes)(res), nil
	}
	data, err := b.EstimateGas(tx, args.From)
	if err != nil {
		return nil, err
	}
	return qcom.Uint32ToBytes(data), nil
}

func sendRawTransaction(b Backend, encodedTx hexutil.Bytes) (hexutil.Bytes, error) {
	evmTx := new(types.EvmTransaction)
	if err := rlp.DecodeBytes(encodedTx, evmTx); err != nil {
		return nil, err
	}
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}

	if err := b.AddTransaction(tx); err != nil {
		return EmptyTxID, err
	}
	return IDEncoder(tx.Hash().Bytes(), tx.EvmTx.FromFullShardKey()), nil
}

func getTransactionReceipt(b Backend, txID hexutil.Bytes) (map[string]interface{}, error) {
	txHash, fullShardKey, err := IDDecoder(txID)
	if err != nil {
		return nil, err
	}

	fullShardId, err := clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardId}
	minorBlock, index, receipt, err := b.GetTransactionReceipt(txHash, branch)
	if err != nil {
		return nil, err
	}
	ret, err := receiptEncoder(minorBlock, int(index), receipt)
	if ret["transactionId"].(string) == "" {
		ret["transactionId"] = txID.String()
		ret["transactionHash"] = txHash.String()
	}
	return ret, err
}

func convertEthCallData(data *EthCallArgs, fullShardKey *hexutil.Uint) (*CallArgs, error) {
	fullShardId, err := getFullShardId(fullShardKey)
	if err != nil {
		return nil, err
	}
	args := &CallArgs{
		Gas:hexutil.Big(),
	}
	if data.To != nil {
		addr := account.NewAddress(*data.To, fullShardId)
		args.To = &addr
	}
	from := account.NewAddress(data.From, fullShardId)
	args.From = &from
}
