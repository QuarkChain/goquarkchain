package master

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"io"
	"sync"
	"sync/atomic"
)

type MasterServerSideOp struct {
	rpcId int64
	mu    sync.RWMutex
}

func NewServerSideOp() *MasterServerSideOp {
	return &MasterServerSideOp{
	}
}

func (m *MasterServerSideOp) AddMinorBlockHeader(stream rpc.MasterServerSideOp_AddMinorBlockHeaderServer) error {
	for {
		point, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&rpc.Response{
			})
		}
		if err != nil {
			return err
		}
		fmt.Println("master service", string(point.Data))
	}
}

func (m *MasterServerSideOp) addRpcId() int64 {
	atomic.AddInt64(&m.rpcId, 1)
	return m.rpcId
}
