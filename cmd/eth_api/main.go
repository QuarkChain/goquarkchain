package main

import (
	"fmt"
	"time"

	"github.com/ybbus/jsonrpc"

	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/rpc"
)

type ShardMetaConfig struct {
	RPC         string
	FullShardID uint32
}

type MetaConfig struct {
	QkcRPC string
	Shards []ShardMetaConfig
}

func getMetaConfig() *MetaConfig {
	return &MetaConfig{
		QkcRPC: "http://44.242.140.83:38391",
		Shards: []ShardMetaConfig{
			{
				RPC:         "0.0.0.0:39000",
				FullShardID: 1,
			},
			{
				RPC:         "0.0.0.0:39001",
				FullShardID: 65537,
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
			Service:   qkcapi.NewMetaMaskEthAPI(shard.FullShardID, qkcClient),
			Public:    true,
		})
		api = append(api, rpc.API{
			Namespace: "net",
			Version:   "1.0",
			Service:   qkcapi.NewMetaMaskNetApi(qkcClient),
			Public:    true,
		})
	}
	for index, api := range apis {
		_, _, err := rpc.StartHTTPEndpoint(config.Shards[index].RPC, api, []string{"eth", "net"}, nil, nil, rpc.DefaultHTTPTimeouts)
		fmt.Println("err", err)
	}
	time.Sleep(1000000000 * time.Second)
}
