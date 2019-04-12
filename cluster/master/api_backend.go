package master

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"syscall"
	"time"
)

const (
	beatTime = 4
)

type QkcAPIBackend struct {
	config []*config.SlaveConfig
	//shardMaskList
	master     *MasterBackend
	clientPool map[string]*SlaveConnection
}

// create slave connection manager
func NewQkcAPIBackend(master *MasterBackend, slavesConfig []*config.SlaveConfig) (*QkcAPIBackend, error) {
	slavePool := make(map[string]*SlaveConnection)
	for _, cfg := range slavesConfig {
		target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
		client, err := NewSlaveConn(target, cfg.ChainMaskList)
		if err != nil {
			return nil, err
		}
		slavePool[target] = client
	}
	log.Info("qkc api backend", "slave client pool", len(slavePool))
	return &QkcAPIBackend{
		config:     slavesConfig,
		master:     master,
		clientPool: slavePool,
	}, nil
}

func (s *QkcAPIBackend) HeartBeat() {
	// shardSize  = s.master.GetShardSize()
	req := qrpc.Request{Op: qrpc.OpHeartBeat, Data: nil}
	go func(normal bool) {
		for normal {
			time.Sleep(time.Duration(beatTime) * time.Second)
			for endpoint := range s.clientPool {
				normal = s.clientPool[endpoint].HeartBeat(&req)
				if !normal {
					s.master.shutdown <- syscall.SIGTERM
					break
				}
			}
		}
	}(true)
}

func (s *QkcAPIBackend) SendConnectToSlaves() bool {
	var (
		slaveInfos = make([]qrpc.SlaveInfo, 0)
	)
	for _, info := range s.config {
		var chainMask []types.ChainMask
		for _, mask := range info.ChainMaskList {
			chainMask = append(chainMask, *mask)
		}
		slaveInfos = append(
			slaveInfos,
			qrpc.SlaveInfo{
				Id:            []byte(info.ID),
				Host:          []byte(info.IP),
				Port:          info.Port,
				ChainMaskList: chainMask,
			})
	}
	for target, conn := range s.clientPool {
		err := conn.SendConnectToSlaves(slaveInfos)
		if err != nil {
			log.Info("slave manager", "send connection cfg to slaves", "target", target, err)
			return false
		}
	}
	return true
}

func (s *QkcAPIBackend) AddTransaction(tx types.Transaction) bool {
	return false
}

// TODO return type is not confirmed.
func (s *QkcAPIBackend) ExecuteTransaction(tx types.Transaction, address account.Address, height uint64) {
}

func (s *QkcAPIBackend) GetMinorBlockByHash(blockHash common.Hash, branch account.Branch) types.MinorBlock {
	return types.MinorBlock{}
}

func (s *QkcAPIBackend) GetMinorBlockByHeight(height uint64, branch account.Branch) types.MinorBlock {
	return types.MinorBlock{}
}

func (s *QkcAPIBackend) GetTransactionByHash(txHash common.Hash, branch account.Branch) (types.MinorBlock, uint32) {
	return types.MinorBlock{}, 0
}

func (s *QkcAPIBackend) GetTransactionReceipt(txHash common.Hash, branch account.Branch) (types.MinorBlock, uint32, types.Receipt) {
	return types.MinorBlock{}, 0, types.Receipt{}
}

func (s *QkcAPIBackend) GetTransactionsByAddress(address account.Address, start uint64, limit uint32) ([]types.Transactions, uint64) {
	return nil, 0
}

func (s *QkcAPIBackend) GetLogs() {}
func (s *QkcAPIBackend) EstimateGas(tx types.Transaction, address account.Address) uint32 {
	return 0
}
func (s *QkcAPIBackend) GetStorageAt(address account.Address, key serialize.Uint256, height uint64) []byte {
	return nil
}
func (s *QkcAPIBackend) GetCode(address account.Address, height uint64) []byte {
	return nil
}
func (s *QkcAPIBackend) GasPrice(branch account.Branch) uint64 {
	return 0
}
func (s *QkcAPIBackend) GetWork(branch account.Branch) consensus.MiningWork {
	return consensus.MiningWork{}
}
func (s *QkcAPIBackend) SubmitWork(branch account.Branch, headerHash common.Hash, nonce uint64, mixHash common.Hash) bool {
	return false
}
