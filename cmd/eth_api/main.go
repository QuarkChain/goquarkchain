package main

import (
	"github.com/ybbus/jsonrpc"

	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/rpc"
)

type ShardMetaConfig struct {
	RPC         string
	FullShardID uint32
	ChainID     uint32
}

type MetaConfig struct {
	QkcRPC string
	Shards []ShardMetaConfig
}

func getMetaConfig() *MetaConfig {
	return &MetaConfig{
		QkcRPC: "http://67.207.94.27:38391",
		//QkcRPC: "http://44.242.140.83:38391",
		Shards: []ShardMetaConfig{
			{
				RPC:         "0.0.0.0:39001",
				FullShardID: 1,
				ChainID:     110001,
			},
			{
				RPC:         "0.0.0.0:39002",
				FullShardID: 65537,
				ChainID:     110002,
			},
		},
	}
}

func main() {

	config := getMetaConfig()
	qkcClient := jsonrpc.NewClient(config.QkcRPC)

	apis := make([][]rpc.API, 0)

	for _, shard := range config.Shards {
		api := make([]rpc.API, 0)
		api = append(api, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   qkcapi.NewMetaMaskEthAPI(shard.FullShardID, shard.ChainID, qkcClient),
			Public:    true,
		})
		api = append(api, rpc.API{
			Namespace: "net",
			Version:   "1.0",
			Service:   qkcapi.NewMetaMaskNetApi(qkcClient),
			Public:    true,
		})
		apis = append(apis, api)
	}
	for index, api := range apis {
		rpc.StartHTTPEndpoint(config.Shards[index].RPC, api, []string{"eth", "net"}, nil, nil, rpc.DefaultHTTPTimeouts)
	}
	wait := make(chan struct{})
	<-wait
}
