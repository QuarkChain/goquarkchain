package master

import (
	"bou.ke/monkey"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	ethRPC "github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
	"time"
)

type fakeRpcClient struct {
	target       string
	chainMaskLst []*types.ChainMask
	slaveID      string
	chanOP       chan uint32
	config       *config.ClusterConfig
	branchs      []*account.Branch
}

func NewFakeRPCClient(chanOP chan uint32, target string, shardMaskLst []*types.ChainMask, slaveID string, config *config.ClusterConfig) *fakeRpcClient {
	f := &fakeRpcClient{
		chanOP:       chanOP,
		target:       target,
		chainMaskLst: shardMaskLst,
		slaveID:      slaveID,
		config:       config,
		branchs:      make([]*account.Branch, 0),
	}
	f.initBranch()
	return f
}
func (c *fakeRpcClient) initBranch() {
	for _, v := range c.config.Quarkchain.GetGenesisShardIds() {
		if c.coverShardID(v) {
			c.branchs = append(c.branchs, &account.Branch{Value: v})
		}
	}
}
func (c *fakeRpcClient) GetOpName(op uint32) string {
	return "SB"
}

func (c *fakeRpcClient) coverShardID(fullShardID uint32) bool {
	for _, chainMask := range c.chainMaskLst {
		if chainMask.ContainFullShardId(fullShardID) {
			return true
		}
	}
	return false

}
func (c *fakeRpcClient) Call(hostport string, req *rpc.Request) (*rpc.Response, error) {
	switch req.Op {
	case rpc.OpHeartBeat:
		if c.chanOP != nil {
			c.chanOP <- rpc.OpHeartBeat
		}
		return nil, nil
	case rpc.OpPing:
		rsp := new(rpc.Pong)
		rsp.Id = []byte(c.slaveID)
		rsp.ChainMaskList = c.chainMaskLst
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpConnectToSlaves:
		rsp := new(rpc.ConnectToSlavesResponse)
		rsp.ResultList = make([]*rpc.ConnectToSlavesResult, len(c.config.SlaveList))
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetUnconfirmedHeaders:
		rsp := new(rpc.GetUnconfirmedHeadersResponse)
		for _, v := range c.branchs {
			rsp.HeadersInfoList = append(rsp.HeadersInfoList, &rpc.HeadersInfo{
				Branch:     account.Branch{Value: v.Value},
				HeaderList: make([]*types.MinorBlockHeader, 0),
			})
			//rsp.HeadersInfoList[0].HeaderList = append(rsp.HeadersInfoList[0].HeaderList, &types.MinorBlockHeader{})
		}
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetNextBlockToMine:
		rsp := new(rpc.GetNextBlockToMineResponse)
		rsp.Block = types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetEcoInfoList:
		rsp := new(rpc.GetEcoInfoListResponse)
		for _, v := range c.branchs {
			rsp.EcoInfoList = append(rsp.EcoInfoList, &rpc.EcoInfo{
				Branch: account.Branch{Value: v.Value},
			})
		}
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetAccountData:
		rsp := new(rpc.GetAccountDataResponse)
		for _, v := range c.branchs {
			rsp.AccountBranchDataList = append(rsp.AccountBranchDataList, &rpc.AccountBranchData{Branch: *v})
		}
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetMine:
		return &rpc.Response{}, nil
	case rpc.OpAddRootBlock:
		rsp := new(rpc.AddRootBlockResponse)
		rsp.Switched = false
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpAddMinorBlock:
		return &rpc.Response{}, nil
	case rpc.OpAddTransaction:
		return &rpc.Response{}, nil
	case rpc.OpExecuteTransaction:
		rsp := new(rpc.ExecuteTransactionResponse)
		rsp.Result = []byte("qkc")
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetMinorBlock:
		rsp := new(rpc.GetMinorBlockResponse)
		rsp.MinorBlock = types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetTransaction:
		rsp := new(rpc.GetTransactionResponse)
		rsp.MinorBlock = types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
		rsp.Index = 1
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetTransactionReceipt:
		reqData := new(rpc.GetTransactionReceiptRequest)
		err := serialize.DeserializeFromBytes(req.Data, reqData)
		if err != nil {
			panic(err)
		}
		rsp := new(rpc.GetTransactionReceiptResponse)
		rsp.MinorBlock = types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
		rsp.Index = 1
		rsp.Receipt = &types.Receipt{
			CumulativeGasUsed: 123,
		}
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		//fmt.Println("data----", hex.EncodeToString(data))
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetTransactionListByAddress:
		rsp := new(rpc.GetTransactionListByAddressResponse)
		rsp.Next = []byte("qkc")
		rsp.TxList = append(rsp.TxList, &rpc.TransactionDetail{
			TxHash: common.BigToHash(new(big.Int).SetUint64(11)),
		})
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetLogs:
		rsp := new(rpc.GetLogResponse)
		rsp.Logs = append(rsp.Logs, &types.Log{
			Data: []byte("qkc"),
		})
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpEstimateGas:
		rsp := new(rpc.EstimateGasResponse)
		rsp.Result = 123
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetStorageAt:
		rsp := new(rpc.GetStorageResponse)
		rsp.Result = common.BigToHash(new(big.Int).SetUint64(123))
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGetCode:
		rsp := new(rpc.GetCodeResponse)
		rsp.Result = []byte("qkc")
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	case rpc.OpGasPrice:
		rsp := new(rpc.GasPriceResponse)
		rsp.Result = uint64(123)
		data, err := serialize.SerializeToBytes(rsp)
		if err != nil {
			return nil, err
		}
		return &rpc.Response{Data: data}, nil
	default:
		fmt.Println("codeM", req.Op)
		return nil, errors.New("unkown code")
	}
}

func initEnv(t *testing.T, chanOp chan uint32) *QKCMasterBackend {
	monkey.Patch(NewSlaveConn, func(target string, shardMaskLst []*types.ChainMask, slaveID string) *SlaveConnection {
		client := NewFakeRPCClient(chanOp, target, shardMaskLst, slaveID, config.NewClusterConfig())
		return &SlaveConnection{
			target:        target,
			client:        client,
			shardMaskList: shardMaskLst,
			slaveID:       slaveID,
		}
	})
	monkey.Patch(createDB, func(ctx *service.ServiceContext, name string) (ethdb.Database, error) {
		return ethdb.NewMemDatabase(), nil
	})

	ctx := &service.ServiceContext{}
	clusterConfig := config.NewClusterConfig()
	clusterConfig.Quarkchain.Root.ConsensusType = config.PoWFake
	master, err := New(ctx, clusterConfig)
	if err != nil {
		panic(err)
	}
	if err := master.InitCluster(); err != nil {
		assert.NoError(t, err)
	}
	return master
}
func TestMasterBackend_InitCluster(t *testing.T) {
	initEnv(t, nil)

}

func TestMasterBackend_HeartBeat(t *testing.T) {
	chanOp := make(chan uint32, 100)
	master := initEnv(t, chanOp)
	master.Heartbeat()
	status := true
	countHeartBeat := 0
	for status {
		select {
		case op := <-chanOp:
			if op == rpc.OpHeartBeat {
				countHeartBeat++
			}
			if countHeartBeat == len(master.clusterConfig.SlaveList) {
				status = false
			}
		case <-time.After(2 * time.Second):
			panic(errors.New("no receive Heartbeat"))
		}
	}
}

func TestGetSlaveConnByBranch(t *testing.T) {
	master := initEnv(t, nil)
	for _, v := range master.clusterConfig.Quarkchain.GetGenesisShardIds() {
		conn := master.getOneSlaveConnection(account.Branch{Value: v})
		assert.NotNil(t, conn)
	}
	fakeFullShardID := uint32(99999)
	conn := master.getOneSlaveConnection(account.Branch{Value: fakeFullShardID})
	assert.Nil(t, conn)
}

func TestCreateRootBlockToMine(t *testing.T) {
	minorBlock := types.NewMinorBlock(&types.MinorBlockHeader{}, &types.MinorBlockMeta{}, nil, nil, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	master := initEnv(t, nil)
	rawdb.WriteMinorBlock(master.chainDb, minorBlock)
	rootBlock, err := master.createRootBlockToMine(add1)
	assert.NoError(t, err)
	assert.Equal(t, rootBlock.Header().Coinbase, add1)
	assert.Equal(t, rootBlock.Header().CoinbaseAmount.Value.String(), "120000000000000000000")
	assert.Equal(t, rootBlock.Header().Difficulty, new(big.Int).SetUint64(1000000))

	rawdb.DeleteBlock(master.chainDb, minorBlock.Hash())
	rootBlock, err = master.createRootBlockToMine(add1)
	assert.NoError(t, err)
	assert.Equal(t, rootBlock.Header().Coinbase, add1)
	assert.Equal(t, rootBlock.Header().CoinbaseAmount.Value.String(), "120000000000000000000")
	assert.Equal(t, rootBlock.Header().Difficulty, new(big.Int).SetUint64(1000000))
	assert.Equal(t, len(rootBlock.MinorBlockHeaders()), 0)
}

func TestGetAccountData(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	master := initEnv(t, nil)
	_, err = master.GetAccountData(add1, nil)
	assert.NoError(t, err)
}

func TestGetPrimaryAccountData(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	_, err = master.GetPrimaryAccountData(add1, nil)
	assert.NoError(t, err)
}

func TestSendMiningConfigToSlaves(t *testing.T) {
	master := initEnv(t, nil)
	err := master.SendMiningConfigToSlaves(true)
	assert.NoError(t, err)
}

func TestAddRootBlock(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	rootBlock := master.rootBlockChain.CreateBlockToMine(nil, &add1, nil)
	err = master.AddRootBlock(rootBlock)
	assert.NoError(t, err)
}

func TestSetTargetBlockTime(t *testing.T) {
	master := initEnv(t, nil)
	rootBlockTime := uint32(12)
	minorBlockTime := uint32(1)
	err := master.SetTargetBlockTime(&rootBlockTime, &minorBlockTime)
	assert.NoError(t, err)
}

func TestAddTransaction(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	evmTx := types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 2, 2, 1, 0, []byte{})
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	err = master.AddTransaction(tx)
	assert.NoError(t, err)

	evmTx = types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 100, 2, 1, 0, []byte{})
	tx = &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	err = master.AddTransaction(tx)
	assert.Error(t, err)
}
func TestExecuteTransaction(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	evmTx := types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 2, 2, 1, 0, []byte{})
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	data, err := master.ExecuteTransaction(tx, add1, nil)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte("qkc"))

	evmTx = types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 222222222, 2, 1, 0, []byte{})
	tx = &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	_, err = master.ExecuteTransaction(tx, add1, nil)
	assert.Error(t, err)
}

func TestGetMinorBlockByHeight(t *testing.T) {
	master := initEnv(t, nil)
	fakeMinorBlock := types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
	fakeShardStatus := rpc.ShardStats{
		Branch: account.Branch{Value: 2},
		Height: 0,
	}
	master.UpdateShardStatus(&fakeShardStatus)
	minorBlock, err := master.GetMinorBlockByHeight(nil, account.Branch{Value: 2})

	assert.NoError(t, err)
	assert.Equal(t, fakeMinorBlock.Hash(), minorBlock.Hash())

	_, err = master.GetMinorBlockByHeight(nil, account.Branch{Value: 2222})
	assert.Error(t, err)
}
func TestGetMinorBlockByHash(t *testing.T) {
	master := initEnv(t, nil)
	fakeMinorBlock := types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
	minorBlock, err := master.GetMinorBlockByHash(common.Hash{}, account.Branch{Value: 2})
	assert.NoError(t, err)
	assert.Equal(t, fakeMinorBlock.Hash(), minorBlock.Hash())

	_, err = master.GetMinorBlockByHash(common.Hash{}, account.Branch{Value: 2222})
	assert.Error(t, err)
}

func TestGetTransactionByHash(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	evmTx := types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 2, 2, 1, 0, []byte{})
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	fakeMinorBlock := types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
	minorBlock, index, err := master.GetTransactionByHash(tx.Hash(), account.Branch{Value: 2})
	assert.NoError(t, err)
	assert.Equal(t, index, uint32(1))
	assert.Equal(t, fakeMinorBlock.Hash(), minorBlock.Hash())
}

func TestGetTransactionReceipt(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	evmTx := types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 2, 2, 1, 0, []byte{})
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	fakeMinorBlock := types.NewMinorBlock(&types.MinorBlockHeader{Version: 111}, &types.MinorBlockMeta{}, nil, nil, nil)
	MinorBlock, _, rep, err := master.GetTransactionReceipt(tx.Hash(), account.Branch{Value: 2})
	assert.NoError(t, err)
	assert.Equal(t, MinorBlock.Hash(), fakeMinorBlock.Hash())
	assert.Equal(t, rep.CumulativeGasUsed, uint64(123))
}

func TestGetTransactionsByAddress(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	res, bytes, err := master.GetTransactionsByAddress(add1, []byte{}, 0)
	assert.NoError(t, err)
	assert.Equal(t, bytes, []byte("qkc"))
	assert.Equal(t, res[0].TxHash, common.BigToHash(new(big.Int).SetUint64(11)))
}

func TestGetLogs(t *testing.T) {
	master := initEnv(t, nil)

	startBlock := ethRPC.BlockNumber(0)
	endBlock := ethRPC.BlockNumber(0)
	logs, err := master.GetLogs(account.Branch{Value: 2}, nil, nil, startBlock, endBlock)
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 1)
	assert.Equal(t, logs[0].Data, []byte("qkc"))
}

func TestEstimateGas(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	evmTx := types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 2, 2, 1, 0, []byte{})
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	data, err := master.EstimateGas(tx, add1)
	assert.NoError(t, err)
	assert.Equal(t, data, uint32(123))

	evmTx = types.NewEvmTransaction(0, id1.GetRecipient(), new(big.Int), 0, new(big.Int), 2222222, 2, 1, 0, []byte{})
	tx = &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	data, err = master.EstimateGas(tx, add1)
	assert.Error(t, err)
}

func TestGetStorageAt(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	data, err := master.GetStorageAt(add1, common.Hash{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, data.Big().Uint64(), uint64(123))
}
func TestGetCode(t *testing.T) {
	master := initEnv(t, nil)
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	add1 := account.NewAddress(id1.GetRecipient(), 3)
	data, err := master.GetCode(add1, nil)
	assert.NoError(t, err)
	assert.Equal(t, data, []byte("qkc"))

}
func TestGasPrice(t *testing.T) {
	master := initEnv(t, nil)
	data, err := master.GasPrice(account.Branch{Value: 2})
	assert.NoError(t, err)
	assert.Equal(t, data, uint64(123))
}
