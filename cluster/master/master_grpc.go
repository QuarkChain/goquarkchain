package master

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"sync"
	"sync/atomic"
)

type MasterServerSideOp struct {
	rpcId int64
	mu    sync.RWMutex
}

func NewServerSideOp() *MasterServerSideOp {
	return &MasterServerSideOp{
	}
}

func (m *MasterServerSideOp) AddMinorBlockHeader(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId:     m.addRpcId(),
		ErrorCode: 0,
	}, nil
}

func (m *MasterServerSideOp) addRpcId() int64 {
	atomic.AddInt64(&m.rpcId, 1)
	return m.rpcId
}
