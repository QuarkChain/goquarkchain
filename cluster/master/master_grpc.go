package master

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"sync"
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
	broadcastTxsReq := new(rpc.BroadcastTransactions)
	if err := serialize.DeserializeFromBytes(req.Data, broadcastTxsReq); err != nil {
		return nil, err
	}
	m.p2pApi.BroadcastTransactions(broadcastTxsReq.Branch, broadcastTxsReq.Txs)
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) BroadcastNewMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	broadcastMinorBlockReq := new(rpc.BroadcastMinorBlock)
	if err := serialize.DeserializeFromBytes(req.Data, broadcastMinorBlockReq); err != nil {
		return nil, err
	}
	err := m.p2pApi.BroadcastMinorBlock(broadcastMinorBlockReq.Branch, broadcastMinorBlockReq.MinorBlock)
	if err != nil {
		return nil, err
	}
	return &rpc.Response{
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) GetMinorBlockList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		err                  error
		getMinorBlockListReq = new(rpc.GetMinorBlockListRequest)
		getMinorBlockListRes = new(rpc.GetMinorBlockListResponse)
	)

	if err = serialize.DeserializeFromBytes(req.Data, getMinorBlockListReq); err != nil {
		return nil, err
	}
	getMinorBlockListRes.MinorBlockList, err = m.p2pApi.GetMinorBlockList(getMinorBlockListReq.MinorBlockHashList, getMinorBlockListReq.Branch, getMinorBlockListReq.PeerId)
	if err != nil {
		return nil, err
	}
	bytes, err := serialize.SerializeToBytes(getMinorBlockListRes)
	if err != nil {
		return nil, err
	}

	return &rpc.Response{
		Data:  bytes,
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) GetMinorBlockHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		err             error
		getMBHeadersReq = new(p2p.MinorHeaderListWithSkip)
	)

	if err = serialize.DeserializeFromBytes(req.Data, getMBHeadersReq); err != nil {
		return nil, err
	}
	//hash common.Hash, amount uint32, branch uint32, reverse bool, peerId string
	gRes, err := m.p2pApi.GetMinorBlockHeaderList(getMBHeadersReq)
	if err != nil {
		return nil, err
	}
	bytes, err := serialize.SerializeToBytes(gRes)
	if err != nil {
		return nil, err
	}

	return &rpc.Response{
		Data:  bytes,
		RpcId: req.RpcId,
	}, nil
}

func (m *MasterServerSideOp) MinorHead(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		res = new(rpc.MinorHeadRequest)
		err error
	)

	if err = serialize.DeserializeFromBytes(req.Data, res); err != nil {
		return nil, err
	}

	mBHeader, err := m.p2pApi.MinorHead(res)
	if err != nil {
		return nil, err
	}

	bytes, err := serialize.SerializeToBytes(mBHeader)
	if err != nil {
		return nil, err
	}

	return &rpc.Response{
		Data:  bytes,
		RpcId: req.RpcId,
	}, nil
}
