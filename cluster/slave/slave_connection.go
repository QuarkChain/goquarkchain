package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
)

type SlaveConn struct {
	target        string
	id            string
	chainMaskList []types.ChainMask
	client        rpc.Client
}

func NewToSlaveConn(target, id string, chainMaskList []types.ChainMask) (*SlaveConn, error) {
	return &SlaveConn{
		target:        target,
		id:            id,
		chainMaskList: chainMaskList,
		client:        rpc.NewClient(rpc.SlaveServer),
	}, nil
}

func (s *SlaveConn) SendPing() (string, []types.ChainMask) {
	// req := rpc.Ping{Id: []byte(s.id), ChainMaskList: s.chainMaskList, RootTip: nil}
	// res, err := s.client.Call(rpc.OpPing, )
	return "", nil
}

func (s *SlaveConn) EqualChainMask(chainMask []types.ChainMask) bool {
	if len(chainMask) != len(s.chainMaskList) {
		return false
	}
	for i, id := range s.chainMaskList {
		if chainMask[i] == id {
			return true
		}
	}
	return false
}

func (s *SlaveConn) HasShard(msk uint32) bool {
	for _, id := range s.chainMaskList {
		if id.ContainFullShardId(msk) {
			return true
		}
	}
	return false
}

func (s *SlaveConn) AddXshardTxList(xshardReq *rpc.AddXshardTxListRequest) bool {
	return false
}

func (s *SlaveConn) BatchAddXshardTxList(xshardReqs []*rpc.AddXshardTxListRequest) bool {
	return false
}
