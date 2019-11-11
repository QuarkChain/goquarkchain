// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

// This file contains some shares testing functionality, common to  multiple
// different files and modules being tested.

package master

import (
	"crypto/ecdsa"
	"crypto/rand"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	synchronizer "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/mocks/mock_master"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/golang/mock/gomock"
	"math/big"
	"sort"
	"sync"
	"testing"
)

var (
	privKey       = "b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"
	qkcconfig     = config.NewQuarkChainConfig()
	clusterconfig = config.NewClusterConfig()
)

// newTestProtocolManager creates a new protocol manager for testing purposes,
// with the given number of blocks already known, and potential notification
// channels for different events.
func newTestProtocolManager(blocks int, generator func(int, *core.RootBlockGen),
	synchronizer synchronizer.Synchronizer, slaveConns rpc.ConnManager) (*ProtocolManager, *ethdb.MemDatabase, error) {
	var (
		engine        = new(consensus.FakeEngine)
		db            = ethdb.NewMemDatabase()
		genesis       = core.NewGenesis(qkcconfig)
		genesisBlock  = genesis.MustCommitRootBlock(db)
		blockChain, _ = core.NewRootBlockChain(db, qkcconfig, engine)
	)
	qkcconfig.SkipRootCoinbaseCheck = true
	clusterconfig.P2P.PrivKey = privKey
	chain := core.GenerateRootBlockChain(genesisBlock, engine, blocks, generator)
	if _, err := blockChain.InsertChain(toIBlocks(chain)); err != nil {
		panic(err)
	}

	pm, err := NewProtocolManager(*clusterconfig, blockChain, nil, synchronizer, slaveConns)
	if err != nil {
		return nil, nil, err
	}
	pm.Start(1000)
	return pm, db, nil
}

// newTestProtocolManagerMust creates a new protocol manager for testing purposes,
// with the given number of blocks already known, and potential notification
// channels for different events. In case of an error, the constructor force-
// fails the test.
func newTestProtocolManagerMust(t *testing.T, blocks int, generator func(int, *core.RootBlockGen),
	synchronizer synchronizer.Synchronizer, slaveConns rpc.ConnManager) (*ProtocolManager, *ethdb.MemDatabase) {
	pm, db, err := newTestProtocolManager(blocks, generator, synchronizer, slaveConns)
	if err != nil {
		t.Fatalf("Failed to create protocol manager: %v", err)
	}
	return pm, db
}

// testTxPool is a fake, helper transaction pool for testing purposes
type testTxPool struct {
	txFeed event.Feed
	pool   []*types.Transaction        // Collection of all transactions
	added  chan<- []*types.Transaction // Notification channel for new transactions

	lock sync.RWMutex // Protects the transaction pool
}

// AddRemotes appends a batch of transactions to the pool, and notifies any
// listeners if the addition channel is non nil
func (p *testTxPool) AddRemotes(txs []*types.Transaction) []error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.pool = append(p.pool, txs...)
	if p.added != nil {
		p.added <- txs
	}
	return make([]error, len(txs))
}

// Pending returns all the transactions known to the pool
func (p *testTxPool) Pending() (map[common.Address]types.Transactions, error) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	batches := make(map[common.Address]types.Transactions)
	for _, tx := range p.pool {
		signer := types.MakeSigner(tx.EvmTx.NetworkId())
		from, _ := types.Sender(signer, tx.EvmTx)
		batches[from] = append(batches[from], tx)
	}
	for _, batch := range batches {
		sort.Sort(types.TxByNonce(batch))
	}
	return batches, nil
}

func (p *testTxPool) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return p.txFeed.Subscribe(ch)
}

func newTestTransactionList(count int) (*rpc.P2PRedirectRequest, error) {
	key, _ := crypto.HexToECDSA("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8")
	txs := make([]*types.Transaction, 0, count)
	for i := 0; i < count; i++ {
		tx := newTestTransaction(key, uint64(i), 100)
		txs = append(txs, tx)
	}
	data, err := serialize.SerializeToBytes(&p2p.NewTransactionList{TransactionList: txs})
	if err != nil {
		return nil, err
	}
	return &rpc.P2PRedirectRequest{Branch: 0, Data: data}, nil
}

// newTestTransaction create a new dummy transaction.
func newTestTransaction(from *ecdsa.PrivateKey, nonce uint64, datasize int) *types.Transaction {
	tx := types.NewEvmTransaction(nonce, account.Recipient{}, big.NewInt(0), 100000,
		big.NewInt(0), 0, 1, 0, 1, make([]byte, datasize), 0, 0)
	tx, _ = types.SignTx(tx, types.MakeSigner(tx.NetworkId()), from)
	return &types.Transaction{EvmTx: tx, TxType: types.EvmTx}
}

// testPeer is a simulated peer to allow testing direct network calls.
type testPeer struct {
	net p2p.MsgReadWriter // Network layer reader/writer to simulate remote messaging
	app *p2p.MsgPipeRW    // Application layer reader/writer to simulate the local side
	*Peer
}

// newTestPeer creates a new peer registered at the given protocol manager.
func newTestPeer(name string, version int, pm *ProtocolManager, shake bool) (*testPeer, error) {
	// Create a message pipe to communicate through
	app, net := p2p.MsgPipe()

	// Generate a random id and create the peer
	var id enode.ID
	rand.Read(id[:])

	peer := newPeer(version, p2p.NewPeer(id, name, nil), net)

	// Start the peer on a new thread
	var err error
	go func() {
		pm.handle(peer)
	}()
	tp := &testPeer{app: app, net: net, Peer: peer}
	// Execute any implicitly requested handshakes and return
	if shake {
		err = tp.handshake(pm.rootBlockChain.CurrentBlock().Header(), pm.rootBlockChain.GetBlockByNumber(0).Hash())
	}

	return tp, err
}

func newTestClientPeer(version int, msgrw p2p.MsgReadWriter) *Peer {
	var id enode.ID
	rand.Read(id[:])

	return newPeer(version, p2p.NewPeer(id, "client", nil), msgrw)
}

// handshake simulates a trivial handshake that expects the same state from the
// remote side as we are simulating locally.
func (p *testPeer) handshake(rootBlockHeader *types.RootBlockHeader, geneHash common.Hash) error {
	privateKey, _ := p2p.GetPrivateKeyFromConfig(clusterconfig.P2P.PrivKey)
	id := crypto.FromECDSAPub(&privateKey.PublicKey)
	helloMsg := p2p.HelloCmd{
		Version:              qkcconfig.P2PProtocolVersion,
		NetWorkID:            qkcconfig.NetworkID,
		PeerID:               common.BytesToHash(id),
		PeerPort:             uint16(clusterconfig.P2PPort),
		RootBlockHeader:      rootBlockHeader,
		GenesisRootBlockHash: geneHash,
	}
	msg, err := p2p.MakeMsg(p2p.Hello, p.getRpcId(), p2p.Metadata{}, helloMsg)
	if err != nil {
		return err
	}

	if _, err := ExpectMsg(p.app, p2p.Hello, p2p.Metadata{}, helloMsg); err != nil {
		return err
	}

	if err := p.app.WriteMsg(msg); err != nil {
		return err
	}
	return nil
}

func toIBlocks(rootBlocks []*types.RootBlock) []types.IBlock {
	blocks := make([]types.IBlock, len(rootBlocks))
	for i, block := range rootBlocks {
		blocks[i] = block
	}
	return blocks
}

// close terminates the local side of the peer, notifying the remote protocol
// manager of termination.
func (p *testPeer) close() {
	p.app.Close()
}

func generateMinorBlocks(n int) []*types.MinorBlock {
	var (
		engine       = new(consensus.FakeEngine)
		db           = ethdb.NewMemDatabase()
		genesis      = core.NewGenesis(qkcconfig)
		genesisBlock = genesis.MustCommitMinorBlock(db, genesis.CreateRootBlock(), 2)
	)
	blocks := make([]*types.MinorBlock, n)
	genblock := func(i int, parent *types.MinorBlock) *types.MinorBlock {
		difficulty, _ := engine.CalcDifficulty(nil, parent.Time(), parent.Header())
		header := &types.MinorBlockHeader{
			ParentHash: parent.Hash(),
			Branch:     account.Branch{Value: 2},
			Coinbase:   parent.Coinbase(),
			Difficulty: difficulty,
			Number:     parent.Number() + 1,
			Time:       parent.Time() + 10,
		}
		meta := types.MinorBlockMeta{
			TxHash:            common.Hash{},
			Root:              common.Hash{},
			ReceiptHash:       common.Hash{},
			GasUsed:           &serialize.Uint256{Value: big.NewInt(10000)},
			CrossShardGasUsed: &serialize.Uint256{Value: big.NewInt(10)},
		}
		return types.NewMinorBlock(header, &meta, nil, nil, nil)
	}
	parent := genesisBlock
	for i := 0; i < n; i++ {
		block := genblock(i, parent)
		blocks[i] = block
		parent = block
	}
	return blocks
}

type fakeSynchronizer struct {
	Task chan synchronizer.Task
}

func NewFakeSynchronizer(n int) *fakeSynchronizer {
	return &fakeSynchronizer{make(chan synchronizer.Task, n)}
}

func (s *fakeSynchronizer) SubscribeSyncEvent(ch chan<- *synchronizer.SyncingResult) event.Subscription {
	panic("not implemented")
}

func (s *fakeSynchronizer) IsSyncing() bool {
	return false
}

func (s *fakeSynchronizer) AddTask(task synchronizer.Task) error {
	s.Task <- task
	return nil
}

func (s *fakeSynchronizer) Close() error {
	return nil
}

type fakeConnManager struct {
	conns []rpc.ISlaveConn
}

func newFakeConnManager(n int, ctrl *gomock.Controller) *fakeConnManager {
	conns := make([]rpc.ISlaveConn, 0, n)
	for i := 0; i < n; i++ {
		sc := mock_master.NewMockISlaveConn(ctrl)
		conns = append(conns, sc)
	}

	return &fakeConnManager{conns: conns}
}

func (f *fakeConnManager) GetOneConnById(fullShardId uint32) rpc.ISlaveConn {
	return f.conns[0]
}

func (f *fakeConnManager) GetSlaveConnsById(fullShardId uint32) []rpc.ISlaveConn {
	return f.conns
}

func (f *fakeConnManager) GetSlaveConns() []rpc.ISlaveConn {
	return f.conns
}

func (f *fakeConnManager) ConnCount() int { return len(f.conns) }
