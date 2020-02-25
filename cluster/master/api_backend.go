package master

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"golang.org/x/sync/errgroup"
	"math/big"
	"net"
	"reflect"
	"strings"
)

func ip2uint32(ip string) uint32 {
	var long uint32
	_ = binary.Read(bytes.NewBuffer(net.ParseIP(ip).To4()), binary.BigEndian, &long)
	return long
}

func (s *QKCMasterBackend) GetPeerList() []*Peer {
	return s.protocolManager.peers.Peers()
}

func (s *QKCMasterBackend) GetPeerInfolist() []rpc.PeerInfoForDisPlay {
	peers := s.protocolManager.peers.Peers() //TODO use real peerList
	result := make([]rpc.PeerInfoForDisPlay, 0)
	for k := range peers {
		temp := rpc.PeerInfoForDisPlay{}
		if tcp, ok := peers[k].RemoteAddr().(*net.TCPAddr); ok {
			temp.IP = ip2uint32(tcp.IP.String())
			temp.Port = uint32(tcp.Port)
			temp.ID = peers[k].ID().Bytes()
		} else {
			panic(fmt.Errorf("not tcp? real type %v", reflect.TypeOf(tcp)))
		}
		result = append(result, temp)
	}
	return result
}

func (s *QKCMasterBackend) AddTransaction(tx *types.Transaction) error {
	evmTx := tx.EvmTx
	if evmTx.GasPrice().Cmp(s.clusterConfig.Quarkchain.MinTXPoolGasPrice) < 0 {
		return errors.New(fmt.Sprintf("invalid gasprice: tx min gas price is %d", s.clusterConfig.Quarkchain.MinTXPoolGasPrice.Uint64()))
	}
	fromShardSize, err := s.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err != nil {
		return err
	}
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return errors.New(fmt.Sprintf("Failed to set fromShardSize, fromShardSize: %d, err: %v", fromShardSize, err))
	}
	fullShardId := evmTx.FromFullShardId()
	slaves := s.GetSlaveConnsById(fullShardId)
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
	err = g.Wait() //TODO?? peer broadcast
	if err != nil {
		return err
	}
	data, err := serialize.SerializeToBytes(&p2p.NewTransactionList{TransactionList: []*types.Transaction{tx}})
	if err != nil {
		return err
	}
	go s.protocolManager.BroadcastTransactions(&rpc.P2PRedirectRequest{Branch: fullShardId, Data: data}, "")
	return nil
}

func (s *QKCMasterBackend) ExecuteTransaction(tx *types.Transaction, address *account.Address, height *uint64) ([]byte, error) {
	evmTx := tx.EvmTx
	fromShardSize, err := s.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err != nil {
		return nil, err
	}
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to set fromShardSize, fromShardSize: %d, err: %v", fromShardSize, err))
	}
	slaves := s.GetSlaveConnsById(evmTx.FromFullShardId())
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

func (s *QKCMasterBackend) GetMinorBlockByHash(blockHash common.Hash, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *rpc.PoSWInfo, error) {
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return nil, nil, ErrNoBranchConn
	}
	return slaveConn.GetMinorBlockByHash(blockHash, branch, needExtraInfo)
}

func (s *QKCMasterBackend) GetMinorBlockByHeight(height *uint64, branch account.Branch, needExtraInfo bool) (*types.MinorBlock, *rpc.PoSWInfo, error) {
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return nil, nil, ErrNoBranchConn
	}
	if height == nil {
		s.lock.RLock()
		shardStats, ok := s.branchToShardStats[branch.Value]
		s.lock.RUnlock()
		if !ok {
			return nil, nil, ErrNoBranchConn
		}
		height = &shardStats.Height
	}
	return slaveConn.GetMinorBlockByHeight(height, branch, needExtraInfo)
}

func (s *QKCMasterBackend) GetTransactionByHash(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, error) {
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return nil, 0, ErrNoBranchConn
	}
	return slaveConn.GetTransactionByHash(txHash, branch)
}

func (s *QKCMasterBackend) GetTransactionReceipt(txHash common.Hash, branch account.Branch) (*types.MinorBlock, uint32, *types.Receipt, error) {
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return nil, 0, nil, ErrNoBranchConn
	}
	return slaveConn.GetTransactionReceipt(txHash, branch)
}

func (s *QKCMasterBackend) GetTransactionsByAddress(address *account.Address, start []byte, limit uint32, transferTokenID *uint64) ([]*rpc.TransactionDetail, []byte, error) {
	fullShardID, err := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	if err != nil {
		return nil, nil, err
	}
	slaveConn := s.GetOneSlaveConnById(fullShardID)
	if slaveConn == nil {
		return nil, nil, ErrNoBranchConn
	}
	return slaveConn.GetTransactionsByAddress(address, start, limit, transferTokenID)
}

func (s *QKCMasterBackend) GetAllTx(branch account.Branch, start []byte, limit uint32) ([]*rpc.TransactionDetail, []byte, error) {
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return nil, nil, ErrNoBranchConn
	}
	return slaveConn.GetAllTx(branch, start, limit)
}

func (s *QKCMasterBackend) GetLogs(args *qrpc.FilterQuery) ([]*types.Log, error) {
	// not support earlist and pending
	slaveConn := s.GetOneSlaveConnById(args.FullShardId)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	return slaveConn.GetLogs(args)
}

func (s *QKCMasterBackend) EstimateGas(tx *types.Transaction, fromAddress *account.Address) (uint32, error) {
	evmTx := tx.EvmTx
	fromShardSize, err := s.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err != nil {
		return 0, err
	}
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return 0, errors.New(fmt.Sprintf("Failed to set fromShardSize, fromShardSize: %d, err: %v", fromShardSize, err))
	}
	slaveConn := s.GetOneSlaveConnById(evmTx.FromFullShardId())
	if slaveConn == nil {
		return 0, ErrNoBranchConn
	}
	if !evmTx.IsCrossShard() {
		return slaveConn.EstimateGas(tx, fromAddress)
	}
	fAddr := account.Address{Recipient: fromAddress.Recipient, FullShardKey: evmTx.ToFullShardKey()}
	res, err := slaveConn.EstimateGas(tx, &fAddr)
	if err != nil {
		return 0, err
	}
	return res + 9000, nil
}

func (s *QKCMasterBackend) GetStorageAt(address *account.Address, key common.Hash, height *uint64) (common.Hash, error) {
	fullShardID, err := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	if err != nil {
		return common.Hash{}, err
	}
	slaveConn := s.GetOneSlaveConnById(fullShardID)
	if slaveConn == nil {
		return common.Hash{}, ErrNoBranchConn
	}
	return slaveConn.GetStorageAt(address, key, height)
}

func (s *QKCMasterBackend) GetCode(address *account.Address, height *uint64) ([]byte, error) {
	fullShardID, err := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	if err != nil {
		return nil, err
	}
	slaveConn := s.GetOneSlaveConnById(fullShardID)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	return slaveConn.GetCode(address, height)
}

func (s *QKCMasterBackend) GasPrice(branch account.Branch, tokenID uint64) (uint64, error) {
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return 0, ErrNoBranchConn
	}
	return slaveConn.GasPrice(branch, tokenID)
}

// return root chain work if branch is nil
func (s *QKCMasterBackend) GetWork(fullShardId *uint32, addr *common.Address) (*consensus.MiningWork, error) {
	coinbaseAddr := &account.Address{}
	if addr != nil {
		coinbaseAddr.Recipient = *addr
		if fullShardId != nil {
			coinbaseAddr.FullShardKey = *fullShardId
		}
	} else {
		coinbaseAddr = nil
	}
	if fullShardId == nil {
		fmt.Println("mmmmmmmm--",coinbaseAddr)
		return s.miner.GetWork(coinbaseAddr)
	}

	branch := account.Branch{Value: *fullShardId}
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	return slaveConn.GetWork(branch, coinbaseAddr)
}

// submit root chain work if branch is nil
func (s *QKCMasterBackend) SubmitWork(fullShardId *uint32, headerHash common.Hash, nonce uint64, mixHash common.Hash, signature *[65]byte) (bool, error) {
	if fullShardId == nil {
		return s.miner.SubmitWork(nonce, headerHash, mixHash, signature), nil
	}

	branch := account.NewBranch(*fullShardId)
	slaveConn := s.GetOneSlaveConnById(branch.Value)
	if slaveConn == nil {
		return false, ErrNoBranchConn
	}
	return slaveConn.SubmitWork(&rpc.SubmitWorkRequest{Branch: branch.Value, HeaderHash: headerHash, Nonce: nonce, MixHash: mixHash})
}

func (s *QKCMasterBackend) GetRootBlockByNumber(blockNumber *uint64, needExtraInfo bool) (*types.RootBlock, *rpc.PoSWInfo, error) {
	if blockNumber == nil {
		temp := s.rootBlockChain.CurrentBlock().NumberU64()
		blockNumber = &temp
	}
	block, ok := s.rootBlockChain.GetBlockByNumber(*blockNumber).(*types.RootBlock)
	if !ok {
		return nil, nil, errors.New("rootBlock is nil")
	}
	if needExtraInfo {
		p, err := s.getPoswInfo(block.Header())
		return block, p, err
	}
	return block, nil, nil
}

func (s *QKCMasterBackend) GetRootBlockByHash(hash common.Hash, needExtraInfo bool) (*types.RootBlock, *rpc.PoSWInfo, error) {
	block, ok := s.rootBlockChain.GetBlock(hash).(*types.RootBlock)
	if !ok {
		return nil, nil, errors.New("rootBlock is nil")
	}
	if needExtraInfo {
		p, err := s.getPoswInfo(block.Header())
		return block, p, err
	}
	return block, nil, nil
}

func (s *QKCMasterBackend) getPoswInfo(header *types.RootBlockHeader) (*rpc.PoSWInfo, error) {
	poswInfo, err := s.rootBlockChain.PoSWInfo(header)
	if err != nil && !strings.Contains(err.Error(), core.ErrPoswOnRootChainIsNotFound.Error()) {
		return nil, err
	}
	return poswInfo, nil
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
		"syncing":          s.IsSyncing(),
		"mining":           s.IsMining(),
		"shardServerCount": hexutil.Uint(len(s.clientPool)),
	}
	return fileds
}

func (s *QKCMasterBackend) GetCurrRootHeader() *types.RootBlockHeader {
	return s.rootBlockChain.CurrentHeader().(*types.RootBlockHeader)
}

func (s *QKCMasterBackend) GetDefaultCoinbaseAddress() account.Address {
	return s.clusterConfig.Quarkchain.Root.CoinbaseAddress
}

// miner api
func (s *QKCMasterBackend) CreateBlockToMine(addr *account.Address) (types.IBlock, *big.Int, uint64, error) {
	coinbaseAddr := s.clusterConfig.Quarkchain.Root.CoinbaseAddress
	if addr != nil {
		coinbaseAddr = *addr
	}
	block, err := s.createRootBlockToMine(coinbaseAddr)
	if err != nil {
		return nil, nil, 0, err
	}
	diff, optionalDivider, err := s.rootBlockChain.GetAdjustedDifficultyToMine(block.Header())
	fmt.Println("CreateBlockToMine",diff,optionalDivider,block.Header().Coinbase.ToHex(),block.NumberU64())
	if err != nil {
		return nil, nil, 0, err
	}
	return block, diff, optionalDivider, nil
}

func (s *QKCMasterBackend) InsertMinedBlock(block types.IBlock) error {
	rBlock := block.(*types.RootBlock)
	return s.AddRootBlock(rBlock)
}

func (s *QKCMasterBackend) AddMinorBlock(branch uint32, mBlock *types.MinorBlock) error {
	clients := s.GetSlaveConnsById(branch)
	if len(clients) == 0 {
		return errors.New(fmt.Sprintf("slave is not exist, branch: %d", branch))
	}
	var (
		g errgroup.Group
	)
	data, err := serialize.SerializeToBytes(&p2p.NewBlockMinor{Block: mBlock})
	if err != nil {
		return err
	}
	for _, cli := range clients {
		cli := cli
		g.Go(func() error {
			err := cli.HandleNewMinorBlock(&rpc.P2PRedirectRequest{Branch: branch, Data: data})
			return err
		})
	}
	return g.Wait()
}

func (s *QKCMasterBackend) GetTip() uint64 {
	return s.rootBlockChain.CurrentBlock().NumberU64()
}

func (s *QKCMasterBackend) GetKadRoutingTable() ([]string, error) {
	if s.srvr != nil {
		return s.srvr.GetKadRoutingTable(), nil
	}
	return nil, errors.New("p2p server is not running")
}

func (s *QKCMasterBackend) IsSyncing() bool {
	return s.synchronizer.IsSyncing()
}

func (s *QKCMasterBackend) IsMining() bool {
	return s.miner.IsMining()
}

func (s *QKCMasterBackend) CurrentBlock() *types.RootBlock {
	return s.rootBlockChain.CurrentBlock()
}

func (s *QKCMasterBackend) GetSlavePoolLen() int {
	return s.ConnCount()
}
