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

var (
	conns = make(map[string]*grpc.ClientConn)
	// include all grpc funcs
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
)

type opType struct {
	// -1 useless, 0 SlaveServerSideOp funcs, 1 MasterServerSideOp funcs
	ty   int8
	name string
}

type RPClient struct {
	run     uint32
	mu      sync.Mutex
	timeout uint16
	conns   *map[string]*grpc.ClientConn
	funcs   *map[int64]opType

	mstrTp  int8
	slaveTp int8
}

func NewRPCLient() *RPClient {
	return &RPClient{
		run:     0,
		conns:   &conns,
		funcs:   &rpcFuncs,
		timeout: 500,
		mstrTp:  1,
	}
}

func (c *RPClient) checkOp(op int64, ty int8) bool {
	if opType, exist := rpcFuncs[op];
		exist && opType.ty == ty {
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

	if !c.checkOp(req.Op, c.mstrTp) {
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

	if !c.checkOp(req.Op, c.slaveTp) {
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

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.timeout)*time.Second)
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

func (c *RPClient) Getfuncs(tp int8) map[int64]string {
	var funcs = make(map[int64]string)
	for op, rpcfunc := range *c.funcs {
		if rpcfunc.ty == tp {
			funcs[op] = rpcfunc.name
		}
	}
	return funcs
}
