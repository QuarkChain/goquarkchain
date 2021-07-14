package main

import (
	"encoding/json"
	"io/ioutil"

	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ybbus/jsonrpc"
)

type ShardConfig struct {
	ShardRPC    string
	FullShardID hexutil.Uint
	EthChainID  uint32
}

type MetaMaskConfig struct {
	QkcRPC       string
	ShardConfigs []ShardConfig
}

func getMetaConfig(file string) *MetaMaskConfig {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	m := new(MetaMaskConfig)
	err = json.Unmarshal(data, m)
	if err != nil {
		panic(err)
	}
	return m
}

func main() {
	config := getMetaConfig("./eth_api.json")
	qkcClient := jsonrpc.NewClient(config.QkcRPC)

	apis := make([][]rpc.API, 0)
	for _, shard := range config.ShardConfigs {
		api := make([]rpc.API, 0)
		api = append(api, rpc.API{
			Namespace: "eth",
			Version:   "1.0",
			Service:   qkcapi.NewShardAPI(uint32(shard.FullShardID), shard.EthChainID, qkcClient),
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
