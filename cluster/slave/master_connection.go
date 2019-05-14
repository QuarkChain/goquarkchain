package slave

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
)

func (s *ConnManager) SendMinorBlockHeaderToMaster(minorHeader *types.MinorBlockHeader,
	txCount, xshardCount uint32, state *rpc.ShardStatus) error {
	var (
		gRep rpc.AddMinorBlockHeaderResponse
	)

	if s.masterCli.target == "" {
		return errors.New("master endpoint is empty")
	}
	gReq := rpc.AddMinorBlockHeaderRequest{
		MinorBlockHeader: minorHeader,
		TxCount:          txCount,
		XShardTxCount:    xshardCount,
		ShardStats:       state,
	}
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}

	res, err := s.masterCli.client.Call(s.masterCli.target, &rpc.Request{Op: rpc.OpAddMinorBlockHeader, Data: data})
	if err != nil {
		return err
	}
	if err := serialize.DeserializeFromBytes(res.Data, &gRep); err != nil {
		return err
	}
	s.artificialTxConfig = gRep.ArtificialTxConfig
	return nil
}

func (s *ConnManager) BroadcastNewTip(mHeaderLst []*types.MinorBlockHeader,
	rHeader *types.RootBlockHeader, branch uint32) error {
	var (
		gReq = rpc.BroadcastNewTip{MinorBlockHeaderList: mHeaderLst, RootBlockHeader: rHeader, Branch: branch}
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}
	_, err = s.masterCli.client.Call(s.masterCli.target, &rpc.Request{Op: rpc.OpBroadcastNewTip, Data: data})
	return err
}

func (s *ConnManager) BroadcastTransactions(txs []*types.Transaction, branch uint32) error {
	var (
		gReq = rpc.BroadcastTransactions{Txs: txs, Branch: branch}
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}

	_, err = s.masterCli.client.Call(s.masterCli.target, &rpc.Request{Op: rpc.OpBroadcastTransactions, Data: data})
	return err
}

func (s *ConnManager) BroadcastMinorBlock(minorBlock *types.MinorBlock, branch uint32) error {
	var (
		gReq = rpc.BroadcastMinorBlock{MinorBlock: minorBlock, Branch: branch}
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}

	_, err = s.masterCli.client.Call(s.masterCli.target, &rpc.Request{Op: rpc.OpBroadcastNewMinorBlock, Data: data})
	return err
}

func (s *ConnManager) GetMinorBlocks(mHeaderList []common.Hash, peerId string, branch uint32) ([]*types.MinorBlock, error) {
	var (
		gReq = rpc.GetMinorBlockListRequest{MinorBlockHashList: mHeaderList, PeerId: peerId, Branch: branch}
		gRep rpc.GetMinorBlockListResponse
		res  *rpc.Response
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return nil, err
	}

	res, err = s.masterCli.client.Call(s.masterCli.target, &rpc.Request{Op: rpc.OpGetMinorBlockList, Data: data})
	if err != nil {
		return nil, err
	}

	buf := serialize.NewByteBuffer(res.Data)
	if err = serialize.Deserialize(buf, &gRep); err != nil {
		return nil, err
	}

	return gRep.MinorBlockList, nil
}

func (s *ConnManager) GetMinorBlockHeaders(mHash common.Hash,
	limit uint32, direction uint8, branch uint32) ([]*types.MinorBlockHeader, error) {
	var (
		gReq = rpc.GetMinorBlockHeaderListRequest{BlockHash: mHash,
			Limit: limit, Direction: direction, Branch: branch}
		gRep rpc.GetMinorBlockHeaderListResponse
		res  *rpc.Response
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return nil, err
	}

	res, err = s.masterCli.client.Call(s.masterCli.target, &rpc.Request{Op: rpc.OpGetMinorBlockHeaderList, Data: data})
	if err != nil {
		return nil, err
	}

	buf := serialize.NewByteBuffer(res.Data)
	if err = serialize.Deserialize(buf, &gRep); err != nil {
		return nil, err
	}

	return gRep.MinorBlockHeaderList, nil
}

func (s *ConnManager) ModifyTarget(target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.masterCli.target = target
}
