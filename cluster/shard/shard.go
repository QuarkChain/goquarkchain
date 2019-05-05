package shard

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
	"sync"
)

type ShardBackend struct {
	Config            *config.ShardConfig
	fullShardId       uint32
	genesisRootHeight uint32
	maxBlocks         uint32

	chainDb ethdb.Database
	engine  consensus.Engine

	gspec *core.Genesis
	conn  ConnManager

	State        *core.MinorBlockChain
	newBlockPool map[common.Hash]*types.MinorBlock

	bstRHObserved *types.RootBlockHeader  // bestRootHeaderObserved
	bstMHObserved *types.MinorBlockHeader // bestMinorHeaderObserved

	mu sync.Mutex

	eventMux *event.TypeMux
}

func New(ctx *service.ServiceContext, rBlock *types.RootBlock, conn ConnManager,
	cfg *config.ClusterConfig, fullshardId uint32) (*ShardBackend, error) {

	if cfg == nil {
		return nil, errors.New("Failed to create shard, cluster config is nil ")
	}
	var (
		shrd = ShardBackend{
			fullShardId:       fullshardId,
			genesisRootHeight: cfg.Quarkchain.GetShardConfigByFullShardID(fullshardId).Genesis.RootHeight,
			Config:            cfg.Quarkchain.GetShardConfigByFullShardID(fullshardId),
			conn:              conn,
			newBlockPool:      make(map[common.Hash]*types.MinorBlock),
			gspec:             core.NewGenesis(cfg.Quarkchain),
			eventMux:          ctx.EventMux,
		}
		err error
	)
	shrd.maxBlocks = shrd.Config.MaxBlocksPerShardInOneRootBlock()

	shrd.chainDb, err = CreateDB(ctx, fmt.Sprintf("shard-%d.db", fullshardId))
	if err != nil {
		return nil, err
	}

	shrd.engine, err = CreateConsensusEngine(ctx, shrd.Config)
	if err != nil {
		return nil, err
	}

	chainConfig, genesisHash, genesisErr := core.SetupGenesisMinorBlock(shrd.chainDb, shrd.gspec, rBlock, fullshardId)
	if _, ok := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !ok {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	shrd.State, err = core.NewMinorBlockChain(shrd.chainDb, nil, &params.ChainConfig{}, cfg, shrd.engine, vm.Config{}, nil, fullshardId)
	if err != nil {
		return nil, err
	}

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		shrd.State.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(shrd.chainDb, genesisHash, chainConfig)
	}

	return &shrd, nil
}

func (s *ShardBackend) Stop() {
	s.mu.Lock()
	s.mu.Unlock()
	if s != nil {
		s.chainDb.Close()
		s.chainDb.Close()
	}
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

func CreateConsensusEngine(ctx *service.ServiceContext, cfg *config.ShardConfig) (consensus.Engine, error) {

	var (
		diffCalculator = consensus.EthDifficultyCalculator{
			MinimumDifficulty: big.NewInt(int64(cfg.Genesis.Difficulty)),
			AdjustmentCutoff:  cfg.DifficultyAdjustmentCutoffTime,
			AdjustmentFactor:  cfg.DifficultyAdjustmentFactor,
		}
		err error
	)

	switch cfg.ConsensusType {
	case "POW_ETHASH", "POW_SIMULATE":
		return qkchash.New(cfg.ConsensusConfig.RemoteMine, &diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	case "POW_DOUBLESHA256":
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	default:
		err = errors.New(fmt.Sprintf("Failed to create consensus engine consensus type %s", cfg.ConsensusType))
	}
	return nil, err
}

type minorBlockHeaderList []*types.MinorBlockHeader

func (m minorBlockHeaderList) Len() int           { return len(m) }
func (m minorBlockHeaderList) Less(i, j int) bool { return m[i].Number > m[j].Number }
func (m minorBlockHeaderList) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

func (s *ShardBackend) initGenesisState(rootBlock *types.RootBlock) error {
	var (
		minorBlock *types.MinorBlock
		xshardList []*types.CrossShardTransactionDeposit
		status     *rpc.ShardStatus
		err        error
	)
	// TODO InitGenesisState's second parameter will be removed.
	minorBlock, err = s.State.InitGenesisState(rootBlock)
	if err != nil {
		return err
	}

	s.conn.BroadcastXshardTxList(minorBlock, xshardList, rootBlock.Header().Number)
	if status, err = s.State.GetShardStatus(); err != nil {
		return err
	}

	err = s.conn.SendMinorBlockHeaderToMaster(minorBlock.Header(), 1, 0, status)
	return err
}

func (s *ShardBackend) addTxList(txs []*types.Transaction) {
	if txs == nil {
		return
	}
	validTxList := make([]*types.Transaction, 0)
	for _, tx := range txs {
		if s.addTx(tx) {
			validTxList = append(validTxList, tx)
		}
	}

	if len(validTxList) != 0 {
		if err := s.conn.BroadcastTransactions(validTxList, s.fullShardId); err != nil {
			log.Error("add tx list", "failed to broadcast tx list", "err", err)
		}
	}
}

func (s *ShardBackend) addTx(tx *types.Transaction) bool {
	if err := s.State.AddTx(tx); err != nil {
		log.Error("add tx", "failed to add tx", "tx hash", tx.Hash())
		return false
	}
	return true
}
