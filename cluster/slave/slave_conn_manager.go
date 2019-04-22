package slave

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/ethereum/go-ethereum/log"
	"sync"
)

type SlaveConnManager struct {
	qkcCfg              *config.QuarkChainConfig
	slavesConn          map[string]*SlaveConn
	fullShardIdToSlaves map[uint32][]*SlaveConn

	mu sync.Mutex
}

func NewToSlaveConnManager(qkcCfg *config.QuarkChainConfig) (*SlaveConnManager, error) {
	return &SlaveConnManager{
		qkcCfg:              qkcCfg,
		slavesConn:          make(map[string]*SlaveConn),
		fullShardIdToSlaves: make(map[uint32][]*SlaveConn),
	}, nil
}

func (s *SlaveConnManager) addSlaveConnection(target string, conn *SlaveConn) {
	s.slavesConn[target] = conn
	for _, id := range s.qkcCfg.GetGenesisShardIds() {
		if conn.HasShard(id) {
			s.fullShardIdToSlaves[id] = append(s.fullShardIdToSlaves[id], conn)
		}
	}
}

func (s *SlaveConnManager) AddConnectToSlave(info *rpc.SlaveInfo) bool {

	var (
		conn   *SlaveConn
		target = fmt.Sprintf("%s:%d", info.Host, info.Port)
		err    error
	)

	conn, err = NewToSlaveConn(target, string(info.Id), info.ChainMaskList)
	if err != nil {
		log.Error("Failed to add connection to slave", "target", target, "err", err)
	}
	log.Info("slave conn manager, add connect to slave", "add target", target)
	// TODO Fill sendPing func to be enabled.
	//id, maskLst := conn.SendPing()
	//if string(info.Id) != id || !conn.EqualChainMask(maskLst) {
	//	return false
	//}
	s.addSlaveConnection(target, conn)
	return true
}

func (s *SlaveConnManager) GetConnectionsByFullShardId(id uint32) []*SlaveConn {
	if conns, ok := s.fullShardIdToSlaves[id]; ok {
		return conns
	}
	return []*SlaveConn{}
}

func (s *SlaveConnManager) AddXshardTxList(fullShardId uint32, xshardReq *rpc.AddXshardTxListRequest) bool {

	// TODO Like this, if failed that need to retry?
	// TODO parallel type or serial type?
	var status = true
	if clients, ok := s.fullShardIdToSlaves[fullShardId]; ok {
		for _, cli := range clients {
			if !cli.AddXshardTxList(xshardReq) && status {
				status = false
			}
		}
	}
	return status
}

func (s *SlaveConnManager) BatchAddXshardTxList(fullShardId uint32, xshardReqs []*rpc.AddXshardTxListRequest) bool {

	// TODO If shard number if greater than 32 need to broadcast and jump.
	var status = true
	if clients, ok := s.fullShardIdToSlaves[fullShardId]; ok {
		for _, cli := range clients {
			if !cli.BatchAddXshardTxList(xshardReqs) && status {
				status = false
			}
		}
	}
	return status
}
