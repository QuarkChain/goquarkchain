package slave

import (
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/core/types"
)

type MasterConnection struct {
	target string
}

func NewMasterConnection(ctx *service.ServiceContext) (*MasterConnection, error) {
	return &MasterConnection{
	}, nil
}

func (m *MasterConnection) SendMinorBlockHeaderToMaster() *rpc.ArtificialTxConfig {
	return &rpc.ArtificialTxConfig{}
}

func (m *MasterConnection) sendMinorBlockHeader(header types.MinorBlockHeader, txCount int, shardState *rpc.ShardStats) bool {
	return false
}
