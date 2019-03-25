package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"reflect"
	"sync/atomic"
)

type Slave struct {
	run    uint32
	config config.SlaveConfig
}

func New(ctx *service.ServiceContext, cfg *config.SlaveConfig) (*Slave, error) {

	log.Info("slave area", "create slave", cfg.Id)
	return &Slave{
		run:    0,
		config: *cfg,
	}, nil
}

func (c *Slave) Protocols() (protos []p2p.Protocol) { return nil }

func (c *Slave) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "rpc." + reflect.TypeOf(SlaveServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(c.config.Id),
			Public:    false,
		},
	}
	return apis
}

func (c *Slave) Stop() error {
	atomic.StoreUint32(&c.run, 0)
	return nil
}

func (c *Slave) Start(srvr *p2p.Server) error {
	atomic.StoreUint32(&c.run, 1)
	return nil
}
