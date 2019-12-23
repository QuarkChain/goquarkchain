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
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/QuarkChain/goquarkchain/consensus/simulate"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/qkcapi"
	"github.com/QuarkChain/goquarkchain/p2p"
	qrpc "github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/shirou/gopsutil/cpu"
	"golang.org/x/sync/errgroup"
	"gopkg.in/karalabe/cookiejar.v1/collections/deque"
	"math/big"
	"net"
	"os"
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
	branchToShardStats map[uint32]*rpc.ShardStatus
	shardStatsChan     chan *rpc.ShardStatus

	SlaveConnManager
	miner *miner.Miner

	maxPeers int
	srvr     *p2p.Server

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
	if mstr.chainDb, err = createDB(ctx, "db", cfg.Clean, cfg.CheckDB); err != nil {
		return nil, err
	}

	if mstr.engine, err = createConsensusEngine(cfg.Quarkchain.Root, cfg.Quarkchain.GuardianPublicKey, cfg.Quarkchain.EnableQkcHashXHeight); err != nil {
		return nil, err
	}

	chainConfig, genesisHash, genesisErr := core.SetupGenesisRootBlock(mstr.chainDb, mstr.gspc)
	// TODO check config err
	if genesisErr != nil {
		log.Info("Fill in block into chain db.")
		rawdb.WriteChainConfig(mstr.chainDb, genesisHash, cfg.Quarkchain)
	}
	log.Debug("Initialised chain configuration", "config", chainConfig)

	if mstr.rootBlockChain, err = core.NewRootBlockChain(mstr.chainDb, cfg.Quarkchain, mstr.engine); err != nil {
		return nil, err
	}

	mstr.rootBlockChain.SetEnableCountMinorBlocks(cfg.EnableTransactionHistory)
	mstr.rootBlockChain.SetBroadcastRootBlockFunc(mstr.AddRootBlock)
	mstr.rootBlockChain.SetRootChainStakesFunc(mstr.GetRootChainStakes)

	mstr.synchronizer = Synchronizer.NewSynchronizer(mstr.rootBlockChain)
	if mstr.protocolManager, err = NewProtocolManager(*cfg, mstr.rootBlockChain, mstr.shardStatsChan, mstr.synchronizer, &mstr.SlaveConnManager); err != nil {
		return nil, err
	}

	mstr.miner = miner.New(ctx, mstr, mstr.engine)

	return mstr, nil
}

func createDB(ctx *service.ServiceContext, name string, clean bool, isReadOnly bool) (ethdb.Database, error) {
	db, err := ctx.OpenDatabase(name, clean, isReadOnly)
	if err != nil {
		return nil, err
	}
	return db, nil
}

func createConsensusEngine(cfg *config.RootConfig, pubKey []byte, qkcHashXHeight uint64) (consensus.Engine, error) {
	diffCalculator := consensus.EthDifficultyCalculator{
		MinimumDifficulty: big.NewInt(int64(cfg.Genesis.Difficulty)),
		AdjustmentCutoff:  cfg.DifficultyAdjustmentCutoffTime,
		AdjustmentFactor:  cfg.DifficultyAdjustmentFactor,
	}
	switch cfg.ConsensusType {
	case config.PoWSimulate:
		return simulate.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey, uint64(cfg.ConsensusConfig.TargetBlockTime)), nil
	case config.PoWEthash:
		return ethash.New(ethash.Config{CachesInMem: 3, CachesOnDisk: 10, CacheDir: "", PowMode: ethash.ModeNormal}, &diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	case config.PoWQkchash:
		return qkchash.New(true, &diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey, qkcHashXHeight), nil
	case config.PoWDoubleSha256:
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine, pubKey), nil
	}
	return nil, fmt.Errorf("Failed to create consensus engine consensus type %s ", cfg.ConsensusType)
}

func (s *QKCMasterBackend) GetProtocolManager() *ProtocolManager {
	return s.protocolManager
}

func (s *QKCMasterBackend) GetClusterConfig() *config.ClusterConfig {
	return s.clusterConfig
}

// Protocols p2p protocols, p2p Server will start in node.Start
func (s *QKCMasterBackend) Protocols() []p2p.Protocol {
	return s.protocolManager.subProtocols
}

func (s *QKCMasterBackend) CheckDB() {
	startTime := time.Now()
	defer log.Info("Integrity check completed!", "run time", time.Now().Sub(startTime).Seconds())

	// Start with root db
	rb := s.rootBlockChain.CurrentBlock()
	fromHeight := s.clusterConfig.CheckDBRBlockFrom
	toHeight := uint64(s.clusterConfig.CheckDBRBlockTo)
	if toHeight < 1 {
		toHeight = 1
	}
	if fromHeight >= 0 && uint64(fromHeight) < rb.NumberU64() {
		rb = s.rootBlockChain.GetBlockByNumber(uint64(fromHeight)).(*types.RootBlock)
		log.Info("Starting from root block ", "height", fromHeight)
	}
	if b := s.rootBlockChain.GetBlock(rb.Hash()); b == nil || b.Hash() != rb.Hash() {
		log.Error("Root block mismatches local root block by hash", "height", rb.NumberU64())
		return
	}

	size := s.clusterConfig.CheckDBRBlockBatch
	count := 0
	for rb.NumberU64() >= toHeight {
		batch, index := make([]types.IBlock, size), 0
		for ; index < size && rb.NumberU64() >= toHeight; index++ {
			if count%100 == 0 {
				log.Info("Checking root block", "height", rb.NumberU64())
			}
			count++
			if s.rootBlockChain.GetBlockByNumber(rb.NumberU64()).Hash() != rb.Hash() {
				log.Error("Root block mismatches canonical chain", "height", rb.NumberU64())
				return
			}
			prevRb := s.rootBlockChain.GetBlock(rb.ParentHash())
			if prevRb == nil || prevRb.Hash() != rb.ParentHash() {
				log.Error("Root block mismatches previous block hash", "height", rb.NumberU64())
				return
			}
			if prevRb.NumberU64()+1 != rb.NumberU64() {
				log.Error("Root block no equal to previous block Height + 1", "height", rb.NumberU64())
				return
			}
			batch[size-index-1] = rb
			rb = prevRb.(*types.RootBlock)
		}
		batch = batch[size-index:]
		var g errgroup.Group
		for _, b := range batch {
			for _, slvConn := range s.GetSlaveConns() {
				conn, block := slvConn, b.(*types.RootBlock)
				g.Go(func() error {
					return conn.CheckMinorBlocksInRoot(block)
				})
			}
		}

		_, err := s.rootBlockChain.InsertChain(batch)
		if err != nil {
			log.Error("Failed to check root block", "height", rb.NumberU64(), "error", err.Error())
			return
		}
		if err := g.Wait(); err != nil {
			log.Error("Failed to check root block ", "height", rb.NumberU64(), "error", err.Error())
			return
		}
	}
}

// APIs return all apis for master Server
func (s *QKCMasterBackend) APIs() []qrpc.API {
	apis := qkcapi.GetAPIs(s)
	return append(apis, []qrpc.API{
		{
			Namespace: "grpc",
			Version:   "3.0",
			Service:   NewServerSideOp(s),
			Public:    false,
		},
	}...)
}

// Stop stop node -> stop qkcMaster
func (s *QKCMasterBackend) Stop() error {
	s.synchronizer.Close()
	s.protocolManager.Stop()
	s.miner.Stop()
	s.engine.Close()
	s.rootBlockChain.Stop()
	s.eventMux.Stop()
	s.chainDb.Close()
	close(s.exitCh)
	for _, slv := range s.GetSlaveConns() {
		conn := slv.(*SlaveConnection)
		conn.client.Close()
	}
	return nil
}

// Start start node -> start qkcMaster
func (s *QKCMasterBackend) Init(srvr *p2p.Server) error {
	if srvr != nil {
		s.srvr = srvr
		s.maxPeers = srvr.MaxPeers
	}
	err := s.SlaveConnManager.InitConnManager(s.clusterConfig)
	if err != nil {
		return err
	}
	log.Info(s.logInfo, "slave client pool", s.ConnCount())

	if err := s.initShards(); err != nil {
		return err
	}

	s.Heartbeat()
	return nil
}

func (s *QKCMasterBackend) SetMining(mining bool) {
	var g errgroup.Group
	for _, slvConn := range s.GetSlaveConns() {
		conn := slvConn
		g.Go(func() error {
			return conn.SetMining(mining)
		})
	}
	if err := g.Wait(); err != nil {
		log.Error("Set slave mining failed", "err", err)
		return
	}

	s.miner.SetMining(mining)
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

	if s.clusterConfig.Quarkchain.Root.ConsensusConfig.RemoteMine {
		s.SetMining(true)
	}

	log.Info("Start cluster successful", "slaveSize", s.ConnCount())
	return nil
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
	ip, port := s.clusterConfig.Quarkchain.GRPCHost, s.clusterConfig.Quarkchain.GRPCPort
	for _, client := range s.GetSlaveConns() {
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
	for _, client := range s.GetSlaveConns() {
		client := client
		g.Go(func() error {
			err := client.AddRootBlock(block, false)
			if err != nil {
				log.Error("broadcastRootBlockToSlaves failed", "slave", client.GetSlaveID(),
					"block", block.Hash(), "root parent hash", block.ParentHash().Hex(), "height", block.NumberU64(), "err", err)
			}
			return err
		})
	}
	return g.Wait()
}

func (s *QKCMasterBackend) Heartbeat() {
	go func(normal bool) {
		for normal {
			select {
			case <-s.exitCh:
				normal = false
				break
			default:
				timeGap := time.Now()
				s.ctx.LockTimestamp()
				s.ctx.Timestamp = timeGap
				s.ctx.UnlockTimestamp()
				for _, conn := range s.GetSlaveConns() {
					normal = conn.HeartBeat()
					if !normal {
						s.SetMining(false)
						s.shutdown <- syscall.SIGTERM
						break
					}
				}
				log.Trace(s.logInfo, "heart beat duration", time.Now().Sub(timeGap).String())
				time.Sleep(config.HeartbeatInterval)
			}
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

func (s *QKCMasterBackend) createRootBlockToMine(address account.Address) (*types.RootBlock, error) {
	var (
		g     errgroup.Group
		conns = s.GetSlaveConns()
	)
	rspList := make(chan *rpc.GetUnconfirmedHeadersResponse, s.ConnCount())

	for _, conn := range conns {
		conn := conn
		g.Go(func() error {
			rsp, err := conn.GetUnconfirmedHeaders()
			rspList <- rsp
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	fullShardIDToHeaderList := make(map[uint32][]*types.MinorBlockHeader, 0)
	for index := 0; index < len(conns); index++ {
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
	var (
		g     errgroup.Group
		conns = s.GetSlaveConns()
	)
	rspList := make(chan *rpc.GetAccountDataResponse, len(conns))
	for _, conn := range conns {
		conn := conn
		g.Go(func() error {
			rsp, err := conn.GetAccountData(address, height)
			rspList <- rsp
			return err
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	branchToAccountBranchData := make(map[uint32]*rpc.AccountBranchData)
	for index := 0; index < len(conns); index++ {
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
	fullShardID, err := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	if err != nil {
		return nil, err
	}
	slaveConn := s.GetOneSlaveConnById(fullShardID)
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
	for _, conn := range s.GetSlaveConns() {
		conn := conn
		g.Go(func() error {
			return conn.SendMiningConfigToSlaves(s.artificialTxConfig, mining)
		})
	}
	return g.Wait()
}

// AddRootBlock add root block to all slaves
func (s *QKCMasterBackend) AddRootBlock(rootBlock *types.RootBlock) error {
	header := s.rootBlockChain.CurrentBlock().Header()
	s.rootBlockChain.WriteCommittingHash(rootBlock.Hash())
	_, err := s.rootBlockChain.InsertChain([]types.IBlock{rootBlock})
	if err != nil {
		return err
	}
	if err := s.broadcastRootBlockToSlaves(rootBlock); err != nil {
		return err
	}
	s.rootBlockChain.ClearCommittingHash()
	if header.Hash() != s.rootBlockChain.CurrentBlock().Hash() {
		go s.miner.HandleNewTip()
	}
	return nil
}

func (s *QKCMasterBackend) GetRootChainStakes(coinbase account.Address, lastMinor common.Hash) (*big.Int,
	*account.Recipient, error) {

	fullShardId := uint32(1)
	conn := s.GetOneSlaveConnById(fullShardId)
	if conn == nil {
		panic("chain 0 shard 0 missing.")
	}
	stakes, signer, err := conn.GetRootChainStakes(coinbase, lastMinor)
	if err != nil {
		return nil, nil, err
	}
	return stakes, signer, nil
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
	for _, conn := range s.GetSlaveConns() {
		conn := conn
		g.Go(func() error {
			return conn.GenTx(numTxPerShard, xShardPercent, tx)
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

func (s *QKCMasterBackend) GetRootHashConfirmingMinorBlock(mBlockID []byte) common.Hash {
	return s.rootBlockChain.GetRootBlockConfirmingMinorBlock(mBlockID)
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
		powConfig := s.clusterConfig.Quarkchain.GetShardConfigByFullShardID(shardState.Branch.GetFullShardID()).PoswConfig
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
		shard["poswEnabled"] = powConfig.Enabled
		shard["poswMinStake"] = powConfig.TotalStakePerBlock
		shard["poswWindowSize"] = powConfig.WindowSize
		shard["difficultyDivider"] = powConfig.DiffDivider
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
	if tip.NumberU64() >= 3 {
		prev := s.rootBlockChain.GetBlock(tip.ParentHash())
		rootLastBlockTime = tip.Time() - prev.Time()
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
	cc, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"networkId":            s.clusterConfig.Quarkchain.NetworkID,
		"chainSize":            s.clusterConfig.Quarkchain.ChainSize,
		"shardServerCount":     s.ConnCount(),
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
		"syncing":              s.IsSyncing(),
		"mining":               s.IsMining(),
		"shards":               shards,
		"peers":                peerForDisplay,
		"minor_block_interval": s.artificialTxConfig.TargetMinorBlockTime,
		"root_block_interval":  s.artificialTxConfig.TargetRootBlockTime,
		"cpus":                 cc,
		"txCountHistory":       txCountHistory,
	}, nil
}

//TODO need delete later
func (s *QKCMasterBackend) disPlayPeers() {
	go func() {
		for true {
			time.Sleep(disPlayPeerInfoInterval)
			peers := s.protocolManager.peers.Peers()
			for _, v := range peers {
				log.Info(s.logInfo, "remote addr", v.RemoteAddr().String())
			}
		}
	}()

}
