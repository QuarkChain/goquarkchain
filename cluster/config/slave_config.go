package config

import (
	"encoding/json"
	"errors"

	"github.com/QuarkChain/goquarkchain/common/hexutil"
)

type SlaveConfig struct {
	IP                       string   `json:"HOST"` // DEFAULT_HOST
	Port                     uint16   `json:"PORT"` // 38392
	ID                       string   `json:"ID"`
	WSPortList               []uint16 `json:"WEBSOCKET_JSON_RPC_PORT_LIST"`
	FullShardList            []uint32 `json:"-"`
	ChainMaskListForBackward []uint32 `json:"-"`
}

type SlaveConfigAlias SlaveConfig

func (s *SlaveConfig) MarshalJSON() ([]byte, error) {
	shardMaskList := make([]hexutil.Uint, len(s.FullShardList))
	for i, m := range s.FullShardList {
		shardMaskList[i] = hexutil.Uint(m)
	}
	jsonConfig := struct {
		SlaveConfigAlias
		ShardMaskList []hexutil.Uint `json:"FULL_SHARD_ID_LIST"`
	}{SlaveConfigAlias(*s), shardMaskList}
	return json.Marshal(jsonConfig)
}

func (s *SlaveConfig) UnmarshalJSON(input []byte) error {
	var jsonConfig struct {
		SlaveConfigAlias
		ChainMaskListJson *[]uint32       `json:"CHAIN_MASK_LIST"`
		FullShardListJson *[]hexutil.Uint `json:"FULL_SHARD_ID_LIST"`
	}
	if err := json.Unmarshal(input, &jsonConfig); err != nil {
		return err
	}
	*s = SlaveConfig(jsonConfig.SlaveConfigAlias)
	if len(s.WSPortList) == 0 {
		for i, shard := range s.FullShardList {
			s.WSPortList[i] = DefaultWSPort + uint16(shard>>16)
		}
	}

	if jsonConfig.ChainMaskListJson != nil && jsonConfig.FullShardListJson != nil {
		return errors.New("Can only have either FULL_SHARD_ID_LIST or CHAIN_MASK_LIST")
	} else if jsonConfig.FullShardListJson != nil {
		s.FullShardList = make([]uint32, len(*jsonConfig.FullShardListJson))
		for k, v := range *jsonConfig.FullShardListJson {
			s.FullShardList[k] = uint32(v)
		}
	} else if jsonConfig.ChainMaskListJson != nil {
		//handle it after call SlaveConfig.UnmarshalJSON
		// can not get ClusterConfig.QuarkChain.Chains config
		s.FullShardList = nil
		s.ChainMaskListForBackward = make([]uint32, len(*jsonConfig.ChainMaskListJson))
		for k, v := range *jsonConfig.ChainMaskListJson {
			s.ChainMaskListForBackward[k] = v
		}
	} else {
		return errors.New("Missing FULL_SHARD_ID_LIST (or CHAIN_MASK_LIST as legacy config)")
	}
	return nil
}

func NewDefaultSlaveConfig() *SlaveConfig {
	slaveConfig := SlaveConfig{
		IP:   DefaultHost,
		Port: slavePort,
	}
	return &slaveConfig
}
