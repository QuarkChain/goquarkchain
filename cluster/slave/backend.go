package slave

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/event"
	"sync"
)

type SlaveBackend struct {
	clstrCfg      *config.ClusterConfig
	config        *config.SlaveConfig
	fullShardList []uint32

	connManager *ConnManager

	lock   sync.RWMutex
	shards map[uint32]*shard.ShardBackend

	ctx      *service.ServiceContext
	eventMux *event.TypeMux
	logInfo  string
}

func New(ctx *service.ServiceContext, clusterCfg *config.ClusterConfig, cfg *config.SlaveConfig) (*SlaveBackend, error) {
	slave := &SlaveBackend{
		config:        cfg,
		clstrCfg:      clusterCfg,
		fullShardList: make([]uint32, 0),
		shards:        make(map[uint32]*shard.ShardBackend),
		ctx:           ctx,
		eventMux:      ctx.EventMux,
		logInfo:       "SlaveBackend",
	}

	fullShardIds := slave.clstrCfg.Quarkchain.GetGenesisShardIds()
	for _, id := range fullShardIds {
		if !slave.coverShardId(id) {
			continue
		}
		slave.fullShardList = append(slave.fullShardList, id)
	}

	slave.connManager = NewToSlaveConnManager(slave.clstrCfg, slave)
	slave.setPrecompiledContractsEnableTime(clusterCfg.Quarkchain.EnableEvmTimeStamp)
	return slave, nil
}

func (s *SlaveBackend) setPrecompiledContractsEnableTime(enableEvmTime uint64) {
	for _, v := range params.PrecompliedContractsAfterEvmEnabled {
		if vm.PrecompiledContractsByzantium[v] != nil {
			vm.PrecompiledContractsByzantium[v].SetEnableTime(enableEvmTime)
		}
	}
}

func (s *SlaveBackend) GetFullShardList() []uint32 {
	return s.fullShardList
}

func (s *SlaveBackend) coverShardId(id uint32) bool {
	for _, msk := range s.config.ChainMaskList {
		if msk.ContainFullShardId(id) {
			return true
		}
	}
	return false
}

func (s *SlaveBackend) getBranch(address *account.Address) (account.Branch, error) {
	fullShardID, err := s.clstrCfg.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	if err != nil {
		return account.Branch{}, err
	}
	return account.NewBranch(fullShardID), nil
}

func (s *SlaveBackend) addShard(id uint32, shard *shard.ShardBackend) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.shards[id] = shard
}

func (s *SlaveBackend) GetConfig() *config.SlaveConfig {
	return s.config
}

func (s *SlaveBackend) GetShard(fullShardId uint32) *shard.ShardBackend {
	return s.shards[fullShardId]
}

func (s *SlaveBackend) Protocols() (protos []p2p.Protocol) { return nil }

func (s *SlaveBackend) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "grpc",
			Version:   "3.0",
			Service:   NewServerSideOp(s),
			Public:    false,
		},
	}
	if s.ctx.WSIsAlive() {
		apis = append(apis,
			rpc.API{
				Namespace: "ws",
				Version:   "3.0",
				Service:   NewPublicFilterAPI(s), // Private slave api
				Public:    true,
			})
	}
	return apis
}

func (s *SlaveBackend) Stop() error {
	s.eventMux.Stop()
	for target := range s.shards {
		s.shards[target].Stop()
		delete(s.shards, target)
	}
	s.connManager.Stop()
	return nil
}

func (s *SlaveBackend) Init(srvr *p2p.Server) error {
	return nil
}
