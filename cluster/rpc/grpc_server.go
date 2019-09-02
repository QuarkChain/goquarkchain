package rpc

import (
	"fmt"
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/grpc"
	"net"
	"reflect"
	"strings"
)

func StartGRPCServer(hostport string, apis []rpc.API) (net.Listener, *grpc.Server, error) {
	handler := grpc.NewServer()
	// register rpc services
	for _, api := range apis {
		srvsplit := strings.Split(reflect.TypeOf(api.Service).String(), ".")
		if len(srvsplit) == 0 {
			panic(fmt.Sprintf("%s service is nil", api.Namespace))
		}
		svrname := srvsplit[len(srvsplit)-1]
		switch {
		case strings.HasSuffix(_MasterServerSideOp_serviceDesc.ServiceName, svrname):
			handler.RegisterService(&_MasterServerSideOp_serviceDesc, api.Service)
		case strings.HasSuffix(_SlaveServerSideOp_serviceDesc.ServiceName, svrname):
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
