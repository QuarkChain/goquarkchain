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
	"net"
	"reflect"
)

func ip2Long(ip string) uint32 {
	var long uint32
	binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.BigEndian, &long)
	return long
}

func (s *QKCMasterBackend) GetPeers() []rpc.PeerInfoForDisPlay {
	fakePeers := make([]p2p.Peer, 0) //TODO use real peerList
	result := make([]rpc.PeerInfoForDisPlay, 0)
	for k := range fakePeers {
		temp := rpc.PeerInfoForDisPlay{}
		if tcp, ok := fakePeers[k].RemoteAddr().(*net.TCPAddr); ok {
			temp.IP = ip2Long(tcp.IP.String())
			temp.Port = uint32(tcp.Port)
			temp.ID = fakePeers[k].ID().Bytes()
		} else {
			panic(fmt.Errorf("not tcp? real type %v", reflect.TypeOf(tcp)))
		}
		result = append(result, temp)
	}
	return result

}

func (s *QKCMasterBackend) AddRootBlockFromMine(block *types.RootBlock) error {
	currTip := s.rootBlockChain.CurrentBlock()
	if block.Header().ParentHash != currTip.Hash() {
		return errors.New("parent hash not match")
	}
	return s.AddRootBlock(block)
}
func (s *QKCMasterBackend) AddRawMinorBlock(branch account.Branch, blockData []byte) error {
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return ErrNoBranchConn
	}
	return slaveConn.AddMinorBlock(blockData)
}
func (s *QKCMasterBackend) AddTransaction(tx *types.Transaction) error {
	evmTx := tx.EvmTx
	//TODO :SetQKCConfig
	branch := account.Branch{Value: evmTx.FromFullShardId()}
	slaves, ok := s.branchToSlaves[branch.Value]
	if !ok {
		return ErrNoBranchConn
	}
	lenSlaves := len(slaves)
	check := NewCheckErr(lenSlaves)
	for index := range slaves {
		check.wg.Add(1)
		go func(slave *SlaveConnection) {
			defer check.wg.Done()
			err := slave.AddTransaction(tx)
			check.errc = append(check.errc, err)
		}(slaves[index])
	}
	if err := check.check(); err != nil {
		return err
	}
	return nil //TODO?? peer broadcast
}

func (s *QKCMasterBackend) ExecuteTransaction(tx *types.Transaction, address account.Address, height *uint64) ([]byte, error) {
	evmTx := tx.EvmTx
	//TODO setQuarkChain
	branch := account.Branch{Value: evmTx.FromFullShardId()}

	slaves, ok := s.branchToSlaves[branch.Value]
	if !ok {
		return nil, ErrNoBranchConn
	}
	lenSlaves := len(slaves)
	check := NewCheckErr(lenSlaves)
	rspList := make([][]byte, 0)
	for index := range slaves {
		check.wg.Add(1)
		go func(slave *SlaveConnection) {
			defer check.wg.Done()
			rsp, err := slave.ExecuteTransaction(tx, address, height)
			check.errc = append(check.errc, err)
			rspList = append(rspList, rsp)

		}(slaves[index])
	}
	if err := check.check(); err != nil {
		return nil, err
	}

	valueList := make(map[string]struct{}, 0)
	for _, res := range rspList {
		if res != nil {
			valueList[string(res)] = struct{}{}
		} else {
			return nil, errors.New("unexpected err res==nil")
		}
	}
	if len(valueList) != 1 {
		return nil, errors.New("exist more than one value from slaves")
	}
	for k, _ := range valueList {
		return []byte(k), nil
	}
	return nil, errors.New("unexpected err")
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

func (s *QKCMasterBackend) GetTransactionsByAddress(address account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: fullShardID})
	if slaveConn == nil {
		return nil, nil, ErrNoBranchConn
	}
	return slaveConn.GetTransactionsByAddress(address, start, limit)
}
func (s *QKCMasterBackend) GetLogs(branch account.Branch, address []account.Address, topics []*rpc.Topic, startBlock, endBlock ethRpc.BlockNumber) ([]*types.Log, error) {
	// not support earlist and pending
	slaveConn := s.getOneSlaveConnection(branch)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}

	var (
		startBlockNumber uint64
		endBlockNumber   uint64
	)

	s.lock.RLock()
	if startBlock == ethRpc.LatestBlockNumber {
		startBlockNumber = s.branchToShardStats[branch.Value].Height
	} else {
		startBlockNumber = uint64(startBlock.Int64())
	}
	if endBlock == ethRpc.LatestBlockNumber {
		endBlockNumber = s.branchToShardStats[branch.Value].Height
	} else {
		endBlockNumber = uint64(endBlock.Int64())
	}
	s.lock.RUnlock() // lock branchToShardStats
	return slaveConn.GetLogs(branch, address, topics, startBlockNumber, endBlockNumber)
}

func (s *QKCMasterBackend) EstimateGas(tx *types.Transaction, fromAddress account.Address) (uint32, error) {
	evmTx := tx.EvmTx
	//TODO set config
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: evmTx.FromFullShardId()})
	if slaveConn == nil {
		return 0, ErrNoBranchConn
	}
	return slaveConn.EstimateGas(tx, fromAddress)
}

func (s *QKCMasterBackend) GetStorageAt(address account.Address, key common.Hash, height *uint64) (common.Hash, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: fullShardID})
	if slaveConn == nil {
		return common.Hash{}, ErrNoBranchConn
	}
	return slaveConn.GetStorageAt(address, key, height)
}

func (s *QKCMasterBackend) GetCode(address account.Address, height *uint64) ([]byte, error) {
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
func (s *QKCMasterBackend) GetWork(branch *account.Branch) consensus.MiningWork {
	// TODO @liuhuan
	panic("not ")
}
func (s *QKCMasterBackend) SubmitWork(branch *account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) bool {
	// TODO @liuhuan
	return false
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
