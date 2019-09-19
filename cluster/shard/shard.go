package shard

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"math/big"
	"sync"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/miner"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	synchronizer "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/ethash"
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
)

type BlockCommitCode int

const (
	BLOCK_UNCOMMITTED BlockCommitCode = iota
	BLOCK_COMMITTING                  // TODO not support yet,need discuss
	BLOCK_COMMITTED
)

type ShardBackend struct {
	Config            *config.ShardConfig
	branch            account.Branch
	genesisRootHeight uint32
	maxBlocks         uint32

	chainDb ethdb.Database
	engine  consensus.Engine

	gspec *core.Genesis
	conn  ConnManager

	miner           *miner.Miner
	MinorBlockChain *core.MinorBlockChain

	mBPool      newBlockPool
	txGenerator *TxGenerator

	running      bool
	mu           sync.Mutex
	eventMux     *event.TypeMux
	synchronizer synchronizer.Synchronizer
	logInfo      string

	posw consensus.PoSWCalculator
}

func New(ctx *service.ServiceContext, rBlock *types.RootBlock, conn ConnManager,
	cfg *config.ClusterConfig, fullshardId uint32) (*ShardBackend, error) {

	if cfg == nil {
		return nil, errors.New("Failed to create shard, cluster config is nil ")
	}
	var (
		shard = &ShardBackend{
			branch:            account.Branch{Value: fullshardId},
			genesisRootHeight: cfg.Quarkchain.GetShardConfigByFullShardID(fullshardId).Genesis.RootHeight,
			Config:            cfg.Quarkchain.GetShardConfigByFullShardID(fullshardId),
			conn:              conn,
			mBPool:            newBlockPool{BlockPool: make(map[common.Hash]*types.MinorBlockHeader)},
			gspec:             core.NewGenesis(cfg.Quarkchain),
			eventMux:          ctx.EventMux,
			logInfo:           fmt.Sprintf("shard:%d", fullshardId),
			running:           true,
		}
		err error
	)
	shard.maxBlocks = shard.Config.MaxBlocksPerShardInOneRootBlock()

	shard.chainDb, err = createDB(ctx, fmt.Sprintf("shard-%d.db", fullshardId), cfg.Clean, cfg.CheckDB)
	if err != nil {
		return nil, err
	}

	shard.txGenerator = NewTxGenerator(cfg.GenesisDir, shard.branch.Value, cfg.Quarkchain)

	shard.engine, err = createConsensusEngine(cfg.Quarkchain.EnableQkcHashXHeight, shard.Config)
	if err != nil {
		shard.chainDb.Close()
		return nil, err
	}

	chainConfig, genesisHash, genesisErr := core.SetupGenesisMinorBlock(shard.chainDb, shard.gspec, rBlock, fullshardId)
	// TODO check config err
	if genesisErr != nil {
		log.Info("Fill in block into chain db.")
		rawdb.WriteChainConfig(shard.chainDb, genesisHash, cfg.Quarkchain)
	}
	log.Debug("Initialised chain configuration", "config", chainConfig)

	shard.MinorBlockChain, err = core.NewMinorBlockChain(shard.chainDb, nil, &params.ChainConfig{}, cfg, shard.engine, vm.Config{}, nil, fullshardId)
	if err != nil {
		shard.chainDb.Close()
		return nil, err
	}
	shard.MinorBlockChain.SetBroadcastMinorBlockFunc(shard.AddMinorBlock)
	shard.synchronizer = synchronizer.NewSynchronizer(shard.MinorBlockChain)
	shard.posw = consensus.CreatePoSWCalculator(shard.MinorBlockChain, shard.Config.PoswConfig)

	shard.miner = miner.New(ctx, shard, shard.engine)

	return shard, nil
}

func (s *ShardBackend) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return
	}
	s.running = false
	s.synchronizer.Close()
	s.miner.Stop()
	s.eventMux.Stop()
	s.engine.Close()
	s.MinorBlockChain.Stop()
	s.chainDb.Close()
}

func (s *ShardBackend) SetMining(mining bool) {
	s.miner.SetMining(mining)
}

func createDB(ctx *service.ServiceContext, name string, clean bool, isReadOnly bool) (ethdb.Database, error) {
	// handlers and caches size should be set in different environment.
	db, err := ctx.OpenDatabase(name, clean, isReadOnly)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func createConsensusEngine(qkcHashXHeight uint64, cfg *config.ShardConfig) (consensus.Engine, error) {
	difficulty := new(big.Int)
	diffCalculator := consensus.EthDifficultyCalculator{
		MinimumDifficulty: difficulty.SetUint64(cfg.Genesis.Difficulty),
		AdjustmentCutoff:  cfg.DifficultyAdjustmentCutoffTime,
		AdjustmentFactor:  cfg.DifficultyAdjustmentFactor,
	}
	pubKey := []byte{}
	switch cfg.ConsensusType {
	case config.PoWSimulate: //TODO pow_simulate is fake
		return &consensus.FakeEngine{}, nil
	case config.PoWEthash:
		return ethash.New(ethash.Config{CachesInMem: 3, CachesOnDisk: 10, CacheDir: "", PowMode: ethash.ModeNormal}, &diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	case config.PoWQkchash:
		return qkchash.New(true, &diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey, qkcHashXHeight), nil
	case config.PoWDoubleSha256:
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	}
	return nil, fmt.Errorf("Failed to create consensus engine consensus type %s ", cfg.ConsensusType)
}

func (s *ShardBackend) initGenesisState(rootBlock *types.RootBlock) error {
	var (
		minorBlock *types.MinorBlock
		xshardList = make([]*types.CrossShardTransactionDeposit, 0)
		status     *rpc.ShardStatus
		err        error
	)
	minorBlock, err = s.MinorBlockChain.InitGenesisState(rootBlock)
	if err != nil {
		return err
	}

	if err = s.conn.BroadcastXshardTxList(minorBlock, xshardList, rootBlock.Header().Number); err != nil {
		return err
	}
	if status, err = s.MinorBlockChain.GetShardStats(); err != nil {
		return err
	}
	request := &rpc.AddMinorBlockHeaderRequest{
		MinorBlockHeader:  minorBlock.Header(),
		TxCount:           uint32(len(minorBlock.GetTransactions())),
		XShardTxCount:     uint32(len(xshardList)),
		ShardStats:        status,
		CoinbaseAmountMap: minorBlock.Header().CoinbaseAmount,
	}
	return s.conn.SendMinorBlockHeaderToMaster(request)
}

func (s *ShardBackend) getBlockCommitStatusByHash(blockHash common.Hash) BlockCommitCode {
	// If the block is committed, it means
	// - All neighbor shards/slaves receives x-shard tx list
	// - The block header is sent to master
	// then return immediately
	if s.MinorBlockChain.IsMinorBlockCommittedByHash(blockHash) {
		return BLOCK_COMMITTED
	}

	//TODO support BLOCK_COMMITTING???

	// Check if the block is being propagating to other slaves and the master
	// Let's make sure all the shards and master got it before committing it
	return BLOCK_UNCOMMITTED
}

// minor block pool
type newBlockPool struct {
	Mu        sync.RWMutex
	BlockPool map[common.Hash]*types.MinorBlockHeader
}

func (n *newBlockPool) getBlockInPool(hash common.Hash) *types.MinorBlockHeader {
	n.Mu.RLock()
	defer n.Mu.RUnlock()
	return n.BlockPool[hash]
}

func (n *newBlockPool) setBlockInPool(header *types.MinorBlockHeader) {
	n.Mu.Lock()
	defer n.Mu.Unlock()
	n.BlockPool[header.Hash()] = header
}

func (n *newBlockPool) delBlockInPool(header *types.MinorBlockHeader) {
	n.Mu.Lock()
	defer n.Mu.Unlock()
	delete(n.BlockPool, header.Hash())
}
