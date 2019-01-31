package rpc

import (
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/grpc"
	"net"
	"strings"
)

func StartGRPCServer(grpcEndpoint string, apis []rpc.API) (net.Listener, *grpc.Server, error) {
	handler := grpc.NewServer()
	// register service
	for _, api := range apis {
		if strings.HasSuffix(_MasterServerSideOp_serviceDesc.ServiceName, api.Namespace) {
			handler.RegisterService(&_MasterServerSideOp_serviceDesc, api.Service)
		} else if strings.HasSuffix(_SlaveServerSideOp_serviceDesc.ServiceName, api.Namespace) {
			handler.RegisterService(&_SlaveServerSideOp_serviceDesc, api.Service)
		} else if strings.HasSuffix(_CommonOp_serviceDesc.ServiceName, api.Namespace) {
			handler.RegisterService(&_CommonOp_serviceDesc, api.Service)
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
