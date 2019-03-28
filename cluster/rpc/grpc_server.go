// Modified from go-ethereum under GNU Lesser General Public License
package rpc

import (
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/grpc"
	"net"
)

func StartGRPCServer(grpcEndpoint string, apis []rpc.API) (net.Listener, *grpc.Server, error) {
	handler := grpc.NewServer()
	// regist rpc services
	for _, api := range apis {
		switch api.Namespace {
		// regist master handle service
		case _MasterServerSideOp_serviceDesc.ServiceName:
			handler.RegisterService(&_MasterServerSideOp_serviceDesc, api.Service)
		// regist slave handle service
		case _SlaveServerSideOp_serviceDesc.ServiceName:
			handler.RegisterService(&_SlaveServerSideOp_serviceDesc, api.Service)
		}
	}
	var (
		listener net.Listener
		err      error
	)
	if listener, err = net.Listen("tcp", grpcEndpoint); err != nil {
		return nil, nil, err
	}
	go handler.Serve(listener)
	return listener, handler, err
}
