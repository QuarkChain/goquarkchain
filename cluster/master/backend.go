package master

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"os"
	"reflect"
	"sync"
)

type MasterBackend struct {
	lock       sync.RWMutex
	config     *config.RootConfig
	APIBackend *QkcAPIBackend

	engine    consensus.Engine
	rootblock *core.RootBlockChain

	chainDb ethdb.Database

	eventMux *event.TypeMux
	shutdown chan os.Signal
}

func New(ctx *service.ServiceContext, cfg *config.ClusterConfig) (*MasterBackend, error) {
	var (
		mstr = &MasterBackend{
			config:   cfg.Quarkchain.Root,
			eventMux: ctx.EventMux,
			shutdown: ctx.Shutdown,
		}
		err error
	)
	if mstr.chainDb, err = CreateDB(ctx, cfg.DbPathRoot); err != nil {
		goto FALSE
	}

	if mstr.APIBackend, err = NewQkcAPIBackend(mstr, cfg.SlaveList); err != nil {
		goto FALSE
	}

	if mstr.engine, err = CreateConsensusEngine(ctx, mstr.config); err != nil {
		goto FALSE
	}

	if mstr.rootblock, err = core.NewRootBlockChain(mstr.chainDb, nil, cfg.Quarkchain, mstr.engine, mstr.isLocalBlock);
		err != nil {
		goto FALSE
	}

	return mstr, nil
FALSE:
	return nil, err
}

func CreateDB(ctx *service.ServiceContext, name string) (ethdb.Database, error) {
	db, err := ctx.OpenDatabase(name)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func CreateConsensusEngine(ctx *service.ServiceContext, cfg *config.RootConfig) (consensus.Engine, error) {

	diffCalculator := consensus.EthDifficultyCalculator{
		MinimumDifficulty: big.NewInt(int64(cfg.Genesis.Difficulty)),
		AdjustmentCutoff:  cfg.DifficultyAdjustmentCutoffTime,
		AdjustmentFactor:  cfg.DifficultyAdjustmentFactor,
	}

	switch cfg.ConsensusType {
	case "POW_ETHASH", "POW_SIMULATE":
		return qkchash.New(cfg.ConsensusConfig.RemoteMine, &diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	case "POW_DOUBLESHA256":
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	}
	return nil, errors.New(fmt.Sprintf("Failed to create consensus engine consensus type %s", cfg.ConsensusType))
}

func (c *MasterBackend) isLocalBlock(block *types.RootBlock) bool {
	return false
}

func (c *MasterBackend) Protocols() (protos []p2p.Protocol) { return nil }

func (c *MasterBackend) APIs() []rpc.API {
	apis := qkcapi.GetAPIs(c.APIBackend)
	return append(apis, []rpc.API{
		{
			Namespace: "rpc." + reflect.TypeOf(MasterServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(c),
			Public:    false,
		},
	}...)
}

func (c *MasterBackend) Stop() error {
	if c.engine != nil {
		c.engine.Close()
	}
	c.eventMux.Stop()
	return nil
}

func (c *MasterBackend) Start(srvr *p2p.Server) error {
	// start heart beat pre 3 seconds.
	c.APIBackend.HeartBeat()
	c.APIBackend.SendConnectToSlaves()
	return nil
}

func (c *MasterBackend) StartMing(threads int) error {
	return nil
}
