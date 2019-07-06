package test

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
)

func defaultNodeConfig() *service.Config {
	serviceConfig := &service.DefaultConfig
	serviceConfig.Name = ""
	serviceConfig.Version = ""
	serviceConfig.IPCPath = "qkc.ipc"
	serviceConfig.SvrModule = "rpc."
	serviceConfig.SvrPort = config.GrpcPort
	serviceConfig.SvrHost = "127.0.0.1"
	return serviceConfig
}
