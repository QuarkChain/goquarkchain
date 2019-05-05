package slave

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"sync"
)

type masterConn struct {
	target string
	client rpc.Client
}

type SlaveConnManager struct {
	qkcCfg *config.QuarkChainConfig

	// master connection
	masterCli *masterConn
	// slave connection list
	slavesConn map[string]*SlaveConn
	// branch to slave list connection
	fullShardIdToSlaves map[uint32][]*SlaveConn

	// slave backend
	slave *SlaveBackend

	artificialTxConfig *rpc.ArtificialTxConfig
	mu                 sync.Mutex
}

func (s *SlaveConnManager) ModifyMasterConn(target string) {
	s.masterCli.target = target
}

func (s *SlaveConnManager) AddConnectToSlave(info *rpc.SlaveInfo) error {

	var (
		conn   *SlaveConn
		target = fmt.Sprintf("%s:%d", info.Host, info.Port)
		err    error
	)

	conn, err = NewToSlaveConn(target, string(info.Id), info.ChainMaskList)
	if err != nil {
		log.Error("Failed to add connection to slave", "target", target, "err", err)
		return err
	}
	log.Info("slave conn manager, add connect to slave", "add target", target)

	// Tell the remote slave who I am.
	if ok := conn.SendPing(); ok {
		s.addSlaveConnection(target, conn)
	}

	return nil
}

func (s *SlaveConnManager) GetConnectionsByFullShardId(id uint32) []*SlaveConn {
	if conns, ok := s.fullShardIdToSlaves[id]; ok {
		return conns
	}
	return []*SlaveConn{}
}

func (s *SlaveConnManager) AddXshardTxList(fullShardId uint32, xshardReq *rpc.AddXshardTxListRequest) bool {

	var status = true
	if clients, ok := s.fullShardIdToSlaves[fullShardId]; ok {
		for _, cli := range clients {
			if !cli.AddXshardTxList(xshardReq) {
				status = false
			}
		}
	}
	return status
}

func (s *SlaveConnManager) BatchAddXshardTxList(fullShardId uint32, xshardReqs []*rpc.AddXshardTxListRequest) bool {

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

// Broadcast x-shard transactions to their recipient shards
func (s *SlaveConnManager) BroadcastXshardTxList(block *types.MinorBlock,
	xshardTxList []*types.CrossShardTransactionDeposit, height uint32) {

	var (
		hash                = block.Header().Hash()
		xshardTxListRequest = s.getBranchToAddXshardTxListRequest(hash, xshardTxList, height)
		shardSize           = len(s.qkcCfg.GetInitializedShardIdsBeforeRootHeight(height))
	)

	for branch, request := range xshardTxListRequest {
		if branch == block.Header().Branch ||
			account.IsNeighbor(block.Header().Branch, branch, uint32(shardSize)) {

			if len(request.TxList) == 0 {

				log.Warn("there shouldn't be xshard list for non-neighbor shard",
					"actual branch", block.Header().Branch.Value, "target branch", branch.Value)
			}
			continue
		}
		if shrd, ok := s.slave.shards[branch.Value]; ok {
			shrd.State.AddCrossShardTxListByMinorBlockHash(hash, request.TxList)
		}
		ok := s.AddXshardTxList(branch.GetFullShardID(), request)
		if !ok {
			log.Error("Failed to broadcast xshard transactions", "full shard id", branch.GetFullShardID())
		}
	}
}

func (s *SlaveConnManager) BatchBroadcastXshardTxList(
	blokHshToXLstAdPrvRotHg map[common.Hash]*shard.XshardListTuple, sorBrch account.Branch) bool {

	brchToAddXsdTxLstReqLst := make(map[account.Branch][]*rpc.AddXshardTxListRequest)
	for hash, xSadLstAndPrevRotHg := range blokHshToXLstAdPrvRotHg {
		xshardTxList := xSadLstAndPrevRotHg.XshardTxList
		prevRootHeight := xSadLstAndPrevRotHg.PrevRootHeight

		brchToAdXsdTxLstReq := s.getBranchToAddXshardTxListRequest(hash, xshardTxList, prevRootHeight)
		for branch, request := range brchToAdXsdTxLstReq {
			if branch == sorBrch {
				lg := len(s.qkcCfg.GetInitializedShardIdsBeforeRootHeight(prevRootHeight))
				if !account.IsNeighbor(branch, sorBrch, uint32(lg)) {
					if len(request.TxList) == 0 {
						log.Error(fmt.Sprintf("there shouldn't be xshard list for non-neighbor shard (%d -> %d)",
							sorBrch.Value,
							branch.Value))
						continue
					}
				}
			}
			if _, ok := brchToAddXsdTxLstReqLst[branch]; !ok {
				brchToAddXsdTxLstReqLst[branch] = make([]*rpc.AddXshardTxListRequest, 0)
			}
			brchToAddXsdTxLstReqLst[branch] = append(brchToAddXsdTxLstReqLst[branch], request)
		}
	}

	var status = true
	for brch, request := range brchToAddXsdTxLstReqLst {
		if shrd, ok := s.slave.shards[brch.Value]; ok {
			for _, req := range request {
				shrd.State.AddCrossShardTxListByMinorBlockHash(req.MinorBlockHash, req.TxList)
			}
		}
		if !s.BatchAddXshardTxList(brch.GetFullShardID(), request) {
			status = false
			log.Error("Failed to batch add xshard tx list", "branch", brch.GetFullShardID())
		}
	}
	return status
}

func (s *SlaveConnManager) getBranchToAddXshardTxListRequest(blockHash common.Hash,
	xshardTxList []*types.CrossShardTransactionDeposit,
	height uint32) map[account.Branch]*rpc.AddXshardTxListRequest {

	var (
		xshardMap = make(map[account.Branch][]*types.CrossShardTransactionDeposit)
		// only broadcast to the shards that have been initialized
		initialShardIds = s.qkcCfg.GetInitializedShardIdsBeforeRootHeight(height)
	)

	for _, id := range initialShardIds {
		branch := account.NewBranch(id)
		xshardMap[branch] = make([]*types.CrossShardTransactionDeposit, 0)
	}

	for _, xshardTx := range xshardTxList {
		fullShardId := s.qkcCfg.GetFullShardIdByFullShardKey(xshardTx.To.FullShardKey)
		branch := account.NewBranch(fullShardId)
		if _, ok := xshardMap[branch]; ok {
			xshardMap[branch] = append(xshardMap[branch], xshardTx)
		}
	}

	xshardTxListRequest := make(map[account.Branch]*rpc.AddXshardTxListRequest)
	for branch, txLst := range xshardMap {
		request := rpc.AddXshardTxListRequest{
			Branch:         branch.Value,
			MinorBlockHash: blockHash,
			TxList:         txLst,
		}
		xshardTxListRequest[branch] = &request
	}
	return xshardTxListRequest
}

func (s *SlaveConnManager) addSlaveConnection(target string, conn *SlaveConn) {
	s.slavesConn[target] = conn
	for _, slv := range s.slave.shards {
		id := slv.Config.ShardID
		if _, ok := s.fullShardIdToSlaves[id]; !ok {
			s.fullShardIdToSlaves[id] = make([]*SlaveConn, 0)
		}
		s.fullShardIdToSlaves[id] = append(s.fullShardIdToSlaves[id], conn)
	}
}

func NewToSlaveConnManager(qkcCfg *config.QuarkChainConfig, slave *SlaveBackend) (*SlaveConnManager, error) {
	slaveConnManager := &SlaveConnManager{
		qkcCfg:              qkcCfg,
		slavesConn:          make(map[string]*SlaveConn),
		fullShardIdToSlaves: make(map[uint32][]*SlaveConn),
		slave:               slave,
	}
	slaveConnManager.masterCli = &masterConn{
		client: rpc.NewClient(rpc.MasterServer),
	}
	return slaveConnManager, nil
}
