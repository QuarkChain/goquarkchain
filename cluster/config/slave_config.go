package config

import (
	"encoding/json"

	"github.com/QuarkChain/goquarkchain/core/types"
)

type SlaveConfig struct {
	IP            string             `json:"HOST"` // DEFAULT_HOST
	Port          int                `json:"PORT"` // 38392
	ID            string             `json:"ID"`
	ChainMaskList []*types.ChainMask `json:"-"`
}

type SlaveConfigAlias SlaveConfig

func (s *SlaveConfig) MarshalJSON() ([]byte, error) {
	shardMaskList := make([]uint32, len(s.ChainMaskList))
	for i, m := range s.ChainMaskList {
		shardMaskList[i] = m.GetMask()
	}
	jsonConfig := struct {
		SlaveConfigAlias
		ShardMaskList []uint32 `json:"CHAIN_MASK_LIST"`
	}{SlaveConfigAlias(*s), shardMaskList}
	return json.Marshal(jsonConfig)
}

func (s *SlaveConfig) UnmarshalJSON(input []byte) error {
	var jsonConfig struct {
		SlaveConfigAlias
		ChainMaskList []uint32 `json:"CHAIN_MASK_LIST"`
	}
	if err := json.Unmarshal(input, &jsonConfig); err != nil {
		return err
	}
	*s = SlaveConfig(jsonConfig.SlaveConfigAlias)
	s.ChainMaskList = make([]*types.ChainMask, len(jsonConfig.ChainMaskList))
	for i, value := range jsonConfig.ChainMaskList {
		s.ChainMaskList[i] = types.NewChainMask(value)
	}
	return nil
}

func NewDefaultSlaveConfig() *SlaveConfig {
	slaveConfig := SlaveConfig{
		IP:   "127.0.0.1",
		Port: slavePort,
	}
	return &slaveConfig
}
