package rpc

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"reflect"
	"sync"
	"time"
)

var (
	conns = make(map[string]*grpc.ClientConn)
	mu    sync.Mutex
	run   uint32 = 0

	rpcFuncs = map[int64]opType{
		1:  {name: "Ping"},
		2:  {name: "ConnectToSlaves"},
		3:  {name: "AddRootBlock"},
		4:  {name: "GetEcoInfoList"},
		5:  {name: "GetNextBlockToMine"},
		6:  {name: "GetUnconfirmedHeaders"},
		7:  {name: "GetAccountData"},
		8:  {name: "AddTransaction"},
		9:  {name: "AddMinorBlockHeader", ty: 1},
		10: {name: "AddXshardTxList"},
		11: {name: "SyncMinorBlockList"},
		12: {name: "AddMinorBlock"},
		13: {name: "CreateClusterPeerConnection"},
		14: {name: "DestroyClusterPeerConnectionCommand", ty: -1},
		15: {name: "GetMinorBlock"},
		16: {name: "GetTransaction"},
		17: {name: "BatchAddXshardTxList"},
		18: {name: "ExecuteTransaction"},
		19: {name: "GetTransactionReceipt"},
		20: {name: "GetMine"},
		21: {name: "GenTx"},
		22: {name: "GetTransactionListByAddress"},
		23: {name: "GetLogs"},
		24: {name: "EstimateGas"},
		25: {name: "GetStorageAt"},
		26: {name: "GetCode"},
		27: {name: "GasPrice"},
		28: {name: "GetWork"},
		29: {name: "SubmitWork"},
	}
	rpcTimeout = 10
)

type opType struct {
	// -1 useless, 0 SlaveServerSideOp funcs, 1 MasterServerSideOp funcs
	ty   int8
	name string
}

func preCheck(op int64, ty int8) bool {
	if opType, exist := rpcFuncs[op];
		exist && opType.ty == ty {
		return true
	}
	return false
}

func GetMasterServerSideOp(target string, req *Request) (response *Response, err error) {

	if !preCheck(req.Op, 1) {
		return nil, errors.New(fmt.Sprintf("%s don't belong to master server grpc functions", rpcFuncs[req.Op].name))
	}
	conn, err := GetConn(target)
	if err != nil {
		return
	}

	client := NewMasterServerSideOpClient(conn)
	return grpcOp(req, reflect.ValueOf(client))
}

func GetSlaveSideOp(target string, req *Request) (response *Response, err error) {

	if !preCheck(req.Op, 0) {
		return nil, errors.New(fmt.Sprintf("%s don't belong to slave server grpc functions", rpcFuncs[req.Op].name))
	}
	conn, err := GetConn(target)
	if err != nil {
		return
	}

	client := NewSlaveServerSideOpClient(conn)
	return grpcOp(req, reflect.ValueOf(client))
}

func grpcOp(req *Request, ele reflect.Value) (response *Response, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(rpcTimeout)*time.Second)
	defer cancel()

	rs := ele.MethodByName(rpcFuncs[req.Op].name).Call([]reflect.Value{reflect.ValueOf(ctx)})
	if rs[1].Interface() != nil {
		err = rs[1].Interface().(error)
		return
	}

	stream := rs[0].Interface().(SlaveServerSideOp_PingClient)
	if err = stream.Send(req); err != nil {
		return
	}

	if response, err = stream.CloseAndRecv(); err != nil {
		return
	}
	return
}

func AddConn(target string) error {
	var (
		conn *grpc.ClientConn
		err  error
	)
	mu.Lock()
	if _, ok := conns[target]; !ok {
		// TODO add certificate or enciphered data
		var opts []grpc.DialOption
		opts = append(opts, grpc.WithInsecure())
		conn, err = grpc.Dial(target, opts...)
		// defer conn.Close()
		if err == nil {
			conns[target] = conn
		}
		fmt.Println("create new connection", target)
	}
	mu.Unlock()
	return err
}

func GetConn(target string) (conn *grpc.ClientConn, err error) {
	if conn = conns[target]; conn == nil {
		if err = AddConn(target); err != nil {
			return nil, err
		}
	}
	return conns[target], nil
}
