package slave

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/log"
)

type SlaveServerSideOp struct {
	slave *SlaveBackend
}

func NewServerSideOp(slave *SlaveBackend) *SlaveServerSideOp {
	return &SlaveServerSideOp{
		slave: slave,
	}
}

func (s *SlaveServerSideOp) HeartBeat(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	s.slave.ctx.Timestamp = time.Now()
	if len(s.slave.shards) == 0 {
		return nil, errors.New("shards uninitialized")
	}
	return &rpc.Response{}, nil
}

func (s *SlaveServerSideOp) MasterInfo(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.MasterInfo
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	s.slave.connManager.ModifyTarget(fmt.Sprintf("%s:%d", gReq.Ip, gReq.Port))

	if gReq.RootTip == nil {
		return nil, errors.New("handle masterInfo err:rootTip is nil")
	}
	//createShards
	if err = s.slave.CreateShards(gReq.RootTip, true); err != nil {
		return nil, err
	}

	//ping with other slaves
	for _, slv := range s.slave.clstrCfg.SlaveList {
		if slv.ID == s.slave.config.ID {
			continue
		}
		s.slave.connManager.AddConnectToSlave(&rpc.SlaveInfo{Id: slv.ID, Host: slv.IP, Port: slv.Port, ChainMaskList: slv.ChainMaskList})
	}

	log.Info("slave master info response", "master endpoint", s.slave.connManager.masterClient.target)

	return response, nil
}

func (s *SlaveServerSideOp) Ping(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gRes     rpc.Pong
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	gRes.Id, gRes.ChainMaskList = []byte(s.slave.config.ID), s.slave.config.ChainMaskList
	log.Info("slave ping response", "request op", req.Op)

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) GenTx(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GenTxRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if err = s.slave.GenTx(&gReq); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) AddRootBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddRootBlockRequest
		gRes     rpc.AddRootBlockResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.Switched, err = s.slave.AddRootBlock(gReq.RootBlock); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}

	if err = s.slave.CreateShards(gReq.RootBlock, false); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetUnconfirmedHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gRes     rpc.GetUnconfirmedHeadersResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if gRes.HeadersInfoList, err = s.slave.GetUnconfirmedHeaderList(); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetAccountData(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetAccountDataRequest
		gRes     rpc.GetAccountDataResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.AccountBranchDataList, err = s.slave.GetAccountData(gReq.Address, gReq.BlockHeight); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) AddTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddTransactionRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.AddTx(gReq.Tx); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) GetMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetMinorBlockRequest
		gRes     rpc.GetMinorBlockResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.MinorBlock, err = s.slave.GetMinorBlock(gReq.MinorBlockHash, gReq.Height, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetTransactionRequest
		gRes     rpc.GetTransactionResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.MinorBlock, gRes.Index, err = s.slave.GetTransactionByHash(gReq.TxHash, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) ExecuteTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.ExecuteTransactionRequest
		gRes     rpc.ExecuteTransactionResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if gRes.Result, err = s.slave.ExecuteTx(gReq.Tx, gReq.FromAddress, gReq.BlockHeight); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetTransactionReceipt(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetTransactionReceiptRequest
		gRes     rpc.GetTransactionReceiptResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	gRes.MinorBlock, gRes.Index, gRes.Receipt, err = s.slave.GetTransactionReceipt(gReq.TxHash, gReq.Branch)
	if err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetTransactionListByAddress(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetTransactionListByAddressRequest
		gRes     rpc.GetTransactionListByAddressResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.TxList, gRes.Next, err = s.slave.GetTransactionListByAddress(gReq.Address,
		gReq.Start, gReq.Limit); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetLogs(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetLogRequest
		gRes     rpc.GetLogResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.Logs, err = s.slave.GetLogs(gReq.Topics, gReq.Addresses, gReq.StartBlock, gReq.EndBlock, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) EstimateGas(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.EstimateGasRequest
		gRes     rpc.EstimateGasResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	response = &rpc.Response{RpcId: req.RpcId}
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.Result, err = s.slave.EstimateGas(gReq.Tx, gReq.FromAddress); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetStorageAt(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetStorageRequest
		gRes     rpc.GetStorageResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.Result, err = s.slave.GetStorageAt(gReq.Address, gReq.Key, gReq.BlockHeight); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetCode(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetCodeRequest
		gRes     rpc.GetCodeResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.Result, err = s.slave.GetCode(gReq.Address, gReq.BlockHeight); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GasPrice(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GasPriceRequest
		gRes     rpc.GasPriceResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.Result, err = s.slave.GasPrice(gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetWork(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetWorkRequest
		work     *consensus.MiningWork
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if work, err = s.slave.GetWork(gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(work); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) SubmitWork(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.SubmitWorkRequest
		gRes     rpc.SubmitWorkResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.SubmitWork(gReq.HeaderHash, gReq.Nonce, gReq.MixHash, gReq.Branch); err != nil {
		return nil, err
	}
	gRes.Success = true

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) AddXshardTxList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddXshardTxListRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.AddCrossShardTxListByMinorBlockHash(gReq.MinorBlockHash, gReq.TxList, gReq.Branch); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) BatchAddXshardTxList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.BatchAddXshardTxListRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	for _, req := range gReq.AddXshardTxListRequestList {
		if err = s.slave.AddCrossShardTxListByMinorBlockHash(req.MinorBlockHash, req.TxList, req.Branch); err != nil {
			return nil, err
		}
	}

	return response, nil
}

// check if the blocks are vailed.
func (s *SlaveServerSideOp) AddMinorBlockListForSync(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.AddBlockListForSyncRequest
		gRes     rpc.AddBlockListForSyncResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if len(gReq.MinorBlockHashList) == 0 {
		return response, nil
	}
	if gRes.ShardStatus, err = s.slave.AddBlockListForSync(gReq.MinorBlockHashList, gReq.PeerId, gReq.Branch); err != nil {
		return nil, err
	}
	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

// p2p apis.
func (s *SlaveServerSideOp) GetMinorBlockList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetMinorBlockListRequest
		gRes     rpc.GetMinorBlockListResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.MinorBlockList, err = s.slave.GetMinorBlockListByHashList(gReq.MinorBlockHashList, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetMinorBlockHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     p2p.GetMinorBlockHeaderListRequest
		gRes     p2p.GetMinorBlockHeaderListResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.BlockHeaderList, err = s.slave.GetMinorBlockHeaderList(gReq.BlockHash, gReq.Limit, gReq.Direction, gReq.Branch.Value); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) HandleNewTip(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.HandleNewTipRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if err = s.slave.HandleNewTip(&gReq); err != nil {
		return nil, err
	}

	return response, nil
}

func (s *SlaveServerSideOp) AddTransactions(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     p2p.NewTransactionList
		gRes     rpc.HashList
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	for _, tx := range gReq.TransactionList {
		if err = s.slave.AddTx(tx); err != nil {
			log.Error("Add transaction failed", "tx", tx, "err", err)
			continue
		}
		gRes.Hashes = append(gRes.Hashes, tx.Hash())
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) HandleNewMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     types.MinorBlock
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if err = s.slave.NewMinorBlock(&gReq); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) SetMining(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		mining   bool
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &mining); err != nil {
		return nil, err
	}
	s.slave.SetMining(mining)
	return response, nil
}

func (s *SlaveServerSideOp) GetShardStat(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var ()

}
