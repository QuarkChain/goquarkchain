package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"

	"github.com/ybbus/jsonrpc"

	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/rpc"
)

var metaMaskConfig = flag.String("config", "", "metaMask config file")

type ShardConfig struct {
	ShardRPC    string
	FullShardID hexutil.Uint
	EthChainID  uint32
}

type MetaMaskConfig struct {
	QkcRPC       string
	VHost        []string
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
	flag.Parse()
	config := getMetaConfig(*metaMaskConfig)
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
		api = append(api, rpc.API{
			Namespace: "web3",
			Version:   "1.0",
			Service:   qkcapi.NewWeb3Api(qkcClient),
			Public:    true,
		})
		apis = append(apis, api)
	}
	for index, api := range apis {
		if _, _, err := rpc.StartHTTPEndpoint(config.ShardConfigs[index].ShardRPC, api, []string{"eth", "net", "web3"}, nil, config.VHost, rpc.DefaultHTTPTimeouts); err != nil {
			panic(err)
		}
	}
	wait := make(chan struct{})
	<-wait
}
