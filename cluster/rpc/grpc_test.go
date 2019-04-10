package rpc

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/rpc"
)

func testSlaveConfig(idx int) *config.SlaveConfig {
	return &config.SlaveConfig{
		IP:            "127.0.0.1",
		Port:          38000 + idx,
		ID:            fmt.Sprintf("S%d", idx),
		ChainMaskList: nil,
	}
}

func TestGRPCAPI(t *testing.T) {
	var (
		apis = []rpc.API{
			{
				Namespace: "rpc." + reflect.TypeOf(MasterServerSideOp{}).Name(),
				Version:   "3.0",
				Service:   NewMasterTestOp(),
				Public:    false,
			},
		}
		cfg            = testSlaveConfig(0)
		hostport       = fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
		rpcID    int64 = 1
	)

	listener, handler, err := StartGRPCServer(hostport, apis)
	if err != nil {
		t.Fatalf("failed to create grpc server %v", err)
	}

	// create rpc client and request AddMinorBlockHeader function
	cli := NewClient()
	res, err := cli.Call(MasterServer, hostport, &Request{Op: OpAddMinorBlockHeader, Data: []byte(fmt.Sprintf("%s op request", cli.GetOpName(OpAddMinorBlockHeader)))})
	if err != nil || res.ErrorCode != 0 {
		t.Fatalf("request master function %s %v", cli.GetOpName(OpAddMinorBlockHeader), err)
	}
	// check rpc id is in order
	if res.RpcId != rpcID {
		t.Fatalf("rpc id %d not returned be order", res.RpcId)
	}
	rpcID = res.RpcId

	if string(res.Data) != fmt.Sprintf("%s response", cli.GetOpName(OpAddMinorBlockHeader)) {
		t.Fatalf("response data %s is not the value of expection", string(res.Data))
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("close grpc server port error: %v", err)
	}
	handler.Stop()
}
