package rpc

import (
	"fmt"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/rpc"
	"google.golang.org/grpc"
	"net"
	"reflect"
	"strings"
)

func StartGRPCServer(hostport string, apis []rpc.API) (net.Listener, *grpc.Server, error) {
	handler := grpc.NewServer()
	for _, api := range apis {
		if qcom.IsNil(api.Service) {
			panic(fmt.Sprintf("%s service is nil", api.Namespace))
		}
		// if service name is MasterServerSideOp so it can match rpc.MasterServerSideOp
		srvsplit := strings.Split(reflect.TypeOf(api.Service).String(), ".")
		svrname := srvsplit[len(srvsplit)-1]
		switch {
		// match MasterServerSideOp
		case strings.HasSuffix(_MasterServerSideOp_serviceDesc.ServiceName, svrname):
			handler.RegisterService(&_MasterServerSideOp_serviceDesc, api.Service)
			// match SlaveServerSideOp
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
