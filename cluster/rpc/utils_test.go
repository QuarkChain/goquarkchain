package rpc

import (
	"context"
	"sync"
	"sync/atomic"
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
		RpcId:     m.addRpcId(),
		Data:      []byte("AddMinorBlockHeader response"),
		ErrorCode: 0,
	}, nil
}

// rpc id atomic increase in every request
func (m *MasterServerSideOp) addRpcId() int64 {
	atomic.AddInt64(&m.rpcId, 1)
	return m.rpcId
}
