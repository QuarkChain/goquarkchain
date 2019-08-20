package master

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/miner"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	Synchronizer "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/ethash"
	"github.com/QuarkChain/goquarkchain/consensus/gmhash"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	ethRPC "github.com/ethereum/go-ethereum/rpc"
	"github.com/shirou/gopsutil/cpu"
	"golang.org/x/sync/errgroup"
	"gopkg.in/karalabe/cookiejar.v1/collections/deque"
	"math/big"
	"net"
	"os"
	"reflect"
	"sort"
	"sync"
	"syscall"
	"time"
)

const (
	disPlayPeerInfoInterval = time.Duration(5 * time.Second)
)

var (
	ErrNoBranchConn = errors.New("no such branch's connection")
)

type TxForQueue struct {
	Time          uint64
	TxCount       uint32
	XShardTxCount uint32
}

// QKCMasterBackend masterServer include connections
type QKCMasterBackend struct {
	ctx                *service.ServiceContext
	gspc               *core.Genesis
	lock               sync.RWMutex
	engine             consensus.Engine
	eventMux           *event.TypeMux
	chainDb            ethdb.Database
	shutdown           chan os.Signal
	clusterConfig      *config.ClusterConfig
	clientPool         map[string]rpc.ISlaveConn
	branchToSlaves     map[uint32][]rpc.ISlaveConn
	branchToShardStats map[uint32]*rpc.ShardStatus
	shardStatsChan     chan *rpc.ShardStatus

	miner *miner.Miner

	maxPeers           int
	artificialTxConfig *rpc.ArtificialTxConfig
	rootBlockChain     *core.RootBlockChain
	protocolManager    *ProtocolManager
	synchronizer       Synchronizer.Synchronizer
	txCountHistory     *deque.Deque
	logInfo            string
	exitCh             chan struct{}
}

// New new master with config
func New(ctx *service.ServiceContext, cfg *config.ClusterConfig) (*QKCMasterBackend, error) {
	var (
		mstr = &QKCMasterBackend{
			ctx:                ctx,
			clusterConfig:      cfg,
			gspc:               core.NewGenesis(cfg.Quarkchain),
			eventMux:           ctx.EventMux,
			clientPool:         make(map[string]rpc.ISlaveConn),
			branchToSlaves:     make(map[uint32][]rpc.ISlaveConn, 0),
			branchToShardStats: make(map[uint32]*rpc.ShardStatus),
			shardStatsChan:     make(chan *rpc.ShardStatus, len(cfg.Quarkchain.GetGenesisShardIds())),
			artificialTxConfig: &rpc.ArtificialTxConfig{
				TargetRootBlockTime:  cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime,
				TargetMinorBlockTime: cfg.Quarkchain.GetShardConfigByFullShardID(cfg.Quarkchain.GetGenesisShardIds()[0]).ConsensusConfig.TargetBlockTime,
			},
			maxPeers:       25,
			logInfo:        "masterServer",
			shutdown:       ctx.Shutdown,
			txCountHistory: deque.New(),
			exitCh:         make(chan struct{}),
		}
		err error
	)
	if mstr.chainDb, err = createDB(ctx, cfg.DbPathRoot, cfg.Clean); err != nil {
		return nil, err
	}

	if mstr.engine, err = createConsensusEngine(cfg.Quarkchain.Root, cfg.Quarkchain.GuardianPublicKey); err != nil {
		return nil, err
	}

	mstr.miner = miner.New(ctx, mstr, mstr.engine, cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime)

	chainConfig, genesisHash, genesisErr := core.SetupGenesisRootBlock(mstr.chainDb, mstr.gspc)
	// TODO check config err
	if genesisErr != nil {
		log.Info("Fill in block into chain db.")
		rawdb.WriteChainConfig(mstr.chainDb, genesisHash, cfg.Quarkchain)
	}
	log.Debug("Initialised chain configuration", "config", chainConfig)

	if mstr.rootBlockChain, err = core.NewRootBlockChain(mstr.chainDb, cfg.Quarkchain, mstr.engine, nil); err != nil {
		return nil, err
	}

	mstr.rootBlockChain.SetEnableCountMinorBlocks(cfg.EnableTransactionHistory)
	mstr.rootBlockChain.SetBroadcastRootBlockFunc(mstr.AddRootBlock)
	for _, cfg := range cfg.SlaveList {
		target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
		client := NewSlaveConn(target, cfg.ChainMaskList, cfg.ID)
		mstr.clientPool[target] = client
	}
	log.Info(mstr.logInfo, "slave client pool", len(mstr.clientPool))

	mstr.synchronizer = Synchronizer.NewSynchronizer(mstr.rootBlockChain)
	if mstr.protocolManager, err = NewProtocolManager(*cfg, mstr.rootBlockChain, mstr.shardStatsChan, mstr.synchronizer, mstr.getShardConnForP2P); err != nil {
		return nil, err
	}

	return mstr, nil
}

func createDB(ctx *service.ServiceContext, name string, clean bool) (ethdb.Database, error) {
	db, err := ctx.OpenDatabase(name, clean)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func createConsensusEngine(cfg *config.RootConfig, pubKeyStr string) (consensus.Engine, error) {
	diffCalculator := consensus.EthDifficultyCalculator{
		MinimumDifficulty: big.NewInt(int64(cfg.Genesis.Difficulty)),
		AdjustmentCutoff:  cfg.DifficultyAdjustmentCutoffTime,
		AdjustmentFactor:  cfg.DifficultyAdjustmentFactor,
	}
	pubKey := common.FromHex(pubKeyStr)
	switch cfg.ConsensusType {
	case config.PoWSimulate: // TODO pow_simulate is fake
		return &consensus.FakeEngine{}, nil
	case config.PoWEthash:
		return ethash.New(ethash.Config{CachesInMem: 3, CachesOnDisk: 10, CacheDir: "", PowMode: ethash.ModeNormal}, &diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	case config.PoWQkchash:
		return qkchash.New(true, &diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	case config.PoWDoubleSha256:
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	case config.PoWGmhash:
		return gmhash.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	}
	return nil, fmt.Errorf("Failed to create consensus engine consensus type %s ", cfg.ConsensusType)
}

func (s *QKCMasterBackend) GetClusterConfig() *config.ClusterConfig {
	return s.clusterConfig
}

// Protocols p2p protocols, p2p Server will start in node.Start
func (s *QKCMasterBackend) Protocols() []p2p.Protocol {
	return s.protocolManager.subProtocols
}

// APIs return all apis for master Server
func (s *QKCMasterBackend) APIs() []ethRPC.API {
	apis := qkcapi.GetAPIs(s)
	return append(apis, []ethRPC.API{
		{
			Namespace: "rpc." + reflect.TypeOf(MasterServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(s),
			Public:    false,
		},
	}...)
}

// Stop stop node -> stop qkcMaster
func (s *QKCMasterBackend) Stop() error {
	s.miner.Stop()
	s.engine.Close()
	s.rootBlockChain.Stop()
	s.protocolManager.Stop()
	s.eventMux.Stop()
	s.synchronizer.Close()
	s.chainDb.Close()
	close(s.exitCh)
	for _, slv := range s.clientPool {
		conn := slv.(*SlaveConnection)
		conn.client.Close()
	}
	return nil
}

// Start start node -> start qkcMaster
func (s *QKCMasterBackend) Init(srvr *p2p.Server) error {
	if srvr != nil {
		s.maxPeers = srvr.MaxPeers
	}
	if err := s.ConnectToSlaves(); err != nil {
		return err
	}
	s.logSummary()

	if err := s.hasAllShards(); err != nil {
		return err
	}

	if err := s.initShards(); err != nil {
		return err
	}
	s.Heartbeat()
	s.miner.Init()
	log.Info(s.logInfo, "superAccount len", len(s.clusterConfig.Quarkchain.SuperAccount))
	for _, v := range s.clusterConfig.Quarkchain.SuperAccount {
		log.Info(s.logInfo, "super account", v.String())
	}
	return nil
}

func (s *QKCMasterBackend) SetMining(mining bool) error {
	if err := s.CheckAccountPermission(s.clusterConfig.Quarkchain.Root.CoinbaseAddress); err != nil {
		return err
	}
	var g errgroup.Group
	for _, slvConn := range s.clientPool {
		conn := slvConn
		g.Go(func() error {
			return conn.SetMining(mining)
		})
	}
	if err := g.Wait(); err != nil {
		log.Error("Set slave mining failed", "err", err)
		for _, slvConn := range s.clientPool {
			conn := slvConn
			conn.SetMining(false)
		}
		return err
	}

	s.miner.SetMining(mining)
	return nil
}

// InitCluster init cluster :
// 1:ConnectToSlaves
// 2:logSummary
// 3:check if has all shards
// 4.setup slave to slave
// 5:init shards
func (s *QKCMasterBackend) Start() error {
	s.protocolManager.Start(s.maxPeers)
	// start heart beat pre 3 seconds.
	s.updateShardStatsLoop()
	log.Info("Start cluster successful", "slaveSize", len(s.clientPool))
	return nil
}

func (s *QKCMasterBackend) ConnectToSlaves() error {
	fullShardIds := s.clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, slaveConn := range s.clientPool {
		id, chainMaskList, err := slaveConn.SendPing()
		if err != nil {
			return err
		}
		if err := checkPing(slaveConn, id, chainMaskList); err != nil {
			return err
		}
		for _, fullShardID := range fullShardIds {
			if slaveConn.HasShard(fullShardID) {
				s.branchToSlaves[fullShardID] = append(s.branchToSlaves[fullShardID], slaveConn)
			}
		}
	}
	return nil
}
func (s *QKCMasterBackend) logSummary() {
	for branch, slaves := range s.branchToSlaves {
		for _, slave := range slaves {
			log.Info(s.logInfo, "branch:", branch, "is run by slave", slave.GetSlaveID())
		}
	}
}

func (s *QKCMasterBackend) hasAllShards() error {
	if len(s.branchToSlaves) == len(s.clusterConfig.Quarkchain.GetGenesisShardIds()) {
		for _, v := range s.branchToSlaves {
			if len(v) == 0 {
				return errors.New("branch's slave<=0")
			}
		}
		return nil
	}
	return errors.New("len not match")
}

func (s *QKCMasterBackend) getSlaveInfoListFromClusterConfig() []*rpc.SlaveInfo {
	slaveInfos := make([]*rpc.SlaveInfo, 0)
	for _, slave := range s.clusterConfig.SlaveList {
		slaveInfos = append(slaveInfos, &rpc.SlaveInfo{
			Id:            slave.ID,
			Host:          slave.IP,
			Port:          slave.Port,
			ChainMaskList: slave.ChainMaskList,
		})
	}
	return slaveInfos
}

func (s *QKCMasterBackend) initShards() error {
	var g errgroup.Group
	ip, port := s.clusterConfig.Quarkchain.Root.GRPCHost, s.clusterConfig.Quarkchain.Root.GRPCPort
	for _, client := range s.clientPool {
		client := client
		g.Go(func() error {
			err := client.MasterInfo(ip, port, s.rootBlockChain.CurrentBlock())
			return err
		})
	}
	return g.Wait()
}

func (s *QKCMasterBackend) updateShardStatsLoop() {
	go func() {
		for true {
			select {
			case stats := <-s.shardStatsChan:
				s.UpdateShardStatus(stats)
			case <-s.exitCh:
				return
			}
		}
	}()
}

func (s *QKCMasterBackend) broadcastRootBlockToSlaves(block *types.RootBlock) error {
	var g errgroup.Group
	for _, client := range s.clientPool {
		client := client
		g.Go(func() error {
			err := client.AddRootBlock(block, false)
			if err != nil {
				log.Error("broadcastRootBlockToSlaves failed", "slave", client.GetSlaveID(),
					"block", block.Hash(), "root parent hash", block.Header().ParentHash.Hex(), "height", block.NumberU64(), "err", err)
			}
			return err
		})
	}
	return g.Wait()
}

func (s *QKCMasterBackend) Heartbeat() {
	go func(normal bool) {
		for normal {
			timeGap := time.Now()
			s.ctx.Timestamp = timeGap
			for endpoint := range s.clientPool {
				normal = s.clientPool[endpoint].HeartBeat()
				if !normal {
					s.SetMining(false)
					s.shutdown <- syscall.SIGTERM
					break
				}
			}
			duration := time.Now().Sub(timeGap)
			log.Trace(s.logInfo, "heart beat duration", duration.String())
			time.Sleep(config.HeartbeatInterval)
		}
	}(true)
}

func checkPing(slaveConn rpc.ISlaveConn, id []byte, chainMaskList []*types.ChainMask) error {
	if slaveConn.GetSlaveID() != string(id) {
		return errors.New("slaveID is not match")
	}
	if len(chainMaskList) != len(slaveConn.GetShardMaskList()) {
		return errors.New("chainMaskList is not match")
	}
	lenChainMaskList := len(chainMaskList)

	for index := 0; index < lenChainMaskList; index++ {
		if chainMaskList[index].GetMask() != slaveConn.GetShardMaskList()[index].GetMask() {
			return errors.New("chainMaskList index is not match")
		}
	}
	return nil
}

func (s *QKCMasterBackend) getOneSlaveConnection(branch account.Branch) rpc.ISlaveConn {
	slaves := s.branchToSlaves[branch.Value]
	if len(slaves) < 1 {
		return nil
	}
	return slaves[0]
}

func (s *QKCMasterBackend) getAllSlaveConnection(fullShardID uint32) []rpc.ISlaveConn {
	slaves := s.branchToSlaves[fullShardID]
	if len(slaves) < 1 {
		return nil
	}
	return slaves
}

func (s *QKCMasterBackend) getShardConnForP2P(fullShardID uint32) []rpc.ShardConnForP2P {
	slaves := s.branchToSlaves[fullShardID]
	if len(slaves) < 1 {
		return nil
	}
	slavesInterface := make([]rpc.ShardConnForP2P, 0)
	for _, v := range slaves {
		slavesInterface = append(slavesInterface, v)
	}
	return slavesInterface
}

func (s *QKCMasterBackend) createRootBlockToMine(address account.Address) (*types.RootBlock, error) {
	var g errgroup.Group
	rspList := make(chan *rpc.GetUnconfirmedHeadersResponse, len(s.clientPool))

	for target := range s.clientPool {
		target := target
		g.Go(func() error {
			rsp, err := s.clientPool[target].GetUnconfirmedHeaders()
			rspList <- rsp
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	fullShardIDToHeaderList := make(map[uint32][]*types.MinorBlockHeader, 0)
	for index := 0; index < len(s.clientPool); index++ {
		resp := <-rspList
		for _, headersInfo := range resp.HeadersInfoList {
			if _, ok := fullShardIDToHeaderList[headersInfo.Branch]; ok { // to avoid overlap
				continue // skip it if has added
			}
			height := uint64(0)
			for _, header := range headersInfo.HeaderList {
				if height != 0 && height+1 != header.Number {
					return nil, errors.New("headers must ordered by height")
				}
				height = header.Number

				if !s.rootBlockChain.IsMinorBlockValidated(header.Hash()) {
					break
				}
				fullShardIDToHeaderList[headersInfo.Branch] = append(fullShardIDToHeaderList[headersInfo.Branch], header)
			}
		}
	}

	headerList := make([]*types.MinorBlockHeader, 0)
	currTipHeight := s.rootBlockChain.CurrentBlock().Number()
	fullShardIdToCheck := s.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(currTipHeight + 1)
	sort.Slice(fullShardIdToCheck, func(i, j int) bool { return fullShardIdToCheck[i] < fullShardIdToCheck[j] })
	for _, fullShardID := range fullShardIdToCheck {
		headers := fullShardIDToHeaderList[fullShardID]
		headerList = append(headerList, headers...)
	}
	newblock, err := s.rootBlockChain.CreateBlockToMine(headerList, &address, nil)
	if err != nil {
		return nil, err
	}
	return newblock, nil
}

// GetAccountData get account Data for jsonRpc
func (s *QKCMasterBackend) GetAccountData(address *account.Address, height *uint64) (map[uint32]*rpc.AccountBranchData, error) {
	var g errgroup.Group
	rspList := make(chan *rpc.GetAccountDataResponse, len(s.clientPool))
	for target := range s.clientPool {
		target := target
		g.Go(func() error {
			rsp, err := s.clientPool[target].GetAccountData(address, height)
			rspList <- rsp
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	branchToAccountBranchData := make(map[uint32]*rpc.AccountBranchData)
	for index := 0; index < len(s.clientPool); index++ {
		rsp := <-rspList
		for _, accountBranchData := range rsp.AccountBranchDataList {
			branchToAccountBranchData[accountBranchData.Branch] = accountBranchData
		}
	}
	//if len(branchToAccountBranchData) != len(s.clusterConfig.Quarkchain.GetGenesisShardIds()) {//maybe not be created?
	//	return nil, errors.New("len is not match")
	//}
	return branchToAccountBranchData, nil
}

// GetPrimaryAccountData get primary account data for jsonRpc
func (s *QKCMasterBackend) GetPrimaryAccountData(address *account.Address, blockHeight *uint64) (*rpc.AccountBranchData, error) {
	fullShardID, err := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	if err != nil {
		return nil, err
	}
	slaveConn := s.getOneSlaveConnection(account.Branch{Value: fullShardID})
	if slaveConn == nil {
		return nil, ErrNoBranchConn
	}
	rsp, err := slaveConn.GetAccountData(address, blockHeight)
	if err != nil {
		return nil, err
	}
	for _, accountBranchData := range rsp.AccountBranchDataList {
		if accountBranchData.Branch == fullShardID {
			return accountBranchData, nil
		}
	}
	return nil, errors.New("no such data")
}

// SendMiningConfigToSlaves send mining config to slaves,used in jsonRpc
func (s *QKCMasterBackend) SendMiningConfigToSlaves(mining bool) error {
	var g errgroup.Group
	for index := range s.clientPool {
		i := index
		g.Go(func() error {
			return s.clientPool[i].SendMiningConfigToSlaves(s.artificialTxConfig, mining)
		})
	}
	return g.Wait()
}

func (s *QKCMasterBackend) CheckAccountPermission(addr account.Address) error {
	fullShardID, err := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(addr.FullShardKey)
	if err != nil {
		return err
	}
	slave, ok := s.branchToSlaves[fullShardID]
	if !ok {
		return fmt.Errorf("no such fullShardID:%v fullShardKey:%v", fullShardID, addr.FullShardKey)
	}
	if len(slave) == 0 {
		return errors.New("slave len is 0")
	}
	return slave[0].CheckAccountPermission(addr)
}

// AddRootBlock add root block to all slaves
func (s *QKCMasterBackend) AddRootBlock(rootBlock *types.RootBlock) error {
	if err := s.CheckAccountPermission(rootBlock.Header().Coinbase); err != nil {
		return err
	}
	head := s.rootBlockChain.CurrentBlock().NumberU64()

	s.rootBlockChain.WriteCommittingHash(rootBlock.Hash())
	_, err := s.rootBlockChain.InsertChain([]types.IBlock{rootBlock})
	if err != nil {
		return err
	}
	if err := s.broadcastRootBlockToSlaves(rootBlock); err != nil {
		if err := s.rootBlockChain.SetHead(head); err != nil {
			panic(err)
		}
		return err
	}
	s.rootBlockChain.ClearCommittingHash()
	go s.miner.HandleNewTip()
	return nil
}

// SetTargetBlockTime set target Time from jsonRpc
func (s *QKCMasterBackend) SetTargetBlockTime(rootBlockTime *uint32, minorBlockTime *uint32) error {
	if rootBlockTime == nil {
		temp := s.artificialTxConfig.TargetMinorBlockTime
		rootBlockTime = &temp
	}

	if minorBlockTime == nil {
		temp := s.artificialTxConfig.TargetMinorBlockTime
		minorBlockTime = &temp
	}
	s.artificialTxConfig = &rpc.ArtificialTxConfig{
		TargetRootBlockTime:  *rootBlockTime,
		TargetMinorBlockTime: *minorBlockTime,
	}
	return nil
}

// CreateTransactions Create transactions and add to the network for load testing
func (s *QKCMasterBackend) CreateTransactions(numTxPerShard, xShardPercent uint32, tx *types.Transaction) error {
	var g errgroup.Group
	for index := range s.clientPool {
		i := index
		g.Go(func() error {
			return s.clientPool[i].GenTx(numTxPerShard, xShardPercent, tx)
		})
	}
	return g.Wait()
}

// UpdateShardStatus update shard status for branchg
func (s *QKCMasterBackend) UpdateShardStatus(status *rpc.ShardStatus) {
	s.lock.Lock()
	s.branchToShardStats[status.Branch.Value] = status
	s.lock.Unlock()
}

func (s *QKCMasterBackend) GetLastMinorBlockByFullShardID(fullShardId uint32) (uint64, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	data, ok := s.branchToShardStats[fullShardId]
	if !ok {
		return 0, errors.New("no such fullShardId") //TODO 0?
	}
	return data.Height, nil
}

// UpdateTxCountHistory update Tx count queue
func (s *QKCMasterBackend) UpdateTxCountHistory(txCount, xShardTxCount uint32, createTime uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	minute := createTime / 60 * 60
	if s.txCountHistory.Size() == 0 || s.txCountHistory.Right().(TxForQueue).Time < minute {
		s.txCountHistory.PushRight(TxForQueue{
			Time:          minute,
			TxCount:       txCount,
			XShardTxCount: xShardTxCount,
		})
	} else {
		old := s.txCountHistory.PopRight().(TxForQueue)
		s.txCountHistory.PushRight(TxForQueue{
			Time:          old.Time,
			TxCount:       old.TxCount + txCount,
			XShardTxCount: old.XShardTxCount + xShardTxCount,
		})
	}

	for s.txCountHistory.Size() > 0 && s.txCountHistory.Right().(TxForQueue).Time < uint64(time.Now().Unix()-3600*12) {
		s.txCountHistory.PopLeft()
	}

}

func (s *QKCMasterBackend) GetBlockCount() (map[uint32]map[account.Recipient]uint32, error) {
	headerTip := s.rootBlockChain.CurrentBlock()
	return s.rootBlockChain.GetBlockCount(headerTip.Number())
}

func (s *QKCMasterBackend) GetStats() (map[string]interface{}, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()
	branchToShardStats := s.branchToShardStats
	shards := make([]map[string]interface{}, 0)
	for _, shardState := range branchToShardStats {
		shard := make(map[string]interface{})
		shard["fullShardId"] = shardState.Branch.GetFullShardID()
		shard["chainId"] = shardState.Branch.GetChainID()
		shard["shardId"] = shardState.Branch.GetShardID()
		shard["height"] = shardState.Height
		shard["difficulty"] = shardState.Difficulty
		shard["coinbaseAddress"] = hexutil.Bytes(shardState.CoinbaseAddress.ToBytes())
		shard["timestamp"] = shardState.Timestamp
		shard["txCount60s"] = shardState.TxCount60s
		shard["pendingTxCount"] = shardState.PendingTxCount
		shard["totalTxCount"] = shardState.TotalTxCount
		shard["blockCount60s"] = shardState.BlockCount60s
		shard["staleBlockCount60s"] = shardState.StaleBlockCount60s
		shard["lastBlockTime"] = shardState.LastBlockTime
		shards = append(shards, shard)
	}
	sort.Slice(shards, func(i, j int) bool { return shards[i]["fullShardId"].(uint32) < shards[j]["fullShardId"].(uint32) }) //Right???
	var (
		sumTxCount60s         = uint32(0)
		sumBlockCount60       = uint32(0)
		sumPendingTxCount     = uint32(0)
		sumStaleBlockCount60s = uint32(0)
		sumTotalTxCount       = uint32(0)
	)
	for _, v := range branchToShardStats {
		sumTxCount60s += v.TxCount60s
		sumBlockCount60 += v.BlockCount60s
		sumPendingTxCount += v.PendingTxCount
		sumStaleBlockCount60s += v.StaleBlockCount60s
		sumTotalTxCount += v.TotalTxCount
	}
	tip := s.rootBlockChain.CurrentBlock()
	rootLastBlockTime := uint64(0)
	if tip.Header().Number >= 3 {
		prev := s.rootBlockChain.GetBlock(tip.ParentHash())
		rootLastBlockTime = tip.Time() - prev.IHeader().GetTime()
	}
	//anything else easy handle?
	txCountHistory := make([]map[string]interface{}, 0)
	dequeSize := s.txCountHistory.Size()
	for index := 0; index < dequeSize; index++ {
		right := s.txCountHistory.PopLeft().(TxForQueue)

		field := map[string]interface{}{
			"timestamp":     right.Time,
			"txCount":       right.TxCount,
			"xShardTxCount": right.XShardTxCount,
		}
		txCountHistory = append(txCountHistory, field)
		s.txCountHistory.PushRight(right)
	}
	s.GetPeers()
	peerForDisplay := make([]string, 0)
	for _, v := range s.protocolManager.peers.Peers() {
		var temp string
		if tcp, ok := v.RemoteAddr().(*net.TCPAddr); ok {
			temp = fmt.Sprintf("%s:%v", tcp.IP.String(), tcp.Port)
		} else {
			panic(errors.New("Peer not tcp?"))
		}
		peerForDisplay = append(peerForDisplay, temp)
	}
	cc, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"networkId":            s.clusterConfig.Quarkchain.NetworkID,
		"chainSize":            s.clusterConfig.Quarkchain.ChainSize,
		"shardServerCount":     len(s.clientPool),
		"rootHeight":           s.rootBlockChain.CurrentBlock().Number(),
		"rootDifficulty":       s.rootBlockChain.CurrentBlock().Difficulty(),
		"rootCoinbaseAddress":  hexutil.Bytes(s.rootBlockChain.CurrentBlock().Coinbase().ToBytes()),
		"rootTimestamp":        s.rootBlockChain.CurrentBlock().Time(),
		"rootLastBlockTime":    rootLastBlockTime,
		"txCount60s":           sumTxCount60s,
		"blockCount60s":        sumBlockCount60,
		"staleBlockCount60s":   sumStaleBlockCount60s,
		"pendingTxCount":       sumPendingTxCount,
		"totalTxCount":         sumTotalTxCount,
		"syncing":              false, //TODO fake
		"mining":               false, //TODO fake
		"shards":               shards,
		"peers":                peerForDisplay,
		"minor_block_interval": s.artificialTxConfig.TargetMinorBlockTime,
		"root_block_interval":  s.artificialTxConfig.TargetRootBlockTime,
		"cpus":                 cc,
		"txCountHistory":       txCountHistory,
	}, nil
}

func (s *QKCMasterBackend) IsSyncing() bool {
	return s.synchronizer.IsSyncing()
}

func (s *QKCMasterBackend) IsMining() bool {
	return s.miner.IsMining()
}

func (s *QKCMasterBackend) CurrentBlock() *types.RootBlock {
	return s.rootBlockChain.CurrentBlock()
}

func (s *QKCMasterBackend) GetSlavePoolLen() int {
	return len(s.clientPool)
}

//TODO need delete later
func (s *QKCMasterBackend) disPlayPeers() {
	go func() {
		for true {
			time.Sleep(disPlayPeerInfoInterval)
			peers := s.protocolManager.peers.Peers()
			log.Info(s.logInfo, "len(peers)", len(peers))
			for _, v := range peers {
				log.Info(s.logInfo, "remote addr", v.RemoteAddr().String())
			}
		}
	}()

}
