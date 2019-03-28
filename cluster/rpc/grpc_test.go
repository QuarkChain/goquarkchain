package rpc

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/rpc"
	"reflect"
	"testing"
)

func testSlaveConfig(idx int) *config.SlaveConfig {
	return &config.SlaveConfig{
		Ip:            config.HOST,
		Port:          uint64(config.SLAVE_PORT + idx),
		Id:            "S" + string(idx),
		ShardMaskList: nil,
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
		cfg          = testSlaveConfig(0)
		target       = fmt.Sprintf("%s:%d", cfg.Ip, cfg.Port)
		rpcId  int64 = 1
	)

	listener, handler, err := StartGRPCServer(target, apis)
	if err != nil {
		t.Fatalf("failed to create grpc server %v", err)
	}

	// create rpc client and request AddMinorBlockHeader function
	cli, err := NewRPCLient(target)
	if err != nil {
		t.Fatalf("create connection %v", err)
	}
	res, err := cli.GetMasterServerSideOp(&Request{Op: OpAddMinorBlockHeader, Data: []byte(fmt.Sprintf("%s op request", cli.GetOpName(OpAddMinorBlockHeader)))})
	if err != nil || res.ErrorCode != 0 {
		t.Fatalf("request master function %s %v", cli.GetOpName(OpAddMinorBlockHeader), err)
	}
	// check rpc id is in order
	if res.RpcId != rpcId {
		t.Fatalf("rpc id %d not returned be order", res.RpcId)
	}
	rpcId = res.RpcId

	if string(res.Data) != fmt.Sprintf("%s response", cli.GetOpName(OpAddMinorBlockHeader)) {
		t.Fatalf("response data %s is not the value of expection", string(res.Data))
	}

	if err := listener.Close(); err != nil {
		t.Fatalf("close grpc server port error: %v", err)
	}
	handler.Stop()
}
