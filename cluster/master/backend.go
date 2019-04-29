package master

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
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
	"github.com/ethereum/go-ethereum/log"
	ethRPC "github.com/ethereum/go-ethereum/rpc"
	"math/big"
	"os"
	"reflect"
	"sync"
	"syscall"
	"time"
)

var (
	beatTime = 4
)

// QKCMasterBackend masterServer include connections
type QKCMasterBackend struct {
	lock               sync.RWMutex
	engine             consensus.Engine
	eventMux           *event.TypeMux
	chainDb            ethdb.Database
	shutdown           chan os.Signal
	clusterConfig      *config.ClusterConfig
	clientPool         map[string]*SlaveConnection
	branchToSlaves     map[uint32][]*SlaveConnection
	branchToShardStats map[uint32]*rpc.ShardStats
	artificialTxConfig *rpc.ArtificialTxConfig
	rootBlockChain     *core.RootBlockChain
	logInfo            string
}

// New new master with config
func New(ctx *service.ServiceContext, cfg *config.ClusterConfig) (*QKCMasterBackend, error) {
	var (
		mstr = &QKCMasterBackend{
			clusterConfig:      cfg,
			eventMux:           ctx.EventMux,
			clientPool:         make(map[string]*SlaveConnection),
			branchToSlaves:     make(map[uint32][]*SlaveConnection, 0),
			branchToShardStats: make(map[uint32]*rpc.ShardStats),
			artificialTxConfig: &rpc.ArtificialTxConfig{
				TargetRootBlockTime:  cfg.Quarkchain.Root.ConsensusConfig.TargetBlockTime,
				TargetMinorBlockTime: cfg.Quarkchain.GetShardConfigByFullShardID(cfg.Quarkchain.GetGenesisShardIds()[0]).ConsensusConfig.TargetBlockTime,
			},
			logInfo: "masterServer",
		}
		err error
	)
	if mstr.chainDb, err = createDB(ctx, cfg.DbPathRoot); err != nil {
		return nil, err
	}

	if mstr.engine, err = createConsensusEngine(ctx, cfg.Quarkchain.Root); err != nil {
		return nil, err
	}

	genesis := core.NewGenesis(cfg.Quarkchain)
	genesis.MustCommitRootBlock(mstr.chainDb)
	if mstr.rootBlockChain, err = core.NewRootBlockChain(mstr.chainDb, nil, cfg.Quarkchain, mstr.engine, mstr.isLocalBlock); err != nil {
		return nil, err
	}

	return mstr, nil
}

func createDB(ctx *service.ServiceContext, name string) (ethdb.Database, error) {
	db, err := ctx.OpenDatabase(name, 128, 1024)
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
	case "ModeFake":
		return &consensus.FakeEngine{}, nil
	case "POW_ETHASH", "POW_SIMULATE":
		return qkchash.New(cfg.ConsensusConfig.RemoteMine, &diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	case "POW_DOUBLESHA256":
		return doublesha256.New(&diffCalculator, cfg.ConsensusConfig.RemoteMine), nil
	}
	return nil, fmt.Errorf("Failed to create consensus engine consensus type %s", cfg.ConsensusType)
}

func (s *QKCMasterBackend) GetClusterConfig() *config.ClusterConfig {
	return s.clusterConfig
}

func (s *QKCMasterBackend) isLocalBlock(block *types.RootBlock) bool {
	return false
}

// Protocols p2p protocols, p2p Server will start in node.Start
func (s *QKCMasterBackend) Protocols() []p2p.Protocol { return nil }

// APIs return all apis for master Server
func (s *QKCMasterBackend) APIs() []ethRPC.API {
	apis := qkcapi.GetAPIs(s)
	return append(apis, []ethRPC.API{
		{
			Namespace: "rpc." + reflect.TypeOf(QKCMasterServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(s),
			Public:    false,
		},
	}...)
}

// Stop stop node -> stop qkcMaster
func (s *QKCMasterBackend) Stop() error {
	if s.engine != nil {
		s.engine.Close()
	}
	s.eventMux.Stop()
	return nil
}

// Start start node -> start qkcMaster
func (s *QKCMasterBackend) Start(srvr *p2p.Server) error {
	// start heart beat pre 3 seconds.
	s.HeartBeat()
	return nil
}

// StartMining start mining
func (s *QKCMasterBackend) StartMining(threads int) error {
	return nil
}

// StopMining stop mining
func (s *QKCMasterBackend) StopMining(threads int) error {
	return nil
}

// InitCluster init cluster :
//                          1:ConnectToSlaves
// 							2:logSummary
// 							3:check if has all shards
// 							4.setup slave to slave
//                          5:init shards
func (s *QKCMasterBackend) InitCluster() error {
	if err := s.ConnectToSlaves(); err != nil {
		return err
	}
	s.logSummary()
	if err := s.hasAllShards(); err != nil {
		return err
	}
	if err := s.setUpSlaveToSlaveConnections(); err != nil {
		return err
	}
	if err := s.initShards(); err != nil {
		return err
	}
	return nil
}

func (s *QKCMasterBackend) ConnectToSlaves() error {
	for _, cfg := range s.clusterConfig.SlaveList {
		target := fmt.Sprintf("%s:%d", cfg.IP, cfg.Port)
		client := NewSlaveConn(target, cfg.ChainMaskList, cfg.ID)
		s.clientPool[target] = client
	}
	log.Info("qkc api backend", "slave client pool", len(s.clientPool))

	fullShardIDS := s.clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, slaveConn := range s.clientPool {
		id, chainMaskList, err := slaveConn.SendPing(s.rootBlockChain, false)
		if err := checkPing(slaveConn, id, chainMaskList, err); err != nil {
			return err
		}
		for _, fullShardID := range fullShardIDS {
			if slaveConn.hasShard(fullShardID) {
				s.saveFullShardID(fullShardID, slaveConn)
			}
		}
	}
	return nil
}
func (s *QKCMasterBackend) logSummary() {
	for branch, slaves := range s.branchToSlaves {
		for _, slave := range slaves {
			log.Info(s.logInfo, "branch:", branch, "is run by slave", slave.slaveID)
		}
	}
}

func (s *QKCMasterBackend) hasAllShards() error {
	if len(s.branchToSlaves) == len(s.clusterConfig.Quarkchain.GetGenesisShardIds()) {
		for _, v := range s.branchToSlaves {
			if len(v) <= 0 {
				return errors.New("branch's slave<=0")
			}
		}
		return nil
	}
	return errors.New("len not match")
}
func (s *QKCMasterBackend) setUpSlaveToSlaveConnections() error {
	for _, slave := range s.clientPool {
		err := slave.SendConnectToSlaves(s.getSlaveInfoListFromClusterConfig())
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *QKCMasterBackend) getSlaveInfoListFromClusterConfig() []*rpc.SlaveInfo {
	slaveInfos := make([]*rpc.SlaveInfo, 0)
	for _, slave := range s.clusterConfig.SlaveList {
		slaveInfos = append(slaveInfos, &rpc.SlaveInfo{
			Id:            []byte(slave.ID),
			Host:          []byte(slave.IP),
			Port:          slave.Port,
			ChainMaskList: slave.ChainMaskList,
		})
	}
	return slaveInfos
}
func (s *QKCMasterBackend) initShards() error {
	lenPool := len(s.clientPool)
	check := NewCheckErr(lenPool)
	for index := range s.clientPool {
		check.wg.Add(1)
		go func(slaveConn *SlaveConnection) {
			defer check.wg.Done()
			_, _, err := slaveConn.SendPing(s.rootBlockChain, true)
			check.errc <- err
		}(s.clientPool[index])
	}
	check.wg.Wait()
	return check.check()
}

func (s *QKCMasterBackend) HeartBeat() {
	go func(normal bool) {
		for normal {
			time.Sleep(time.Duration(beatTime) * time.Second)
			for endpoint := range s.clientPool {
				normal = s.clientPool[endpoint].HeartBeat()
				if !normal {
					s.shutdown <- syscall.SIGTERM
					break
				}
			}
		}
	}(true)
}

func (s *QKCMasterBackend) saveFullShardID(fullShardID uint32, slaveConn *SlaveConnection) {
	if _, ok := s.branchToSlaves[fullShardID]; !ok {
		s.branchToSlaves[fullShardID] = make([]*SlaveConnection, 0)
	}
	s.branchToSlaves[fullShardID] = append(s.branchToSlaves[fullShardID], slaveConn)
}

func checkPing(slaveConn *SlaveConnection, id []byte, chainMaskList []*types.ChainMask, err error) error {
	if err != nil {
		return err
	}
	if slaveConn.slaveID != string(id) {
		return errors.New("slaveID is not match")
	}
	if len(chainMaskList) != len(slaveConn.chainMaskLst) {
		return errors.New("chainMaskList is not match")
	}
	lenChainMaskList := len(chainMaskList)

	for index := 0; index < lenChainMaskList; index++ {
		if chainMaskList[index].GetMask() != slaveConn.chainMaskLst[index].GetMask() {
			return errors.New("chainMaskList index is not match")
		}
	}
	return nil
}

func (s *QKCMasterBackend) getSlaveConnection(branch account.Branch) (*SlaveConnection, error) {
	slaves, ok := s.branchToSlaves[branch.Value]
	if !ok {
		return nil, errors.New("no such branch")
	}
	if len(slaves) < 1 {
		return nil, errors.New("len slaves <1")
	}
	return slaves[0], nil
}

func (s *QKCMasterBackend) createRootBlockToMine(address account.Address) (*types.RootBlock, error) {
	lenSlaves := len(s.clientPool)
	check := NewCheckErr(lenSlaves)
	chanRsp := make(chan *rpc.GetUnconfirmedHeadersResponse, lenSlaves)
	for index := range s.clientPool {
		check.wg.Add(1)
		go func(slaveConn *SlaveConnection) {
			defer check.wg.Done()
			rsp, err := slaveConn.GetUnconfirmedHeaders()
			check.errc <- err
			chanRsp <- rsp
		}(s.clientPool[index])
	}
	check.wg.Wait()
	close(chanRsp)
	if err := check.check(); err != nil {
		return nil, err
	}

	fullShardIDToHeaderList := make(map[uint32][]*types.MinorBlockHeader, 0)
	for resp := range chanRsp {
		for _, headersInfo := range resp.HeadersInfoList {
			height := uint64(0)
			for _, header := range headersInfo.HeaderList {
				if height != 0 && height+1 != header.Number {
					return nil, errors.New("headers must ordered by height")
				}
				height = header.Number

				if !s.rootBlockChain.IsMinorBlockValidated(header.Hash()) {
					break
				}
				if _, ok := fullShardIDToHeaderList[headersInfo.Branch.Value]; !ok {
					fullShardIDToHeaderList[headersInfo.Branch.Value] = make([]*types.MinorBlockHeader, 0)
				}
				fullShardIDToHeaderList[headersInfo.Branch.Value] = append(fullShardIDToHeaderList[headersInfo.Branch.Value], header)
			}
		}
	}
	headerList := make([]*types.MinorBlockHeader, 0)
	currTipHeight := s.rootBlockChain.CurrentBlock().Number()
	fullShardIDToCHeck := s.clusterConfig.Quarkchain.GetInitializedShardIdsBeforeRootHeight(currTipHeight + 1)
	for _, fullShardID := range fullShardIDToCHeck {
		headers := fullShardIDToHeaderList[fullShardID]
		headerList = append(headerList, headers...)
	}
	return s.rootBlockChain.CreateBlockToMine(headerList, &address, nil), nil
}

func (s *QKCMasterBackend) getMinorBlockToMine(branch account.Branch, address account.Address) (*types.MinorBlock, error) {
	slaveConn, err := s.getSlaveConnection(branch)
	if err != nil {
		return nil, err
	}
	return slaveConn.GetMinorBlockToMine(branch, address, s.artificialTxConfig)

}

// GetAccountData get account Data for jsonRpc
func (s *QKCMasterBackend) GetAccountData(address account.Address) (map[account.Branch]*rpc.AccountBranchData, error) {
	lenSlaves := len(s.clientPool)
	check := NewCheckErr(lenSlaves)
	chanRsp := make(chan *rpc.GetAccountDataResponse, lenSlaves)
	for index := range s.clientPool {
		check.wg.Add(1)
		go func(slaveConn *SlaveConnection) {
			defer check.wg.Done()
			rsp, err := slaveConn.GetAccountData(address, nil)
			check.errc <- err
			chanRsp <- rsp
		}(s.clientPool[index])
	}
	check.wg.Wait()
	close(chanRsp)
	if err := check.check(); err != nil {
		return nil, err
	}
	branchToAccountBranchData := make(map[account.Branch]*rpc.AccountBranchData)
	for rsp := range chanRsp {
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
func (s *QKCMasterBackend) GetPrimaryAccountData(address account.Address, blockHeight *uint64) (*rpc.AccountBranchData, error) {
	fullShardID := s.clusterConfig.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
	slaveConn, err := s.getSlaveConnection(account.Branch{Value: fullShardID})
	if err != nil {
		return nil, err
	}
	rsp, err := slaveConn.GetAccountData(address, blockHeight)
	if err != nil {
		return nil, err
	}
	for _, accountBranchData := range rsp.AccountBranchDataList {
		if accountBranchData.Branch.Value == fullShardID {
			return accountBranchData, nil
		}
	}
	return nil, errors.New("no such data")
}

// SendMiningConfigToSlaves send mining config to slaves,used in jsonRpc
func (s *QKCMasterBackend) SendMiningConfigToSlaves(mining bool) error {
	lenSlaves := len(s.clientPool)
	check := NewCheckErr(lenSlaves)
	for index := range s.clientPool {
		check.wg.Add(1)
		go func(slaveConn *SlaveConnection) {
			defer check.wg.Done()
			err := slaveConn.SendMiningConfigToSlaves(s.artificialTxConfig, mining)
			check.errc <- err
		}(s.clientPool[index])
	}
	check.wg.Wait()
	return check.check()
}

// AddRootBlock add root block to all slaves
func (s *QKCMasterBackend) AddRootBlock(rootBlock *types.RootBlock) error {
	_, err := s.rootBlockChain.InsertChain([]types.IBlock{rootBlock})
	if err != nil {
		return err
	}
	lenSlaves := len(s.clientPool)
	check := NewCheckErr(lenSlaves)
	for index := range s.clientPool {
		check.wg.Add(1)
		go func(slaveConn *SlaveConnection) {
			defer check.wg.Done()
			err := slaveConn.AddRootBlock(rootBlock, false)
			check.errc <- err
		}(s.clientPool[index])
	}
	check.wg.Wait()
	return check.check()
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
	if mining {
		return s.StartMining(1)
	}
	return s.StopMining(1)
}

// CreateTransactions Create transactions and add to the network for load testing
func (s *QKCMasterBackend) CreateTransactions(numTxPerShard, xShardPercent uint32, tx *types.Transaction) error {
	lenSlaves := len(s.clientPool)
	check := NewCheckErr(lenSlaves)
	for index := range s.clientPool {
		check.wg.Add(1)
		go func(slaveConn *SlaveConnection) {
			defer check.wg.Done()
			err := slaveConn.GenTx(numTxPerShard, xShardPercent, tx)
			check.errc <- err
		}(s.clientPool[index])
	}
	check.wg.Wait()
	return check.check()
}

// UpdateShardStatus update shard status for branchg
func (s *QKCMasterBackend) UpdateShardStatus(status *rpc.ShardStats) {
	s.lock.Lock()
	s.branchToShardStats[status.Branch.Value] = status
	s.lock.Unlock()
}

// UpdateTxCountHistory update Tx count queue
func (s *QKCMasterBackend) UpdateTxCountHistory(txCount, xShardTxCount uint32, createTime uint64) {
	panic("not implement")
}

func (s *QKCMasterBackend) GetBlockCount() map[string]interface{} {
	panic("not implement")
}

func (s *QKCMasterBackend) GetStats() map[string]interface{} {
	panic("not implement")
	//TODO :only calc
}

func (s *QKCMasterBackend) isSyning() bool {
	return false
}

func (s *QKCMasterBackend) isMining() bool {
	return false
}

func (s *QKCMasterBackend) CurrentBlock() *types.RootBlock {
	return s.rootBlockChain.CurrentBlock()
}

type CheckErr struct {
	len  int
	errc chan error
	wg   sync.WaitGroup
}

// NewCheckErr needed len for check
func NewCheckErr(len int) *CheckErr {
	return &CheckErr{
		errc: make(chan error, len),
		len:  len,
	}
}

// check if it has err in chans
func (c *CheckErr) check() error {
	c.wg.Wait()
	close(c.errc)
	if len(c.errc) != c.len {
		return errors.New("len is not match")
	}
	for err := range c.errc {
		if err != nil {
			return err
		}
	}
	return nil
}
