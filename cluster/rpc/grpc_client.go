// Modified from go-ethereum under GNU Lesser General Public License
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

type opType struct {
	// -1 useless, 0 SlaveServerSideOp funcs, 1 MasterServerSideOp funcs
	serverType ServerType
	name       string
}

type RPCClient struct {
	mu      sync.RWMutex
	target  string
	timeout time.Duration
	conn    *grpc.ClientConn
	funcs   map[int64]opType
}

func NewRPCLient(target string) (*RPCClient, error) {
	client := &RPCClient{
		target:  target,
		funcs:   rpcFuncs,
		timeout: 10 * time.Second,
	}
	if err := client.createConn(); err != nil {
		return nil, err
	}
	return client, nil
}

func (c *RPCClient) checkOp(op int64, serverType ServerType) bool {
	if opType, exist := rpcFuncs[op];
		exist && opType.serverType == serverType {
		return true
	}
	return false
}

func (c *RPCClient) GetOpName(op int64) string {
	opType, exist := rpcFuncs[op]
	if exist {
		return opType.name
	}
	return ""
}

func (c *RPCClient) GetMasterServerSideOp(req *Request) (*Response, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.checkOp(req.Op, MasterServer) {
		return nil, errors.New(fmt.Sprintf("%s don't belong to master server grpc functions", rpcFuncs[req.Op].name))
	}
	client := NewMasterServerSideOpClient(c.conn)
	return c.grpcOp(req, reflect.ValueOf(client))
}

func (c *RPCClient) GetSlaveSideOp(target string, req *Request) (*Response, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.checkOp(req.Op, SlaveServer) {
		return nil, errors.New(fmt.Sprintf("%s don't belong to slave server grpc functions", rpcFuncs[req.Op].name))
	}
	client := NewSlaveServerSideOpClient(c.conn)
	return c.grpcOp(req, reflect.ValueOf(client))
}

func (c *RPCClient) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

func (c *RPCClient) grpcOp(req *Request, ele reflect.Value) (response *Response, err error) {

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var val []reflect.Value
	val = append(val, reflect.ValueOf(ctx), reflect.ValueOf(req))
	rs := ele.MethodByName(rpcFuncs[req.Op].name).Call(val)
	if rs[1].Interface() != nil {
		err = rs[1].Interface().(error)
		return
	}

	res := rs[0].Interface().(*Response)
	if err != nil {
		return
	}
	return res, nil
}

func (c *RPCClient) createConn() error {
	var (
		opts = []grpc.DialOption{grpc.WithInsecure()}
		err  error
	)
	c.mu.Lock()
	c.conn, err = grpc.Dial(c.target, opts...)
	if err == nil {
		fmt.Println("create new connection", c.target)
	}
	c.mu.Unlock()
	return err
}

func (c *RPCClient) Getfuncs(serverType ServerType) map[int64]string {
	var funcs = make(map[int64]string)
	for op, rpcfunc := range c.funcs {
		if rpcfunc.serverType == serverType {
			funcs[op] = rpcfunc.name
		}
	}
	return funcs
}
