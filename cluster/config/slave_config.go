package config

import "github.com/QuarkChain/goquarkchain/core/types"

type SlaveConfig struct {
	Ip            string             `json:"IP"`   // DEFAULT_HOST
	Port          uint64             `json:"PORT"` // 38392
	Id            string             `json:"ID"`
	ShardMaskList []*types.ChainMask `json:"SHARD_MASK_LIST"`
}

func NewSlaveConfig() *SlaveConfig {
	slaveConfig := SlaveConfig{
		Ip:            HOST,
		Port:          SLAVE_PORT,
		Id:            "",
		ShardMaskList: nil,
	}
	return &slaveConfig
}
