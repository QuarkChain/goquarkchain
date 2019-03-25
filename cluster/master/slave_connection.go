package master

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/log"
	"io"
)

type SlaveConnection struct {
	target string
	config config.SlaveConfig
	//shardMaskList
	client *qrpc.RPClient
}

// create slave connection manager
func NewSlaveConn(cfg *config.SlaveConfig) *SlaveConnection {
	return &SlaveConnection{
		client: qrpc.NewRPCLient(),
		config: *cfg,
		target: fmt.Sprintf("%s:%d", cfg.Ip, cfg.Port),
	}
}

/*
// wait slaves all alive
func (s *SlaveConnection) ConnecToSlaves() bool {
	var (
		slaveList = make(map[string]bool)
		request   = qrpc.Request{Op: 1, Data: []byte("master ping")}
	)

	for _, cfg := range s.slaveList {
		target := fmt.Sprintf("%s:%d", cfg.Ip, cfg.Port)
		slaveList[target] = false
	}

	for len(slaveList) > 0 {
		for target := range slaveList {
			_, err := s.client.GetSlaveSideOp(target, &request)
			if err == nil {
				log.Info("master service", "successful to connect to slave", target)
				delete(slaveList, target)
				continue
			}

			log.Debug("master service", "can't connect to", target)
			time.Sleep(time.Duration(500) * time.Millisecond)
		}
	}
	log.Info("master service", "all slaves are alive", nil)
	return true
}

func (s *SlaveConnection) SlaveToSlaveConnection() bool {
	var (
		slaveInfos = make([]qrpc.SlaveInfo, 0)
		request    = qrpc.Request{Op: 2}
	)

	for _, cfg := range s.slaveList {
		slaveInfos = append(
			slaveInfos,
			qrpc.SlaveInfo{
				Id:   []byte(cfg.Id),
				Host: []byte(cfg.Ip),
				Port: uint16(cfg.Port),
				// TODO if ChainMask type is defined fill it with thisx`
				ChainMaskList: nil,
			})
	}

	slaveRequest := qrpc.ConnectToSlavesRequest{SlaveInfoList: slaveInfos}
	bytes, err := serialize.SerializeToBytes(slaveRequest)
	if err != nil {
		log.Error("master service", "slave to slave connection", err)
	}
	for _, cfg := range s.slaveList {
		target := fmt.Sprintf("%s:%d", cfg.Ip, cfg.Port)
		request.Data = bytes
		response, err := s.client.GetSlaveSideOp(target, &request)
		if err != io.EOF && err != nil {
			log.Info("master client", "send connect to slaves", err)
			return false
		} else {
			fmt.Println("receive ", response.RpcId, string(response.Data))
		}
	}
	return true
}*/

// validate the slave is alive or not
func (s *SlaveConnection) ValidateConnection(id string) bool {
	if s.config.Id == id {
		target := fmt.Sprintf("%s:%d", s.config.Ip, s.config.Port)
		_, err := s.client.GetSlaveSideOp(target, &qrpc.Request{Op: 1})
		if err == nil {
			return true
		}
	}
	return false
}

func (s *SlaveConnection) SendPing(initializeShardState bool) (string, []types.ChainMask) {
	// TODO fill sendPing operation.
	return "", nil
}

func (s *SlaveConnection) HasShard(fullShardId uint32) bool {
	for _, chainMask := range s.config.ShardMaskList {
		if chainMask.ContainFullShardId(fullShardId) {
			return true
		}
	}
	return false
}

func (s *SlaveConnection) SendConnectToSlaves(slaveInfoList []config.SlaveConfig) bool {
	var (
		slaveInfos = make([]qrpc.SlaveInfo, 0)
	)
	for _, info := range slaveInfoList {
		var chainMask []types.ChainMask
		for _, mask := range info.ShardMaskList {
			chainMask = append(chainMask, *mask)
		}
		slaveInfos = append(
			slaveInfos,
			qrpc.SlaveInfo{
				Id:            []byte(info.Id),
				Host:          []byte(info.Ip),
				Port:          info.Port,
				ChainMaskList: chainMask,
			})
	}
	req := qrpc.ConnectToSlavesRequest{SlaveInfoList: slaveInfos}
	bytes, err := serialize.SerializeToBytes(req)
	if err != nil {
		log.Error("master service", "send connection to slaves", err)
	}
	response, err := s.client.GetSlaveSideOp(s.target, &qrpc.Request{Op: 2, Data: bytes})
	if err != io.EOF && err != nil {
		log.Info("master client", "send connect to slaves", err)
		return false
	} else {
		fmt.Println("receive ", response.RpcId, string(response.Data))
	}
	return true
}
