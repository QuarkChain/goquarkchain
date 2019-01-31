package config

import (
	"github.com/QuarkChain/goquarkchain/cluster/shard"
)

type SlaveConfig struct {
	Ip            string             `json:"IP"`   // DEFAULT_HOST
	Port          uint64             `json:"PORT"` // 38392
	Id            string             `json:"ID"`
	ShardMaskList []*shard.ShardMask `json:"SHARD_MASK_LIST"`
}

func NewSlaveConfig() *SlaveConfig {
	return &SlaveConfig{
		Ip:            HOST,
		Port:          SLAVE_PORT,
		Id:            "",
		ShardMaskList: nil,
	}
}
