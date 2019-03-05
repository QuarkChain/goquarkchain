package rpc

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type serverType int

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

	MasterServer  = serverType(1)
	SlaveServer   = serverType(0)
	UnknownServer = serverType(-1)
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
	serverType serverType
	name       string
}

// Client wraps the GRPC client.
type Client interface {
	Call(server serverType, hostport string, req *Request) (*Response, error)
	GetOpName(int64) string
}

type rpcClient struct {
	mu      sync.RWMutex
	timeout time.Duration
	conns   map[string]*grpc.ClientConn
	funcs   map[int64]opType
	running bool
	logger  log.Logger
}

func (c *rpcClient) GetOpName(op int64) string {
	return rpcFuncs[op].name
}

func (c *rpcClient) Call(server serverType, hostport string, req *Request) (*Response, error) {
	c.mu.RLock()
	running := c.running
	c.mu.RUnlock()

	if !running {
		return nil, errors.New("client has closed")
	}

	opType, ok := rpcFuncs[req.Op]
	if !(ok && opType.serverType == server) {
		return nil, errors.New("invalid op")
	}

	conn, err := c.getConn(hostport)
	if err != nil {
		return nil, err
	}
	var client reflect.Value
	switch server {
	case MasterServer:
		client = reflect.ValueOf(NewMasterServerSideOpClient(conn))
	case SlaveServer:
		client = reflect.ValueOf(NewSlaveServerSideOpClient(conn))
	default:
		return nil, errors.New("unrecognized server type")
	}
	return c.grpcOp(req, client)
}

func (c *rpcClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	for _, client := range c.conns {
		client.Close()
	}
}

func (c *rpcClient) getConn(hostport string) (*grpc.ClientConn, error) {
	c.mu.RLock()
	running := c.running
	conn, ok := conns[hostport]
	c.mu.RUnlock()

	if !running {
		return nil, errors.New("client has closed")
	}

	// add new connection if not existing or has failed
	// note that race may happen when adding duplicate connections
	if !ok || conn.GetState() > connectivity.TransientFailure {
		return c.addConn(hostport)
	}

	return conn, nil
}

func (c *rpcClient) grpcOp(req *Request, ele reflect.Value) (response *Response, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	val := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(req)}
	rs := ele.MethodByName(rpcFuncs[req.Op].name).Call(val)
	if rs[1].Interface() != nil {
		err = rs[1].Interface().(error)
		return nil, err
	}

	res := rs[0].Interface().(*Response)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (c *rpcClient) addConn(hostport string) (*grpc.ClientConn, error) {
	var (
		conn *grpc.ClientConn
		err  error
	)
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(conns, hostport)
	// TODO add certificate or enciphered data
	opts := []grpc.DialOption{grpc.WithInsecure()}
	conn, err = grpc.Dial(hostport, opts...)
	if err != nil {
		return nil, err
	}
	conns[hostport] = conn
	c.logger.Debug("Created new connection", "hostport", hostport)
	return conn, nil
}

// NewClient returns a new GRPC client wrapper.
func NewClient() Client {
	return &rpcClient{
		conns:   conns,
		funcs:   rpcFuncs,
		timeout: 10 * time.Second,
		running: true,
		logger:  log.New("rpcclient"),
	}
}
