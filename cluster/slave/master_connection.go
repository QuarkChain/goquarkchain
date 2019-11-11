package slave

import (
	"errors"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
)

func (s *ConnManager) SendMinorBlockHeaderToMaster(request *rpc.AddMinorBlockHeaderRequest) error {
	var (
		gRsp rpc.AddMinorBlockHeaderResponse
	)

	if s.masterClient.target == "" {
		return errors.New("master endpoint is empty")
	}

	data, err := serialize.SerializeToBytes(request)
	if err != nil {
		return err
	}

	res, err := s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpAddMinorBlockHeader, Data: data})
	if err != nil {
		return err
	}
	if err := serialize.DeserializeFromBytes(res.Data, &gRsp); err != nil {
		return err
	}
	s.artificialTxConfig = gRsp.ArtificialTxConfig
	return nil
}

func (s *ConnManager) SendMinorBlockHeaderListToMaster(request *rpc.AddMinorBlockHeaderListRequest) error {
	data, err := serialize.SerializeToBytes(request)
	if err != nil {
		return err
	}
	_, err = s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpAddMinorBlockHeaderList, Data: data})
	if err != nil {
		return err
	}
	return err
}

func (s *ConnManager) BroadcastNewTip(mHeaderLst []*types.MinorBlockHeader, rHeader *types.RootBlockHeader, branch uint32) error {
	gReq := rpc.BroadcastNewTip{MinorBlockHeaderList: mHeaderLst, RootBlockHeader: rHeader, Branch: branch}
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}
	_, err = s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpBroadcastNewTip, Data: data})
	return err
}

func (s *ConnManager) BroadcastTransactions(peerId string, branch uint32, txs []*types.Transaction) error {
	raw, err := serialize.SerializeToBytes(&p2p.NewTransactionList{TransactionList: txs})
	if err != nil {
		return err
	}
	gReq := rpc.P2PRedirectRequest{PeerID: peerId, Branch: branch, Data: raw}
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}
	_, err = s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpBroadcastTransactions, Data: data})
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

	_, err = s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpBroadcastNewMinorBlock, Data: data})
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

	res, err = s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpGetMinorBlockList, Data: data})
	if err != nil {
		return nil, err
	}

	if err = serialize.DeserializeFromBytes(res.Data, &gRep); err != nil {
		return nil, err
	}

	return gRep.MinorBlockList, nil
}

func (s *ConnManager) GetMinorBlockHeaderList(gReq *rpc.GetMinorBlockHeaderListWithSkipRequest) ([]*types.MinorBlockHeader, error) {
	var (
		gRep p2p.GetMinorBlockHeaderListResponse
		res  *rpc.Response
	)
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return nil, err
	}

	res, err = s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpGetMinorBlockHeaderList, Data: data})
	if err != nil {
		return nil, err
	}

	if err = serialize.DeserializeFromBytes(res.Data, &gRep); err != nil {
		return nil, err
	}

	return gRep.BlockHeaderList, nil
}

func (s *ConnManager) ModifyTarget(target string) {
	s.masterClient.target = target
}
