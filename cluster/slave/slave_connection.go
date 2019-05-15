package slave

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/log"
)

// this struct is used to communicate between slaves
type SlaveConn struct {
	target        string
	id            string
	chainMaskList []*types.ChainMask
	client        rpc.Client
}

func NewToSlaveConn(target, id string, chainMaskList []*types.ChainMask) *SlaveConn {
	return &SlaveConn{
		target:        target,
		id:            id,
		chainMaskList: chainMaskList,
		client:        rpc.NewClient(rpc.SlaveServer),
	}
}

func (s *SlaveConn) SendPing() bool {
	var (
		gReq = rpc.Ping{Id: []byte(s.id), ChainMaskList: s.chainMaskList}
		gRes rpc.Pong
		err  error
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		log.Error("Can't serialize rpc.Ping when ping to slave", "err", err)
		return false
	}

	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpPing, Data: data})
	if err != nil {
		log.Error("Failed to Ping to slave", "slave endpoint", s.target, "err", err)
		return false
	}
	if err = serialize.DeserializeFromBytes(res.Data, &gRes); err != nil {
		log.Error("Can't deserialize response data by rpc.Pong", "err", err)
	}

	if s.id != string(gRes.Id) {
		log.Error("Id doesn't match", "target id", s.id, "actual id", string(gRes.Id))
		return false
	}

	if !s.EqualChainMask(gRes.ChainMaskList) {
		log.Error("Chain_mask_list doesn't match", "target list", s.chainMaskList, "actual list", gRes.ChainMaskList)
		return false
	}

	return true
}

func (s *SlaveConn) AddXshardTxList(xshardReq *rpc.AddXshardTxListRequest) error {
	if !s.HasShard(xshardReq.Branch) {
		return fmt.Errorf("Branch don't match when call AddXshardTxList, wrong branch: %d ", xshardReq.Branch)
	}
	bytes, err := serialize.SerializeToBytes(xshardReq)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddXshardTxList, Data: bytes})
	return err
}

func (s *SlaveConn) BatchAddXshardTxList(xshardReqs []*rpc.AddXshardTxListRequest) error {
	bytes, err := serialize.SerializeToBytes(rpc.BatchAddXshardTxListRequest{AddXshardTxListRequestList: xshardReqs})
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpBatchAddXshardTxList, Data: bytes})
	return err
}

func (s *SlaveConn) EqualChainMask(chainMask []*types.ChainMask) bool {
	if len(chainMask) != len(s.chainMaskList) {
		return false
	}
	for i, id := range s.chainMaskList {
		if chainMask[i].GetMask() != id.GetMask() {
			return false
		}
	}
	return true
}

func (s *SlaveConn) HasShard(fullshardId uint32) bool {
	for _, id := range s.chainMaskList {
		if id.ContainFullShardId(fullshardId) {
			return true
		}
	}
	return false
}
