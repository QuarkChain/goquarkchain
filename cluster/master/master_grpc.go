package master

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
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
	data := new(rpc.AddMinorBlockHeaderRequest)
	if err := serialize.DeserializeFromBytes(req.Data, data); err != nil {
		return nil, err
	}
	m.master.rootBlockChain.AddValidatedMinorBlockHeader(data.MinorBlockHeader)
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
