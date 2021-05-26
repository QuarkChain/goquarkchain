package main

import (
	"fmt"
	"time"

	"github.com/ybbus/jsonrpc"

	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/rpc"
)

func main() {
	client := jsonrpc.NewClient("http://44.242.140.83:38391")
	endPoint := "0.0.0.0:39000"

	apis := make([]rpc.API, 0)
	apis = append(apis, rpc.API{
		Namespace: "eth",
		Version:   "1.0",
		Service:   qkcapi.NewMetaMaskEthAPI(1, client),
		Public:    true,
	})
	_, _, err := rpc.StartHTTPEndpoint(endPoint, apis, []string{"eth"}, nil, nil, rpc.DefaultHTTPTimeouts)
	fmt.Println("err", err)
	time.Sleep(1000000000 * time.Second)

}
