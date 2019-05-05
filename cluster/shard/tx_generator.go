package shard

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core"
)

type TransactionGenerator struct {
	qkcCfg      *config.QuarkChainConfig
	fullShardId uint32
	state       *core.MinorBlockChain
	accounts    []*account.Account
}

func NewTransactionGenerator(qkcCfg *config.QuarkChainConfig, shrd *ShardBackend) *TransactionGenerator {
	gen := &TransactionGenerator{
		qkcCfg:      qkcCfg,
		fullShardId: shrd.fullShardId,
		state:       shrd.State,
	}
	// TODO need to fill content.
	return gen
}
