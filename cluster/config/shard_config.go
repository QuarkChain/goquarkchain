package config

import (
	"encoding/json"
	"github.com/QuarkChain/goquarkchain/account"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type ShardGenesis struct {
	RootHeight         uint32                       `json:"ROOT_HEIGHT"`
	Version            uint32                       `json:"VERSION"`
	Height             uint64                       `json:"HEIGHT"`
	HashPrevMinorBlock string                       `json:"HASH_PREV_MINOR_BLOCK"`
	HashMerkleRoot     string                       `json:"HASH_MERKLE_ROOT"`
	ExtraData          []byte                       `json:"-"`
	Timestamp          uint64                       `json:"TIMESTAMP"`
	Difficulty         uint64                       `json:"DIFFICULTY"`
	GasLimit           uint64                       `json:"GAS_LIMIT"`
	Nonce              uint32                       `json:"NONCE"`
	Alloc              map[account.Address]*big.Int `json:"-"`
}

func NewShardGenesis() *ShardGenesis {
	return &ShardGenesis{
		RootHeight:         0,
		Version:            0,
		Height:             0,
		HashPrevMinorBlock: "",
		HashMerkleRoot:     "",
		ExtraData:          common.FromHex("497420776173207468652062657374206f662074696d65732c206974207761732074686520776f727374206f662074696d65732c202e2e2e202d20436861726c6573204469636b656e73"),
		Timestamp:          NewRootGenesis().Timestamp,
		Difficulty:         10000,
		GasLimit:           30000 * 400,
		Nonce:              0,
		Alloc:              make(map[account.Address]*big.Int),
	}
}

type ShardGenesisAlias ShardGenesis

func (s *ShardGenesis) MarshalJSON() ([]byte, error) {
	var alloc = make(map[string]*big.Int)
	for addr, val := range s.Alloc {
		alloc[string(addr.ToHex())] = val
	}
	jsonConfig := struct {
		ShardGenesisAlias
		ExtraData string              `json:"EXTRA_DATA"`
		Alloc     map[string]*big.Int `json:"ALLOC"`
	}{ShardGenesisAlias(*s), common.Bytes2Hex(s.ExtraData), alloc}
	return json.Marshal(jsonConfig)
}

func (s *ShardGenesis) UnmarshalJSON(input []byte) error {
	var jsonConfig struct {
		ShardGenesisAlias
		ExtraData string              `json:"EXTRA_DATA"`
		Alloc     map[string]*big.Int `json:"ALLOC"`
	}
	if err := json.Unmarshal(input, &jsonConfig); err != nil {
		return err
	}
	*s = ShardGenesis(jsonConfig.ShardGenesisAlias)
	s.ExtraData = common.Hex2Bytes(jsonConfig.ExtraData)
	s.Alloc = make(map[account.Address]*big.Int)
	for addr, val := range jsonConfig.Alloc {
		address, err := account.CreatAddressFromBytes(common.FromHex(addr))
		if err != nil {
			return err
		}
		s.Alloc[address] = val
	}
	return nil
}

type ShardConfig struct {
	ShardID    uint32
	rootConfig *RootConfig
	*ChainConfig
}

func NewShardConfig(chainCfg *ChainConfig) *ShardConfig {
	var cfg = new(ChainConfig)
	_ = qcom.DeepCopy(cfg, chainCfg)
	shardConfig := &ShardConfig{
		ShardID:     0,
		ChainConfig: cfg,
	}
	return shardConfig
}

func (s *ShardConfig) SetRootConfig(value *RootConfig) {
	s.rootConfig = value
}

func (s *ShardConfig) GetRootConfig() *RootConfig {
	return s.rootConfig
}

func (s *ShardConfig) MaxBlocksPerShardInOneRootBlock() uint32 {
	return s.rootConfig.ConsensusConfig.TargetBlockTime/
		s.ConsensusConfig.TargetBlockTime + s.ExtraShardBlocksInRootBlock
}

func (s *ShardConfig) MaxStaleMinorBlockHeightDiff() uint64 {
	return s.rootConfig.MaxStaleRootBlockHeightDiff *
		uint64(s.rootConfig.ConsensusConfig.TargetBlockTime) /
		uint64(s.ConsensusConfig.TargetBlockTime)
}

func (s *ShardConfig) MaxMinorBlocksInMemory() uint64 {
	return s.MaxStaleMinorBlockHeightDiff() * 2
}

func (s *ShardConfig) GetFullShardId() uint32 {
	return (s.ChainID << 16) | s.ShardSize | s.ShardID
}
