package config

import (
	"bytes"
	"encoding/json"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

type ShardGenesis struct {
	RootHeight         uint32                       `json:"ROOT_HEIGHT"`
	Version            uint32                       `json:"VERSION"`
	Height             uint64                       `json:"HEIGHT"`
	HashPrevMinorBlock common.Hash                  `json:"HASH_PREV_MINOR_BLOCK"`
	HashMerkleRoot     common.Hash                  `json:"HASH_MERKLE_ROOT"`
	ExtraData          []byte                       `json:"EXTRA_DATA"`
	Timestamp          uint64                       `json:"TIMESTAMP"`
	Difficulty         uint64                       `json:"DIFFICULTY"`
	GasLimit           uint64                       `json:"GAS_LIMIT"`
	Nonce              uint32                       `json:"NONCE"`
	Alloc              map[account.Address]*big.Int `json:"ALLOC"`
}

func NewShardGenesis() *ShardGenesis {
	return &ShardGenesis{
		RootHeight:         0,
		Version:            0,
		Height:             0,
		HashPrevMinorBlock: common.Hash{},
		HashMerkleRoot:     common.Hash{},
		ExtraData:          []byte("It was the best of times, it was the worst of times, ... - Charles Dickens"),
		Timestamp:          NewRootGenesis().Timestamp,
		Difficulty:         10000,
		GasLimit:           30000 * 400,
		Nonce:              0,
		Alloc:              make(map[account.Address]*big.Int),
	}
}

func (s *ShardGenesis) MarshalJSON() ([]byte, error) {
	type ShardGenesis struct {
		RootHeight         uint32              `json:"ROOT_HEIGHT"`
		Version            uint32              `json:"VERSION"`
		Height             uint64              `json:"HEIGHT"`
		HashPrevMinorBlock string              `json:"HASH_PREV_MINOR_BLOCK"`
		HashMerkleRoot     string              `json:"HASH_MERKLE_ROOT"`
		ExtraData          string              `json:"EXTRA_DATA"`
		Timestamp          uint64              `json:"TIMESTAMP"`
		Difficulty         uint64              `json:"DIFFICULTY"`
		GasLimit           uint64              `json:"GAS_LIMIT"`
		Nonce              uint32              `json:"NONCE"`
		Alloc              map[string]*big.Int `json:"ALLOC"`
	}

	var enc ShardGenesis
	enc.RootHeight = s.RootHeight
	enc.Version = s.Version
	enc.Height = s.Height
	if bytes.Equal(s.HashPrevMinorBlock.Bytes(), common.Hash{}.Bytes()) {
		enc.HashPrevMinorBlock = ""
	} else {
		enc.HashPrevMinorBlock = s.HashPrevMinorBlock.String()[2:]
	}

	if bytes.Equal(s.HashMerkleRoot.Bytes(), common.Hash{}.Bytes()) {
		enc.HashMerkleRoot = ""
	} else {
		enc.HashMerkleRoot = s.HashMerkleRoot.String()[2:]
	}

	enc.ExtraData = common.Bytes2Hex(s.ExtraData)
	enc.Timestamp = s.Timestamp
	enc.Difficulty = s.Difficulty
	enc.GasLimit = s.GasLimit
	enc.Nonce = s.Nonce
	if s.Alloc != nil {
		enc.Alloc = make(map[string]*big.Int, len(s.Alloc))
		for k, v := range s.Alloc {
			enc.Alloc[k.UnprefixedAddress().Address().ToHex()[2:]] = v
		}
	}
	return json.Marshal(&enc)
}

func (s *ShardGenesis) UnmarshalJSON(input []byte) error {
	type ShardGenesis struct {
		RootHeight         uint32                                 `json:"ROOT_HEIGHT"`
		Version            uint32                                 `json:"VERSION"`
		Height             uint64                                 `json:"HEIGHT"`
		HashPrevMinorBlock string                                 `json:"HASH_PREV_MINOR_BLOCK"`
		HashMerkleRoot     string                                 `json:"HASH_MERKLE_ROOT"`
		ExtraData          string                                 `json:"EXTRA_DATA"`
		Timestamp          uint64                                 `json:"TIMESTAMP"`
		Difficulty         uint64                                 `json:"DIFFICULTY"`
		GasLimit           uint64                                 `json:"GAS_LIMIT"`
		Nonce              uint32                                 `json:"NONCE"`
		Alloc              map[account.UnprefixedAddress]*big.Int `json:"ALLOC"`
	}
	var dec ShardGenesis
	var err error
	if err = json.Unmarshal(input, &dec); err != nil {
		return err
	}
	s.RootHeight = uint32(dec.RootHeight)
	s.Version = uint32(dec.Version)
	s.Height = uint64(dec.Height)
	s.HashPrevMinorBlock = common.HexToHash(dec.HashPrevMinorBlock)
	s.HashMerkleRoot = common.HexToHash(dec.HashMerkleRoot)
	s.ExtraData = common.Hex2Bytes(dec.ExtraData)

	s.Timestamp = uint64(dec.Timestamp)
	s.Difficulty = uint64(dec.Difficulty)
	s.GasLimit = uint64(dec.GasLimit)
	s.Height = uint64(dec.Nonce)

	if dec.Alloc != nil {
		s.Alloc = make(map[account.Address]*big.Int, len(dec.Alloc))
		for k, v := range dec.Alloc {
			s.Alloc[k.Address()] = v

		}
	}
	return nil
}

type ShardConfig struct {
	ShardID    uint32
	rootConfig *RootConfig
	*ChainConfig
}

func NewShardConfig(chainCfg *ChainConfig) *ShardConfig {

	shardConfig := &ShardConfig{
		ShardID:     0,
		ChainConfig: chainCfg,
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
