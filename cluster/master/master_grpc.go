package master

import (
	"context"
	"sync"
	
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
)

type MasterServerSideOp struct {
	mu     sync.RWMutex
	master *QKCMasterBackend
	p2pApi *PrivateP2PAPI
}

func NewServerSideOp(master *QKCMasterBackend) *MasterServerSideOp {
	return &MasterServerSideOp{
		master: master,
		p2pApi: NewPrivateP2PAPI(master.protocolManager.peers),
	}
}

func (m *MasterServerSideOp) AddMinorBlockHeader(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	data := new(rpc.AddMinorBlockHeaderRequest)
	if err := serialize.DeserializeFromBytes(req.Data, data); err != nil {
		return nil, err
	}
	m.master.rootBlockChain.AddValidatedMinorBlockHeader(data.MinorBlockHeader.Hash(), data.CoinbaseAmountMap)
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

func (m *MasterServerSideOp) AddMinorBlockHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	gReq := new(rpc.AddMinorBlockHeaderListRequest)
	if err := serialize.DeserializeFromBytes(req.Data, gReq); err != nil {
		return nil, err
	}
	for _, header := range gReq.MinorBlockHeaderList {
		m.master.rootBlockChain.AddValidatedMinorBlockHeader(header.Hash(), header.CoinbaseAmount)
	}
	return &rpc.Response{RpcId: req.RpcId}, nil
}

// p2p apis
func (m *MasterServerSideOp) BroadcastNewTip(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	broadcastTipReq := new(rpc.BroadcastNewTip)
	if err := serialize.DeserializeFromBytes(req.Data, broadcastTipReq); err != nil {
		return nil, err
	}
	err := m.p2pApi.BroadcastNewTip(broadcastTipReq.Branch, broadcastTipReq.RootBlockHeader, broadcastTipReq.MinorBlockHeaderList)
	if err != nil {
		return nil, err
	}
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) BroadcastTransactions(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	broadcastTxsReq := new(rpc.P2PRedirectRequest)
	if err := serialize.DeserializeFromBytes(req.Data, broadcastTxsReq); err != nil {
		return nil, err
	}
	m.p2pApi.BroadcastTransactions(broadcastTxsReq, broadcastTxsReq.PeerID)
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) BroadcastNewMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	res := new(rpc.P2PRedirectRequest)
	if err := serialize.DeserializeFromBytes(req.Data, res); err != nil {
		return nil, err
	}
	err := m.p2pApi.BroadcastMinorBlock(res)
	if err != nil {
		return nil, err
	}
	return &rpc.Response{}, nil
}

func (m *MasterServerSideOp) GetMinorBlockList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		rep = new(rpc.P2PRedirectRequest)
		err error
	)

	if err = serialize.DeserializeFromBytes(req.Data, rep); err != nil {
		return nil, err
	}
	data, err := m.p2pApi.GetMinorBlockList(rep)
	if err != nil {
		return nil, err
	}

	return &rpc.Response{
		Data:  data,
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) GetMinorBlockHeaderListWithSkip(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		err             error
		getMBHeadersReq = new(rpc.P2PRedirectRequest)
	)

	if err = serialize.DeserializeFromBytes(req.Data, getMBHeadersReq); err != nil {
		return nil, err
	}
	//hash common.Hash, amount uint32, branch uint32, reverse bool, peerId string
	data, err := m.p2pApi.GetMinorBlockHeaderListWithSkip(getMBHeadersReq)
	if err != nil {
		return nil, err
	}

	return &rpc.Response{
		Data:  data,
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) GetMinorBlockHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		err             error
		getMBHeadersReq = new(rpc.P2PRedirectRequest)
	)

	if err = serialize.DeserializeFromBytes(req.Data, getMBHeadersReq); err != nil {
		return nil, err
	}
	//hash common.Hash, amount uint32, branch uint32, reverse bool, peerId string
	data, err := m.p2pApi.GetMinorBlockHeaderList(getMBHeadersReq)
	if err != nil {
		return nil, err
	}

	return &rpc.Response{
		Data:  data,
		RpcId: req.RpcId,
	}, nil
}
