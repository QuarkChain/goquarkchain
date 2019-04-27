package rpc

import (
	"sync"
)

import (
	"context"
)

// MasterServerSideOp juest for test
type MasterServerSideOp struct {
	rpcId int64
	mu    sync.RWMutex
}

func NewMasterTestOp() *MasterServerSideOp {
	return &MasterServerSideOp{}
}

// master handle function
func (m *MasterServerSideOp) AddMinorBlockHeader(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
		Data:  []byte("AddMinorBlockHeader response"),
	}, nil
}

// p2p apis
func (m *MasterServerSideOp) BroadcastNewTip(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) BroadcastTransactions(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) BroadcastMinorBlock(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) GetMinorBlocks(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) GetMinorBlockHeaders(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
