package main

import "github.com/QuarkChain/goquarkchain/cluster/config"

type genConfigParams struct {
	CfgFile           string  `json:"config"`
	ChainSize         *uint64 `json:"chainSize"`
	ShardSizePerChain *uint64 `json:"shardSizePerChain"`
	NumSlaves         *uint64 `json:"numSlaves"`
	SlaveIpList       string  `json:"slaveIpList"` //"etc: ip,ip,ip"
}

func (g *genConfigParams) SetDefault() {
	if g.CfgFile == "" {
		g.CfgFile = "./cluster_config_template.json"
	}

	if g.ChainSize == nil {
		t := uint64(1)
		g.ChainSize = &t
	}
	if g.ShardSizePerChain == nil {
		t := uint64(1)
		g.ShardSizePerChain = &t
	}
	if g.NumSlaves == nil {
		t := uint64(config.DefaultNumSlaves)
		g.NumSlaves = &t
	}
	if g.SlaveIpList == "" {
		g.SlaveIpList = defaultIp
	}
}
