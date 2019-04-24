package shard

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"sync"
)

type ShardBackend struct {
	lock        sync.RWMutex
	config      *config.ShardConfig
	fullShardId uint32

	chainDb ethdb.Database
	engine  consensus.Engine

	shardState *types.MinorBlock

	eventMux *event.TypeMux
}

func NewShard(ctx *service.ServiceContext, cfg *config.ShardConfig) (*ShardBackend, error) {

	chainDb, err := CreateDB(ctx, fmt.Sprintf("shard-%d.db", cfg.ShardID))
	if err != nil {
		return nil, err
	}

	// TODO add shard block create func

	return &ShardBackend{
		fullShardId: cfg.GetFullShardId(),
		config:      cfg,
		chainDb:     chainDb,
		engine:      CreateConsensusEngine(ctx, cfg),
		eventMux:    ctx.EventMux,
	}, nil
}

func (s *ShardBackend) StartMining() bool {
	return false
}

func (s *ShardBackend) StopMining() {}

func CreateDB(ctx *service.ServiceContext, name string) (ethdb.Database, error) {
	db, err := ctx.OpenDatabase(name)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CreateConsensusEngine(ctx *service.ServiceContext, cfg *config.ShardConfig) consensus.Engine {

	diffCalculator := consensus.EthDifficultyCalculator{
		MinimumDifficulty: big.NewInt(int64(cfg.Genesis.Difficulty)),
		AdjustmentCutoff:  cfg.DifficultyAdjustmentCutoffTime,
		AdjustmentFactor:  cfg.DifficultyAdjustmentFactor,
	}

	switch cfg.ConsensusType {
	case "POW_ETHASH":
		return qkchash.New(cfg.ConsensusConfig.RemoteMine, &diffCalculator, cfg.ConsensusConfig.RemoteMine)
	case "POW_DOUBLESHA256":
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine)
	default:
		log.Error("Failed to create consensus engine", "consensus type", cfg.ConsensusType)
	}
	return nil
}
