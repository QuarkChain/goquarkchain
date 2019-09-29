package qkcapi

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/internal/encoder"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
	"sync"
)

var (
	EmptyTxID = encoder.IDEncoder(common.Hash{}.Bytes(), 0)

	once           sync.Once
	clusterCfg     *config.ClusterConfig
	DefaultTokenID = "QKC"
)

func getFullShardId(fullShardKey *hexutil.Uint) (fullShardId uint32, err error) {
	if fullShardKey != nil {
		return clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(uint32(*fullShardKey))
	}
	return 1, nil
}

func convertEthCallData(data *EthCallArgs, fullShardKey *hexutil.Uint) (*CallArgs, error) {
	fullShardId, err := getFullShardId(fullShardKey)
	if err != nil {
		return nil, err
	}
	args := &CallArgs{
		Gas:      (hexutil.Big)(*big.NewInt(int64(data.Gas))),
		GasPrice: data.GasPrice,
		Value:    data.Value,
		Data:     data.Data,
	}
	if data.To != nil {
		addr := account.NewAddress(*data.To, fullShardId)
		args.To = &addr
	}
	from := account.NewAddress(data.From, fullShardId)
	args.From = &from

	return args, nil
}

func decodeBlockNumberToUint64(b Backend, blockNumber *rpc.BlockNumber) (*uint64, error) {
	if blockNumber == nil {
		return nil, nil
	}
	if *blockNumber == rpc.PendingBlockNumber {
		return nil, errors.New("is pending block number")
	}
	if *blockNumber == rpc.LatestBlockNumber {
		return nil, nil
	}
	if *blockNumber == rpc.EarliestBlockNumber {
		tBlock := uint64(0)
		return &tBlock, nil
	}

	if *blockNumber < 0 {
		return nil, errors.New("invalid block Num")
	}
	tBlock := uint64(blockNumber.Int64())
	return &tBlock, nil
}

func transHexutilUint64ToUint64(data *hexutil.Uint64) (*uint64, error) {
	if data == nil {
		return nil, nil
	}
	res := uint64(*data)
	return &res, nil
}
