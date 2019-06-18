package main

import "github.com/QuarkChain/goquarkchain/cluster/config"

type genConfigParams struct {
	CfgFile           string  `json:"config"`
	Difficulty        *uint64 `json:"difficulty"`
	ChainSize         *uint64 `json:"chainSize"`
	ShardSizePerChain *uint64 `json:"shardSizePerChain"`
	RootBlockTime     *uint64 `json:"rootBlockTime"`
	MinorBlockTime    *uint64 `json:"minorBlockTime"`
	CoinBaseAddress   string  `json:"coinBaseAddress"`
	NumSlaves         *uint64 `json:"numSlaves"`
	SlaveIpList       string  `json:"slaveIpList"` //"etc: ip,ip,ip"
	Privatekey        string  `json:"privatekey"`
	ConsensusType     string  `json:"consensusType"`
	NetworkId         *uint64 `json:"networkId"`
	GenesisDir        string  `json:"genesisDir"`
}

func (g *genConfigParams) SetDefault() {
	if g.CfgFile == "" {
		g.CfgFile = "./cluster_config_template.json"
	}
	if g.Difficulty == nil {
		t := uint64(1000000)
		g.Difficulty = &t
	}
	if g.ChainSize == nil {
		t := uint64(1)
		g.ChainSize = &t
	}
	if g.ShardSizePerChain == nil {
		t := uint64(1)
		g.ShardSizePerChain = &t
	}
	if g.RootBlockTime == nil {
		t := uint64(30)
		g.RootBlockTime = &t
	}
	if g.MinorBlockTime == nil {
		t := uint64(10)
		g.MinorBlockTime = &t
	}
	if g.CoinBaseAddress == "" {
		g.CoinBaseAddress = "0xb067ac9ebeeecb10bbcd1088317959d58d1e38f6b0ee10d5"
	}
	if g.NumSlaves == nil {
		t := uint64(config.DefaultNumSlaves)
		g.NumSlaves = &t
	}
	if g.SlaveIpList == "" {
		g.SlaveIpList = defaultIp
	}
	if g.Privatekey == "" {
		g.Privatekey = defaultPriv
	}
	if g.ConsensusType == "" {
		g.ConsensusType = "POW_DOUBLESHA256"
	}
	if g.NetworkId == nil {
		t := uint64(3)
		g.NetworkId = &t
	}
}
