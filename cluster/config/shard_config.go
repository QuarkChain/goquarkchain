package config

import (
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
)

type ShardGenesis struct {
	RootHeight         uint64              `json:"ROOT_HEIGHT"`
	Version            uint32              `json:"VERSION"`
	Height             uint64              `json:"HEIGHT"`
	HashPrevMinorBlock string              `json:"HASH_PREV_MINOR_BLOCK"`
	HashMerkleRoot     string              `json:"HASH_MERKLE_ROOT"`
	ExtraData          []byte              `json:"EXTRA_DATA"`
	Timestamp          uint64              `json:"TIMESTAMP"`
	Difficulty         uint64              `json:"DIFFICULTY"`
	GasLimit           uint64              `json:"GAS_LIMIT"`
	Nonce              uint32              `json:"NONCE"`
	Alloc              map[string]*big.Int `json:"ALLOC"`
}

func NewShardGenesis() *ShardGenesis {
	return &ShardGenesis{
		RootHeight:         0,
		Version:            0,
		Height:             0,
		HashPrevMinorBlock: "",
		HashMerkleRoot:     "",
		ExtraData:          []byte("It was the best of times, it was the worst of times, ... - Charles Dickens"),
		Timestamp:          DefaultRootGenesis.Timestamp,
		Difficulty:         10000,
		GasLimit:           30000 * 400,
		Nonce:              0,
		Alloc:              make(map[string]*big.Int),
	}
}

type ShardConfig struct {
	ShardID    uint32
	rootConfig *RootConfig
	*ChainConfig
}

func NewShardConfig(chainCfg *ChainConfig) *ShardConfig {

	shardConfig := &ShardConfig{
		ShardID:     0,
		ChainConfig: new(ChainConfig),
	}
	if err := common.DeepCopy(shardConfig.ChainConfig, chainCfg); err != nil {
		log.Error("shard config deep copy", "error", err)
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
