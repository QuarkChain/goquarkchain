package test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	grpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cluster/slave"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
)

type testCase struct {
	request   *grpc.Request
	checkFunc func(t *testing.T, response *grpc.Response)
}

func fakeSlaveBackend() (*slave.SlaveBackend, error) {
	var (
		ctx        = &service.ServiceContext{}
		clusterCfg = config.NewClusterConfig()
	)
	return slave.New(ctx, clusterCfg, clusterCfg.SlaveList[0])
}

func casesData(t *testing.T, slv *slave.SlaveBackend) map[int]*grpc.Request {
	evmTx := types.NewEvmTransaction(0, account.Recipient{}, new(big.Int), 0, new(big.Int), 0, 0, 0, 0, nil, 0, 0)
	var (
		fakeTx   = &types.Transaction{EvmTx: evmTx, TxType: types.EvmTx}
		fakeAddr = account.CreatEmptyAddress(0)
		fakeHash = common.EmptyHash
		dt       = make(map[int]*grpc.Request)
		err      error
	)
	dt[grpc.OpMasterInfo] = &grpc.Request{Op: grpc.OpMasterInfo, Data: nil}
	dt[grpc.OpPing] = &grpc.Request{Op: grpc.OpPing, Data: nil}
	dt[grpc.OpAddRootBlock] = &grpc.Request{Op: grpc.OpAddRootBlock, Data: nil}
	dt[grpc.OpGetNextBlockToMine] = &grpc.Request{Op: grpc.OpGetNextBlockToMine, Data: nil}
	dt[grpc.OpGetUnconfirmedHeaderList] = &grpc.Request{Op: grpc.OpGetUnconfirmedHeaderList, Data: nil}
	dt[grpc.OpGetAccountData] = &grpc.Request{Op: grpc.OpGetAccountData, Data: nil}
	dt[grpc.OpAddTransaction] = &grpc.Request{Op: grpc.OpAddTransaction, Data: nil}
	dt[grpc.OpAddXshardTxList] = &grpc.Request{Op: grpc.OpAddXshardTxList, Data: nil}
	dt[grpc.OpGetMinorBlock] = &grpc.Request{Op: grpc.OpGetMinorBlock, Data: nil}
	dt[grpc.OpGetTransaction] = &grpc.Request{Op: grpc.OpGetTransaction, Data: nil}
	dt[grpc.OpBatchAddXshardTxList] = &grpc.Request{Op: grpc.OpBatchAddXshardTxList, Data: nil}
	dt[grpc.OpExecuteTransaction] = &grpc.Request{Op: grpc.OpExecuteTransaction, Data: nil}
	dt[grpc.OpGetTransactionReceipt] = &grpc.Request{Op: grpc.OpGetTransactionReceipt, Data: nil}
	dt[grpc.OpGenTx] = &grpc.Request{Op: grpc.OpGenTx, Data: nil}
	dt[grpc.OpGetTransactionListByAddress] = &grpc.Request{Op: grpc.OpGetTransactionListByAddress, Data: nil}
	dt[grpc.OpEstimateGas] = &grpc.Request{Op: grpc.OpEstimateGas, Data: nil}
	dt[grpc.OpGetStorageAt] = &grpc.Request{Op: grpc.OpGetStorageAt, Data: nil}
	dt[grpc.OpGetCode] = &grpc.Request{Op: grpc.OpGetCode, Data: nil}
	dt[grpc.OpGasPrice] = &grpc.Request{Op: grpc.OpGasPrice, Data: nil}
	dt[grpc.OpGetWork] = &grpc.Request{Op: grpc.OpGetWork, Data: nil}
	dt[grpc.OpSubmitWork] = &grpc.Request{Op: grpc.OpSubmitWork, Data: nil}

	dt[grpc.OpMasterInfo].Data, err = serialize.SerializeToBytes(grpc.MasterInfo{Ip: "127.0.0.1", Port: 38000})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpPing].Data, err = serialize.SerializeToBytes(
		grpc.Ping{
			Id:            []byte(slv.GetConfig().ID),
			ChainMaskList: slv.GetConfig().FullShardList,
		},
	)
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpAddRootBlock].Data, err = serialize.SerializeToBytes(grpc.AddRootBlockRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGetAccountData].Data, err = serialize.SerializeToBytes(grpc.GetAccountDataRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpAddTransaction].Data, err = serialize.SerializeToBytes(grpc.AddTransactionRequest{Tx: fakeTx})
	if err != nil {
		t.Fatalf("Failed to serialize AddTransactionRequest, err %v", err)
	}

	dt[grpc.OpAddXshardTxList].Data, err = serialize.SerializeToBytes(grpc.AddXshardTxListRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGetMinorBlock].Data, err = serialize.SerializeToBytes(grpc.GetMinorBlockRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGetTransaction].Data, err = serialize.SerializeToBytes(grpc.GetTransactionRequest{TxHash: fakeHash})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpBatchAddXshardTxList].Data, err = serialize.SerializeToBytes(grpc.BatchAddXshardTxListRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpExecuteTransaction].Data, err = serialize.SerializeToBytes(grpc.ExecuteTransactionRequest{Tx: fakeTx})
	if err != nil {
		t.Fatalf("Failed to serialize ExecuteTransactionRequest, err %v", err)
	}

	dt[grpc.OpGetTransactionReceipt].Data, err = serialize.SerializeToBytes(grpc.GetTransactionReceiptRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGenTx].Data, err = serialize.SerializeToBytes(grpc.GenTxRequest{Tx: fakeTx, NumTxPerShard: 0})
	if err != nil {
		t.Fatalf("Failed to serialize GenTxRequest, err %v", err)
	}

	dt[grpc.OpGetTransactionListByAddress].Data, err = serialize.SerializeToBytes(grpc.GetTransactionListByAddressRequest{Address: &fakeAddr})
	if err != nil {
		t.Fatalf("Failed to serialize GetTransactionListByAddressRequest, err %v", err)
	}

	dt[grpc.OpEstimateGas].Data, err = serialize.SerializeToBytes(grpc.EstimateGasRequest{Tx: fakeTx, FromAddress: &fakeAddr})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGetStorageAt].Data, err = serialize.SerializeToBytes(grpc.GetStorageRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGetCode].Data, err = serialize.SerializeToBytes(grpc.GetCodeRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGasPrice].Data, err = serialize.SerializeToBytes(grpc.GasPriceRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpGetWork].Data, err = serialize.SerializeToBytes(grpc.GetWorkRequest{})
	if err != nil {
		t.Fatalf("")
	}

	dt[grpc.OpSubmitWork].Data, err = serialize.SerializeToBytes(grpc.SubmitWorkRequest{})
	if err != nil {
		t.Fatalf("")
	}

	return dt
}

func casesFuncs() map[int]func(t *testing.T, response *grpc.Response) {
	var (
		checkFuncs = make(map[int]func(t *testing.T, response *grpc.Response))
	)

	checkFuncs[grpc.OpMasterInfo] = func(t *testing.T, res *grpc.Response) {}

	checkFuncs[grpc.OpPing] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.Pong
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize Pong, err %v", err)
		}
	}

	checkFuncs[grpc.OpAddRootBlock] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.AddRootBlockResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize AddRootBlockResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpGetUnconfirmedHeaderList] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetUnconfirmedHeadersResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetUnconfirmedHeadersResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpGetAccountData] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetAccountDataResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetAccountDataResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpAddTransaction] = func(t *testing.T, res *grpc.Response) {}

	checkFuncs[grpc.OpAddXshardTxList] = func(t *testing.T, res *grpc.Response) {}

	checkFuncs[grpc.OpGetMinorBlock] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetMinorBlockResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetMinorBlockResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpGetTransaction] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetTransactionResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetTransactionResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpBatchAddXshardTxList] = func(t *testing.T, res *grpc.Response) {}

	checkFuncs[grpc.OpExecuteTransaction] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.ExecuteTransactionResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize ExecuteTransactionResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpGetTransactionReceipt] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetTransactionReceiptResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetTransactionReceiptResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpGenTx] = func(t *testing.T, res *grpc.Response) {}

	checkFuncs[grpc.OpGetTransactionListByAddress] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetTxDetailResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetTxDetailResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpEstimateGas] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.EstimateGasResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize EstimateGasResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpGetStorageAt] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetStorageResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetStorageResponse, err %v", err)
		}

	}

	checkFuncs[grpc.OpGetCode] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetCodeResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetCodeResponse, err %v", err)
		}
	}

	checkFuncs[grpc.OpGasPrice] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GasPriceResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GasPriceResponse, err %v", err)
		}
	}

	checkFuncs[grpc.OpGetWork] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.GetWorkResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize GetWorkResponse, err %v", err)
		}
	}

	checkFuncs[grpc.OpSubmitWork] = func(t *testing.T, res *grpc.Response) {
		var (
			gRep grpc.SubmitWorkResponse
			buf  = serialize.NewByteBuffer(res.Data)
			err  error
		)
		if err = serialize.Deserialize(buf, &gRep); err != nil {
			t.Fatalf("Failed to deserialize SubmitWorkResponse, err %v", err)
		}
	}

	return checkFuncs
}

func casesAndCheck(t *testing.T, slv *slave.SlaveBackend) []*testCase {
	var (
		bt = make([]*testCase, 0)
	)

	casesData := casesData(t, slv)

	checkFuncs := casesFuncs()
	for op, res := range casesData {
		if checkFuncs[op] == nil {
			continue
		}
		bt = append(bt, &testCase{
			request:   res,
			checkFunc: checkFuncs[op],
		})
	}
	return bt
}

func TestSLaveGRPC(t *testing.T) {
	slave, err := fakeSlaveBackend()
	if err != nil {
		t.Fatalf("Failed to create a fake slave service")
	}
	target := fmt.Sprintf("%s:%d", slave.GetConfig().IP, slave.GetConfig().Port)

	apis := []rpc.API{
		{
			Namespace: "grpc",
			Version:   "3.0",
			Service:   NewFakeServerSideOp(),
			Public:    false,
		},
	}

	listener, handler, err := grpc.StartGRPCServer(target, apis)
	if err != nil {
		t.Fatalf("failed to create grpc server %v", err)
	}
	cli := grpc.NewClient(grpc.SlaveServer)

	// all slave gprc funcs test cases
	testCases := casesAndCheck(t, slave)
	if err != nil {
		t.Fatalf("Failed to create test cases data, err %v", err)
	}
	for _, tcs := range testCases {
		res, err := cli.Call(target, tcs.request)
		if err != nil {
			t.Fatalf("Failed call slave %s grpc func %s, err %v", slave.GetConfig().ID, cli.GetOpName(tcs.request.Op), err)
		}
		tcs.checkFunc(t, res)
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("close grpc server port error: %v", err)
	}
	handler.Stop()
}
