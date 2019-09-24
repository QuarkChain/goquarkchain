package qkcapi

import (
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"sync"
)

var (
	EmptyTxID = IDEncoder(common.Hash{}.Bytes(), 0)

	once       sync.Once
	clusterCfg *config.ClusterConfig
	DefaultTokenID = "QKC"
)

func getFullShardId(fullShardKey *hexutil.Uint) (fullShardId uint32, err error) {
	if fullShardKey != nil {
		fullShardId, err = clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(uint32(*fullShardKey))
		if err != nil {
			return
		}
	}
	return 0, nil
}
