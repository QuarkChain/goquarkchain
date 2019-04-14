package master

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"os"
	"reflect"
	"sync"
)

type MasterBackend struct {
	lock       sync.RWMutex
	config     config.ClusterConfig
	APIBackend *QkcAPIBackend

	eventMux *event.TypeMux
	shutdown chan os.Signal
}

func New(ctx *service.ServiceContext, cfg *config.ClusterConfig) (*MasterBackend, error) {
	var (
		master = &MasterBackend{
			config:   *cfg,
			eventMux: ctx.EventMux,
			shutdown: ctx.Shutdown,
		}
		err error
	)
	master.APIBackend, err = NewQkcAPIBackend(master, cfg.SlaveList)
	if err != nil {
		return nil, err
	}
	return master, nil
}

func (c *MasterBackend) Protocols() (protos []p2p.Protocol) { return nil }

func (c *MasterBackend) APIs() []rpc.API {
	apis := qkcapi.GetAPIs(c.APIBackend)
	return append(apis, []rpc.API{
		{
			Namespace: "rpc." + reflect.TypeOf(MasterServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(c),
			Public:    false,
		},
	}...)
}

func (c *MasterBackend) Stop() error {
	c.eventMux.Stop()
	return nil
}

func (c *MasterBackend) Start(srvr *p2p.Server) error {
	// start heart beat pre 3 seconds.
	c.APIBackend.HeartBeat()
	return nil
}

func (c *MasterBackend) StartMing(threads int) error {
	return nil
}
