package slave

import (
	"reflect"
	"sync"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
)

type SlaveBackend struct {
	clstrCfg      *config.ClusterConfig
	config        *config.SlaveConfig
	fullShardList []uint32

	connManager *ConnManager

	shards map[uint32]*shard.ShardBackend

	ctx      *service.ServiceContext
	mu       sync.RWMutex
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
	return slave, nil
}

func (s *SlaveBackend) getFullShardList() []uint32 {
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

func (s *SlaveBackend) getBranch(address *account.Address) account.Branch {
	return account.NewBranch(s.clstrCfg.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey))
}

func (s *SlaveBackend) GetConfig() *config.SlaveConfig {
	return s.config
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
	for target := range s.shards {
		s.shards[target].Stop()
		delete(s.shards, target)
	}
	return nil
}

func (s *SlaveBackend) Start(srvr *p2p.Server) error {
	return nil
}
