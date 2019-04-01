package master

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"reflect"
	"sync"
	"sync/atomic"
)

type MasterBackend struct {
	lock       sync.RWMutex
	config     config.ClusterConfig
	APIBackend *MasterAPIBackend
	run        int64
}

func New(ctx *service.ServiceContext, cfg *config.ClusterConfig) (*MasterBackend, error) {
	var (
		master = &MasterBackend{
			run:    0,
			config: *cfg,
		}
		err error
	)
	master.APIBackend, err = NewSlaveConnManafer(master, cfg.SlaveList)
	if err != nil {
		return nil, err
	}
	return master, nil
}

func (c *MasterBackend) Protocols() (protos []p2p.Protocol) { return nil }

func (c *MasterBackend) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "mater",
			Version:   "3.0",
			Service:   c.APIBackend,
			Public:    true,
		},
		{
			Namespace: "rpc." + reflect.TypeOf(MasterAPIBackend{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(c),
			Public:    false,
		},
	}
	return apis
}

func (c *MasterBackend) Stop() error {
	atomic.StoreInt64(&c.run, 0)
	return nil
}

func (c *MasterBackend) Start(srvr *p2p.Server) error {
	atomic.StoreInt64(&c.run, 1)
	c.APIBackend.ConnecToSlaves()
	return nil
}

func (c *MasterBackend) StartMing(threads int) error {
	return nil
}
