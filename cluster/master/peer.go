// Modified from go-ethereum under GNU Lesser General Public License

package master

import (
	"errors"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
	errTimeout           = errors.New("request timeout")
)

const (
	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 128

	// maxQueuedMinorBlocks is the maximum number of block propagations to queue up before
	// dropping broadcasts.
	maxQueuedMinorBlocks = 16

	// maxQueuedTips is the maximum number of block announcements to queue up before
	// dropping broadcasts.
	maxQueuedTips = 16

	handshakeTimeout = 5 * time.Second

	requestTimeout = 5 * time.Second

	rootBlockHeaderListLimit  = 500
	rootBlockBatchSize        = 100
	minorBlockHeaderListLimit = 100
	minorBlockBatchSize       = 50

	directionToGenesis = uint8(0)
	directionToTip     = uint8(1)
)

type newMinorBlock struct {
	branch uint32
	block  *types.MinorBlock
}

type newTxs struct {
	branch uint32
	txs    []*types.Transaction
}

type newTip struct {
	branch uint32
	tip    *p2p.Tip
}

type peerHead struct {
	rootTip   *types.RootBlockHeader
	minorTips map[uint32]*p2p.Tip
}

type peer struct {
	id    string
	rpcId uint64

	*p2p.Peer
	rw p2p.MsgReadWriter

	version  int         // Protocol version negotiated
	forkDrop *time.Timer // Timed connection dropper if forks aren't validated in time

	head *peerHead

	lock             sync.RWMutex
	chanLock         sync.RWMutex
	queuedTxs        chan newTxs        // Queue of transactions to broadcast to the peer
	queuedMinorBlock chan newMinorBlock // Queue of blocks to broadcast to the peer
	queuedTip        chan newTip        // Queue of Tips to announce to the peer
	term             chan struct{}      // Termination channel to stop the broadcaster
	chans            map[uint64]chan interface{}
}

func newPeer(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *peer {
	return &peer{
		Peer:             p,
		rw:               rw,
		version:          version,
		id:               fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		head:             &peerHead{nil, make(map[uint32]*p2p.Tip)},
		queuedTxs:        make(chan newTxs, maxQueuedTxs),
		queuedMinorBlock: make(chan newMinorBlock, maxQueuedMinorBlocks),
		queuedTip:        make(chan newTip, maxQueuedTips),
		term:             make(chan struct{}),
		chans:            make(map[uint64]chan interface{}),
	}
}

// broadcast is a write loop that multiplexes block propagations, announcements
// and transaction broadcasts into the remote peer. The goal is to have an async
// writer that does not lock up node internals.
func (p *peer) broadcast() {
	for {
		select {
		case nTxs := <-p.queuedTxs:
			if err := p.SendTransactions(nTxs.branch, nTxs.txs); err != nil {
				p.Log().Error("Broadcast transactions failed",
					"count", len(nTxs.txs), "branch", nTxs.branch, "error", err.Error())
				return
			}
			p.Log().Trace("Broadcast transactions", "count", len(nTxs.txs), "branch", nTxs.branch)

		case nBlock := <-p.queuedMinorBlock:
			if err := p.SendNewMinorBlock(nBlock.branch, nBlock.block); err != nil {
				p.Log().Error("Broadcast minor block failed",
					"number", nBlock.block.NumberU64(), "hash", nBlock.block.Hash(), "branch", nBlock.branch, "error", err.Error())
				return
			}
			p.Log().Trace("Broadcast minor block", "number", nBlock.block.NumberU64(), "hash", nBlock.block.Hash(), "branch", nBlock.branch)

		case nTip := <-p.queuedTip:
			if err := p.SendNewTip(nTip.branch, nTip.tip); err != nil {
				return
			}
			if nTip.branch != 0 {
				p.Log().Trace("Broadcast new tip", "number", nTip.tip.RootBlockHeader.NumberU64(), "branch", nTip.branch)
			}

		case <-p.term:
			return
		}
	}
}

// close signals the broadcast goroutine to terminate.
func (p *peer) close() {
	close(p.term)
}

func (p *peer) getRpcId() uint64 {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.rpcId = p.rpcId + 1
	return p.rpcId
}

func (p *peer) getRpcIdWithChan() (uint64, chan interface{}) {
	p.lock.Lock()
	defer p.lock.Unlock()
	p.rpcId = p.rpcId + 1
	rpcchan := make(chan interface{}, 1)
	p.addChan(p.rpcId, rpcchan)
	return p.rpcId, rpcchan
}

// RootHead retrieves a copy of the current root head of the
// peer.
func (p *peer) RootHead() *types.RootBlockHeader {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.head.rootTip
}

// SetRootHead updates the root head of the peer.
func (p *peer) SetRootHead(rootTip *types.RootBlockHeader) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.head.rootTip = rootTip
}

// RootHead retrieves a copy of the current root head of the
// peer.
func (p *peer) MinorHead(branch uint32) *p2p.Tip {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.head.minorTips[branch]
}

// SetRootHead updates the root head of the peer.
func (p *peer) SetMinorHead(branch uint32, minorTip *p2p.Tip) {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.head.minorTips[branch] = minorTip
}

func (p *peer) PeerID() string {
	return p.id
}

// SendTransactions sends transactions to the peer and includes the hashes
// in its transaction hash set for future reference.
func (p *peer) SendTransactions(branch uint32, txs []*types.Transaction) error {
	data := p2p.NewTransactionList{}
	data.TransactionList = txs

	msg, err := p2p.MakeMsg(p2p.NewTransactionListMsg, 0, p2p.Metadata{Branch: branch}, data)
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

// AsyncSendTransactions queues list of transactions propagation to a remote
// peer. If the peer's broadcast queue is full, the event is silently dropped.
func (p *peer) AsyncSendTransactions(branch uint32, txs []*types.Transaction) {
	select {
	case p.queuedTxs <- newTxs{branch: branch, txs: txs}:
		p.Log().Debug("add transaction to broadcast queue", "count", len(txs))
	default:
		p.Log().Debug("Dropping transaction propagation", "count", len(txs))
	}
}

// SendNewTip announces the head of each shard or root.
func (p *peer) SendNewTip(branch uint32, tip *p2p.Tip) error {
	msg, err := p2p.MakeMsg(p2p.NewTipMsg, 0, p2p.Metadata{Branch: branch}, tip) //NewTipMsg should rpc=0
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

// AsyncSendNewTip queues the head block for propagation to a remote peer.
// If the peer's broadcast queue is full, the event is silently dropped.
func (p *peer) AsyncSendNewTip(branch uint32, tip *p2p.Tip) {
	select {
	case p.queuedTip <- newTip{branch: branch, tip: tip}:
		p.Log().Debug("Add new tip to broadcast queue", "", tip.RootBlockHeader.NumberU64(), "branch", branch)
	default:
		p.Log().Debug("Dropping new tip", "", tip.RootBlockHeader.NumberU64(), "branch", branch)
	}
}

// SendNewMinorBlock propagates an entire minor block to a remote peer.
func (p *peer) SendNewMinorBlock(branch uint32, block *types.MinorBlock) error {
	data := p2p.NewBlockMinor{Block: block}

	msg, err := p2p.MakeMsg(p2p.NewBlockMinorMsg, 0, p2p.Metadata{Branch: branch}, data)
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

// AsyncSendNewMinorBlock queues an entire minor block for propagation to a remote peer. If
// the peer's broadcast queue is full, the event is silently dropped.
func (p *peer) AsyncSendNewMinorBlock(branch uint32, block *types.MinorBlock) {
	select {
	case p.queuedMinorBlock <- newMinorBlock{branch: branch, block: block}:
		p.Log().Debug("add minor block to broadcast queue", "number", block.NumberU64(), "hash", block.Hash())
	default:
		p.Log().Debug("Dropping block propagation", "number", block.NumberU64(), "hash", block.Hash())
	}
}

func (p *peer) getChan(rpcId uint64) chan interface{} {
	p.chanLock.Lock()
	defer p.chanLock.Unlock()
	return p.chans[rpcId]
}

func (p *peer) addChan(rpcId uint64, rpcchan chan interface{}) {
	p.chanLock.Lock()
	defer p.chanLock.Unlock()
	p.chans[rpcId] = rpcchan
}

func (p *peer) deleteChan(rpcId uint64) {
	p.chanLock.Lock()
	defer p.chanLock.Unlock()
	delete(p.chans, rpcId)
}

// requestRootBlockHeaderList fetches a batch of root blocks' headers corresponding to the
// specified header hashList, based on the hash of an origin block.
func (p *peer) requestRootBlockHeaderList(rpcId uint64, hash common.Hash, amount uint32, reverse bool) error {
	if amount == 0 {
		amount = rootBlockHeaderListLimit
	}
	var direction uint8 = directionToGenesis // 0 to genesis
	if !reverse {
		direction = directionToTip // 1 to tip
	}

	data := p2p.GetRootBlockHeaderListRequest{BlockHash: hash, Limit: amount, Direction: direction}
	msg, err := p2p.MakeMsg(p2p.GetRootBlockHeaderListRequestMsg, rpcId, p2p.Metadata{}, data)
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

func (p *peer) GetRootBlockHeaderList(hash common.Hash, amount uint32, reverse bool) ([]*types.RootBlockHeader, error) {
	rpcId, rpcchan := p.getRpcIdWithChan()
	defer p.deleteChan(rpcId)

	err := p.requestRootBlockHeaderList(rpcId, hash, amount, reverse)
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(requestTimeout)
	select {
	case obj := <-rpcchan:
		if ret, ok := obj.([]*types.RootBlockHeader); !ok {
			panic("invalid return result in GetRootBlockHeaderList")
		} else {
			return ret, nil
		}
	case <-timeout.C:
		return nil, fmt.Errorf("peer %v return GetRootBlockHeaderList Time out for rpcid %d", p.id, rpcId)
	}
}

func (p *peer) requestMinorBlockHeaderList(rpcId uint64, hash common.Hash, amount uint32, branch uint32, reverse bool) error {
	var direction uint8 = directionToGenesis
	if !reverse {
		direction = directionToTip
	}

	data := p2p.GetMinorBlockHeaderListRequest{BlockHash: hash, Branch: account.Branch{Value: branch}, Limit: amount, Direction: direction}
	msg, err := p2p.MakeMsg(p2p.GetMinorBlockHeaderListRequestMsg, rpcId, p2p.Metadata{Branch: branch}, data)
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

func (p *peer) GetMinorBlockHeaderList(origin common.Hash, amount uint32, branch uint32, reverse bool) ([]*types.MinorBlockHeader, error) {
	rpcId, rpcchan := p.getRpcIdWithChan()
	defer p.deleteChan(rpcId)

	err := p.requestMinorBlockHeaderList(rpcId, origin, amount, branch, reverse)
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(requestTimeout)
	select {
	case obj := <-rpcchan:
		if ret, ok := obj.([]*types.MinorBlockHeader); !ok {
			panic("invalid return result in GetMinorBlockHeaderList")
		} else {
			return ret, nil
		}
	case <-timeout.C:
		return nil, fmt.Errorf("peer %v return GetMinorBlockHeaderList disc Read Time out for rpcid %d", p.id, rpcId)
	}
}

// requestRootBlockList fetches a batch of root blocks' corresponding to the hashes
// specified.
func (p *peer) requestRootBlockList(rpcId uint64, hashList []common.Hash) error {
	data := p2p.GetRootBlockListRequest{RootBlockHashList: hashList}
	msg, err := p2p.MakeMsg(p2p.GetRootBlockListRequestMsg, rpcId, p2p.Metadata{}, data)
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

func (p *peer) GetRootBlockList(hashes []common.Hash) ([]*types.RootBlock, error) {
	rpcId, rpcchan := p.getRpcIdWithChan()
	defer p.deleteChan(rpcId)

	err := p.requestRootBlockList(rpcId, hashes)
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(requestTimeout)
	select {
	case obj := <-rpcchan:
		if ret, ok := obj.([]*types.RootBlock); !ok {
			panic("invalid return result in GetRootBlockList")
		} else {
			return ret, nil
		}
	case <-timeout.C:
		return nil, fmt.Errorf("peer %v return GetRootBlockList disc Read Time out for rpcid %d", p.id, rpcId)
	}
}

func (p *peer) requestMinorBlockList(rpcId uint64, hashList []common.Hash, branch uint32) error {
	data := p2p.GetMinorBlockListRequest{MinorBlockHashList: hashList}
	msg, err := p2p.MakeMsg(p2p.GetMinorBlockListRequestMsg, rpcId, p2p.Metadata{Branch: branch}, data)
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

func (p *peer) GetMinorBlockList(hashes []common.Hash, branch uint32) ([]*types.MinorBlock, error) {
	rpcId, rpcchan := p.getRpcIdWithChan()
	defer p.deleteChan(rpcId)

	err := p.requestMinorBlockList(rpcId, hashes, branch)
	if err != nil {
		return nil, err
	}

	timeout := time.NewTimer(requestTimeout)
	select {
	case obj := <-rpcchan:
		if ret, ok := obj.([]*types.MinorBlock); !ok {
			panic("invalid return result in GetMinorBlockList")
		} else {
			return ret, nil
		}
	case <-timeout.C:
		return nil, fmt.Errorf("peer %v return GetMinorBlockList disc Read Time out for rpcid %d", p.id, rpcId)
	}
}

func (p *peer) SendResponse(op p2p.P2PCommandOp, metadata p2p.Metadata, rpcId uint64, response interface{}) error {
	msg, err := p2p.MakeMsg(op, rpcId, metadata, response)
	if err != nil {
		return err
	}
	return p.rw.WriteMsg(msg)
}

// Handshake executes the eth protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
func (p *peer) Handshake(protoVersion, networkId uint32, peerId common.Hash, peerPort uint16, rootBlockHeader *types.RootBlockHeader) error {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)

	hello, err := p2p.MakeMsg(p2p.Hello, 0, p2p.Metadata{}, p2p.HelloCmd{
		Version:         protoVersion,
		NetWorkID:       networkId,
		PeerID:          peerId,
		PeerPort:        peerPort,
		RootBlockHeader: rootBlockHeader,
	})
	if err != nil {
		return err
	}

	go func() {
		errc <- p.readStatus(protoVersion, networkId)
	}()
	go func() {
		errc <- p.rw.WriteMsg(hello)
	}()

	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				return err
			}
		case <-timeout.C:
			fmt.Println("return Handshake disc Read Time out")
			return p2p.DiscReadTimeout
		}
	}
	return nil
}

func (p *peer) readStatus(protoVersion, networkId uint32) (err error) {
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}

	qkcBody, err := ioutil.ReadAll(msg.Payload)
	if err != nil {
		return err
	}
	qkcMsg, err := p2p.DecodeQKCMsg(qkcBody)
	if err != nil {
		return err
	}

	var helloCmd = p2p.HelloCmd{}
	err = serialize.DeserializeFromBytes(qkcMsg.Data, &helloCmd)
	if err != nil {
		return err
	}

	if msg.Code != uint64(p2p.Hello) {
		return errors.New("msgCode is err")
	}
	if helloCmd.NetWorkID != networkId {
		return fmt.Errorf("networkid mismatch, get: %d, want: %d", helloCmd.NetWorkID, networkId)
	}
	if helloCmd.Version != protoVersion {
		return fmt.Errorf("protoco version mismatch, get: %d, want: %d", helloCmd.Version, protoVersion)
	}
	if helloCmd.RootBlockHeader == nil {
		return errors.New("root block header in hello cmd is nil")
	}

	p.SetRootHead(helloCmd.RootBlockHeader)
	return nil
}

// String implements fmt.Stringer.
func (p *peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("eth/%2d", p.version),
	)
}

// peerSet represents the collection of active peers currently participating in
// the sub-protocol.
type peerSet struct {
	peers  map[string]*peer
	lock   sync.RWMutex
	closed bool
}

// newPeerSet creates a new peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		peers: make(map[string]*peer),
	}
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known. If a new peer it registered, its broadcast loop is also
// started.
func (ps *peerSet) Register(p *peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.id]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.id] = p
	go p.broadcast()

	return nil
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *peerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.close()

	return nil
}

// Peer retrieves the registered peer with the given id.
func (ps *peerSet) Peer(id string) *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Peers retrieves all registered peers as a slice
func (ps *peerSet) Peers() []*peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	peers := make([]*peer, 0, len(ps.peers))
	for _, peer := range ps.peers {
		peers = append(peers, peer)
	}
	return peers
}

// Len returns if the current number of peers in the set.
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// BestPeer retrieves the known peer with the currently highest total difficulty.
func (ps *peerSet) BestPeer() *peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var (
		bestPeer   *peer
		bestHeight uint64
	)
	// TODO will update to TD when td add to rootblock
	for _, p := range ps.peers {
		if head := p.RootHead(); head != nil && (bestPeer == nil || head.NumberU64() > bestHeight) {
			bestPeer, bestHeight = p, head.NumberU64()
		}
	}
	return bestPeer
}

// Close disconnects all peers.
// No new peers can be registered after Close has returned.
func (ps *peerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}
