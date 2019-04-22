package slave

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	grpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"reflect"
	"sync"
)

type SlaveBackend struct {
	qkcCfg *config.QuarkChainConfig
	config *config.SlaveConfig
	mining bool

	masterConn       *MasterConnection
	slaveConnManager *SlaveConnManager

	shards map[account.Branch]shard.ShardAPI

	mu       sync.Mutex
	eventMux *event.TypeMux
}

type xshardListTuple struct {
	xshardTxList   []*types.CrossShardTransactionDeposit
	prevRootHeight uint32
}

func New(ctx *service.ServiceContext, clusterCfg *config.ClusterConfig, cfg *config.SlaveConfig) (*SlaveBackend, error) {

	var (
		err error
	)
	slave := &SlaveBackend{
		config:   cfg,
		qkcCfg:   clusterCfg.Quarkchain,
		shards:   make(map[account.Branch]shard.ShardAPI),
		eventMux: ctx.EventMux,
	}
	// TODO the creation of master connection depend on grpc response

	slave.masterConn, err = NewMasterConnection(ctx)
	if err != nil {
		return nil, err
	}

	slave.slaveConnManager, err = NewToSlaveConnManager(slave.qkcCfg)
	if err != nil {
		return nil, err
	}
	return slave, nil
}

func (s *SlaveBackend) getBranchToAddXshardTxListRequest(blockHash common.Hash,
	xshardTxList []*types.CrossShardTransactionDeposit,
	height uint32) map[account.Branch]*grpc.AddXshardTxListRequest {

	var (
		xshardMap       = make(map[account.Branch][]*types.CrossShardTransactionDeposit)
		initialShardIds = s.qkcCfg.GetInitializedShardIdsBeforeRootHeight(height)
	)

	for _, id := range initialShardIds {
		branch := account.NewBranch(id)
		xshardMap[branch] = make([]*types.CrossShardTransactionDeposit, 0, len(initialShardIds))
	}

	for _, xshardTx := range xshardTxList {
		fullShardId := s.qkcCfg.GetFullShardIdByFullShardKey(xshardTx.To.FullShardKey)
		branch := account.NewBranch(fullShardId)
		if _, ok := xshardMap[branch]; ok {
			xshardMap[branch] = append(xshardMap[branch], xshardTx)
		}
	}

	xshardTxListRequest := make(map[account.Branch]*grpc.AddXshardTxListRequest)
	for branch, txList := range xshardMap {
		crossShardTxLst := &grpc.CrossShardTransactionList{TxList: txList}
		request := grpc.AddXshardTxListRequest{Branch: branch, MinorBlockHash: blockHash, TxList: crossShardTxLst}
		xshardTxListRequest[branch] = &request
	}
	return xshardTxListRequest
}

func (s *SlaveBackend) coverShardId(id uint32) bool {
	for _, msk := range s.config.ChainMaskList {
		if msk.ContainFullShardId(id) {
			return true
		}
	}
	return false
}

func (s *SlaveBackend) Protocols() (protos []p2p.Protocol) { return nil }

func (s *SlaveBackend) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "rpc." + reflect.TypeOf(SlaveServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(s),
			Public:    false,
		},
	}
	return apis
}

func (s *SlaveBackend) Stop() error {
	s.eventMux.Stop()
	return nil
}

func (s *SlaveBackend) Start(srvr *p2p.Server) error {
	return nil
}

func (s *SlaveBackend) CreateShards(ctx *service.ServiceContext, rootBlock types.RootBlock) (err error) {

	var (
		fullShardIds = s.qkcCfg.GetGenesisShardIds()
	)

	for _, id := range fullShardIds {
		branch := account.NewBranch(id)
		if _, ok := s.shards[branch]; ok {
			continue
		}
		shardCfg := s.qkcCfg.GetShardConfigByFullShardID(id)
		if !s.coverShardId(id) || shardCfg.Genesis == nil {
			continue
		}
		if rootBlock.Header().Number >= shardCfg.Genesis.RootHeight {
			s.shards[branch], err = shard.NewShard(ctx, shardCfg)
			if err != nil {
				log.Error("Failed to create shard", "slave id", s.config.ID, "shard id", shardCfg.ShardID, "err", err)
				return
			}
		}
	}
	return
}

func (s *SlaveBackend) BroadcastXshardTxList(
	block *types.MinorBlock,
	xshardTxList []*types.CrossShardTransactionDeposit, height uint32) {
	hash := block.Header().Hash()
	xshardTxListRequest := s.getBranchToAddXshardTxListRequest(hash, xshardTxList, height)
	for branch, request := range xshardTxListRequest {
		// TODO add is neighbor judge
		if branch == block.Header().Branch {
			continue
		}
		if shrd, ok := s.shards[branch]; ok {
			shrd.AddCrossShardTxListByMinorBlockHash(hash, request.TxList)
		}
		ok := s.slaveConnManager.AddXshardTxList(branch.GetFullShardID(), request)
		if !ok {
			log.Error("Failed to broadcast xshard transactions", "full shard id", branch.GetFullShardID())
		}
	}
}

func (s *SlaveBackend) BatchBroadcastXshardTxList(
	blokHshToXLstAdPrvRotHg map[common.Hash]*xshardListTuple,
	sorBrch account.Branch) bool {

	brchToAddXsdTxLstReqLst := make(map[account.Branch][]*grpc.AddXshardTxListRequest)
	for hash, xSadLstAndPrevRotHg := range blokHshToXLstAdPrvRotHg {
		xshardTxList := xSadLstAndPrevRotHg.xshardTxList
		prevRootHeight := xSadLstAndPrevRotHg.prevRootHeight

		brchToAdXsdTxLstReq := s.getBranchToAddXshardTxListRequest(hash, xshardTxList, prevRootHeight)
		for branch, request := range brchToAdXsdTxLstReq {
			if branch == sorBrch {
				lg := len(s.qkcCfg.GetInitializedShardIdsBeforeRootHeight(prevRootHeight))
				if !account.IsNeighbor(branch, sorBrch, uint32(lg)) {
					if len(request.TxList.TxList) == 0 {
						log.Error(fmt.Sprintf("there shouldn't be xshard list for non-neighbor shard (%d -> %d)", sorBrch.Value, branch.Value))
						continue
					}
				}
			}
			if _, ok := brchToAddXsdTxLstReqLst[branch]; !ok {
				brchToAddXsdTxLstReqLst[branch] = make([]*grpc.AddXshardTxListRequest, 0)
			}
			brchToAddXsdTxLstReqLst[branch] = append(brchToAddXsdTxLstReqLst[branch], request)
		}
	}

	var status = true
	for brch, request := range brchToAddXsdTxLstReqLst {
		if shrd, ok := s.shards[brch]; ok {
			for _, req := range request {
				shrd.AddCrossShardTxListByMinorBlockHash(req.MinorBlockHash, req.TxList)
			}
		}
		if !s.slaveConnManager.BatchAddXshardTxList(brch.GetFullShardID(), request) {
			status = false
			log.Error("Failed to batch add xshard tx list", "branch", brch.GetFullShardID())
		}
	}
	return status
}

func (s *SlaveBackend) AddBlockListForSync(blockLst []*types.MinorBlock) bool {

	if blockLst == nil {
		return true
	}

	brch := blockLst[0].Header().Branch
	if shrd, ok := s.shards[brch]; ok {
		if !shrd.AddBlockListForSync(blockLst) {
			return false
		}
	}
	return true
}

func (s *SlaveBackend) AddTx(tx *types.Transaction) bool {
	// TODO evm_tx need to be filled.
	var branch = account.NewBranch(0)
	if shrd, ok := s.shards[branch]; ok {
		if !shrd.AddTx(tx) {
			return false
		}
	}
	return true
}

func (s *SlaveBackend) ExecuteTx(tx *types.Transaction, address *account.Address) {
	// TODO like AddTx func.
}

func (s *SlaveBackend) GetTransactionCount(address *account.Address) {
	branch := account.NewBranch(s.qkcCfg.GetFullShardIdByFullShardKey(address.FullShardKey))
	if _, ok := s.shards[branch]; ok {
	}
}

func (s *SlaveBackend) GetBalances()                 {}
func (s *SlaveBackend) GetTokenBalance()             {}
func (s *SlaveBackend) GetAccountData()              {}
func (s *SlaveBackend) GetMinorBlockByHash()         {}
func (s *SlaveBackend) GetMinorBlockByHeight()       {}
func (s *SlaveBackend) GetTransactionByHash()        {}
func (s *SlaveBackend) GetTransactionReceipt()       {}
func (s *SlaveBackend) GetTransactionListByAddress() {}
func (s *SlaveBackend) GetLogs()                     {}
func (s *SlaveBackend) EstimateGas()                 {}
func (s *SlaveBackend) GetStorageAt()                {}
func (s *SlaveBackend) GetCode()                     {}
func (s *SlaveBackend) GasPrice()                    {}
func (s *SlaveBackend) GetWork()                     {}
