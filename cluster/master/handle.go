package master

import (
	"fmt"
	"io/ioutil"
	"reflect"
	"sync"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	synchronizer "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
)

// QKCProtocol details
const (
	QKCProtocolName     = "quarkchain"
	QKCProtocolVersion  = 1
	QKCProtocolLength   = 16
	chainHeadChanSize   = 10
	forceSyncCycle      = 1000 * time.Second
	minDesiredPeerCount = 0
)

// ProtocolManager QKC manager
type ProtocolManager struct {
	networkID      uint32
	rootBlockChain *core.RootBlockChain
	clusterConfig  *config.ClusterConfig

	subProtocols     []p2p.Protocol
	getShardConnFunc func(fullShardId uint32) []rpc.ShardConnForP2P
	synchronizer     synchronizer.Synchronizer

	chainHeadChan     chan core.RootChainHeadEvent
	chainHeadEventSub event.Subscription
	statsChan         chan *rpc.ShardStatus

	maxPeers    int
	peers       *peerSet // Set of active peers from which rootDownloader can proceed
	newPeerCh   chan *peer
	quitSync    chan struct{}
	noMorePeers chan struct{}

	log string
	wg  sync.WaitGroup
}

// NewQKCManager  new qkc manager
func NewProtocolManager(env config.ClusterConfig, rootBlockChain *core.RootBlockChain, statsChan chan *rpc.ShardStatus, synchronizer synchronizer.Synchronizer, getShardConnFunc func(fullShardId uint32) []rpc.ShardConnForP2P) (*ProtocolManager, error) {
	manager := &ProtocolManager{
		networkID:        env.Quarkchain.NetworkID,
		rootBlockChain:   rootBlockChain,
		clusterConfig:    &env,
		peers:            newPeerSet(),
		newPeerCh:        make(chan *peer),
		quitSync:         make(chan struct{}),
		noMorePeers:      make(chan struct{}),
		statsChan:        statsChan,
		synchronizer:     synchronizer,
		getShardConnFunc: getShardConnFunc,
	}
	protocol := p2p.Protocol{
		Name:    QKCProtocolName,
		Version: QKCProtocolVersion,
		Length:  QKCProtocolLength,
		Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
			peer := newPeer(int(QKCProtocolVersion), p, rw)

			return manager.handle(peer)

		},
	}
	manager.subProtocols = []p2p.Protocol{protocol}
	return manager, nil

}

func (pm *ProtocolManager) removePeer(id string) {
	// Short circuit if the peer was already removed
	peer := pm.peers.Peer(id)
	if peer == nil {
		return
	}
	log.Debug("Removing peer", "peer", id)

	if err := pm.peers.Unregister(id); err != nil {
		log.Error("Peer removal failed", "peer", id, "err", err)
	}
	// Hard disconnect at the networking layer
	if peer != nil {
		peer.Peer.Disconnect(p2p.DiscUselessPeer)
	}
}

// Start manager start
func (pm *ProtocolManager) Start(maxPeers int) {
	pm.maxPeers = maxPeers

	pm.chainHeadChan = make(chan core.RootChainHeadEvent, chainHeadChanSize)
	pm.chainHeadEventSub = pm.rootBlockChain.SubscribeChainHeadEvent(pm.chainHeadChan)
	go pm.tipBroadcastLoop()
	go pm.syncer()
}

func (pm *ProtocolManager) Stop() {
	log.Info("Stopping Master protocol")

	pm.chainHeadEventSub.Unsubscribe()

	// Quit the sync loop.
	// After this send has completed, no new peers will be accepted.
	pm.noMorePeers <- struct{}{}

	close(pm.quitSync)

	// Disconnect existing sessions.
	// This also closes the gate for any new registrations on the peer set.
	// sessions which are already established but not added to pm.peers yet
	// will exit when they try to register.
	pm.peers.Close()

	// Wait for all peer handler goroutines and the loops to come down.
	pm.wg.Wait()

	log.Info("cluster protocol stopped")
}

func (pm *ProtocolManager) handle(peer *peer) error {
	if pm.peers.Len() >= pm.maxPeers {
		return p2p.DiscTooManyPeers
	}
	peer.Log().Debug("peer connected", "name", peer.Name())

	privateKey, _ := p2p.GetPrivateKeyFromConfig(pm.clusterConfig.P2P.PrivKey)
	id := crypto.FromECDSAPub(&privateKey.PublicKey)
	if err := peer.Handshake(pm.clusterConfig.Quarkchain.P2PProtocolVersion,
		pm.networkID,
		common.BytesToHash(id),
		uint16(pm.clusterConfig.P2PPort),
		pm.rootBlockChain.CurrentBlock().Header(),
	); err != nil {
		return err
	}

	// Register the peer locally
	if err := pm.peers.Register(peer); err != nil {
		peer.Log().Error("peer registration failed", "err", err)
		return err
	}
	defer pm.removePeer(peer.id)

	pm.newPeerCh <- peer

	// currently we do not broadcast old transaction when connect
	// so the first few block may not have transaction verification failed
	// or transaction drop issue which is temp issue
	// we can add pm.syncTransactions(p) later

	for {
		if err := pm.handleMsg(peer); err != nil {
			peer.Log().Error("message handling failed", "err", err)
			_ = pm.peers.Unregister(peer.id)
			return err
		}
	}
}

func (pm *ProtocolManager) handleMsg(peer *peer) error {
	msg, err := peer.rw.ReadMsg()
	if err != nil {
		return err
	}
	payload, err := ioutil.ReadAll(msg.Payload)
	qkcMsg, err := p2p.DecodeQKCMsg(payload)
	if err != nil {
		return err
	}

	log.Debug(pm.log, " receive QKC Msgop", qkcMsg.Op.String())
	switch {
	case qkcMsg.Op == p2p.Hello:
		return errors.New("Unexpected Hello msg")

	case qkcMsg.Op == p2p.NewTipMsg:
		var tip p2p.Tip
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &tip); err != nil {
			return err
		}
		if tip.RootBlockHeader == nil {
			return fmt.Errorf("invalid NewTip Request: RootBlockHeader is nil. %d for rpc request %d",
				qkcMsg.RpcID, qkcMsg.MetaData.Branch)
		}
		// handle root tip when branch == 0
		if qkcMsg.MetaData.Branch == 0 {
			return pm.HandleNewRootTip(&tip, peer)
		}
		return pm.HandleNewMinorTip(qkcMsg.MetaData.Branch, &tip, peer)

	case qkcMsg.Op == p2p.NewTransactionListMsg:
		var trans p2p.NewTransactionList
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &trans); err != nil {
			return err
		}
		if qkcMsg.MetaData.Branch != 0 {
			return pm.HandleNewTransactionListRequest(peer.id, qkcMsg.RpcID, qkcMsg.MetaData.Branch, &trans)
		}
		branchTxMap := make(map[uint32][]*types.Transaction)
		for _, tx := range trans.TransactionList {
			toShardSize := pm.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.ToChainID())
			if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
				return err
			}
			fromShardSize := pm.clusterConfig.Quarkchain.GetShardSizeByChainId(tx.EvmTx.FromChainID())
			if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
				return err
			}
			branchTxMap[tx.EvmTx.FromFullShardId()] = append(branchTxMap[tx.EvmTx.FromFullShardId()], tx)
		}
		// todo make them run in Parallelized
		for branch, list := range branchTxMap {
			if err := pm.HandleNewTransactionListRequest(peer.id, qkcMsg.RpcID, branch, &p2p.NewTransactionList{TransactionList: list}); err != nil {
				return err
			}
		}

	case qkcMsg.Op == p2p.NewBlockMinorMsg:
		var newBlockMinor p2p.NewBlockMinor
		branch := qkcMsg.MetaData.Branch
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &newBlockMinor); err != nil {
			return err
		}
		if branch != newBlockMinor.Block.Branch().Value {
			return fmt.Errorf("invalid NewBlockMinor Request: mismatch branch value from peer %v. in request meta: %d, in minor header: %d",
				peer.id, branch, newBlockMinor.Block.Branch().Value)
		}
		tip := peer.MinorHead(branch)
		if tip == nil {
			tip = new(p2p.Tip)
			tip.MinorBlockHeaderList = make([]*types.MinorBlockHeader, 1, 1)
		}
		tip.MinorBlockHeaderList[0] = newBlockMinor.Block.Header()
		peer.SetMinorHead(branch, tip)
		clients := pm.getShardConnFunc(branch)
		if len(clients) == 0 {
			return fmt.Errorf("invalid branch %d for rpc request %d", qkcMsg.RpcID, branch)
		}
		//todo make them run in Parallelized
		for _, client := range clients {
			result, err := client.HandleNewMinorBlock(&newBlockMinor)
			if err != nil {
				return fmt.Errorf("branch %d handle NewBlockMinorMsg message failed with error: %v", branch, err.Error())
			}
			if !result {
				return fmt.Errorf("AddMinorBlock (rpcId %d) for branch %d return false",
					qkcMsg.RpcID, branch)
			}
		}

	case qkcMsg.Op == p2p.GetRootBlockHeaderListRequestMsg:
		var blockHeaderReq p2p.GetRootBlockHeaderListRequest
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockHeaderReq); err != nil {
			return err
		}

		resp, err := pm.HandleGetRootBlockHeaderListRequest(&blockHeaderReq)
		if err != nil {
			return err
		}

		return peer.SendResponse(p2p.GetRootBlockHeaderListResponseMsg, p2p.Metadata{Branch: 0}, qkcMsg.RpcID, resp)

	case qkcMsg.Op == p2p.GetRootBlockHeaderListResponseMsg:
		var blockHeaderResp p2p.GetRootBlockHeaderListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockHeaderResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- blockHeaderResp.BlockHeaderList
		} else {
			log.Warn(fmt.Sprintf("chan for rpc %d is missing", qkcMsg.RpcID))
		}

	case qkcMsg.Op == p2p.GetRootBlockListRequestMsg:
		var rootBlockReq p2p.GetRootBlockListRequest
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &rootBlockReq); err != nil {
			return err
		}

		resp, err := pm.HandleGetRootBlockListRequest(&rootBlockReq)
		if err != nil {
			return err
		}

		return peer.SendResponse(p2p.GetRootBlockListResponseMsg, p2p.Metadata{Branch: 0}, qkcMsg.RpcID, resp)

	case qkcMsg.Op == p2p.GetRootBlockListResponseMsg:
		var blockResp p2p.GetRootBlockListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- blockResp.RootBlockList
		} else {
			log.Warn(fmt.Sprintf("chan for rpc %d is missing", qkcMsg.RpcID))
		}

	case qkcMsg.Op == p2p.GetMinorBlockHeaderListRequestMsg:
		var minorHeaderReq p2p.GetMinorBlockHeaderListRequest
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorHeaderReq); err != nil {
			return err
		}

		resp, err := pm.HandleGetMinorBlockHeaderListRequest(qkcMsg.RpcID, qkcMsg.MetaData.Branch, &minorHeaderReq)
		if err != nil {
			return err
		}

		return peer.SendResponse(p2p.GetMinorBlockHeaderListResponseMsg, p2p.Metadata{Branch: qkcMsg.MetaData.Branch}, qkcMsg.RpcID, resp)

	case qkcMsg.Op == p2p.GetMinorBlockHeaderListResponseMsg:
		var minorHeaderResp p2p.GetMinorBlockHeaderListResponse

		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorHeaderResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- minorHeaderResp.BlockHeaderList
		} else {
			log.Warn(fmt.Sprintf("chan for rpc %d is missing", qkcMsg.RpcID))
		}

	case qkcMsg.Op == p2p.GetMinorBlockListRequestMsg:
		var minorBlockReq p2p.GetMinorBlockListRequest
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorBlockReq); err != nil {
			return err
		}

		resp, err := pm.HandleGetMinorBlockListRequest(peer.id, qkcMsg.RpcID, qkcMsg.MetaData.Branch, &minorBlockReq)
		if err != nil {
			return err
		}
		resp.MinorBlockList[0].NumberU64()
		return peer.SendResponse(p2p.GetMinorBlockListResponseMsg, p2p.Metadata{Branch: qkcMsg.MetaData.Branch}, qkcMsg.RpcID, resp)

	case qkcMsg.Op == p2p.GetMinorBlockListResponseMsg:
		var minorBlockResp p2p.GetMinorBlockListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorBlockResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- minorBlockResp.MinorBlockList
		} else {
			log.Warn(fmt.Sprintf("chan for rpc %d is missing", qkcMsg.RpcID))
		}

	default:
		return fmt.Errorf("unknown msg code %d", qkcMsg.Op)
	}
	return nil
}

func (pm *ProtocolManager) HandleNewRootTip(tip *p2p.Tip, peer *peer) error {
	if len(tip.MinorBlockHeaderList) != 0 {
		return errors.New("minor block header list must not be empty")
	}
	head := peer.RootHead()
	if head != nil && tip.RootBlockHeader.NumberU64() < head.NumberU64() {
		return fmt.Errorf("root block height is decreasing %d < %d", tip.RootBlockHeader.NumberU64(), head.NumberU64())
	}
	if head != nil && tip.RootBlockHeader.NumberU64() == head.NumberU64() && tip.RootBlockHeader.Hash() != head.Hash() {
		return fmt.Errorf("root block header changed with same height %d", tip.RootBlockHeader.NumberU64())
	}
	peer.SetRootHead(tip.RootBlockHeader)
	if tip.RootBlockHeader.NumberU64() > pm.rootBlockChain.CurrentBlock().NumberU64() {
		err := pm.synchronizer.AddTask(synchronizer.NewRootChainTask(peer, tip.RootBlockHeader, pm.statsChan, pm.getShardConnFunc))
		if err != nil {
			log.Error("Failed to add root chain task,", "hash", tip.RootBlockHeader.Hash(), "height", tip.RootBlockHeader.NumberU64())
		}
	}
	return nil
}

func (pm *ProtocolManager) HandleNewMinorTip(branch uint32, tip *p2p.Tip, peer *peer) error {
	// handle minor tip when branch != 0 and the minor block only contain 1 heard which is the tip block
	if len(tip.MinorBlockHeaderList) != 1 {
		return fmt.Errorf("invalid NewTip Request: len of MinorBlockHeaderList is %d for branch %d from peer %v",
			len(tip.MinorBlockHeaderList), branch, peer.id)
	}
	if branch != tip.MinorBlockHeaderList[0].Branch.Value {
		return fmt.Errorf("invalid NewTip Request: mismatch branch value from peer %v. in request meta: %d, in minor header: %d",
			peer.id, branch, tip.MinorBlockHeaderList[0].Branch.Value)
	}
	if minorTip := peer.MinorHead(branch); minorTip != nil && minorTip.RootBlockHeader != nil {
		if minorTip.RootBlockHeader.Number > tip.RootBlockHeader.Number {
			return fmt.Errorf("best observed root header height is decreasing %d < %d",
				tip.RootBlockHeader.Number, minorTip.RootBlockHeader.Number)
		}
		if minorTip.RootBlockHeader.Number == tip.RootBlockHeader.Number &&
			minorTip.RootBlockHeader.Hash() != tip.RootBlockHeader.Hash() {
			return fmt.Errorf("best observed root header changed with same height %d", minorTip.RootBlockHeader.Number)
		}
		if minorTip.RootBlockHeader.Number == tip.RootBlockHeader.Number &&
			minorTip.MinorBlockHeaderList[0].Number > tip.MinorBlockHeaderList[0].Number {
			return fmt.Errorf("best observed minor header is decreasing %d < %d",
				tip.MinorBlockHeaderList[0].Number, minorTip.MinorBlockHeaderList[0].Number)
		}
	}
	peer.SetMinorHead(branch, tip)
	clients := pm.getShardConnFunc(branch)
	if len(clients) == 0 {
		return fmt.Errorf("invalid branch %d for rpc request from peer %v", branch, peer.id)
	}
	// todo make the client call in Parallelized
	for _, client := range clients {
		req := &rpc.HandleNewTipRequest{
			RootBlockHeader:      tip.RootBlockHeader,
			MinorBlockHeaderList: tip.MinorBlockHeaderList,
			PeerID:               peer.id,
		}
		result, err := client.HandleNewTip(req)
		if err != nil {
			return fmt.Errorf("branch %d handle NewTipMsg message failed with error: %v", branch, err.Error())
		}
		if !result {
			return fmt.Errorf("HandleNewRootTip for branch %d with height %d return false",
				branch, tip.MinorBlockHeaderList[0].NumberU64())
		}
	}
	return nil
}

func (pm *ProtocolManager) HandleGetRootBlockHeaderListRequest(blockHeaderReq *p2p.GetRootBlockHeaderListRequest) (*p2p.GetRootBlockHeaderListResponse, error) {
	if !pm.rootBlockChain.HasHeader(blockHeaderReq.BlockHash) {
		return nil, fmt.Errorf("hash %v do not exist", blockHeaderReq.BlockHash.Hex())
	}
	if blockHeaderReq.Limit > rootBlockHeaderListLimit {
		return nil, fmt.Errorf("limit in request is larger than expected, limit: %d, want: %d", blockHeaderReq.Limit, rootBlockHeaderListLimit)
	}
	if blockHeaderReq.Direction != directionToGenesis {
		return nil, errors.New("Bad direction")
	}
	blockHeaderResp := p2p.GetRootBlockHeaderListResponse{
		RootTip:         pm.rootBlockChain.CurrentHeader().(*types.RootBlockHeader),
		BlockHeaderList: make([]*types.RootBlockHeader, 0, blockHeaderReq.Limit),
	}

	hash := blockHeaderReq.BlockHash
	for i := uint32(0); i < blockHeaderReq.Limit; i++ {
		header := pm.rootBlockChain.GetHeader(hash)
		if header == nil || reflect.ValueOf(header).IsNil() {
			panic(fmt.Sprintf("hash %v is missing from DB which is not expected", hash))
		}
		hash = header.GetParentHash()
		blockHeaderResp.BlockHeaderList = append(blockHeaderResp.BlockHeaderList, header.(*types.RootBlockHeader))
		if header.NumberU64() == 0 {
			break
		}
	}
	return &blockHeaderResp, nil
}

func (pm *ProtocolManager) HandleGetRootBlockListRequest(request *p2p.GetRootBlockListRequest) (*p2p.GetRootBlockListResponse, error) {
	size := len(request.RootBlockHashList)
	if size > rootBlockBatchSize {
		return nil, fmt.Errorf("len of RootBlockHashList is larger than expected, limit: %d, want: %d", size, rootBlockBatchSize)
	}
	response := p2p.GetRootBlockListResponse{
		RootBlockList: make([]*types.RootBlock, 0, size),
	}

	for _, hash := range request.RootBlockHashList {
		block := pm.rootBlockChain.GetBlock(hash)
		if block != nil {
			response.RootBlockList = append(response.RootBlockList, block.(*types.RootBlock))
		}
	}
	return &response, nil
}

func (pm *ProtocolManager) HandleNewTransactionListRequest(peerId string, rpcId uint64, branch uint32, request *p2p.NewTransactionList) error {
	clients := pm.getShardConnFunc(branch)
	if len(clients) == 0 {
		return fmt.Errorf("invalid branch %d for rpc request %d", rpcId, branch)
	}
	var hashList []common.Hash
	sameResponse := true
	// todo make the client call in Parallelized
	for _, client := range clients {
		result, err := client.AddTransactions(request)
		if err != nil {
			return fmt.Errorf("branch %d HandleNewTransactionListRequest failed with error: %v", rpcId, err.Error())
		}
		if hashList == nil {
			hashList = result.Hashes
		} else if len(hashList) != len(result.Hashes) {
			sameResponse = false
		} else {
			for i := 0; i < len(hashList); i++ {
				if hashList[i] != result.Hashes[i] {
					sameResponse = false
					break
				}
			}
		}
	}

	if !sameResponse {
		panic("same shard in different slave is inconsistent")
	}
	if len(hashList) > 0 {
		tx2broadcast := make([]*types.Transaction, 0, len(request.TransactionList))
		for _, tx := range request.TransactionList {
			for _, hash := range hashList {
				if tx.Hash() == hash {
					tx2broadcast = append(tx2broadcast, tx)
					break
				}
			}
		}
		pm.BroadcastTransactions(branch, tx2broadcast, peerId)
	}
	return nil
}

func (pm *ProtocolManager) HandleGetMinorBlockHeaderListRequest(rpcId uint64, branch uint32, request *p2p.GetMinorBlockHeaderListRequest) (*p2p.GetMinorBlockHeaderListResponse, error) {
	if request.Limit > minorBlockHeaderListLimit {
		return nil, fmt.Errorf("bad limit. rpcId: %d; branch: %d; limit: %d; expected limit: %d",
			rpcId, branch, request.Limit, minorBlockHeaderListLimit)
	}
	if request.Direction != directionToGenesis {
		return nil, fmt.Errorf("Bad direction. rpcId: %d; branch: %d; ", rpcId, branch)
	}
	clients := pm.getShardConnFunc(branch)
	if len(clients) == 0 {
		return nil, fmt.Errorf("invalid branch %d for rpc request %d", rpcId, branch)
	}
	result, err := clients[0].GetMinorBlockHeaders(request)
	if err != nil {
		return nil, fmt.Errorf("branch %d HandleGetMinorBlockHeaderListRequest failed with error: %v", branch, err.Error())
	}

	return result, nil
}

func (pm *ProtocolManager) HandleGetMinorBlockListRequest(peerId string, rpcId uint64, branch uint32, request *p2p.GetMinorBlockListRequest) (*p2p.GetMinorBlockListResponse, error) {
	if len(request.MinorBlockHashList) > minorBlockBatchSize {
		return nil, fmt.Errorf("bad number of minor blocks requested. rpcId: %d; branch: %d; limit: %d; expected limit: %d",
			rpcId, branch, len(request.MinorBlockHashList), minorBlockBatchSize)
	}
	clients := pm.getShardConnFunc(branch)
	if len(clients) == 0 {
		return nil, fmt.Errorf("invalid branch %d for rpc request %d", rpcId, branch)
	}
	result, err := clients[0].GetMinorBlocks(&rpc.GetMinorBlockListRequest{Branch: branch, PeerId: peerId, MinorBlockHashList: request.MinorBlockHashList})
	if err != nil {
		return nil, fmt.Errorf("branch %d HandleGetMinorBlockListRequest failed with error: %v", branch, err.Error())
	}

	return result, nil
}

func (pm *ProtocolManager) BroadcastTip(header *types.RootBlockHeader) {
	for _, peer := range pm.peers.Peers() {
		if peer.RootHead() != nil && header.Number <= peer.RootHead().Number {
			continue
		}
		peer.AsyncSendNewTip(0, &p2p.Tip{RootBlockHeader: header, MinorBlockHeaderList: nil})
	}
	log.Trace("Announced block", "hash", header.Hash(), "recipients", pm.peers.Len())
}

func (pm *ProtocolManager) tipBroadcastLoop() {
	for {
		select {
		case event := <-pm.chainHeadChan:
			pm.BroadcastTip(event.Block.Header())

		// Err() channel will be closed when unsubscribing.
		case <-pm.chainHeadEventSub.Err():
			return
		}
	}
}

func (pm *ProtocolManager) BroadcastTransactions(branch uint32, txs []*types.Transaction, sourcePeerId string) {
	for _, peer := range pm.peers.Peers() {
		if peer.id != sourcePeerId {
			peer.AsyncSendTransactions(branch, txs)
		}
	}
	log.Trace("Announced transaction", "count", len(txs), "recipients", pm.peers.Len()-1)
}

// syncer is responsible for periodically synchronising with the network, both
// downloading hashes and blocks as well as handling the announcement handler.
func (pm *ProtocolManager) syncer() {

	// Wait for different events to fire synchronisation operations
	forceSync := time.NewTicker(forceSyncCycle)
	defer forceSync.Stop()

	for {
		select {
		case <-pm.newPeerCh:
			// Make sure we have peers to select from, then sync
			if pm.peers.Len() < minDesiredPeerCount {
				break
			}
			go pm.synchronise(pm.peers.BestPeer())

		case <-forceSync.C:
			go pm.synchronise(pm.peers.BestPeer())

		case <-pm.noMorePeers:
			return
		}
	}
}

// synchronise tries to sync up our local block chain with a remote peer.
func (pm *ProtocolManager) synchronise(peer *peer) {
	fmt.Println("AAAAAAAAAA")
	// Short circuit if no peers are available
	if peer == nil {
		fmt.Println("?????????????????????")
		return
	}
	if peer.RootHead() != nil {
		fmt.Println("AAAAAAAAAAAAAAAAAAAAA")
		err := pm.synchronizer.AddTask(synchronizer.NewRootChainTask(peer, peer.RootHead(), pm.statsChan, pm.getShardConnFunc))
		if err != nil {
			log.Error("AddTask to synchronizer.", "error", err.Error())
		}
	}
}
