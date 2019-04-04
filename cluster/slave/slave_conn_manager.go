package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
)

type ConnectionManager struct {
	config     []*config.SlaveConfig
	slave      *SlaveBackend
	clientPool map[string]rpc.Client
}

func NewConnectionManager(slave *SlaveBackend, config []*config.SlaveConfig) (*ConnectionManager, error) {
	return &ConnectionManager{
		slave:  slave,
		config: config,
	}, nil
}
