package master

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/pkg/errors"
	"io/ioutil"
	"sync"
	"time"
)

// QKCProtocol details
const (
	QKCProtocolName     = "quarkchain"
	QKCProtocolVersion  = 1
	QKCProtocolLength   = 16
	chainHeadChanSize   = 10
	forceSyncCycle      = 1000 * time.Second
	minDesiredPeerCount = 5
)

// ProtocolManager QKC manager
type ProtocolManager struct {
	networkID      uint32
	rootBlockChain *core.RootBlockChain
	clusterConfig  *config.ClusterConfig

	SubProtocols     []p2p.Protocol
	getShardConnFunc func(fullShardId uint32) []ShardConnForP2P

	chainHeadChan     chan core.RootChainHeadEvent
	chainHeadEventSub event.Subscription

	maxPeers    int
	peers       *peerSet // Set of active peers from which rootDownloader can proceed
	newPeerCh   chan *peer
	quitSync    chan struct{}
	noMorePeers chan struct{}

	// todo add sync
	log string
	wg  sync.WaitGroup
}

// NewQKCManager  new qkc manager
func NewProtocolManager(env config.ClusterConfig, rootBlockChain *core.RootBlockChain, getShardConnFunc func(fullShardId uint32) []ShardConnForP2P) (*ProtocolManager, error) {
	manager := &ProtocolManager{
		networkID:        env.Quarkchain.NetworkID,
		rootBlockChain:   rootBlockChain,
		clusterConfig:    &env,
		peers:            newPeerSet(),
		newPeerCh:        make(chan *peer),
		quitSync:         make(chan struct{}),
		noMorePeers:      make(chan struct{}),
		getShardConnFunc: getShardConnFunc,
	}
	protocol := p2p.Protocol{
		Name:    QKCProtocolName,
		Version: QKCProtocolVersion,
		Length:  QKCProtocolLength,
		Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
			peer := newPeer(int(QKCProtocolVersion), p, rw)
			select {
			case manager.newPeerCh <- peer:
				manager.wg.Add(1)
				defer manager.wg.Done()
				return manager.handle(peer)
			case <-manager.quitSync:
				return p2p.DiscQuitting
			}
		},
	}
	manager.SubProtocols = []p2p.Protocol{protocol}

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
	//todo add sync task loop
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

	//todo add register synchroniser
	// currently we do not broadcast old transaction when connect
	// so the first few block may not have transaction verification failed
	// or transaction drop issue which is temp issue
	// we can add pm.syncTransactions(p) later

	for {
		if err := pm.handleMsg(peer); err != nil {
			peer.Log().Debug("message handling failed", "err", err)
			pm.peers.Unregister(peer.id)
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

	log.Info(pm.log, " receive QKC Msgop", qkcMsg.Op.String())
	switch {
	case qkcMsg.Op == p2p.Hello:
		var hello p2p.HelloCmd
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &hello); err != nil {
			return err
		}
		//todo add sync task ?

	case qkcMsg.Op == p2p.NewTipMsg:
		var tip *p2p.Tip
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, tip); err != nil {
			return err
		}
		if tip.RootBlockHeader == nil {
			return fmt.Errorf("invalid NewTip Request: RootBlockHeader is nil. %d for rpc request %d",
				qkcMsg.RpcID, qkcMsg.MetaData.Branch)
		}
		if qkcMsg.MetaData.Branch == 0 {
			//todo add to sync task
			break
		}
		if tip.MinorBlockHeaderList == nil || len(tip.MinorBlockHeaderList) != 1 {
			return fmt.Errorf("invalid NewTip Request: len of MinorBlockHeaderList is %d. %d for rpc request %d",
				len(tip.MinorBlockHeaderList), qkcMsg.RpcID, qkcMsg.MetaData.Branch)
		}
		clients := pm.getShardConnFunc(qkcMsg.MetaData.Branch)
		if clients == nil || len(clients) == 0 {
			return fmt.Errorf("invalid branch %d for rpc request %d", qkcMsg.RpcID, qkcMsg.MetaData.Branch)
		}
		for _, client := range clients {
			result, err := client.HandleNewTip(tip)
			if err != nil {
				log.Error("branch %d handle NewTipMsg message failed with error: %v", qkcMsg.MetaData.Branch, err.Error())
			}
			if !result {
				return fmt.Errorf("HandleNewTip (rpcId %d) for branch %d  with height %d return false",
					qkcMsg.RpcID, qkcMsg.MetaData.Branch, tip.MinorBlockHeaderList[0])
			}
		}

	case qkcMsg.Op == p2p.NewTransactionListMsg:
		var trans *p2p.NewTransactionList
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, trans); err != nil {
			return err
		}
		if qkcMsg.MetaData.Branch != 0 {
			return pm.HandleNewTransactionListRequest(peer.id, qkcMsg.RpcID, qkcMsg.MetaData.Branch, trans)
		}
		branchTxMap := make(map[uint32][]*types.Transaction)
		for _, tx := range trans.TransactionList {
			branchTxMap[tx.EvmTx.FromFullShardId()] = append(branchTxMap[tx.EvmTx.FromFullShardId()], tx)
		}
		for branch, list := range branchTxMap {
			if err := pm.HandleNewTransactionListRequest(peer.id, qkcMsg.RpcID, branch, &p2p.NewTransactionList{TransactionList: list}); err != nil {
				return err
			}
		}

	case qkcMsg.Op == p2p.NewBlockMinorMsg:
		var newBlockMinor *p2p.NewBlockMinor
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, newBlockMinor); err != nil {
			return err
		}
		clients := pm.getShardConnFunc(qkcMsg.MetaData.Branch)
		if clients == nil || len(clients) == 0 {
			return fmt.Errorf("invalid branch %d for rpc request %d", qkcMsg.RpcID, qkcMsg.MetaData.Branch)
		}
		for _, client := range clients {
			result, err := client.AddMinorBlock(newBlockMinor)
			if err != nil {
				log.Error("branch %d handle NewTipMsg message failed with error: %v", qkcMsg.MetaData.Branch, err.Error())
			}
			if !result {
				return fmt.Errorf("AddMinorBlock (rpcId %d) for branch %d return false",
					qkcMsg.RpcID, qkcMsg.MetaData.Branch)
			}
		}

	case qkcMsg.Op == p2p.GetRootBlockHeaderListRequestMsg:
		var blockHeaderReq p2p.GetRootBlockHeaderListRequest
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockHeaderReq); err != nil {
			return err
		}

		resp, err := pm.HandleGetRootBlockHeaderListRequest(&blockHeaderReq)
		if err != nil {
			log.Error(err.Error())
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
			log.Error(fmt.Sprintf("chan for rpc %d is missing", qkcMsg.RpcID))
		}

	case qkcMsg.Op == p2p.GetMinorBlockHeaderListRequestMsg:
		var minorHeaderReq p2p.GetMinorBlockHeaderListRequest
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorHeaderReq); err != nil {
			return err
		}

		resp, err := pm.HandleGetMinorBlockHeaderListRequest(&minorHeaderReq)
		if err != nil {
			return err
		}

		return peer.SendResponse(p2p.GetMinorBlockHeaderListResponseMsg, p2p.Metadata{Branch: 0}, qkcMsg.RpcID, resp)

	case qkcMsg.Op == p2p.GetMinorBlockHeaderListResponseMsg:
		var minorHeaderResp p2p.GetMinorBlockHeaderListResponse

		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorHeaderResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- minorHeaderResp.BlockHeaderList
		} else {
			log.Error(fmt.Sprintf("chan for rpc %d is missing", qkcMsg.RpcID))
		}

	case qkcMsg.Op == p2p.GetMinorBlockListRequestMsg:
		var minorBlockReq p2p.GetMinorBlockListRequest
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorBlockReq); err != nil {
			return err
		}

		resp, err := pm.HandleGetMinorBlockListRequest(&minorBlockReq)
		if err != nil {
			return err
		}

		return peer.SendResponse(p2p.GetMinorBlockListResponseMsg, p2p.Metadata{Branch: 0}, qkcMsg.RpcID, resp)

	case qkcMsg.Op == p2p.GetMinorBlockListResponseMsg:
		var minorBlockResp p2p.GetMinorBlockListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorBlockResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- minorBlockResp.MinorBlockList
		} else {
			log.Error(fmt.Sprintf("chan for rpc %d is missing", qkcMsg.RpcID))
		}

	default:
		return fmt.Errorf("unknown msg code %d", qkcMsg.Op)
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
		if header == nil { //todo check interface == nil
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

func (pm *ProtocolManager) HandleGetMinorBlockHeaderListRequest(request *p2p.GetMinorBlockHeaderListRequest) (*p2p.GetMinorBlockHeaderListResponse, error) {
	//todo API client call RPC API to get response
	return nil, nil
}

func (pm *ProtocolManager) HandleNewTransactionListRequest(peerId string, rpcId uint64, branch uint32, request *p2p.NewTransactionList) error {
	clients := pm.getShardConnFunc(branch)
	if clients == nil || len(clients) == 0 {
		return fmt.Errorf("invalid branch %d for rpc request %d", rpcId, branch)
	}
	var hashList []common.Hash
	needBroadcast := true
	for _, client := range clients {
		result, err := client.AddTransactions(request)
		if err != nil {
			log.Error("branch %d handle AddTransactions message failed with error: %v", rpcId, err.Error())
		}
		if hashList == nil {
			hashList = result.Hashes
		} else if len(hashList) != len(result.Hashes) {
			needBroadcast = false
		} else {
			for i := 0; i < len(hashList); i++ {
				if hashList[i] != result.Hashes[i] {
					needBroadcast = false
					break
				}
			}
		}
	}

	if needBroadcast && len(hashList) > 0 {
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

func (pm *ProtocolManager) HandleGetMinorBlockListRequest(request *p2p.GetMinorBlockListRequest) (*p2p.GetMinorBlockListResponse, error) {
	//todo API client call RPC API to get response
	return nil, nil
}

func (pm *ProtocolManager) BroadcastTip(header *types.RootBlockHeader) {
	for _, peer := range pm.peers.peers {
		peer.AsyncSendNewTip(0, &p2p.Tip{RootBlockHeader: header, MinorBlockHeaderList: nil})
	}
	log.Trace("Announced block", "hash", header.Hash(), "recipients", len(pm.peers.peers))
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
	for _, peer := range pm.peers.peers {
		if peer.id != sourcePeerId {
			peer.AsyncSendTransactions(branch, txs)
		}
	}
	log.Trace("Announced transaction", "count", len(txs), "recipients", len(pm.peers.peers))
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
			// Force a sync even if not enough peers are present
			go pm.synchronise(pm.peers.BestPeer())

		case <-pm.noMorePeers:
			return
		}
	}
}

// synchronise tries to sync up our local block chain with a remote peer.
func (pm *ProtocolManager) synchronise(peer *peer) {
	// Short circuit if no peers are available
	if peer == nil {
		return
	}
	// todo add sync task & bloadcast tip if updated
	// Make sure the peer's TD is higher than our own
	currentBlock := pm.rootBlockChain.CurrentBlock()
	go pm.BroadcastTip(currentBlock.Header())
}
