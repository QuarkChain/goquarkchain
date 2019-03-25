package slave

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/log"
	"io"
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

func (o *SlaveServerSideOp) Ping(stream rpc.SlaveServerSideOp_PingServer) error {
	for {
		point, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&rpc.Response{
				RpcId:     o.addRpcId(),
				ErrorCode: 0,
				// Data:      []byte(fmt.Sprintf("%s slave server response", o.slave.config.Id)),
			})
		}
		if err != nil {
			return err
		}
		log.Info("slave ping response", "slave id", o.id, "data", string(point.Data))
	}
}

func (o *SlaveServerSideOp) ConnectToSlaves(stream rpc.SlaveServerSideOp_ConnectToSlavesServer) error {
	for {
		point, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&rpc.Response{
				RpcId:     o.addRpcId(),
				ErrorCode: 0,
				// Data:      []byte(fmt.Sprintf("%s server response", o.slave.config.Id)),
			})
		}
		if err != nil {
			return err
		} //else {
		var (
			slvRequest rpc.ConnectToSlavesRequest
			bb         = serialize.NewByteBuffer(point.Data)
		)
		if err := serialize.Deserialize(bb, &slvRequest); err != nil {
			fmt.Println("slave service", slvRequest)
			return err
		}
		for _, slv := range slvRequest.SlaveInfoList {
			fmt.Println(string(slv.Id), string(slv.Host), slv.Port, slv.ChainMaskList)
		}
	}
}

func (o *SlaveServerSideOp) GetMine(stream rpc.SlaveServerSideOp_GetMineServer) error                                                         { panic("not implemented") }
func (o *SlaveServerSideOp) GenTx(stream rpc.SlaveServerSideOp_GenTxServer) error                                                             { panic("not implemented") }
func (o *SlaveServerSideOp) AddRootBlock(stream rpc.SlaveServerSideOp_AddRootBlockServer) error                                               { panic("not implemented") }
func (o *SlaveServerSideOp) GetEcoInfoList(stream rpc.SlaveServerSideOp_GetEcoInfoListServer) error                                           { panic("not implemented") }
func (o *SlaveServerSideOp) GetNextBlockToMine(stream rpc.SlaveServerSideOp_GetNextBlockToMineServer) error                                   { panic("not implemented") }
func (o *SlaveServerSideOp) AddMinorBlock(stream rpc.SlaveServerSideOp_AddMinorBlockServer) error                                             { panic("not implemented") }
func (o *SlaveServerSideOp) GetUnconfirmedHeaders(stream rpc.SlaveServerSideOp_GetUnconfirmedHeadersServer) error                             { panic("not implemented") }
func (o *SlaveServerSideOp) GetAccountData(stream rpc.SlaveServerSideOp_GetAccountDataServer) error                                           { panic("not implemented") }
func (o *SlaveServerSideOp) AddTransaction(stream rpc.SlaveServerSideOp_AddTransactionServer) error                                           { panic("not implemented") }
func (o *SlaveServerSideOp) CreateClusterPeerConnection(stream rpc.SlaveServerSideOp_CreateClusterPeerConnectionServer) error                 { panic("not implemented") }
func (o *SlaveServerSideOp) DestroyClusterPeerConnectionCommand(stream rpc.SlaveServerSideOp_DestroyClusterPeerConnectionCommandServer) error { panic("not implemented") }
func (o *SlaveServerSideOp) GetMinorBlock(stream rpc.SlaveServerSideOp_GetMinorBlockServer) error                                             { panic("not implemented") }
func (o *SlaveServerSideOp) GetTransaction(stream rpc.SlaveServerSideOp_GetTransactionServer) error                                           { panic("not implemented") }
func (o *SlaveServerSideOp) SyncMinorBlockList(stream rpc.SlaveServerSideOp_SyncMinorBlockListServer) error                                   { panic("not implemented") }
func (o *SlaveServerSideOp) ExecuteTransaction(stream rpc.SlaveServerSideOp_ExecuteTransactionServer) error                                   { panic("not implemented") }
func (o *SlaveServerSideOp) GetTransactionReceipt(stream rpc.SlaveServerSideOp_GetTransactionReceiptServer) error                             { panic("not implemented") }
func (o *SlaveServerSideOp) GetTransactionListByAddress(stream rpc.SlaveServerSideOp_GetTransactionListByAddressServer) error                 { panic("not implemented") }
func (o *SlaveServerSideOp) GetLogs(stream rpc.SlaveServerSideOp_GetLogsServer) error                                                         { panic("not implemented") }
func (o *SlaveServerSideOp) EstimateGas(stream rpc.SlaveServerSideOp_EstimateGasServer) error                                                 { panic("not implemented") }
func (o *SlaveServerSideOp) GetStorageAt(stream rpc.SlaveServerSideOp_GetStorageAtServer) error                                               { panic("not implemented") }
func (o *SlaveServerSideOp) GetCode(stream rpc.SlaveServerSideOp_GetCodeServer) error                                                         { panic("not implemented") }
func (o *SlaveServerSideOp) GasPrice(stream rpc.SlaveServerSideOp_GasPriceServer) error                                                       { panic("not implemented") }
func (o *SlaveServerSideOp) GetWork(stream rpc.SlaveServerSideOp_GetWorkServer) error                                                         { panic("not implemented") }
func (o *SlaveServerSideOp) SubmitWork(stream rpc.SlaveServerSideOp_SubmitWorkServer) error                                                   { panic("not implemented") }
func (o *SlaveServerSideOp) AddXshardTxList(stream rpc.SlaveServerSideOp_AddXshardTxListServer) error                                         { panic("not implemented") }
func (o *SlaveServerSideOp) BatchAddXshardTxList(stream rpc.SlaveServerSideOp_BatchAddXshardTxListServer) error                               { panic("not implemented") }

func (o *SlaveServerSideOp) addRpcId() int64 {
	atomic.AddInt64(&o.rpcId, 1)
	return o.rpcId
}
