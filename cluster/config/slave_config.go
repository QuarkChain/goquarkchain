package config

import (
	"encoding/json"

	"github.com/QuarkChain/goquarkchain/core/types"
)

type SlaveConfig struct {
	IP            string             `json:"IP"`   // DEFAULT_HOST
	Port          uint64             `json:"PORT"` // 38392
	ID            string             `json:"ID"`
	ShardMaskList []*types.ChainMask `json:"-"`
}

type SlaveConfigAlias SlaveConfig

func (s *SlaveConfig) MarshalJSON() ([]byte, error) {
	shardMaskList := make([]uint32, len(s.ShardMaskList))
	for i, m := range s.ShardMaskList {
		shardMaskList[i] = m.GetMask()
	}
	jsonConfig := struct {
		SlaveConfigAlias
		ShardMaskList []uint32 `json:"SHARD_MASK_LIST"`
	}{SlaveConfigAlias(*s), shardMaskList}
	return json.Marshal(jsonConfig)
}

func (s *SlaveConfig) UnmarshalJSON(input []byte) error {
	var jsonConfig struct {
		SlaveConfigAlias
		ShardMaskList []uint32 `json:"SHARD_MASK_LIST"`
	}
	if err := json.Unmarshal(input, &jsonConfig); err != nil {
		return err
	}
	*s = SlaveConfig(jsonConfig.SlaveConfigAlias)
	s.ShardMaskList = make([]*types.ChainMask, len(jsonConfig.ShardMaskList))
	for i, value := range jsonConfig.ShardMaskList {
		s.ShardMaskList[i] = types.NewChainMask(value)
	}
	return nil
}

func NewDefaultSlaveConfig() *SlaveConfig {
	slaveConfig := SlaveConfig{
		IP:   "127.0.0.1",
		Port: uint64(SlavePort),
	}
	return &slaveConfig
}
