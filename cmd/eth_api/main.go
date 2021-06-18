package main

import (
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ybbus/jsonrpc"
)

type ShardConfig struct {
	ShardRPC    string
	FullShardID uint32
	EthChainID  uint32
}

type MetaMaskConfig struct {
	QkcRPC       string
	ShardConfigs []ShardConfig
}

func getMetaConfig() *MetaMaskConfig {
	return &MetaMaskConfig{
		QkcRPC: "http://67.207.94.27:38391",
		//QkcRPC: "http://44.242.140.83:38391",
		ShardConfigs: []ShardConfig{
			{
				ShardRPC:    "0.0.0.0:39001",
				FullShardID: 0,
				EthChainID:  110001,
			},
			{
				ShardRPC:    "0.0.0.0:39002",
				FullShardID: 65536,
				EthChainID:  110002,
			},
		},
	}
}

func main() {
	config := getMetaConfig()
	qkcClient := jsonrpc.NewClient(config.QkcRPC)

	apis := make([][]rpc.API, 0)
	for _, shard := range config.ShardConfigs {
		api := make([]rpc.API, 0)
		api = append(api, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   qkcapi.NewShardAPI(shard.FullShardID, shard.EthChainID, qkcClient),
			Public:    true,
		})
		api = append(api, rpc.API{
			Namespace: "net",
			Version:   "1.0",
			Service:   qkcapi.NewNetApi(qkcClient),
			Public:    true,
		})
		apis = append(apis, api)
	}
	for index, api := range apis {
		if _, _, err := rpc.StartHTTPEndpoint(config.ShardConfigs[index].ShardRPC, api, []string{"eth", "net"}, nil, nil, rpc.DefaultHTTPTimeouts); err != nil {
			panic(err)
		}
	}
	wait := make(chan struct{})
	<-wait
}
