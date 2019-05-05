package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/log"
)

type SlaveConn struct {
	target        string
	id            string
	chainMaskList []*types.ChainMask
	client        rpc.Client
}

func NewToSlaveConn(target, id string, chainMaskList []*types.ChainMask) (*SlaveConn, error) {
	return &SlaveConn{
		target:        target,
		id:            id,
		chainMaskList: chainMaskList,
		client:        rpc.NewClient(rpc.SlaveServer),
	}, nil
}

func (s *SlaveConn) SendPing() bool {
	var (
		gReq = rpc.Ping{Id: []byte(s.id), ChainMaskList: s.chainMaskList}
		gRep rpc.Pong
		err  error
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		log.Error("send ping", "failed to serialize Ping", "err", err)
		return false
	}

	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpPing, Data: data})
	buf := serialize.NewByteBuffer(res.Data)
	if err = serialize.Deserialize(buf, &gRep); err != nil {
		log.Error("send ping", "failed to deserialize Pong", "err", err)
	}

	if s.id != string(gRep.Id) {
		log.Error("send ping", "id does not match", "target id", s.id, "actual id", string(gRep.Id))
		return false
	}

	if !s.EqualChainMask(gRep.ChainMaskList) {
		log.Error("send ping", "chain_mask_list does not match", "target list", s.chainMaskList, "actual list", gRep.ChainMaskList)
		return false
	}

	return true
}

func (s *SlaveConn) EqualChainMask(chainMask []*types.ChainMask) bool {
	if len(chainMask) != len(s.chainMaskList) {
		return false
	}
	for i, id := range s.chainMaskList {
		if chainMask[i].GetMask() == id.GetMask() {
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
