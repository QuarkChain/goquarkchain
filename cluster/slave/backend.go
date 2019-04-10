package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"reflect"
	"sync/atomic"
)

type SlaveBackend struct {
	run        uint32
	config     config.SlaveConfig
	masterConn *MasterConnection
}

func New(ctx *service.ServiceContext, cfg *config.SlaveConfig) (*SlaveBackend, error) {
	log.Info("slave area", "create slave", cfg.ID)
	masterConn, err := NewMasterConnection()
	if err != nil {
		return nil, err
	}
	slave := &SlaveBackend{
		run:        0,
		config:     *cfg,
		masterConn: masterConn,
	}
	return slave, nil
}

func (c *SlaveBackend) Protocols() (protos []p2p.Protocol) { return nil }

func (c *SlaveBackend) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "rpc." + reflect.TypeOf(SlaveServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(c),
			Public:    false,
		},
	}
	return apis
}

func (c *SlaveBackend) Stop() error {
	atomic.StoreUint32(&c.run, 0)
	return nil
}

func (c *SlaveBackend) Start(srvr *p2p.Server) error {
	atomic.StoreUint32(&c.run, 1)
	return nil
}
