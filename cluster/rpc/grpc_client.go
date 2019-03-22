// Modified from go-ethereum under GNU Lesser General Public License
package rpc

import (
	"context"
	"errors"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"reflect"
	"sync"
	"time"
)

type ServerType int

const (
	_ = iota
	OpPing
	OpConnectToSlaves
	OpAddRootBlock
	OpGetEcoInfoList
	OpGetNextBlockToMine
	OpGetUnconfirmedHeaders
	OpGetAccountData
	OpAddTransaction
	OpAddMinorBlockHeader
	OpAddXshardTxList
	OpSyncMinorBlockList
	OpAddMinorBlock
	OpCreateClusterPeerConnection
	OpDestroyClusterPeerConnectionCommand
	OpGetMinorBlock
	OpGetTransaction
	OpBatchAddXshardTxList
	OpExecuteTransaction
	OpGetTransactionReceipt
	OpGetMine
	OpGenTx
	OpGetTransactionListByAddress
	OpGetLogs
	OpEstimateGas
	OpGetStorageAt
	OpGetCode
	OpGasPrice
	OpGetWork
	OpSubmitWork

	MasterServer  = ServerType(1)
	SlaveServer   = ServerType(0)
	UnknownServer = ServerType(-1)
)

var (
	conns = make(map[string]*grpc.ClientConn)
	// include all grpc funcs
	rpcFuncs = map[int64]opType{
		OpPing:                                {name: "Ping"},
		OpConnectToSlaves:                     {name: "ConnectToSlaves"},
		OpAddRootBlock:                        {name: "AddRootBlock"},
		OpGetEcoInfoList:                      {name: "GetEcoInfoList"},
		OpGetNextBlockToMine:                  {name: "GetNextBlockToMine"},
		OpGetUnconfirmedHeaders:               {name: "GetUnconfirmedHeaders"},
		OpGetAccountData:                      {name: "GetAccountData"},
		OpAddTransaction:                      {name: "AddTransaction"},
		OpAddMinorBlockHeader:                 {name: "AddMinorBlockHeader", serverType: MasterServer},
		OpAddXshardTxList:                     {name: "AddXshardTxList"},
		OpSyncMinorBlockList:                  {name: "SyncMinorBlockList"},
		OpAddMinorBlock:                       {name: "AddMinorBlock"},
		OpCreateClusterPeerConnection:         {name: "CreateClusterPeerConnection"},
		OpDestroyClusterPeerConnectionCommand: {name: "DestroyClusterPeerConnectionCommand", serverType: UnknownServer},
		OpGetMinorBlock:                       {name: "GetMinorBlock"},
		OpGetTransaction:                      {name: "GetTransaction"},
		OpBatchAddXshardTxList:                {name: "BatchAddXshardTxList"},
		OpExecuteTransaction:                  {name: "ExecuteTransaction"},
		OpGetTransactionReceipt:               {name: "GetTransactionReceipt"},
		OpGetMine:                             {name: "GetMine"},
		OpGenTx:                               {name: "GenTx"},
		OpGetTransactionListByAddress:         {name: "GetTransactionListByAddress"},
		OpGetLogs:                             {name: "GetLogs"},
		OpEstimateGas:                         {name: "EstimateGas"},
		OpGetStorageAt:                        {name: "GetStorageAt"},
		OpGetCode:                             {name: "GetCode"},
		OpGasPrice:                            {name: "GasPrice"},
		OpGetWork:                             {name: "GetWork"},
		OpSubmitWork:                          {name: "SubmitWork"},
	}
)

type opType struct {
	// -1 useless, 0 SlaveServerSideOp funcs, 1 MasterServerSideOp funcs
	serverType ServerType
	name       string
}

type RPClient struct {
	mu      sync.Mutex
	timeout time.Duration
	conns   map[string]*grpc.ClientConn
	funcs   map[int64]opType
}

func NewRPCLient() *RPClient {
	return &RPClient{
		conns:   conns,
		funcs:   rpcFuncs,
		timeout: 10 * time.Second,
	}
}

func (c *RPClient) checkOp(op int64, serverType ServerType) bool {
	if opType, exist := rpcFuncs[op];
		exist && opType.serverType == serverType {
		return true
	}
	return false
}

func (c *RPClient) GetOpName(op int64) string {
	opType, exist := rpcFuncs[op]
	if exist {
		return opType.name
	}
	return ""
}

func (c *RPClient) GetMasterServerSideOp(target string, req *Request) (*Response, error) {

	if !c.checkOp(req.Op, MasterServer) {
		return nil, errors.New(fmt.Sprintf("%s don't belong to master server grpc functions", rpcFuncs[req.Op].name))
	}
	conn, err := c.GetConn(target)
	if err != nil {
		return nil, err
	}
	client := NewMasterServerSideOpClient(conn)
	return c.grpcOp(req, reflect.ValueOf(client))
}

func (c *RPClient) GetSlaveSideOp(target string, req *Request) (*Response, error) {

	if !c.checkOp(req.Op, SlaveServer) {
		return nil, errors.New(fmt.Sprintf("%s don't belong to slave server grpc functions", rpcFuncs[req.Op].name))
	}
	conn, err := c.GetConn(target)
	if err != nil {
		return nil, err
	}

	client := NewSlaveServerSideOpClient(conn)
	return c.grpcOp(req, reflect.ValueOf(client))
}

func (c *RPClient) grpcOp(req *Request, ele reflect.Value) (response *Response, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
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
	if err = stream.CloseSend(); err != nil {
		return
	}
	if response, err = stream.CloseAndRecv(); err != nil {
		return
	}
	return
}

func (c *RPClient) addConn(target string) error {
	var (
		conn *grpc.ClientConn
		err  error
	)
	c.mu.Lock()
	delete(conns, target)
	// TODO add certificate or enciphered data
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithInsecure())
	conn, err = grpc.Dial(target, opts...)
	// defer conn.Close()
	if err == nil {
		conns[target] = conn
		fmt.Println("create new connection", target)
	} else {
		conn.Close()
	}
	c.mu.Unlock()
	return err
}

func (c *RPClient) GetConn(target string) (*grpc.ClientConn, error) {
	if conn, ok := conns[target]; !ok ||
		(ok && conn.GetState() > connectivity.TransientFailure) {
		if err := c.addConn(target); err != nil {
			return nil, err
		}
	}
	return conns[target], nil
}

func (c *RPClient) Getfuncs(serverType ServerType) map[int64]string {
	var funcs = make(map[int64]string)
	for op, rpcfunc := range c.funcs {
		if rpcfunc.serverType == serverType {
			funcs[op] = rpcfunc.name
		}
	}
	return funcs
}
