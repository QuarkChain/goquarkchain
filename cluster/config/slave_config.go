package config

import (
	"github.com/QuarkChain/goquarkchain/core/types"
)

type SlaveConfig struct {
	IP            string             `json:"IP"`   // DEFAULT_HOST
	Port          uint64             `json:"PORT"` // 38392
	ID            string             `json:"ID"`
	ShardMaskList []*types.ChainMask `json:"-"`
}

func NewDefaultSlaveConfig() *SlaveConfig {
	slaveConfig := SlaveConfig{
		IP:   "127.0.0.1",
		Port: uint64(SlavePort),
	}
	return &slaveConfig
}
