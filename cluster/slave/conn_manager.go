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
	"golang.org/x/sync/errgroup"
	"sync"
)

type masterConn struct {
	target string
	client rpc.Client
}

type ConnManager struct {
	qkcCfg *config.QuarkChainConfig

	// master connection
	masterClient *masterConn
	// slave connection list
	slavesConn map[string]*SlaveConn
	// branch to slave list connection
	fullShardIdToSlaves map[uint32][]*SlaveConn

	// slave backend
	slave *SlaveBackend

	artificialTxConfig *rpc.ArtificialTxConfig
	mu                 sync.Mutex
}

func (s *ConnManager) AddConnectToSlave(info *rpc.SlaveInfo) bool {
	var (
		target = fmt.Sprintf("%s:%d", info.Host, info.Port)
	)

	conn := NewToSlaveConn(target, string(info.Id), info.ChainMaskList)
	log.Info("slave conn manager, add connect to slave", "add target", target)

	// Tell the remote slave who I am.
	if ok := conn.SendPing(); ok {
		s.addSlaveConnection(target, conn)
		return true
	}
	return false
}

func (s *ConnManager) GetConnectionsByFullShardId(id uint32) []*SlaveConn {
	if conns, ok := s.fullShardIdToSlaves[id]; ok {
		return conns
	}
	return []*SlaveConn{}
}

func (s *ConnManager) AddXshardTxList(fullShardId uint32, xshardReq *rpc.AddXshardTxListRequest) error {
	var g errgroup.Group
	if clients, ok := s.fullShardIdToSlaves[fullShardId]; ok {
		for _, client := range clients {
			cli := client
			g.Go(func() error {
				return cli.AddXshardTxList(xshardReq)
			})
		}
	}
	return g.Wait()
}

func (s *ConnManager) BatchAddXshardTxList(fullShardId uint32, xshardReqs []*rpc.AddXshardTxListRequest) error {
	var g errgroup.Group
	if clients, ok := s.fullShardIdToSlaves[fullShardId]; ok {
		for _, client := range clients {
			cli := client
			g.Go(func() error {
				return cli.BatchAddXshardTxList(xshardReqs)
			})
		}
	}
	return g.Wait()
}

// Broadcast x-shard transactions to their recipient shards
func (s *ConnManager) BroadcastXshardTxList(block *types.MinorBlock,
	xshardTxList []*types.CrossShardTransactionDeposit, height uint32) error {

	var (
		hash                = block.Header().Hash()
		xshardTxListRequest = s.getBranchToAddXshardTxListRequest(hash, xshardTxList, height)
		shardSize           = len(s.qkcCfg.GetInitializedShardIdsBeforeRootHeight(height))
	)

	for branch, request := range xshardTxListRequest {
		if branch == block.Header().Branch || account.IsNeighbor(block.Header().Branch, branch, uint32(shardSize)) {
			if len(request.TxList) != 0 {
				return fmt.Errorf("there shouldn't be xshard list for non-neighbor shard",
					"actual branch", block.Header().Branch.Value, "target branch", branch.Value)
			}
			continue
		}
		if shrd, ok := s.slave.shards[branch.Value]; ok {
			shrd.MinorBlockChain.AddCrossShardTxListByMinorBlockHash(hash, types.CrossShardTransactionDepositList{TXList: request.TxList})
		}
		err := s.AddXshardTxList(branch.GetFullShardID(), request)
		if err != nil {
			log.Error("Failed to broadcast xshard transactions", "actual branch", block.Header().Branch.Value, "target branch", branch.Value, "err", err)
			return err
		}
	}
	return nil
}

func (s *ConnManager) BatchBroadcastXshardTxList(
	blokHshToXLstAdPrvRotHg map[common.Hash]*shard.XshardListTuple, sorBrch account.Branch) error {

	brchToAddXsdTxLstReqLst := make(map[account.Branch][]*rpc.AddXshardTxListRequest)
	for hash, xSadLstAndPrevRotHg := range blokHshToXLstAdPrvRotHg {
		xshardTxList := xSadLstAndPrevRotHg.XshardTxList
		prevRootHeight := xSadLstAndPrevRotHg.PrevRootHeight

		brchToAdXsdTxLstReq := s.getBranchToAddXshardTxListRequest(hash, xshardTxList, prevRootHeight)
		for branch, request := range brchToAdXsdTxLstReq {
			lg := len(s.qkcCfg.GetInitializedShardIdsBeforeRootHeight(prevRootHeight))
			if branch == sorBrch || !account.IsNeighbor(branch, sorBrch, uint32(lg)) {
				if len(request.TxList) != 0 {
					return fmt.Errorf(fmt.Sprintf("there shouldn't be xshard list for non-neighbor shard (%d -> %d)",
						sorBrch.Value,
						branch.Value))
				}
				continue
			}
			brchToAddXsdTxLstReqLst[branch] = append(brchToAddXsdTxLstReqLst[branch], request)
		}
	}

	for branch, request := range brchToAddXsdTxLstReqLst {
		if shrd, ok := s.slave.shards[branch.Value]; ok {
			for _, req := range request {
				shrd.MinorBlockChain.AddCrossShardTxListByMinorBlockHash(req.MinorBlockHash, types.CrossShardTransactionDepositList{TXList: req.TxList})
			}
		}
		if err := s.BatchAddXshardTxList(branch.Value, request); err != nil {
			return fmt.Errorf("Failed to batch add xshard tx list", "branch", branch.GetFullShardID(), "err", err)
		}
	}
	return nil
}

func (s *ConnManager) getBranchToAddXshardTxListRequest(blockHash common.Hash,
	xshardTxList []*types.CrossShardTransactionDeposit,
	height uint32) map[account.Branch]*rpc.AddXshardTxListRequest {

	var (
		xshardMap = make(map[account.Branch][]*types.CrossShardTransactionDeposit)
	)

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

// TODO need to check
func (s *ConnManager) addSlaveConnection(target string, conn *SlaveConn) {
	fullShardIdList := s.slave.getFullShardList()
	for _, id := range fullShardIdList {
		if conn.HasShard(id) {
			s.fullShardIdToSlaves[id] = append(s.fullShardIdToSlaves[id], conn)
		}
	}
	s.slavesConn[target] = conn
}

func NewToSlaveConnManager(qkcCfg *config.QuarkChainConfig, slave *SlaveBackend) (*ConnManager, error) {
	slaveConnManager := &ConnManager{
		qkcCfg:              qkcCfg,
		slavesConn:          make(map[string]*SlaveConn),
		fullShardIdToSlaves: make(map[uint32][]*SlaveConn),
		slave:               slave,
	}
	slaveConnManager.masterClient = &masterConn{
		client: rpc.NewClient(rpc.MasterServer),
	}
	return slaveConnManager, nil
}
