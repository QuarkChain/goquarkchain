package config

import (
	"encoding/json"
	"github.com/QuarkChain/goquarkchain/core/types"
)

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
		ShardMaskList: make([]*types.ChainMask, 0),
	}
	return &slaveConfig
}

func (s SlaveConfig) MarshalJSON() ([]byte, error) {
	type SlaveConfig struct {
		Ip            string   `json:"IP"`   // DEFAULT_HOST
		Port          uint64   `json:"PORT"` // 38392
		Id            string   `json:"ID"`
		ShardMaskList []uint32 `json:"SHARD_MASK_LIST"`
	}
	var enc = SlaveConfig{
		Ip:            s.Ip,
		Port:          s.Port,
		Id:            s.Id,
		ShardMaskList: make([]uint32, len(s.ShardMaskList)),
	}
	for i, mask := range s.ShardMaskList {
		enc.ShardMaskList[i] = mask.GetMask()
	}
	return json.Marshal(&enc)
}

func (s *SlaveConfig) UnmarshalJSON(input []byte) error {
	type SlaveConfig struct {
		Ip            string   `json:"IP"`   // DEFAULT_HOST
		Port          uint64   `json:"PORT"` // 38392
		Id            string   `json:"ID"`
		ShardMaskList []uint32 `json:"SHARD_MASK_LIST"`
	}
	var dec SlaveConfig
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	s.Ip = dec.Ip
	s.Port = dec.Port
	s.Id = dec.Id
	s.ShardMaskList = make([]*types.ChainMask, len(dec.ShardMaskList))
	for i, value := range dec.ShardMaskList {
		s.ShardMaskList[i] = types.NewChainMask(value)
	}
	return nil
}
