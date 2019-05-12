package master

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"time"
)

type SlaveConnection struct {
	target        string
	shardMaskList []*types.ChainMask
	client        rpc.Client
	slaveID       string
}

// create slave connection manager
func NewSlaveConn(target string, shardMaskList []*types.ChainMask, slaveID string) *SlaveConnection {
	client := rpc.NewClient(rpc.SlaveServer)
	return &SlaveConnection{
		target:        target,
		client:        client,
		shardMaskList: shardMaskList,
		slaveID:       slaveID,
	}
}

func (s *SlaveConnection) hasOverlap(chainMask *types.ChainMask) bool {
	for _, localChainMask := range s.shardMaskList {
		if localChainMask.HasOverlap(chainMask.GetMask()) {
			return true
		}
	}
	return false
}

func (s *SlaveConnection) HeartBeat() bool {
	var tryTimes = 3
	for tryTimes > 0 {
		req := rpc.Request{Op: rpc.OpHeartBeat, Data: nil}
		_, err := s.client.Call(s.target, &req)
		if err != nil {
			log.Error(s.slaveID, "heart beat err", err)
			time.Sleep(time.Duration(1) * time.Second)
			tryTimes -= 1
			continue
		}
		return true
	}
	return false
}

func (s *SlaveConnection) SendPing(rootBlock *types.RootBlock, initializeShardSize bool) ([]byte, []*types.ChainMask, error) {
	req := new(rpc.Ping)
	req.RootTip = rootBlock

	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, nil, err
	}

	request := rpc.Request{Op: rpc.OpPing, Data: bytes}

	rsp, err := s.client.Call(s.target, &request)
	if err != nil {
		return nil, nil, err
	}
	pongMsg := new(rpc.Pong)
	err = serialize.DeserializeFromBytes(rsp.Data, pongMsg)
	if err != nil {
		return nil, nil, err
	}
	return pongMsg.Id, pongMsg.ChainMaskList, nil
}

func (s *SlaveConnection) SendConnectToSlaves(slaveInfoLst []*rpc.SlaveInfo) error {
	req := rpc.ConnectToSlavesRequest{SlaveInfoList: slaveInfoLst}
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}
	rsp, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpConnectToSlaves, Data: bytes})
	if err != nil {
		return err
	}
	connectToSlavesResponse := new(rpc.ConnectToSlavesResponse)
	err = serialize.DeserializeFromBytes(rsp.Data, connectToSlavesResponse)
	if err != nil {
		return err
	}

	if len(connectToSlavesResponse.ResultList) != len(slaveInfoLst) {
		return errors.New("len not match")
	}

	for _, result := range connectToSlavesResponse.ResultList {
		if len(result.Result) > 0 {
			return errors.New("result len >0")
		}
	}
	return nil
}

func (s *SlaveConnection) hasShard(fullShardID uint32) bool {
	for _, chainMask := range s.shardMaskList {
		if chainMask.ContainFullShardId(fullShardID) {
			return true
		}
	}
	return false
}

func (s *SlaveConnection) AddTransaction(tx *types.Transaction) error {
	var (
		req = rpc.AddTransactionRequest{Tx: tx}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}

	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddTransaction, Data: bytes})
	if err != nil {
		return err
	}
	return nil

}

func (s *SlaveConnection) ExecuteTransaction(tx *types.Transaction, fromAddress *account.Address, height *uint64) ([]byte, error) {
	var (
		req = rpc.ExecuteTransactionRequest{Tx: tx, FromAddress: fromAddress, BlockHeight: height}
		rsp = new(rpc.ExecuteTransactionResponse)
		res = new(rpc.Response)
	)

	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpExecuteTransaction, Data: bytes})
	if err != nil {
		return nil, err
	}

	err = serialize.DeserializeFromBytes(res.Data, rsp)
	if err != nil {
		return nil, err
	}
	return rsp.Result, nil

}

func (s *SlaveConnection) GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) (*types.MinorBlock, error) {
	var (
		req              = rpc.GetMinorBlockRequest{Branch: branch.Value, MinorBlockHash: blockHash}
		minBlockResponse = rpc.GetMinorBlockResponse{}
		res              *rpc.Response
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlock, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), &minBlockResponse); err != nil {
		return nil, err
	}
	return minBlockResponse.MinorBlock, nil
}

func (s *SlaveConnection) GetMinorBlockByHeight(height uint64, branch account.Branch) (*types.MinorBlock, error) {
	var (
		req              = rpc.GetMinorBlockRequest{Branch: branch.Value, Height: height}
		minBlockResponse = rpc.GetMinorBlockResponse{}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlock, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err := serialize.Deserialize(serialize.NewByteBuffer(res.Data), &minBlockResponse); err != nil {
		return nil, err
	}
	return minBlockResponse.MinorBlock, nil
}

func (s *SlaveConnection) GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error) {
	var (
		req   = rpc.GetTransactionRequest{Branch: branch.Value, TxHash: txHash}
		trans = rpc.GetTransactionResponse{}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, 0, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetTransaction, Data: bytes})
	if err != nil {
		return nil, 0, err
	}
	if err := serialize.Deserialize(serialize.NewByteBuffer(res.Data), &trans); err != nil {
		return nil, 0, err
	}
	return trans.MinorBlock, trans.Index, nil
}

func (s *SlaveConnection) GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error) {
	var (
		req = rpc.GetTransactionReceiptRequest{Branch: branch.Value, TxHash: txHash}
		rsp = new(rpc.GetTransactionReceiptResponse)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, 0, nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetTransactionReceipt, Data: bytes})
	if err != nil {
		return nil, 0, nil, err
	}

	if err := serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp); err != nil {
		return nil, 0, nil, err
	}
	return rsp.MinorBlock, rsp.Index, rsp.Receipt, nil
}

func (s *SlaveConnection) GetTransactionsByAddress(address *account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	var (
		req   = rpc.GetTransactionListByAddressRequest{Address: address, Start: start, Limit: limit}
		trans = rpc.GetTransactionListByAddressResponse{}
		res   *rpc.Response
		bytes []byte
		err   error
	)
	bytes, err = serialize.SerializeToBytes(req)
	if err != nil {
		return nil, nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetTransactionListByAddress, Data: bytes})
	if err != nil {
		return nil, nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), &trans); err != nil {
		return nil, nil, err
	}
	return trans.TxList, trans.Next, nil
}

func (s *SlaveConnection) GetLogs(branch account.Branch, address []*account.Address, topics []*rpc.Topic, startBlock, endBlock uint64) ([]*types.Log, error) {
	var (
		req = rpc.GetLogRequest{
			Branch:     branch.Value,
			Addresses:  address,
			Topics:     topics,
			StartBlock: startBlock,
			EndBlock:   endBlock,
		}
		rsp = new(rpc.GetLogResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetLogs, Data: bytes})
	if err != nil {
		return nil, err
	}
	err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp)
	return rsp.Logs, err

}
func (s *SlaveConnection) EstimateGas(tx *types.Transaction, fromAddress *account.Address) (uint32, error) {
	var (
		req = rpc.EstimateGasRequest{
			Tx:          tx,
			FromAddress: fromAddress,
		}
		rsp = new(rpc.EstimateGasResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return 0, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpEstimateGas, Data: bytes})
	if err != nil {
		return 0, err
	}
	err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp)
	return rsp.Result, err
}
func (s *SlaveConnection) GetStorageAt(address *account.Address, key common.Hash, height *uint64) (common.Hash, error) {
	var (
		req = rpc.GetStorageRequest{
			Address:     address,
			Key:         key,
			BlockHeight: height,
		}
		rsp = new(rpc.GetStorageResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return common.Hash{}, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetStorageAt, Data: bytes})
	if err != nil {
		return common.Hash{}, err
	}
	err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp)
	return rsp.Result, err
}
func (s *SlaveConnection) GetCode(address *account.Address, height *uint64) ([]byte, error) {
	var (
		req = rpc.GetCodeRequest{
			Address:     address,
			BlockHeight: height,
		}
		rsp = new(rpc.GetCodeResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetCode, Data: bytes})
	if err != nil {
		return nil, err
	}
	err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp)
	return rsp.Result, err
}
func (s *SlaveConnection) GasPrice(branch account.Branch) (uint64, error) {
	var (
		req = rpc.GasPriceRequest{
			Branch: branch.Value,
		}
		rsp = new(rpc.GasPriceResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return 0, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGasPrice, Data: bytes})
	if err != nil {
		return 0, err
	}
	err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp)
	return rsp.Result, err
}
func (s *SlaveConnection) GetWork(branch account.Branch) (*consensus.MiningWork, error) {
	return &consensus.MiningWork{}, nil
}
func (s *SlaveConnection) SubmitWork(branch account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) error {
	return nil
}

func (s *SlaveConnection) SendMiningConfigToSlaves(artificialTxConfig *rpc.ArtificialTxConfig, mining bool) error {
	var (
		req = rpc.MineRequest{
			ArtificialTxConfig: artificialTxConfig,
			Mining:             mining,
		}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMine, Data: bytes})
	if err != nil {
		return err
	}
	return nil
}

func (s *SlaveConnection) GetUnconfirmedHeaders() (*rpc.GetUnconfirmedHeadersResponse, error) {
	var (
		rsp = new(rpc.GetUnconfirmedHeadersResponse)
	)

	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetUnconfirmedHeaderList})
	if err != nil {
		return nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), &rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (s *SlaveConnection) GetEcoInfoListRequest() (*rpc.GetEcoInfoListResponse, error) {
	var (
		rsp = new(rpc.GetEcoInfoListResponse)
	)
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetEcoInfoList})
	if err != nil {
		return nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (s *SlaveConnection) GetAccountData(address *account.Address, height *uint64) (*rpc.GetAccountDataResponse, error) {
	var (
		req = rpc.GetAccountDataRequest{
			Address:     address,
			BlockHeight: height,
		}
		rsp = new(rpc.GetAccountDataResponse)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetAccountData, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (s *SlaveConnection) AddRootBlock(rootBlock *types.RootBlock, expectSwitch bool) error {
	var (
		req = rpc.AddRootBlockRequest{
			RootBlock:    rootBlock,
			ExpectSwitch: expectSwitch,
		}
		rsp = new(rpc.AddRootBlockResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddRootBlock, Data: bytes})
	if err != nil {
		return err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp); err != nil {
		return err
	}
	return nil
}

func (s *SlaveConnection) GenTx(numTxPerShard, xShardPercent uint32, tx *types.Transaction) error {
	var (
		req = rpc.GenTxRequest{
			NumTxPerShard: numTxPerShard,
			XShardPercent: xShardPercent,
			Tx:            tx,
		}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGenTx, Data: bytes})
	if err != nil {
		return err
	}
	return nil
}
func (s *SlaveConnection) AddTransactions(request *p2p.NewTransactionList) (*rpc.HashList, error) {
	var (
		rsp = new(rpc.HashList)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddTransactions, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(res.Data, rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (s *SlaveConnection) GetMinorBlocks(request *p2p.GetMinorBlockListRequest) (*p2p.GetMinorBlockListResponse, error) {
	var (
		rsp = new(p2p.GetMinorBlockListResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlockList, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(res.Data, rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (s *SlaveConnection) GetMinorBlockHeaders(request *p2p.GetMinorBlockHeaderListRequest) (*p2p.GetMinorBlockHeaderListResponse, error) {
	var (
		rsp = new(p2p.GetMinorBlockHeaderListResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlockHeaderList, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(res.Data, rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}
func (s *SlaveConnection) HandleNewTip(request *p2p.Tip) (bool, error) {
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return false, err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpHandleNewTip, Data: bytes})
	if err != nil {
		return false, err
	}

	return true, nil
}
func (s *SlaveConnection) AddMinorBlock(request *p2p.NewBlockMinor) (bool, error) {
	blockData, err := serialize.SerializeToBytes(request.Block)
	if err != nil {
		return false, err
	}
	var (
		req = rpc.AddMinorBlockRequest{
			MinorBlockData: blockData,
		}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return false, err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddMinorBlock, Data: bytes})
	if err != nil {
		return false, err
	}
	return true, nil
}
