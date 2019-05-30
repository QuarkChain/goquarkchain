package master

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	Synchronizer "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	ethRPC "github.com/ethereum/go-ethereum/rpc"
	"github.com/shirou/gopsutil/mem"
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
	heartbeatInterval       = time.Duration(4 * time.Second)
	disPlayPeerInfoInterval = time.Duration(5 * time.Second)
	rootChainChanSize       = 256
	rootChainSideChanSize   = 256
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

	rootChainChan         chan core.RootChainEvent
	rootChainSideChan     chan core.RootChainSideEvent
	rootChainEventSub     event.Subscription
	rootChainSideEventSub event.Subscription

	artificialTxConfig *rpc.ArtificialTxConfig
	rootBlockChain     *core.RootBlockChain
	protocolManager    *ProtocolManager
	synchronizer       Synchronizer.Synchronizer
	txCountHistory     *deque.Deque
	logInfo            string
}

// New new master with config
func New(ctx *service.ServiceContext, cfg *config.ClusterConfig) (*QKCMasterBackend, error) {
	var (
		mstr = &QKCMasterBackend{
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
			logInfo:        "masterServer",
			shutdown:       ctx.Shutdown,
			txCountHistory: deque.New(),
		}
		err error
	)
	if mstr.chainDb, err = createDB(ctx, cfg.DbPathRoot, cfg.Clean); err != nil {
		return nil, err
	}

	if mstr.engine, err = createConsensusEngine(ctx, cfg.Quarkchain.Root); err != nil {
		return nil, err
	}

	chainConfig, genesisHash, genesisErr := core.SetupGenesisRootBlock(mstr.chainDb, mstr.gspc)
	// TODO check config err
	if genesisErr != nil {
		log.Info("Fill in block into chain db.")
		rawdb.WriteChainConfig(mstr.chainDb, genesisHash, cfg.Quarkchain)
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	if mstr.rootBlockChain, err = core.NewRootBlockChain(mstr.chainDb, nil, cfg.Quarkchain, mstr.engine, nil); err != nil {
		return nil, err
	}

	mstr.rootBlockChain.SetEnableCountMinorBlocks(cfg.EnableTransactionHistory)
	for _, cfg := range cfg.SlaveList {
		target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
		client := NewSlaveConn(target, cfg.ChainMaskList, cfg.ID)
		mstr.clientPool[target] = client
	}
	log.Info("qkc api backend", "slave client pool", len(mstr.clientPool))

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

func createConsensusEngine(ctx *service.ServiceContext, cfg *config.RootConfig) (consensus.Engine, error) {
	diffCalculator := consensus.EthDifficultyCalculator{
		MinimumDifficulty: big.NewInt(int64(cfg.Genesis.Difficulty)),
		AdjustmentCutoff:  cfg.DifficultyAdjustmentCutoffTime,
		AdjustmentFactor:  cfg.DifficultyAdjustmentFactor,
	}
	switch cfg.ConsensusType {
	case config.PoWSimulate: // TODO pow_simulate is fake
		return &consensus.FakeEngine{}, nil
	case config.PoWEthash:
		return qkchash.New(cfg.ConsensusConfig.RemoteMine, &diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
		// panic(errors.New("not support PoWEthash PoWSimulate"))
	case config.PoWQkchash:
		return qkchash.New(cfg.ConsensusConfig.RemoteMine, &diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	case config.PoWDoubleSha256:
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	}
	return nil, fmt.Errorf("Failed to create consensus engine consensus type %s", cfg.ConsensusType)
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
	s.rootBlockChain.Stop()
	s.protocolManager.Stop()
	s.rootChainEventSub.Unsubscribe()
	s.rootChainSideEventSub.Unsubscribe()
	if s.engine != nil {
		s.engine.Close()
	}
	s.eventMux.Stop()
	s.chainDb.Close()
	return nil
}

// Start start node -> start qkcMaster
func (s *QKCMasterBackend) Start(srvr *p2p.Server) error {
	s.rootChainChan = make(chan core.RootChainEvent, rootChainChanSize)
	s.rootChainEventSub = s.rootBlockChain.SubscribeChainEvent(s.rootChainChan)
	s.rootChainSideChan = make(chan core.RootChainSideEvent, rootChainSideChanSize)
	s.rootChainSideEventSub = s.rootBlockChain.SubscribeChainSideEvent(s.rootChainSideChan)

	maxPeers := srvr.MaxPeers
	s.protocolManager.Start(maxPeers)
	// start heart beat pre 3 seconds.
	s.updateShardStatsLoop()
	s.Heartbeat()
	s.disPlayPeers()
	go s.broadcastRpcLoop()
	return nil
}

// StartMining start mining
func (s *QKCMasterBackend) StartMining(threads int) error {
	// TODO @liuhuan
	return nil
}

// StopMining stop mining
func (s *QKCMasterBackend) StopMining(threads int) error {
	// TODO @liuhuan
	return nil
}

// InitCluster init cluster :
// 1:ConnectToSlaves
// 2:logSummary
// 3:check if has all shards
// 4.setup slave to slave
// 5:init shards
func (s *QKCMasterBackend) InitCluster() error {
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
	ip, port := s.clusterConfig.Quarkchain.Root.Ip, s.clusterConfig.Quarkchain.Root.Port
	for endpoint := range s.clientPool {
		if err := s.clientPool[endpoint].MasterInfo(ip, port, s.rootBlockChain.CurrentBlock()); err != nil {
			return err
		}
	}
	return nil
}

func (s *QKCMasterBackend) updateShardStatsLoop() {
	go func() {
		for true {
			select {
			case stats := <-s.shardStatsChan:
				s.UpdateShardStatus(stats)
			}
		}
	}()
}

func (s *QKCMasterBackend) broadcastRpcLoop() {
	go func() {
		for true {
			select {
			// TODO verify whether rootChainChan & rootChainSideChan would be blocked,
			// as the chan len is 256, they should not be blocked
			case event := <-s.rootChainChan:
				s.broadcastRootBlockToSlaves(event.Block)
				// Err() channel will be closed when unsubscribing.
			case err := <-s.rootChainEventSub.Err():
				log.Error("rootChainEventSub error in broadcastRpcLoop ", "error", err)
				return
			case event := <-s.rootChainSideChan:
				s.broadcastRootBlockToSlaves(event.Block)
				// Err() channel will be closed when unsubscribing.
			case err := <-s.rootChainSideEventSub.Err():
				log.Error("rootChainSideEventSub error in broadcastRpcLoop", "error", err)
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
					"block", block.Hash(), "height", block.NumberU64(), "err", err)
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
			for endpoint := range s.clientPool {
				normal = s.clientPool[endpoint].HeartBeat()
				if !normal {
					s.shutdown <- syscall.SIGTERM
					break
				}
			}
			duration := time.Now().Sub(timeGap)
			log.Trace(s.logInfo, "heart beat duration", duration.String())
			time.Sleep(heartbeatInterval)
		}
	}(true)
	//TODO :add send master info
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
	if err := s.rootBlockChain.Validator().ValidateBlock(newblock); err != nil {
		//TODO :only for exposure problem ,need to delete later
		panic(err)
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
	if len(branchToAccountBranchData) != len(s.clusterConfig.Quarkchain.GetGenesisShardIds()) {
		return nil, errors.New("len is not match")
	}
	return branchToAccountBranchData, nil
}

// GetPrimaryAccountData get primary account data for jsonRpc
func (s *QKCMasterBackend) GetPrimaryAccountData(address *account.Address, blockHeight *uint64) (*rpc.AccountBranchData, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
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

// AddRootBlock add root block to all slaves
func (s *QKCMasterBackend) AddRootBlock(rootBlock *types.RootBlock) error {
	s.rootBlockChain.WriteCommittingHash(rootBlock.Hash())
	_, err := s.rootBlockChain.InsertChain([]types.IBlock{rootBlock})
	if err != nil {
		return err
	}
	var g errgroup.Group
	for index := range s.clientPool {
		i := index
		g.Go(func() error {
			return s.clientPool[i].AddRootBlock(rootBlock, false)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	s.rootBlockChain.ClearCommittingHash()
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
	return s.StartMining(1)
}

// SetMining setmiming status
func (s *QKCMasterBackend) SetMining(mining bool) error {
	//TODO need liuhuan to finish
	if mining {
		return s.StartMining(1)
	}
	return s.StopMining(1)
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

	for s.txCountHistory.Size() > 0 && s.txCountHistory.PopRight().(TxForQueue).Time < uint64(time.Now().Unix()-3600*12) {
		s.txCountHistory.PopLeft()
	}

}

func (s *QKCMasterBackend) GetBlockCount() (map[uint32]map[account.Recipient]uint32, error) {
	headerTip := s.rootBlockChain.CurrentBlock()
	return s.rootBlockChain.GetBlockCount(headerTip.Number())
}

func (s *QKCMasterBackend) GetStats() map[string]interface{} {
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
		shard["coinbaseAddress"] = shardState.CoinbaseAddress.ToBytes()
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
		right := s.txCountHistory.PopRight().(TxForQueue)

		field := map[string]interface{}{
			"timestamp":     right.Time,
			"txCount":       right.TxCount,
			"xShardTxCount": right.XShardTxCount,
		}
		txCountHistory = append(txCountHistory, field)
		s.txCountHistory.PushLeft(right)
	}
	s.GetPeers()
	peerForDisplay := make([]string, 0)
	for _, v := range s.protocolManager.peers.Peers() {
		var temp string
		if tcp, ok := v.RemoteAddr().(*net.TCPAddr); ok {
			temp = fmt.Sprintf("%s:%v", tcp.IP.String(), tcp.Port)
		} else {
			panic(errors.New("peer not tcp?"))
		}
		peerForDisplay = append(peerForDisplay, temp)
	}
	v, _ := mem.VirtualMemory()
	return map[string]interface{}{
		"networkId":           s.clusterConfig.Quarkchain.NetworkID,
		"chainSize":           s.clusterConfig.Quarkchain.ChainSize,
		"shardServerCount":    len(s.clientPool),
		"rootHeight":          s.rootBlockChain.CurrentBlock().Number(),
		"rootDifficulty":      s.rootBlockChain.CurrentBlock().Difficulty(),
		"rootCoinbaseAddress": s.rootBlockChain.CurrentBlock().Coinbase().ToBytes(),
		"rootTimestamp":       s.rootBlockChain.CurrentBlock().Time(),
		"rootLastBlockTime":   rootLastBlockTime,
		"txCount60s":          sumTxCount60s,
		"blockCount60s":       sumBlockCount60,
		"staleBlockCount60s":  sumStaleBlockCount60s,
		"pendingTxCount":      sumPendingTxCount,
		"totalTxCount":        sumTotalTxCount,
		//"syncing":s.synchronizer.Running TODO running
		// "mining": root_miner.is_enabled()
		"shards":               shards,
		"peers":                peerForDisplay,
		"minor_block_interval": s.artificialTxConfig.TargetMinorBlockTime,
		"root_block_interval":  s.artificialTxConfig.TargetRootBlockTime,
		"cpus":                 v.UsedPercent,
		"txCountHistory":       txCountHistory,
	}
}

func (s *QKCMasterBackend) isSyning() bool {
	// TODO @liuhuan
	return false
}

func (s *QKCMasterBackend) isMining() bool {
	// TODO @liuhuan
	return false
}

func (s *QKCMasterBackend) CurrentBlock() *types.RootBlock {
	return s.rootBlockChain.CurrentBlock()
}

func (s *QKCMasterBackend) IsSyncing() bool {
	panic("@junjia")
}
func (s *QKCMasterBackend) IsMining() bool {
	panic("@liuhuan")
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
