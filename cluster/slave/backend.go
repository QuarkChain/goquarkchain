package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
	"reflect"
)

type SlaveBackend struct {
	config     config.SlaveConfig
	masterConn *MasterConnection

	eventMux  *event.TypeMux
}

func New(ctx *service.ServiceContext, cfg *config.SlaveConfig) (*SlaveBackend, error) {
	log.Info("slave area", "create slave", cfg.ID)
	masterConn, err := NewMasterConnection()
	if err != nil {
		return nil, err
	}
	slave := &SlaveBackend{
		config:     *cfg,
		masterConn: masterConn,
		eventMux:   ctx.EventMux,
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
	c.eventMux.Stop()
	return nil
}

func (c *SlaveBackend) Start(srvr *p2p.Server) error {
	return nil
}
