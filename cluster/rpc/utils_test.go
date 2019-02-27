package rpc

import (
	"io"
	"sync"
	"sync/atomic"
)

// MasterServerSideOp juest for test
type MasterServerSideOp struct {
	rpcId int64
	mu    sync.RWMutex
}

func NewMasterTestOp() *MasterServerSideOp {
	return &MasterServerSideOp{}
}

// master handle function
func (t *MasterServerSideOp) AddMinorBlockHeader(stream MasterServerSideOp_AddMinorBlockHeaderServer) error {
	for {
		_, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&Response{
				RpcId: t.addRpcId(),
				Data:  []byte("AddMinorBlockHeader response"),
			})
		}
		if err != nil {
			return err
		}
	}
}

// rpc id atomic increase in every request
func (m *MasterServerSideOp) addRpcId() int64 {
	atomic.AddInt64(&m.rpcId, 1)
	return m.rpcId
}
