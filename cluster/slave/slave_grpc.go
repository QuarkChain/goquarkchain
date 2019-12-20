package slave

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/params"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
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
	//s.slave.lock.Lock()
	s.slave.ctx.Timestamp = time.Now()
	//s.slave.lock.Unlock()
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

	if gReq.RootTip == nil {
		return nil, errors.New("handle masterInfo err:rootTip is nil")
	}

	s.slave.connManager.ModifyTarget(fmt.Sprintf("%s:%d", gReq.Ip, gReq.Port))

	//createShards
	if err = s.slave.CreateShards(gReq.RootTip, true); err != nil {
		return nil, err
	}

	//ping with other slaves
	s.slave.connManager.SetConnectToMasterAndSlaves(s.slave.clstrCfg.SlaveList)

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
	if err = s.slave.GenTx(gReq); err != nil {
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

	if gReq.NeedExtraInfo {
		gRes.Extra, err = s.slave.GetMinorBlockExtraInfo(gRes.MinorBlock, gReq.Branch)
		if err != nil {
			return nil, err
		}
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
		gRes     rpc.GetTxDetailResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.TxList, gRes.Next, err = s.slave.GetTransactionListByAddress(gReq.Address, gReq.TransferTokenID,
		gReq.Start, gReq.Limit); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetAllTx(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetAllTxRequest
		gRes     rpc.GetTxDetailResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if gRes.TxList, gRes.Next, err = s.slave.GetAllTx(gReq.Branch, gReq.Start, gReq.Limit); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gReq); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetLogs(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     qrpc.FilterQuery
		gRes     rpc.GetLogResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	if gRes.Logs, err = s.slave.GetLogs(&gReq); err != nil {
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

	if gRes.Result, err = s.slave.GasPrice(gReq.Branch, gReq.TokenID); err != nil {
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

	if work, err = s.slave.GetWork(gReq.Branch, gReq.CoinbaseAddr); err != nil {
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
func (s *SlaveServerSideOp) GetRootChainStakes(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.GetRootChainStakesRequest
		gRes     rpc.GetRootChainStakesResponse
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if gRes.Stakes, gRes.Signer, err = s.slave.GetRootChainStakes(gReq.Address, gReq.MinorBlockHash); err != nil {
		return nil, err
	}
	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
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
		gReq     rpc.P2PRedirectRequest
		gRes     rpc.GetMinorBlockListResponse
		hashList p2p.GetMinorBlockListRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(gReq.Data, &hashList); err != nil {
		return nil, err
	}
	if len(hashList.MinorBlockHashList) > 2*qsync.MinorBlockBatchSize {
		return nil, fmt.Errorf("bad number of minor blocks requested. branch: %d; limit: %d; expected limit: %d",
			gReq.Branch, len(hashList.MinorBlockHashList), qsync.MinorBlockBatchSize)
	}

	if gRes.MinorBlockList, err = s.slave.GetMinorBlockListByHashList(hashList.MinorBlockHashList, gReq.Branch); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetMinorBlockHeaderListWithSkip(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.P2PRedirectRequest
		gRes     p2p.GetMinorBlockHeaderListResponse
		reqSkip  p2p.GetMinorBlockHeaderListWithSkipRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(gReq.Data, &reqSkip); err != nil {
		return nil, err
	}
	if reqSkip.Limit <= 0 || uint64(reqSkip.Limit) > 2*qsync.MinorBlockHeaderListLimit {
		return nil, fmt.Errorf("bad limit. branch: %d; limit: %d; expected limit: %d",
			gReq.Branch, reqSkip.Limit, qsync.MinorBlockHeaderListLimit)
	}
	if reqSkip.Direction != qcom.DirectionToGenesis && reqSkip.Direction != qcom.DirectionToTip {
		return nil, errors.New("Bad direction")
	}
	if reqSkip.Type != qcom.SkipHash && reqSkip.Type != qcom.SkipHeight {
		return nil, errors.New("Bad type value")
	}

	if gRes.BlockHeaderList, err = s.slave.GetMinorBlockHeaderList(&reqSkip); err != nil {
		return nil, err
	}

	if response.Data, err = serialize.SerializeToBytes(gRes); err != nil {
		return nil, err
	}
	return response, nil
}

func (s *SlaveServerSideOp) GetMinorBlockHeaderList(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq     rpc.P2PRedirectRequest
		gRes     p2p.GetMinorBlockHeaderListResponse
		mHeader  p2p.GetMinorBlockHeaderListRequest
		response = &rpc.Response{RpcId: req.RpcId}
		err      error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(gReq.Data, &mHeader); err != nil {
		return nil, err
	}
	if mHeader.Limit <= 0 || uint64(mHeader.Limit) > 2*qsync.MinorBlockHeaderListLimit {
		return nil, fmt.Errorf("bad limit. branch: %d; limit: %d; expected limit: %d", gReq.Branch, mHeader.Limit, qsync.MinorBlockHeaderListLimit)
	}
	if mHeader.Direction != qcom.DirectionToGenesis {
		return nil, fmt.Errorf("Bad direction. branch: %d; ", gReq.Branch)
	}

	if gRes.BlockHeaderList, err = s.slave.GetMinorBlockHeaderList(&p2p.GetMinorBlockHeaderListWithSkipRequest{
		Type:      qcom.SkipHash,
		Data:      mHeader.BlockHash,
		Limit:     mHeader.Limit,
		Skip:      0,
		Direction: mHeader.Direction,
		Branch:    mHeader.Branch,
	}); err != nil {
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
		gReq rpc.P2PRedirectRequest
		txs  p2p.NewTransactionList
		err  error
	)

	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}

	err = serialize.DeserializeFromBytes(gReq.Data, &txs)
	if err != nil {
		return nil, err
	}
	if len(txs.TransactionList) > params.NEW_TRANSACTION_LIST_LIMIT {
		return nil, fmt.Errorf("too many txs in one command, tx count: %d\n", len(txs.TransactionList))
	}
	addTxList := func(branch uint32, txs []*types.Transaction) error {
		err := s.slave.AddTxList(gReq.PeerID, branch, txs)
		if err != nil {
			return err
		}
		return nil
	}

	if gReq.Branch != 0 {
		err := addTxList(gReq.Branch, txs.TransactionList)
		return new(rpc.Response), err
	}

	var (
		txList       = make(map[uint32][]*types.Transaction)
		fullShardIds = s.slave.fullShardList
	)

	for _, tx := range txs.TransactionList {
		fromShardSize, err := s.slave.clstrCfg.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
		if err != nil {
			return nil, err
		}
		tx.EvmTx.SetFromShardSize(fromShardSize)
		fId := tx.EvmTx.FromFullShardId()
		for _, id := range fullShardIds {
			if fId != id {
				continue
			}
			if _, ok := txList[id]; !ok {
				txList[id] = make([]*types.Transaction, 0, len(txs.TransactionList))
			}
			txList[id] = append(txList[id], tx)
		}
	}

	var (
		g errgroup.Group
	)
	for branch, txs := range txList {
		branch, txs := branch, txs
		g.Go(func() error {
			return addTxList(branch, txs)
		})
	}

	return new(rpc.Response), g.Wait()
}

func (s *SlaveServerSideOp) HandleNewMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		gReq   rpc.P2PRedirectRequest
		mblock p2p.NewBlockMinor
		err    error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &gReq); err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(gReq.Data, &mblock); err != nil {
		return nil, err
	}
	if gReq.Branch != mblock.Block.Branch().Value {
		return nil, fmt.Errorf("invalid NewBlockMinor Request: mismatch branch value from peer %v. in request meta: %d, in minor header: %d",
			gReq.PeerID, gReq.Branch, mblock.Block.Branch().Value)
	}
	if err = s.slave.NewMinorBlock(gReq.PeerID, mblock.Block); err != nil {
		return nil, err
	}
	return &rpc.Response{}, nil
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

func (s *SlaveServerSideOp) CheckMinorBlocksInRoot(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var (
		rootBlock types.RootBlock
		response  = &rpc.Response{RpcId: req.RpcId}
		err       error
	)
	if err = serialize.DeserializeFromBytes(req.Data, &rootBlock); err != nil {
		return nil, err
	}
	return response, s.slave.CheckMinorBlocksInRoot(&rootBlock)
}
