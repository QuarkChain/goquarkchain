package test

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
)

type SlaveServerSideOp struct {
}

func NewFakeServerSideOp() *SlaveServerSideOp {
	return &SlaveServerSideOp{}
}

func (s *SlaveServerSideOp) HeartBeat(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{}, nil
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

	return response, nil
}

func (s *SlaveServerSideOp) Ping(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     = new(rpc.Ping)
		gRep     = new(rpc.Pong)
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

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

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
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

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) AddMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddMinorBlockRequest
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
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

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetTransactionListByAddress(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetTransactionListByAddressRequest
		gRep     rpc.GetTxDetailResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	response = &rpc.Response{RpcId: req.RpcId}
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetAllTx(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetAllTxRequest
		gRep     rpc.GetTxDetailResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	response = &rpc.Response{RpcId: req.RpcId}
	if err = serialize.Deserialize(buf, &gReq); err != nil {
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
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

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

	return response, nil
}

func (s *SlaveServerSideOp) AddMinorBlockListForSync(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddBlockListForSyncRequest
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
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

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetMinorBlockHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     p2p.GetMinorBlockHeaderListWithSkipRequest
		gRep     rpc.GetMinorBlockHeaderListResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	gRep.MinorBlockHeaderList = make([]*types.MinorBlockHeader, 0)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
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
		gRep     rpc.GetTransactionResponse
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
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

	if response.Data, err = serialize.SerializeToBytes(gRep); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) HandleNewMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     types.MinorBlock
		buf      = serialize.NewByteBuffer(req.Data)
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.Deserialize(buf, &gReq); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) SetMining(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		mining   bool
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &mining); err != nil {
		return nil, err
	}
	return response, nil
}
