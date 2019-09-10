package rpc

import (
	"net"

	"github.com/QuarkChain/goquarkchain/rpc"
	"google.golang.org/grpc"
)

func StartGRPCServer(hostport string, apis []rpc.API) (net.Listener, *grpc.Server, error) {
	handler := grpc.NewServer()
	// register rpc services
	for _, api := range apis {
		switch api.Namespace {
		// register master handle service
		case _MasterServerSideOp_serviceDesc.ServiceName:
			handler.RegisterService(&_MasterServerSideOp_serviceDesc, api.Service)
		// register slave handle service
		case _SlaveServerSideOp_serviceDesc.ServiceName:
			handler.RegisterService(&_SlaveServerSideOp_serviceDesc, api.Service)
		}
	}
	var (
		listener net.Listener
		err      error
	)
	if listener, err = net.Listen("tcp", hostport); err != nil {
		return nil, nil, err
	}
	go handler.Serve(listener)
	return listener, handler, err
}
