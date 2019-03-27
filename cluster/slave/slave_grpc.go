package slave

import (
	"context"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/ethereum/go-ethereum/log"
	"sync"
	"sync/atomic"
)

type SlaveServerSideOp struct {
	rpcId int64
	id    string
	mu    sync.Mutex
}

func NewServerSideOp(id string) *SlaveServerSideOp {
	return &SlaveServerSideOp{
		id: id,
	}
}

func (s *SlaveServerSideOp) Ping(ctx context.Context, req *rpc.Request) (*rpc.Response, error) {

	log.Info("slave ping response", "request op", req.Op, "rpc id", s.rpcId)
	return &rpc.Response{
		RpcId:     s.addRpcId(),
		ErrorCode: 0,
	}, nil
}

func (s *SlaveServerSideOp) ConnectToSlaves(ctx context.Context, req *rpc.Request) (*rpc.Response, error)                     { panic("not implemented") }
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

func (o *SlaveServerSideOp) addRpcId() int64 {
	atomic.AddInt64(&o.rpcId, 1)
	return o.rpcId
}
