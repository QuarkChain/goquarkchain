package master

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type SlaveConnection struct {
	target       string
	chainMaskLst []*types.ChainMask
	client       rpc.Client
	slaveID      string
}

// create slave connection manager
func NewSlaveConn(target string, shardMaskLst []*types.ChainMask, slaveID string) *SlaveConnection {
	client := rpc.NewClient(rpc.SlaveServer)
	return &SlaveConnection{
		target:       target,
		client:       client,
		chainMaskLst: shardMaskLst,
		slaveID:      slaveID,
	}
}

func (s *SlaveConnection) hasOverlap(chainMask *types.ChainMask) bool {
	for _, localChainMask := range s.chainMaskLst {
		if localChainMask.HasOverlap(chainMask.GetMask()) {
			return true
		}
	}
	return false
}
func (s *SlaveConnection) HeartBeat(request *rpc.Request) bool {
	//fmt.Println("hhhhhhhhh")
	_, err := s.client.Call(s.target, request)
	//fmt.Println("ree", err)
	if err != nil {
		return false
	}
	return true
}

func (s *SlaveConnection) SendPing(rootBlockChain *core.RootBlockChain, initializeShardSize bool) ([]byte, []*types.ChainMask, error) {
	req := new(rpc.Ping)
	if initializeShardSize {
		req.RootTip = rootBlockChain.CurrentBlock()
	} else {
		req.RootTip = nil
	}

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
	for _, chainMask := range s.chainMaskLst {
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
		goto FALSE
	}

	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddTransaction, Data: bytes})
	if err != nil {
		goto FALSE
	}
	return nil
FALSE:
	log.Error("slave connection", "target", s.target, err)
	return err
}

// TODO return type is not confirmed.
func (s *SlaveConnection) ExecuteTransaction(tx *types.Transaction, fromAddress account.Address, height *uint64) ([]byte, error) {
	var (
		req = rpc.ExecuteTransactionRequest{Tx: tx, FromAddress: fromAddress, BlockHeight: height}
		rsp = new(rpc.ExecuteTransactionResponse)
		res = new(rpc.Response)
	)

	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		goto FALSE
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpExecuteTransaction, Data: bytes})
	if err != nil {
		goto FALSE
	}

	err = serialize.DeserializeFromBytes(res.Data, rsp)
	if err != nil {
		goto FALSE
	}
	return rsp.Result, nil
FALSE:
	log.Error("slave connection", "target", s.target, err)
	return nil, err
}

func (s *SlaveConnection) GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) (*types.MinorBlock, error) {
	var (
		req              = rpc.GetMinorBlockRequest{Branch: branch, MinorBlockHash: blockHash}
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
		req              = rpc.GetMinorBlockRequest{Branch: branch, Height: height}
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
		req   = rpc.GetTransactionRequest{Branch: branch, TxHash: txHash}
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
		req = rpc.GetTransactionReceiptRequest{Branch: branch, TxHash: txHash}
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
	//fmt.Println("RRRRRR-data", hex.EncodeToString(res.Data))
	if err := serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp); err != nil {
		return nil, 0, nil, err
	}
	//fmt.Println("RRRRRRRRRR", rsp.Receipt.TxHash.String(), rsp.Receipt.Status)
	return rsp.MinorBlock, rsp.Index, rsp.Receipt, nil
}

func (s *SlaveConnection) GetTransactionsByAddress(address account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	var (
		req   = rpc.GetTransactionListByAddressRequest{Address: address, Start: start, Limit: limit}
		trans = rpc.GetTransactionListByAddressResponse{}
		res   *rpc.Response
		bytes []byte
		err   error
	)
	bytes, err = serialize.SerializeToBytes(req)
	if err != nil {
		goto FALSE
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetTransactionListByAddress, Data: bytes})
	if err != nil {
		goto FALSE
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), &trans); err != nil {
		goto FALSE
	}
	return trans.TxList, trans.Next, nil
FALSE:
	return nil, nil, err
}

func (s *SlaveConnection) GetLogs(branch account.Branch, address []account.Address, topics []*rpc.Topic, startBlock, endBlock uint64) ([]*types.Log, error) {
	var (
		req = rpc.GetLogRequest{
			Branch:     branch,
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
func (s *SlaveConnection) EstimateGas(tx *types.Transaction, fromAddress account.Address) (uint32, error) {
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
func (s *SlaveConnection) GetStorageAt(address account.Address, key *serialize.Uint256, height uint64) (*serialize.Uint256, error) {
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
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetStorageAt, Data: bytes})
	if err != nil {
		return nil, err
	}
	err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp)
	return rsp.Result, err
}
func (s *SlaveConnection) GetCode(address account.Address, height uint64) ([]byte, error) {
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
			Branch: branch,
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

	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetUnconfirmedHeaders})
	if err != nil {
		return nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), &rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (s *SlaveConnection) GetMinorBlockToMine(branch account.Branch, address account.Address, artificialTxConfig *rpc.ArtificialTxConfig) (*types.MinorBlock, error) {
	var (
		req = rpc.GetNextBlockToMineRequest{
			Branch:             branch,
			Address:            address.AddressInBranch(branch),
			ArtificialTxConfig: artificialTxConfig,
		}
		rsp = new(rpc.GetNextBlockToMineResponse)
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetNextBlockToMine, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), rsp); err != nil {
		return nil, err
	}
	return rsp.Block, nil
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

func (s *SlaveConnection) GetAccountData(address account.Address, height uint64) (*rpc.GetAccountDataResponse, error) {
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

func (s *SlaveConnection) AddMinorBlock(blockData []byte) error {
	var (
		req = rpc.AddMinorBlockRequest{
			MinorBlockData: blockData,
		}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddMinorBlock, Data: bytes})
	if err != nil {
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
