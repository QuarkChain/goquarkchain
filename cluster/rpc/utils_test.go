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

func (m *MasterServerSideOp) AddMinorBlockHeaderList(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
		Data:  []byte("AddMinorBlockHeaderList response"),
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
func (m *MasterServerSideOp) BroadcastNewMinorBlock(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) GetMinorBlockList(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) GetMinorBlockHeaderList(ctx context.Context, req *Request) (*Response, error) {
	return &Response{
		RpcId: req.RpcId,
	}, nil
}
