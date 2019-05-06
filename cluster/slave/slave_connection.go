package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
)

type ToSlaveConnection struct {
	target       string
	shardMaskLst []*types.ChainMask
	client       rpc.Client
}

//func NewToSlaveConnection(target string, shardMaskLst []*types.ChainMask) (*ToSlaveConnection, error) {
//	return &ToSlaveConnection{
//		target:       target,
//		client:       rpc.NewClient(rpc.MasterServer),
//		shardMaskLst: shardMaskLst,
//	}, nil
//}
