package config

import (
	"github.com/QuarkChain/goquarkchain/cluster/shard"
)

var (
	DefaultSlaveonfig = SlaveConfig{
		Ip:            HOST,
		Port:          38392,
		Id:            "",
		ShardMaskList: nil,
	}
)

type SlaveConfig struct {
	Ip            string             `json:"IP"`   // DEFAULT_HOST
	Port          uint               `json:"PORT"` // 38392
	Id            string             `json:"ID"`
	ShardMaskList []*shard.ShardMask `json:"SHARD_MASK_LIST"`
}
