package master

import (
	"bytes"
	"encoding/binary"
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethRpc "github.com/ethereum/go-ethereum/rpc"
	"net"
)

func ip2Long(ip string) uint32 {
	var long uint32
	binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.BigEndian, &long)
	return long
}

func (s *QKCMasterBackend) GetPeers() []rpc.PeerInfoForDisPlay {
	fake := make([]p2p.Peer, 0)
	res := make([]rpc.PeerInfoForDisPlay, 0)
	for k := range fake {
		temp := rpc.PeerInfoForDisPlay{}
		if tcp, ok := fake[k].RemoteAddr().(*net.TCPAddr); ok {
			temp.IP = ip2Long(tcp.IP.String())
			temp.Port = uint32(tcp.Port)
			temp.ID = fake[k].ID().Bytes()
		} else {
			panic("not tcp")
		}
		res = append(res, temp)
	}
	return res

}

func (s *QKCMasterBackend) AddRootBlockFromMine(block *types.RootBlock) error {
	currTip := s.rootBlockChain.CurrentBlock()
	if block.Header().ParentHash != currTip.Hash() {
		return errors.New("parent hash not match")
	}
	return s.AddRootBlock(block)
}
func (s *QKCMasterBackend) AddRawMinorBlock(branch account.Branch, blockData []byte) error {
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return err
	}
	return slaveConn.AddMinorBlock(blockData)
}
func (s *QKCMasterBackend) AddTransaction(tx *types.Transaction) error {
	evmTx := tx.EvmTx
	//TODO :SetQKCConfig
	branch := account.Branch{Value: evmTx.FromFullShardId()}
	slaves, ok := s.branchToSlaves[branch.Value]
	if !ok {
		return errors.New("no such slave")
	}
	lenSlaves := len(slaves)
	check := NewCheckErr(lenSlaves)
	for index := range slaves {
		check.wg.Add(1)
		go func(slave *SlaveConnection) {
			defer check.wg.Done()
			err := slave.AddTransaction(tx)
			check.errc <- err

		}(slaves[index])
	}
	check.wg.Wait()
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
		return nil, errors.New("no such slave")
	}
	lenSlaves := len(slaves)
	check := NewCheckErr(lenSlaves)
	chanRsp := make(chan []byte, lenSlaves)
	for index := range slaves {
		check.wg.Add(1)
		go func(slave *SlaveConnection) {
			defer check.wg.Done()
			rsp, err := slave.ExecuteTransaction(tx, address, height)
			check.errc <- err
			chanRsp <- rsp

		}(slaves[index])
	}
	check.wg.Wait()
	close(chanRsp)
	if err := check.check(); err != nil {
		return nil, err
	}

	flag := true
	firstFlag := 1
	var onlyValue []byte
	for res := range chanRsp {
		if firstFlag == 1 {
			firstFlag = 0
			onlyValue = res
		}
		if res == nil || !bytes.Equal(res, onlyValue) {
			flag = false
			break
		}
	}
	if flag == false {
		return nil, errors.New("flag==false")
	}
	return onlyValue, nil
}
func (s *QKCMasterBackend) GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) (*types.MinorBlock, error) {
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return nil, err
	}
	return slaveConn.GetMinorBlockByHash(blockHash, branch)
}
func (s *QKCMasterBackend) GetMinorBlockByHeight(height *uint64, branch account.Branch) (*types.MinorBlock, error) {
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return nil, err
	}
	if height == nil {
		shardStats, ok := s.branchToShardStats[branch.Value]
		if !ok {
			return nil, errors.New("no such branch")
		}
		height = &shardStats.Height
	}
	return slaveConn.GetMinorBlockByHeight(*height, branch)
}
func (s *QKCMasterBackend) GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error) {
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return nil, 0, err
	}
	return slaveConn.GetTransactionByHash(txHash, branch)
}
func (s *QKCMasterBackend) GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error) {
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return nil, 0, nil, err
	}
	return slaveConn.GetTransactionReceipt(txHash, branch)
}

func (s *QKCMasterBackend) GetTransactionsByAddress(address account.Address, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn, err := s.getSlaveConnection(account.Branch{Value: fullShardID})
	if err != nil {
		return nil, nil, err
	}
	return slaveConn.GetTransactionsByAddress(address, start, limit)
}
func (s *QKCMasterBackend) GetLogs(branch account.Branch, address []account.Address, topics []*rpc.Topic, startBlock, endBlock ethRpc.BlockNumber) ([]*types.Log, error) {
	// not support earlist and pending
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return nil, err
	}

	var (
		startBlockNumber uint64
		endBlockNumber   uint64
	)
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
	return slaveConn.GetLogs(branch, address, topics, startBlockNumber, endBlockNumber)
}

func (s *QKCMasterBackend) EstimateGas(tx *types.Transaction, fromAddress account.Address) (uint32, error) {
	evmTx := tx.EvmTx
	//TODO set config
	slaveConn, err := s.getSlaveConnection(account.Branch{Value: evmTx.FromFullShardId()})
	if err != nil {
		return 0, err
	}
	return slaveConn.EstimateGas(tx, fromAddress)
}
func (s *QKCMasterBackend) GetStorageAt(address account.Address, key common.Hash, height *uint64) (common.Hash, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn, err := s.getSlaveConnection(account.Branch{Value: fullShardID})
	if err != nil {
		return common.Hash{}, err
	}
	return slaveConn.GetStorageAt(address, key, height)
}

func (s *QKCMasterBackend) GetCode(address account.Address, height *uint64) ([]byte, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn, err := s.getSlaveConnection(account.Branch{Value: fullShardID})
	if err != nil {
		return nil, err
	}
	return slaveConn.GetCode(address, height)
}

func (s *QKCMasterBackend) GasPrice(branch account.Branch) (uint64, error) {
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return 0, err
	}
	return slaveConn.GasPrice(branch)
}
func (s *QKCMasterBackend) GetWork(branch *account.Branch) consensus.MiningWork {
	panic("not ")
}
func (s *QKCMasterBackend) SubmitWork(branch *account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) bool {
	return false
}

func (s *QKCMasterBackend) GetRootBlockByNumber(blockNumber *uint64) (*types.RootBlock, error) {
	if blockNumber == nil {
		temp := s.rootBlockChain.CurrentBlock().NumberU64()
		blockNumber = &temp
	}
	block := s.rootBlockChain.GetBlockByNumber(*blockNumber)
	if block == nil {
		return nil, errors.New("rootBlock is nil")
	}
	return block.(*types.RootBlock), nil
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
