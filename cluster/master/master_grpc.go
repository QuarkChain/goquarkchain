package master

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
	"sync"
)

type QKCMasterServerSideOp struct {
	mu     sync.RWMutex
	master *QKCMasterBackend
}

func NewServerSideOp(master *QKCMasterBackend) *QKCMasterServerSideOp {
	return &QKCMasterServerSideOp{
		master: master,
	}
}

func (m *QKCMasterServerSideOp) AddMinorBlockHeader(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	data := new(rpc.AddMinorBlockHeaderRequest)
	if err := serialize.DeserializeFromBytes(req.Data, data); err != nil {
		return nil, err
	}
	m.master.rootBlockChain.AddValidatedMinorBlockHeader(data.MinorBlockHeader.Hash())
	m.master.UpdateShardStatus(data.ShardStats)
	m.master.UpdateTxCountHistory(data.TxCount, data.XShardTxCount, data.MinorBlockHeader.Time)

	rsp := new(rpc.AddMinorBlockHeaderResponse)
	rsp.ArtificialTxConfig = m.master.artificialTxConfig
	rspData, err := serialize.SerializeToBytes(rsp)
	if err != nil {
		return nil, err
	}

	return &rpc.Response{
		RpcId: req.RpcId,
		Data:  rspData,
	}, nil
}

//TODO @pingke
// p2p apis
func (m *QKCMasterServerSideOp) BroadcastNewTip(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *QKCMasterServerSideOp) BroadcastTransactions(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *QKCMasterServerSideOp) BroadcastMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *QKCMasterServerSideOp) GetMinorBlocks(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
func (m *QKCMasterServerSideOp) GetMinorBlockHeaders(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}
