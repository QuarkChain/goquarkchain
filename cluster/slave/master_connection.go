package slave

import "github.com/QuarkChain/goquarkchain/cluster/rpc"

type MasterConnection struct {
	target string
}

func NewMasterConnection() (*MasterConnection, error) {
	return &MasterConnection{
	}, nil
}

func (m *MasterConnection) SendMinorBlockHeaderToMaster() *rpc.ArtificialTxConfig {
	return &rpc.ArtificialTxConfig{}
}
