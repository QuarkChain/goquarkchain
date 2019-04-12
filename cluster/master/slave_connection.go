package master

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

type SlaveConnection struct {
	target       string
	shardMaskLst []*types.ChainMask
	client       rpc.Client
}

// create slave connection manager
func NewSlaveConn(target string, shardMaskLst []*types.ChainMask) (*SlaveConnection, error) {
	client := rpc.NewClient(rpc.SlaveServer)
	return &SlaveConnection{
		target:       target,
		client:       client,
		shardMaskLst: shardMaskLst,
	}, nil
}

func (s *SlaveConnection) HeartBeat(request *rpc.Request) bool {
	_, err := s.client.Call(s.target, request)
	if err != nil {
		return false
	}
	return true
}

func (s *SlaveConnection) SendPing() (string, []types.ChainMask, error) {
	request := rpc.Request{Op: rpc.OpPing, Data: nil}
	_, err := s.client.Call(s.target, &request)
	if err != nil {
		return "", nil, err
	}
	return "", nil, nil
}

func (s *SlaveConnection) SendConnectToSlaves(slaveInfoLst []rpc.SlaveInfo) error {
	req := rpc.ConnectToSlavesRequest{SlaveInfoList: slaveInfoLst}
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return err
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpConnectToSlaves, Data: bytes})
	if err != nil {
		return err
	}
	return nil
}

func (s *SlaveConnection) AddTransaction(tx types.Transaction) bool {
	req := rpc.AddTransactionRequest{Tx: tx}
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		goto FALSE
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpAddTransaction, Data: bytes})
	if err != nil {
		goto FALSE
	}
	return true
FALSE:
	log.Error("slave connection", "target", s.target, err)
	return false
}

// TODO return type is not confirmed.
func (s *SlaveConnection) ExecuteTransaction(tx types.Transaction, fromAddress account.Address, height uint64) bool {
	req := rpc.ExecuteTransactionRequest{Tx: tx, FromAddress: fromAddress, BlockHeight: height}
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		goto FALSE
	}
	_, err = s.client.Call(s.target, &rpc.Request{Op: rpc.OpExecuteTransaction, Data: bytes})
	if err != nil {
		goto FALSE
	}
	return true
FALSE:
	log.Error("slave connection", "target", s.target, err)
	return false
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
	return &minBlockResponse.MinorBlock, nil
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
	return &minBlockResponse.MinorBlock, nil
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
	return &trans.MinorBlock, 0, nil
}

func (s *SlaveConnection) GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error) {
	var (
		req   = rpc.GetTransactionReceiptRequest{Branch: branch, TxHash: txHash}
		trans = rpc.GetTransactionReceiptResponse{}
	)
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		return nil, 0, nil, err
	}
	res, err := s.client.Call(s.target, &rpc.Request{Op: rpc.OpGetTransactionReceipt, Data: bytes})
	if err != nil {
		return nil, 0, nil, err
	}
	if err := serialize.Deserialize(serialize.NewByteBuffer(res.Data), &trans); err != nil {
		return nil, 0, nil, err
	}
	return &trans.MinorBlock, trans.Index, &trans.Receipt, nil
}

func (s *SlaveConnection) GetTransactionsByAddress(address account.Address, start []byte, limit uint32) ([]rpc.TransactionDetail, []byte, error) {
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

func (s *SlaveConnection) GetLogs(branch account.Branch, address []account.Address, topics []byte, startBlock, endBlock uint64) ([]types.Log, error) {
	return nil, nil
}
func (s *SlaveConnection) EstimateGas(tx types.Transaction, address account.Address) (uint32, error) {
	return 0, nil
}
func (s *SlaveConnection) GetStorageAt(address account.Address, key serialize.Uint256, height uint64) ([]byte, error) {
	return nil, nil
}
func (s *SlaveConnection) GetCode(address account.Address, height uint64) ([]byte, error) {
	return nil, nil
}
func (s *SlaveConnection) GasPrice(branch account.Branch) (uint64, error) {
	return 0, nil
}
func (s *SlaveConnection) GetWork(branch account.Branch) (*consensus.MiningWork, error) {
	return &consensus.MiningWork{}, nil
}
func (s *SlaveConnection) SubmitWork(branch account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) error {
	return nil
}
