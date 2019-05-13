package slave

import (
	"context"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"sync"
	"time"
)

type SlaveServerSideOp struct {
	rpcId int64
	mu    sync.RWMutex
	slave *SlaveBackend

	run     uint8
	curTime int64
}

func NewServerSideOp(slave *SlaveBackend) *SlaveServerSideOp {
	return &SlaveServerSideOp{
		slave: slave,
	}
}

func (s *SlaveServerSideOp) HeartBeat(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	s.curTime = time.Now().Unix()
	log.Info("slave heart beat response", "request op", req.Op, "current time", s.curTime)
	return &rpc.Response{
	}, nil
}

func (s *SlaveServerSideOp) MasterInfo(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.MasterInfo
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	s.slave.connManager.ModifyTarget(fmt.Sprintf("%s:%d", gReq.Ip, gReq.Port))
	log.Info("slave master info response", "master endpoint", s.slave.connManager.masterCli.target)

	return response, nil
}

func (s *SlaveServerSideOp) Ping(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.Ping
		gRep     rpc.Pong
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gReq.RootTip != nil {
		if err = s.slave.CreateShards(gReq.RootTip); err != nil {
			return nil, err
		}
	}
	gRep.Id, gRep.ChainMaskList = []byte(s.slave.config.ID), s.slave.config.ChainMaskList
	log.Info("slave ping response", "request op", req.Op, "rpc id", s.rpcId)

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) GetMine(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetMinorBlockRequest
		gRep     rpc.GetMinorBlockResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gReq.MinorBlockHash != (common.Hash{}) {
		if gRep.MinorBlock, err = s.slave.GetMinorBlockByHeight(gReq.Height, gReq.Branch); err != nil {
			return nil, err
		}
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GenTx(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GenTxRequest
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	// TODO CreateTransactions

	return response, nil

}

func (s *SlaveServerSideOp) AddRootBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddRootBlockRequest
		gRep     rpc.AddRootBlockResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	gRep.Switched, err = s.slave.AddRootBlock(gReq.RootBlock)

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}

	if err = s.slave.CreateShards(gReq.RootBlock); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetEcoInfoList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gRep     rpc.GetEcoInfoListResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	// TODO fill content.

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) AddMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq       rpc.AddMinorBlockRequest
		buf        = serialize.NewByteBuffer(req.Data)
		minorBlock = types.MinorBlock{}
		mBuf       *serialize.ByteBuffer
		response   = &rpc.Response{}
		err        error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	mBuf = serialize.NewByteBuffer(gReq.MinorBlockData)
	if err = serialize.Deserialize(mBuf, &minorBlock); err != nil {
		return nil, err
	}
	if err = s.slave.AddMinorBlock(&minorBlock); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) GetUnconfirmedHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gRep     rpc.GetUnconfirmedHeadersResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if gRep.HeadersInfoList, err = s.slave.GetUnconfirmedHeaderList(); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetAccountData(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetAccountDataRequest
		gRep     rpc.GetAccountDataResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.AccountBranchDataList, err = s.slave.GetAccountData(gReq.Address, *gReq.BlockHeight); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) AddTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddTransactionRequest
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.AddTx(gReq.Tx); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) GetMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetMinorBlockRequest
		gRep     rpc.GetMinorBlockResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gReq.MinorBlockHash != (common.Hash{}) {
		if gRep.MinorBlock, err = s.slave.GetMinorBlockByHash(gReq.MinorBlockHash, gReq.Branch); err != nil {
			return nil, err
		}
	} else {
		if gRep.MinorBlock, err = s.slave.GetMinorBlockByHeight(gReq.Height, gReq.Branch); err != nil {
			return nil, err
		}
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetTransactionRequest
		gRep     rpc.GetTransactionResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.MinorBlock, gRep.Index, err = s.slave.GetTransactionByHash(gReq.TxHash, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) ExecuteTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.ExecuteTransactionRequest
		gRep     rpc.ExecuteTransactionResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.Result, err = s.slave.ExecuteTx(gReq.Tx, gReq.FromAddress); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetTransactionReceipt(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetTransactionReceiptRequest
		gRep     rpc.GetTransactionReceiptResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	response = &rpc.Response{RpcId: req.RpcId}
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	gRep.MinorBlock, gRep.Index, gRep.Receipt, err = s.slave.GetTransactionReceipt(gReq.TxHash, gReq.Branch)
	if err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetTransactionListByAddress(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetTransactionListByAddressRequest
		gRep     rpc.GetTransactionListByAddressResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	response = &rpc.Response{RpcId: req.RpcId}
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.TxList, gRep.Next, err = s.slave.GetTransactionListByAddress(gReq.Address,
		gReq.Start, gReq.Limit); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetLogs(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetLogRequest
		gRep     rpc.GetLogResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	response = &rpc.Response{RpcId: req.RpcId}
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.Logs, err = s.slave.GetLogs(gReq.Addresses, gReq.StartBlock, gReq.EndBlock, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) EstimateGas(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.EstimateGasRequest
		gRep     rpc.EstimateGasResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	response = &rpc.Response{RpcId: req.RpcId}
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.Result, err = s.slave.EstimateGas(gReq.Tx, gReq.FromAddress); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetStorageAt(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetStorageRequest
		gRep     rpc.GetStorageResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.Result, err = s.slave.GetStorageAt(gReq.Address, gReq.Key, *gReq.BlockHeight); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetCode(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetCodeRequest
		gRep     rpc.GetCodeResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.Result, err = s.slave.GetCode(gReq.Address, *gReq.BlockHeight); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GasPrice(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GasPriceRequest
		gRep     rpc.GasPriceResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.Result, err = s.slave.GasPrice(gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetWork(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetWorkRequest
		gRep     rpc.GetWorkResponse
		buf      = serialize.NewByteBuffer(req.Data)
		work     *consensus.MiningWork
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if work, err = s.slave.GetWork(gReq.Branch); err != nil {
		return nil, err
	}
	gRep.HeaderHash, gRep.Height, gRep.Difficulty = work.HeaderHash, work.Number, work.Difficulty

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) SubmitWork(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.SubmitWorkRequest
		gRep     rpc.SubmitWorkResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.SubmitWork(gReq.HeaderHash, gReq.Nonce, gReq.MixHash, gReq.Branch); err != nil {
		return nil, err
	}
	gRep.Success = true

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) AddXshardTxList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddXshardTxListRequest
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.AddCrossShardTxListByMinorBlockHash(gReq.MinorBlockHash, gReq.TxList, gReq.Branch); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) BatchAddXshardTxList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.BatchAddXshardTxListRequest
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	for _, req := range gReq.AddXshardTxListRequestList {
		if err = s.slave.AddCrossShardTxListByMinorBlockHash(req.MinorBlockHash, req.TxList, req.Branch); err != nil {
			return nil, err
		}
	}

	return response, nil
}

// check if the blocks are vailed.
func (s *SlaveServerSideOp) AddMinorBlockListForSync(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddBlockListForSyncRequest
		gRep     rpc.AddBlockListForSyncResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.ShardStatus, err = s.slave.AddBlockListForSync(gReq.MinorBlockHashList, gReq.PeerId, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

// p2p apis.
func (s *SlaveServerSideOp) GetMinorBlockList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetMinorBlockListRequest
		gRep     rpc.GetMinorBlockListResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if gRep.MinorBlockList, err = s.slave.GetMinorBlockListByHashList(gReq.MinorBlockHashList, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetMinorBlockHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     p2p.GetMinorBlockHeaderListRequest
		gRep     p2p.GetMinorBlockHeaderListResponse
		buf      = serialize.NewByteBuffer(req.Data)
		blockLst []*types.MinorBlock
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if blockLst, err = s.slave.GetMinorBlockListByDirection(gReq.BlockHash, gReq.Limit, gReq.Direction, gReq.Branch.Value); err == nil {
		for _, blok := range blockLst {
			gRep.BlockHeaderList = append(gRep.BlockHeaderList, blok.Header())
		}
	} else {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) HandleNewTip(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     p2p.Tip
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.HandleNewTip(&gReq); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) AddTransactions(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     p2p.NewTransactionList
		gRep     rpc.HashList
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	for _, tx := range gReq.TransactionList {
		if err = s.slave.AddTx(tx); err != nil {
			return nil, err
		}
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) NewMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     types.MinorBlock
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}
	if err = s.slave.NewMinorBlock(&gReq); err != nil {
		return nil, err
	}

	return response, nil
}
