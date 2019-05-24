package master

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"golang.org/x/sync/errgroup"
	"net"
	"reflect"
)

func ip2uint32(ip string) uint32 {
	var long uint32
	_ = binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.BigEndian, &long)
	return long
}

func (s *QKCMasterBackend) GetPeers() []rpc.PeerInfoForDisPlay {
	fakePeers := make([]p2p.Peer, 0) //TODO use real peerList
	result := make([]rpc.PeerInfoForDisPlay, 0)
	for k := range fakePeers {
		temp := rpc.PeerInfoForDisPlay{}
		if tcp, ok := fakePeers[k].RemoteAddr().(*net.TCPAddr); ok {
			temp.IP = ip2uint32(tcp.IP.String())
			temp.Port = uint32(tcp.Port)
			temp.ID = fakePeers[k].ID().Bytes()
		} else {
			panic(fmt.Errorf("not tcp? real type %v", reflect.TypeOf(tcp)))
		}
		result = append(result, temp)
	}
	return result

}

func (s *QKCMasterBackend) AddTransaction(tx *types.Transaction) error {
	evmTx := tx.EvmTx
	//TODO :SetQKCConfig
	branch := account.Branch{Value: evmTx.FromFullShardId()}
	slaves := s.getAllSlaveConnection(branch.Value)
	if len(slaves) == 0 {
		return ErrNoBranchConn
	}
	var g errgroup.Group
	for index := range slaves {
		i := index
		g.Go(func() error {
			return slaves[i].AddTransaction(tx)
		})
	}
	return g.Wait() //TODO?? peer broadcast
}

func (s *QKCMasterBackend) ExecuteTransaction(tx *types.Transaction, address *account.Address, height *uint64) ([]byte, error) {
	evmTx := tx.EvmTx
	//TODO setQuarkChain
	branch := account.Branch{Value: evmTx.FromFullShardId()}
	slaves := s.getAllSlaveConnection(branch.Value)
	if len(slaves) == 0 {
		return nil, ErrNoBranchConn
	}
	var g errgroup.Group
	rspList := make([][]byte, len(slaves))
	for index := range slaves {
		i := index
		g.Go(func() error {
			rsp, err := slaves[i].ExecuteTransaction(tx, address, height)
			rspList[i] = rsp
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	resultBytes := rspList[0] // before already this len>0
	for _, res := range rspList {
		if res != nil && !bytes.Equal(resultBytes, res) {
			return nil, errors.New("exist more than one result")
		}
	}
	return resultBytes, nil

}

func (s *QKCMasterBackend) GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) (*types.MinorBlock, error) {
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	return slaveConn.GetMinorBlockByHash(blockHash, branch)
}

func (s *QKCMasterBackend) GetMinorBlockByHeight(height *uint64, branch account.Branch) (*types.MinorBlock, error) {
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	if height == nil {
		s.lock.RLock()
		shardStats, ok := s.branchToShardStats[branch.Value]
		s.lock.RUnlock()
		if !ok {
			return nil, ErrNoBranchConn
		}
		height = &shardStats.Height
	}
	return slaveConn.GetMinorBlockByHeight(*height, branch)
}

func (s *QKCMasterBackend) GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error) {
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return nil, 0, ErrNoBranchConn
	}
	return slaveConn.GetTransactionByHash(txHash, branch)
}

func (s *QKCMasterBackend) GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error) {
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return nil, 0, nil, ErrNoBranchConn
	}
	return slaveConn.GetTransactionReceipt(txHash, branch)
}

func (s *QKCMasterBackend) GetTransactionsByAddress(address *account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: fullShardID})
	if slaveConn == nil {
		return nil, nil, ErrNoBranchConn
	}
	return slaveConn.GetTransactionsByAddress(address, start, limit)
}

func (s *QKCMasterBackend) GetLogs(branch account.Branch, address []*account.Address, topics []*rpc.Topic, startBlockNumber, endBlockNumber ethRpc.BlockNumber) ([]*types.Log, error) {
	// not support earlist and pending
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}

	s.lock.RLock()
	if startBlockNumber == ethRpc.LatestBlockNumber {
		startBlockNumber = ethRpc.BlockNumber(s.branchToShardStats[branch.Value].Height)
	}
	if endBlockNumber == ethRpc.LatestBlockNumber {
		endBlockNumber = ethRpc.BlockNumber(s.branchToShardStats[branch.Value].Height)
	}
	s.lock.RUnlock() // lock branchToShardStats
	return slaveConn.GetLogs(branch, address, topics, uint64(startBlockNumber), uint64(endBlockNumber))
}

func (s *QKCMasterBackend) EstimateGas(tx *types.Transaction, fromAddress *account.Address) (uint32, error) {
	evmTx := tx.EvmTx
	//TODO set config
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: evmTx.FromFullShardId()})
	if slaveConn == nil {
		return 0, ErrNoBranchConn
	}
	return slaveConn.EstimateGas(tx, fromAddress)
}

func (s *QKCMasterBackend) GetStorageAt(address *account.Address, key common.Hash, height *uint64) (common.Hash, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: fullShardID})
	if slaveConn == nil {
		return common.Hash{}, ErrNoBranchConn
	}
	return slaveConn.GetStorageAt(address, key, height)
}

func (s *QKCMasterBackend) GetCode(address *account.Address, height *uint64) ([]byte, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: fullShardID})
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	return slaveConn.GetCode(address, height)
}

func (s *QKCMasterBackend) GasPrice(branch account.Branch) (uint64, error) {
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return 0, ErrNoBranchConn
	}
	return slaveConn.GasPrice(branch)
}

// return root chain work if branch is nil
func (s *QKCMasterBackend) GetWork(branch account.Branch) (*consensus.MiningWork, error) {
	if branch.Value == 0 {
		return s.engine.GetWork()
	}
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	return slaveConn.GetWork(branch)
}

// submit root chain work if branch is nil
func (s *QKCMasterBackend) SubmitWork(branch account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) (bool, error) {
	if branch.Value == 0 {
		return s.engine.SubmitWork(nonce, headerHash, mixHash), nil
	}
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return false, ErrNoBranchConn
	}
	return slaveConn.SubmitWork(&rpc.SubmitWorkRequest{Branch: branch.Value, HeaderHash: headerHash, Nonce: nonce, MixHash: mixHash})
}

func (s *QKCMasterBackend) GetRootBlockByNumber(blockNumber *uint64) (*types.RootBlock, error) {
	if blockNumber == nil {
		temp := s.rootBlockChain.CurrentBlock().NumberU64()
		blockNumber = &temp
	}
	block, ok := s.rootBlockChain.GetBlockByNumber(*blockNumber).(*types.RootBlock)
	if !ok {
		return nil, errors.New("rootBlock is nil")
	}
	return block, nil
}

func (s *QKCMasterBackend) GetRootBlockByHash(hash common.Hash) (*types.RootBlock, error) {
	block, ok := s.rootBlockChain.GetBlock(hash).(*types.RootBlock)
	if !ok {
		return nil, errors.New("rootBlock is nil")
	}
	return block, nil
}

func (s *QKCMasterBackend) NetWorkInfo() map[string]interface{} {
	shardSizeList := make([]hexutil.Uint, 0)
	for _, v := range s.clusterConfig.Quarkchain.Chains {
		shardSizeList = append(shardSizeList, hexutil.Uint(v.ShardSize))
	}

	fileds := map[string]interface{}{
		"networkId":        hexutil.Uint(s.clusterConfig.Quarkchain.NetworkID),
		"chainSize":        hexutil.Uint(s.clusterConfig.Quarkchain.ChainSize),
		"shardSizes":       shardSizeList,
		"syncing":          s.isSyning(),
		"mining":           s.isMining(),
		"shardServerCount": hexutil.Uint(len(s.clientPool)),
	}
	return fileds
}

// miner api
func (s *QKCMasterBackend) CreateBlockAsyncFunc() (types.IBlock, error) {
	return s.createRootBlockToMine(s.clusterConfig.Quarkchain.Root.CoinbaseAddress)
}

func (s *QKCMasterBackend) AddBlockAsyncFunc(block types.IBlock) error {
	_, err := s.rootBlockChain.InsertChain([]types.IBlock{block})
	return err
}
