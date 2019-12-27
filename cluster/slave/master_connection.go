package slave

import (
	"errors"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
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
	s.mu.Lock()
	s.artificialTxConfig = gRsp.ArtificialTxConfig
	s.mu.Unlock()
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

func (s *ConnManager) BroadcastMinorBlock(peerId string, minorBlock *types.MinorBlock) error {
	if minorBlock == nil {
		return errors.New("block is nil or branch mismatch")
	}
	var (
		gReq = rpc.P2PRedirectRequest{PeerID: peerId, Branch: minorBlock.Branch().Value}
		err  error
	)
	gReq.Data, err = serialize.SerializeToBytes(p2p.NewBlockMinor{Block: minorBlock})
	if err != nil {
		return err
	}
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}

	_, err = s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpBroadcastNewMinorBlock, Data: data})
	return err
}

func (s *ConnManager) GetMinorBlocks(mHeaderList []common.Hash, peerId string, branch uint32) ([]*types.MinorBlock, error) {
	var (
		gReq = rpc.P2PRedirectRequest{PeerID: peerId, Branch: branch}
		gRep rpc.GetMinorBlockListResponse
		err  error
	)
	gReq.Data, err = serialize.SerializeToBytes(p2p.GetMinorBlockListRequest{MinorBlockHashList: mHeaderList})
	if err != nil {
		return nil, err
	}
	data, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return nil, err
	}

	res, err := s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: rpc.OpGetMinorBlockList, Data: data})
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
		data []byte
		op   uint32
		err  error
	)

	if gReq.Type == qcom.SkipHash && gReq.Direction == qcom.DirectionToGenesis {
		data, err = serialize.SerializeToBytes(&p2p.GetMinorBlockHeaderListRequest{
			BlockHash: gReq.GetHash(),
			Branch:    gReq.Branch,
			Limit:     gReq.Limit,
			Direction: gReq.Direction,
		})
		if err != nil {
			return nil, err
		}
		op = rpc.OpGetMinorBlockHeaderList
	} else {
		data, err = serialize.SerializeToBytes(&gReq.GetMinorBlockHeaderListWithSkipRequest)
		if err != nil {
			return nil, err
		}
		op = rpc.OpGetMinorBlockHeaderListWithSkip
	}
	rawReq, err := serialize.SerializeToBytes(&rpc.P2PRedirectRequest{
		PeerID: gReq.PeerID,
		Branch: gReq.Branch.Value,
		Data:   data,
	})

	res, err := s.masterClient.client.Call(s.masterClient.target, &rpc.Request{Op: op, Data: rawReq})
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
