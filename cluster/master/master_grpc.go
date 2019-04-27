package master

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"sync"
)

type MasterServerSideOp struct {
	mu     sync.RWMutex
	master *MasterBackend
}

func NewServerSideOp(master *MasterBackend) *MasterServerSideOp {
	return &MasterServerSideOp{
		master: master,
	}
}

func (m *MasterServerSideOp) AddMinorBlockHeader(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}

// p2p apis
func (m *MasterServerSideOp) BroadcastNewTip(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) BroadcastTransactions(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) BroadcastMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) GetMinorBlocks(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *MasterServerSideOp) GetMinorBlockHeaders(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
