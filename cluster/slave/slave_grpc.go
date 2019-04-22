package slave

import (
	"context"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/log"
	"sync"
	"sync/atomic"
	"time"
)

type SlaveServerSideOp struct {
	rpcId int64
	mu    sync.RWMutex
	slave *SlaveBackend

	run     uint8
	curTime int64
}

func NewServerSideOp(slave *SlaveBackend) *SlaveServerSideOp {
	return &SlaveServerSideOp{
		slave: slave,
	}
}

func (s *SlaveServerSideOp) HeartBeat(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	s.curTime = time.Now().Unix()
	log.Info("slave heart beat response", "request op", req.Op, "current time", s.curTime)
	return &rpc.Response{
		ErrorCode: 0,
	}, nil
}

func (s *SlaveServerSideOp) MasterInfo(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {

	var mstrInfo rpc.MasterInfo
	buf := serialize.NewByteBuffer(req.Data)

	if err := serialize.Deserialize(buf, &mstrInfo); err != nil {
		return nil, err
	}
	s.slave.masterConn.target = fmt.Sprintf("%s:%d", mstrInfo.Ip, mstrInfo.Port)
	log.Info("slave master info response", "master info", s.slave.masterConn.target)
	return &rpc.Response{ErrorCode: 0}, nil
}

func (s *SlaveServerSideOp) Ping(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	log.Info("slave ping response", "request op", req.Op, "rpc id", s.rpcId)
	return &rpc.Response{
		RpcId:     s.addRpcId(),
		ErrorCode: 0,
	}, nil
}

func (s *SlaveServerSideOp) ConnectToSlaves(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {
	var slavesReq rpc.ConnectToSlavesRequest
	buf := serialize.NewByteBuffer(req.Data)

	if err := serialize.Deserialize(buf, &slavesReq); err != nil {
		return nil, err
	}

	for _, slvInfo := range slavesReq.SlaveInfoList {
		if ok := s.slave.slaveConnManager.AddConnectToSlave(&slvInfo); !ok {

		}
	}

	return &rpc.Response{}, nil
}
func (s *SlaveServerSideOp) GetMine(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                             { panic("not implemented") }
func (s *SlaveServerSideOp) GenTx(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                               { panic("not implemented") }
func (s *SlaveServerSideOp) AddRootBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                        { panic("not implemented") }
func (s *SlaveServerSideOp) GetEcoInfoList(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                      { panic("not implemented") }
func (s *SlaveServerSideOp) GetNextBlockToMine(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                  { panic("not implemented") }
func (s *SlaveServerSideOp) AddMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                       { panic("not implemented") }
func (s *SlaveServerSideOp) GetUnconfirmedHeaders(ctx context.Context, req *rpc.Request) (*rpc.Response, error)               { panic("not implemented") }
func (s *SlaveServerSideOp) GetAccountData(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                      { panic("not implemented") }
func (s *SlaveServerSideOp) AddTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                      { panic("not implemented") }
func (s *SlaveServerSideOp) CreateClusterPeerConnection(ctx context.Context, req *rpc.Request) (*rpc.Response, error)         { panic("not implemented") }
func (s *SlaveServerSideOp) DestroyClusterPeerConnectionCommand(ctx context.Context, req *rpc.Request) (*rpc.Response, error) { panic("not implemented") }
func (s *SlaveServerSideOp) GetMinorBlock(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                       { panic("not implemented") }
func (s *SlaveServerSideOp) GetTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                      { panic("not implemented") }
func (s *SlaveServerSideOp) SyncMinorBlockList(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                  { panic("not implemented") }
func (s *SlaveServerSideOp) ExecuteTransaction(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                  { panic("not implemented") }
func (s *SlaveServerSideOp) GetTransactionReceipt(ctx context.Context, req *rpc.Request) (*rpc.Response, error)               { panic("not implemented") }
func (s *SlaveServerSideOp) GetTransactionListByAddress(ctx context.Context, req *rpc.Request) (*rpc.Response, error)         { panic("not implemented") }
func (s *SlaveServerSideOp) GetLogs(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                             { panic("not implemented") }
func (s *SlaveServerSideOp) EstimateGas(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                         { panic("not implemented") }
func (s *SlaveServerSideOp) GetStorageAt(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                        { panic("not implemented") }
func (s *SlaveServerSideOp) GetCode(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                             { panic("not implemented") }
func (s *SlaveServerSideOp) GasPrice(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                            { panic("not implemented") }
func (s *SlaveServerSideOp) GetWork(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                             { panic("not implemented") }
func (s *SlaveServerSideOp) SubmitWork(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                          { panic("not implemented") }
func (s *SlaveServerSideOp) AddXshardTxList(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                     { panic("not implemented") }
func (s *SlaveServerSideOp) BatchAddXshardTxList(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                { panic("not implemented") }

// p2p apis
func (s *SlaveServerSideOp) GetMinorBlocks(ctx context.Context, req *rpc.Request) (*rpc.Response, error)       { panic("not implemented") }
func (s *SlaveServerSideOp) GetMinorBlockHeaders(ctx context.Context, req *rpc.Request) (*rpc.Response, error) { panic("not implemented") }
func (s *SlaveServerSideOp) HandleNewTip(ctx context.Context, req *rpc.Request) (*rpc.Response, error)         { panic("not implemented") }
func (s *SlaveServerSideOp) AddTransactions(ctx context.Context, req *rpc.Request) (*rpc.Response, error)      { panic("not implemented") }

func (s *SlaveServerSideOp) addRpcId() int64 {
	atomic.AddInt64(&s.rpcId, 1)
	return s.rpcId
}
