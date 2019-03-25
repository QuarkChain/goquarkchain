package master

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/service"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rpc"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
)

type Master struct {
	lock           sync.RWMutex
	config         config.ClusterConfig
	branchToSlaves map[account.Branch][]*SlaveConnection
	slavePool      map[string]*SlaveConnection
	run            int64
}

func New(ctx *service.ServiceContext, cfg *config.ClusterConfig) (*Master, error) {

	master := &Master{
		run:            0,
		config:         *cfg,
		branchToSlaves: make(map[account.Branch][]*SlaveConnection),
		slavePool:      make(map[string]*SlaveConnection),
	}
	for _, shardConfig := range cfg.SlaveList {
		slaveConn := NewSlaveConn(shardConfig)
		target := fmt.Sprintf("%s:%d", shardConfig.Ip, shardConfig.Port)
		master.slavePool[target] = slaveConn
		for shardId := 0; shardId < master.getShardSize(); shardId++ {
			branch, err := account.CreatBranch(
				uint32(cfg.Quarkchain.ShardSize),
				uint32(shardId),
			)
			if err != nil {
				return nil, err
			}
			if slaveConn.HasShard(uint32(shardId)) {
				if _, ok := master.branchToSlaves[branch]; !ok {
					master.branchToSlaves[branch] = make([]*SlaveConnection, 0)
				}
				master.branchToSlaves[branch] = append(
					master.branchToSlaves[branch],
					slaveConn,
				)
			}
		}
	}
	return master, nil
}

func (c *Master) Protocols() (protos []p2p.Protocol) { return nil }

func (c *Master) APIs() []rpc.API {
	apis := []rpc.API{
		{
			Namespace: "rpc." + reflect.TypeOf(MasterServerSideOp{}).Name(),
			Version:   "3.0",
			Service:   NewServerSideOp(),
			Public:    false,
		},
	}
	return apis
}

func (c *Master) Stop() error {
	atomic.StoreInt64(&c.run, 0)
	return nil
}

func (c *Master) Start(srvr *p2p.Server) error {
	atomic.StoreInt64(&c.run, 1)
	c.connecToSlaves()
	return nil
}

func (c *Master) StartMing(threads int) error {
	return nil
}

func (c *Master) getShardSize() int {
	return int(c.config.Quarkchain.ShardSize)
}

func (c *Master) connecToSlaves() {
	slaveList := make([]string, 0)
	for target := range c.slavePool {
		slaveList = append(slaveList, target)
	}
	// TODO request.Data is about root_block, should to to filled later.
	request := qrpc.Request{Op: qrpc.OpPing, Data: nil}
	for atomic.LoadInt64(&c.run) == 1 && len(slaveList) > 0 {
		for i, target := range slaveList {
			_, err := c.slavePool[target].client.GetSlaveSideOp(target, &request)
			if err == nil {
				slaveList = append(slaveList[:i], slaveList[i+1:]...)
				break
			}
			log.Info("master service", "slave list length", len(slaveList), "slave", target)
			time.Sleep(time.Duration(1) * time.Second)
		}
	}
	log.Info("master service", "connect to all slaves successful.")
}
