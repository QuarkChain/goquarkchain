package master

import (
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type SlaveConnManager struct {
	count              int
	clientPool         []rpc.ISlaveConn
	branchToSlaveConns map[uint32][]rpc.ISlaveConn
	logInfo            string
}

func (s *SlaveConnManager) InitConnManager(cfg *config.ClusterConfig) error {
	s.clientPool = make([]rpc.ISlaveConn, 0, len(cfg.SlaveList))
	s.branchToSlaveConns = make(map[uint32][]rpc.ISlaveConn)
	s.logInfo = "slave connection manager"

	fullShardIds := cfg.Quarkchain.GetGenesisShardIds()
	for _, cfg := range cfg.SlaveList {
		target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
		client := NewSlaveConn(target, cfg.FullShardList, cfg.ID)
		s.clientPool = append(s.clientPool, client)

		id, chainMaskList, err := client.SendPing()
		if err != nil {
			return err
		}
		if err := checkPing(client, id, chainMaskList); err != nil {
			return err
		}
		for _, fullShardID := range fullShardIds {
			if client.HasShard(fullShardID) {
				s.branchToSlaveConns[fullShardID] = append(s.branchToSlaveConns[fullShardID], client)
				log.Info(s.logInfo, "branch:", fmt.Sprintf("%x", fullShardID), "is run by slave", client.GetSlaveID())
			}
		}
	}
	s.count = len(s.clientPool)

	return nil
}

func (c *SlaveConnManager) GetOneSlaveConnById(fullShardId uint32) rpc.ISlaveConn {
	if conns, ok := c.branchToSlaveConns[fullShardId]; ok {
		return conns[0]
	}
	return nil
}

func (c *SlaveConnManager) GetSlaveConnsById(fullShardId uint32) []rpc.ISlaveConn {
	if conns, ok := c.branchToSlaveConns[fullShardId]; ok {
		return conns
	}
	return nil
}

func (c *SlaveConnManager) GetSlaveConns() []rpc.ISlaveConn {
	return c.clientPool
}

func (c *SlaveConnManager) ConnCount() int {
	return c.count
}

type SlaveConnection struct {
	target        string
	shardMaskList []uint32
	client        rpc.Client
	slaveID       string
	logInfo       string
	mu            sync.Mutex
}

// create slave connection manager
func NewSlaveConn(target string, shardMaskList []uint32, slaveID string) *SlaveConnection {
	client := rpc.NewClient(rpc.SlaveServer)
	return &SlaveConnection{
		target:        target,
		client:        client,
		shardMaskList: shardMaskList,
		slaveID:       slaveID,
		logInfo:       fmt.Sprintf("%v", slaveID),
	}
}

func (s *SlaveConnection) GetSlaveID() string {
	return s.slaveID
}

func (s *SlaveConnection) GetFullShardList() []uint32 {
	return s.shardMaskList
}

func (s *SlaveConnection) HeartBeat() bool {
	var tryTimes = 3
	for tryTimes > 0 {
		req := rpc.Request{Op: rpc.OpHeartBeat, Data: nil}
		_, err := s.client.Call(s.target, &req)
		if err != nil {
			time.Sleep(time.Duration(1) * time.Second)
			tryTimes -= 1
			continue
		}
		return true
	}
	log.Error(s.logInfo, "heartBeat err", "will shut down")
	return false
}

func (s *SlaveConnection) MasterInfo(ip string, port uint16, rootTip *types.RootBlock) error {
	if rootTip == nil {
		return errors.New("send MasterInfo failed :rootTip is nil")
	}
	var (
		gReq = rpc.MasterInfo{Ip: ip, Port: port, RootTip: rootTip}
	)
	bytes, err := serialize.SerializeToBytes(gReq)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpMasterInfo, Data: bytes})
	return err
}

func (s *SlaveConnection) SendPing() ([]byte, []uint32, error) {
	req := new(rpc.Ping)

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
	return pongMsg.Id, pongMsg.FullShardList, nil
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

func (s *SlaveConnection) HasShard(fullShardID uint32) bool {
	for _, v := range s.shardMaskList {
		if v == fullShardID {
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

func (s *SlaveConnection) GetMinorBlockByHash(blockHash common.Hash, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *rpc.PoSWInfo, error) {
	return s.getMinorBlock(blockHash, nil, branch, needExtraInfo)
}

func (s *SlaveConnection) GetMinorBlockByHeight(height *uint64, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *rpc.PoSWInfo, error) {
	return s.getMinorBlock(common.Hash{}, height, branch, needExtraInfo)
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

func (s *SlaveConnection) GetTransactionsByAddress(address *account.Address, start []byte, limit uint32, transferTokenID *uint64) ([]*rpc.TransactionDetail, []byte, error) {
	var (
		req   = rpc.GetTransactionListByAddressRequest{Address: address, TransferTokenID: transferTokenID, Start: start, Limit: limit}
		trans = rpc.GetTxDetailResponse{}
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
	if err = serialize.DeserializeFromBytes(res.Data, &trans); err != nil {
		return nil, nil, err
	}
	return trans.TxList, trans.Next, nil
}

func (s *SlaveConnection) GetAllTx(branch account.Branch, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	var (
		req     = rpc.GetAllTxRequest{Branch: branch, Start: start, Limit: limit}
		trans   = rpc.GetTxDetailResponse{}
		res     *rpc.Response
		reqData []byte
		err     error
	)
	reqData, err = serialize.SerializeToBytes(req)
	if err != nil {
		return nil, nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetAllTx, Data: reqData})
	if err != nil {
		return nil, nil, err
	}
	if err = serialize.DeserializeFromBytes(res.Data, &trans); err != nil {
		return nil, nil, err
	}
	return trans.TxList, trans.Next, nil
}

func (s *SlaveConnection) GetLogs(args *qrpc.FilterQuery) ([]*types.Log, error) {
	var (
		rsp = new(rpc.GetLogResponse)
		res = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(args)
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

func (s *SlaveConnection) GasPrice(branch account.Branch, tokenID uint64) (uint64, error) {
	var (
		req = rpc.GasPriceRequest{
			Branch:  branch.Value,
			TokenID: tokenID,
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

func (s *SlaveConnection) GetWork(branch account.Branch, coinbaseAddr *account.Address) (*consensus.MiningWork, error) {
	var (
		req = rpc.GetWorkRequest{
			Branch:       branch.Value,
			CoinbaseAddr: coinbaseAddr,
		}
		rsp consensus.MiningWork
	)
	bytes, err := serialize.SerializeToBytes(&req)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetWork, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(res.Data, &rsp); err != nil {
		return nil, err
	}
	return &rsp, nil
}

func (s *SlaveConnection) SubmitWork(work *rpc.SubmitWorkRequest) (success bool, err error) {
	var (
		gRes  rpc.SubmitWorkResponse
		bytes []byte
		res   *rpc.Response
	)
	bytes, err = serialize.SerializeToBytes(work)
	if err != nil {
		return
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpSubmitWork, Data: bytes})
	if err != nil {
		return
	}
	if err = serialize.DeserializeFromBytes(res.Data, &gRes); err != nil {
		return
	}
	return gRes.Success, nil
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

	tryCnt := 3
	for tryCnt > 0 {
		tryCnt--
		res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddRootBlock, Data: bytes})
		if err == nil {
			break
		}
		log.Info("SlaveConnection AddRootBlock", "reTryCnt", 3-tryCnt, "height", rootBlock.NumberU64(), "err", err)

	}

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

func (s *SlaveConnection) AddTransactions(request *rpc.P2PRedirectRequest) error {
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddTransactions, Data: bytes})
	if err != nil {
		return err
	}
	return nil
}

func (s *SlaveConnection) GetMinorBlocks(request *rpc.P2PRedirectRequest) ([]byte, error) {
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlockList, Data: bytes})
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (s *SlaveConnection) GetMinorBlockHeaderListWithSkip(req *rpc.P2PRedirectRequest) ([]byte, error) {
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlockHeaderListWithSkip, Data: bytes})
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (s *SlaveConnection) GetMinorBlockHeaderList(req *rpc.P2PRedirectRequest) ([]byte, error) {
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlockHeaderList, Data: bytes})
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func (s *SlaveConnection) HandleNewTip(request *rpc.HandleNewTipRequest) error {
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpHandleNewTip, Data: bytes})
	return err
}

func (s *SlaveConnection) HandleNewMinorBlock(req *rpc.P2PRedirectRequest) error {
	data, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpHandleNewMinorBlock, Data: data})
	if err != nil {
		return err
	}
	return nil
}

func (s *SlaveConnection) AddBlockListForSync(request *rpc.AddBlockListForSyncRequest) (*rpc.ShardStatus, error) {
	var (
		shardStatus = new(rpc.ShardStatus)
		res         = new(rpc.Response)
	)
	bytes, err := serialize.SerializeToBytes(request)
	if err != nil {
		return nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddMinorBlockListForSync, Data: bytes})
	if err != nil {
		return nil, err
	}
	if err = serialize.DeserializeFromBytes(res.Data, shardStatus); err != nil {
		return nil, err
	}
	return shardStatus, nil
}

func (s *SlaveConnection) SetMining(mining bool) error {
	bytes, err := serialize.SerializeToBytes(mining)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpSetMining, Data: bytes})
	return err
}

func (s *SlaveConnection) CheckMinorBlocksInRoot(rootBlock *types.RootBlock) error {
	bytes, err := serialize.SerializeToBytes(rootBlock)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpCheckMinorBlocksInRoot, Data: bytes})
	return err
}

// get minor block by hash or by height
func (s *SlaveConnection) getMinorBlock(hash common.Hash, height *uint64,
	branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *rpc.PoSWInfo, error) {
	var (
		req              = rpc.GetMinorBlockRequest{Branch: branch.Value, MinorBlockHash: hash, Height: height, NeedExtraInfo: needExtraInfo}
		minBlockResponse = rpc.GetMinorBlockResponse{}
		res              *rpc.Response
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetMinorBlock, Data: bytes})
	if err != nil {
		return nil, nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), &minBlockResponse); err != nil {
		return nil, nil, err
	}
	return minBlockResponse.MinorBlock, minBlockResponse.Extra, nil
}

func (s *SlaveConnection) GetRootChainStakes(address account.Address, lastMinor common.Hash) (*big.Int,
	*account.Recipient, error) {
	var (
		getRootChainStakesRequest  = rpc.GetRootChainStakesRequest{Address: address, MinorBlockHash: lastMinor}
		getRootChainStakesResponse = rpc.GetRootChainStakesResponse{}
		res                        *rpc.Response
	)
	bytes, err := serialize.SerializeToBytes(getRootChainStakesRequest)
	if err != nil {
		return nil, nil, err
	}
	res, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetRootChainStakes, Data: bytes})
	if err != nil {
		return nil, nil, err
	}
	if err = serialize.Deserialize(serialize.NewByteBuffer(res.Data), &getRootChainStakesResponse); err != nil {
		return nil, nil, err
	}
	return getRootChainStakesResponse.Stakes, getRootChainStakesResponse.Signer, nil
}
