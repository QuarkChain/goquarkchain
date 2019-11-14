package master

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/mocks/mock_master"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

var (
	// only used in Test
	rootBlockHeaderListLimit  = sync.RootBlockHeaderListLimit
	rootBlockBatchSize        = sync.RootBlockBatchSize
	minorBlockHeaderListLimit = sync.MinorBlockHeaderListLimit
	minorBlockBatchSize       = sync.MinorBlockBatchSize
)

// Tests that protocol versions and modes of operations are matched up properly.
func TestProtocolCompatibility(t *testing.T) {
	pm, _, err := newTestProtocolManager(0, nil, NewFakeSynchronizer(1), nil)
	if pm != nil {
		defer pm.Stop()
	}
	if err != nil {
		t.Errorf("have error %v", err)
	}
}

func TestGetRootBlockHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	fakeConnMngr := newFakeConnManager(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, int(rootBlockHeaderListLimit)+15, nil, NewFakeSynchronizer(1), fakeConnMngr)
	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	// Create a batch of tests for various scenarios
	limit := uint64(rootBlockHeaderListLimit)
	tests := []struct {
		query  *p2p.GetRootBlockHeaderListWithSkipRequest // The hashList to execute for header retrieval
		expect []common.Hash                              // The hashes of the block whose headers are expected
	}{
		// A single random block should be retrievable by hash
		{
			&p2p.GetRootBlockHeaderListWithSkipRequest{Data: pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(), Limit: 1, Direction: 0},
			[]common.Hash{pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash()},
		}, {
			&p2p.GetRootBlockHeaderListWithSkipRequest{Data: pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(), Limit: 3, Direction: 0},
			[]common.Hash{
				pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 1).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 2).Hash(),
			},
		},
		// The chain endpoints should be retrievable
		{
			&p2p.GetRootBlockHeaderListWithSkipRequest{Data: pm.rootBlockChain.Genesis().Hash(), Limit: 1, Direction: 0},
			[]common.Hash{pm.rootBlockChain.GetBlockByNumber(0).Hash()},
		}, {
			&p2p.GetRootBlockHeaderListWithSkipRequest{Data: pm.rootBlockChain.CurrentBlock().Hash(), Limit: 1, Direction: 0},
			[]common.Hash{pm.rootBlockChain.CurrentBlock().Hash()},
		},
		// Ensure protocol limits are honored
		{
			&p2p.GetRootBlockHeaderListWithSkipRequest{Data: pm.rootBlockChain.CurrentBlock().ParentHash(), Limit: uint32(limit), Direction: 0},
			//GetBlockHashesFromHash return hash up to limit which do not include the hash in parameter
			pm.rootBlockChain.GetBlockHashesFromHash(pm.rootBlockChain.CurrentBlock().Hash(), limit),
		}, {
			// Check that requesting more than available is handled gracefully
			&p2p.GetRootBlockHeaderListWithSkipRequest{Data: pm.rootBlockChain.GetHeaderByNumber(2).Hash(), Limit: 3, Direction: 0},
			[]common.Hash{
				pm.rootBlockChain.GetBlockByNumber(2).Hash(),
				pm.rootBlockChain.GetBlockByNumber(1).Hash(),
				pm.rootBlockChain.GetBlockByNumber(0).Hash(),
			},
		},
	}
	// Run each of the tests and verify the results against the chain
	for i, tt := range tests {
		// Collect the headers to expect in the response
		var headers []*types.RootBlockHeader
		for _, hash := range tt.expect {
			headers = append(headers, pm.rootBlockChain.GetHeader(hash).(*types.RootBlockHeader))
		}
		// Send the hash request and verify the response
		go handleMsg(clientPeer)
		// rheaders, err := clientPeer.GetRootBlockHeaderList(tt.query.BlockHash, tt.query.Limit, qcom.DirectionToGenesis)
		res, err := clientPeer.GetRootBlockHeaderList(tt.query)
		if err != nil {
			t.Errorf("test %d: make message failed: %v", i, err)
		}

		rheaders := res.BlockHeaderList
		if len(rheaders) != len(headers) {
			t.Errorf("test %d: peer result count is mismatch: got %d, want %d", i, len(rheaders), len(headers))
		}
		for index, header := range rheaders {
			if header.Hash() != headers[index].Hash() {
				t.Errorf("test %d: peer result %d count is mismatch: got %v, want %v", i, index, header.Number, headers[index].Number)
			}
		}
	}
}

func TestCloseConnWithErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	fakeConnMngr := newFakeConnManager(1, ctrl)
	chainLength := uint64(1024)
	pm, _ := newTestProtocolManagerMust(t, int(chainLength), nil, NewFakeSynchronizer(10), fakeConnMngr)

	// Create a "random" unknown hash for testing
	var unknown common.Hash
	for i := range unknown {
		unknown[i] = byte(i)
	}
	// Create a batch of tests for various scenarios
	invalidReqs := []struct {
		op      p2p.P2PCommandOp
		content interface{}
	}{
		{p2p.GetRootBlockHeaderListRequestMsg, &p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.GetBlockByNumber(chainLength / 2).Hash(), Limit: 1, Direction: 1}},               // Wrong direction}
		{p2p.GetRootBlockHeaderListRequestMsg, &p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.Genesis().Hash(), Limit: uint32(2*rootBlockHeaderListLimit + 10), Direction: 0}}, // limit larger than expected
		{p2p.GetRootBlockHeaderListRequestMsg, &p2p.GetRootBlockHeaderListRequest{BlockHash: unknown, Limit: 1, Direction: 0}},                                                                  // no exist block hash
	}
	// Run each of the tests and verify the results against the chain
	for i, request := range invalidReqs {
		peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
		assert.NoError(t, err)

		// Send the hash request and verify the response
		msg, err := p2p.MakeMsg(request.op, peer.getRpcId(), p2p.Metadata{}, request.content)
		if err != nil {
			t.Errorf("test %d: make message failed: %v", i, err)
		}
		peer.app.WriteMsg(msg)

		err = readOrTimeOut(peer)
		if err != p2p.DiscReadTimeout {
			t.Errorf("test %d: read msg should be timeout", i)
		}
		if pm.peers.Peer(peer.id) != nil {
			t.Errorf("test %d: peer should be Unregister", i)
		}
	}
}

func TestGetRootBlocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	fakeConnMngr := newFakeConnManager(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, rootBlockBatchSize+15, nil, NewFakeSynchronizer(1), fakeConnMngr)
	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	var unknown common.Hash
	for i := range unknown {
		unknown[i] = byte(i)
	}
	// Create a batch of tests for various scenarios
	limit := uint64(rootBlockBatchSize)
	tests := []*p2p.GetRootBlockListRequest{
		// A single random block should be retrievable by hash
		{RootBlockHashList: []common.Hash{pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash()}},
		{ // multi hash retrievable
			RootBlockHashList: []common.Hash{
				pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 1).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 2).Hash(),
			},
		},
		// The chain endpoints should be retrievable
		{RootBlockHashList: []common.Hash{pm.rootBlockChain.Genesis().Hash()}},
		{RootBlockHashList: []common.Hash{pm.rootBlockChain.CurrentBlock().Hash()}},
		// Ensure protocol limits are honored
		{RootBlockHashList: pm.rootBlockChain.GetBlockHashesFromHash(pm.rootBlockChain.CurrentBlock().Hash(), limit)},
		{RootBlockHashList: []common.Hash{pm.rootBlockChain.CurrentBlock().Hash(), unknown}},
	}
	// Run each of the tests and verify the results against the chain
	for i, request := range tests {
		// Collect the headers to expect in the response
		blocks := make([]*types.RootBlock, 0, len(request.RootBlockHashList))
		for _, hash := range request.RootBlockHashList {
			if pm.rootBlockChain.HasBlock(hash) {
				blocks = append(blocks, pm.rootBlockChain.GetBlock(hash).(*types.RootBlock))
			}
		}
		go handleMsg(clientPeer)
		rblocks, err := clientPeer.GetRootBlockList(request.RootBlockHashList)
		if err != nil {
			t.Errorf("test %d: make message failed: %v", i, err)
		}
		if len(rblocks) != len(blocks) {
			t.Errorf("test %d: peer result count is mismatch: got %d, want %d", i, len(rblocks), len(blocks))
		}
		for index, block := range rblocks {
			if block.Hash() != blocks[index].Hash() {
				t.Errorf("test %d: peer result %d count is mismatch: got %v, want %v", i, index, block.Hash(), blocks[index].Hash())
			}
		}
	}
}

func TestGetMinorBlockHeaders(t *testing.T) {
	ctrl := gomock.NewController(t)
	blockcount := minorBlockHeaderListLimit + 15
	fakeConnMngr := newFakeConnManager(2, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, NewFakeSynchronizer(1), fakeConnMngr)
	minorBlocks := generateMinorBlocks(blockcount)
	minorHeaders := make([]*types.MinorBlockHeader, blockcount)
	for i, block := range minorBlocks {
		minorHeaders[i] = block.Header()
	}

	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	reqRaw := func(hashs common.Hash, limit uint32, direct uint8) []byte {
		res := &p2p.GetMinorBlockHeaderListRequest{
			Limit:     limit,
			Direction: direct,
			BlockHash: hashs,
		}
		data, err := serialize.SerializeToBytes(res)
		if err != nil {
			return nil
		}
		return data
	}

	// Create a batch of tests for various scenarios
	limit := uint64(minorBlockHeaderListLimit)
	tests := []struct {
		query  *rpc.P2PRedirectRequest   // The hashList to execute for header retrieval
		expect []*types.MinorBlockHeader // The hashes of the block whose headers are expected
	}{
		// A single random block should be retrievable by hash
		{
			&rpc.P2PRedirectRequest{Data: reqRaw(minorHeaders[limit/2].Hash(), 1, 0)},
			[]*types.MinorBlockHeader{minorHeaders[limit/2]},
		}, {
			&rpc.P2PRedirectRequest{Data: reqRaw(minorHeaders[limit/2].Hash(), 3, 0)},
			minorHeaders[limit/2-2 : limit/2+1],
		},
		// The chain endpoints should be retrievable
		{
			&rpc.P2PRedirectRequest{Data: reqRaw(minorHeaders[0].Hash(), 1, 0)},
			minorHeaders[:1],
		}, {
			&rpc.P2PRedirectRequest{Data: reqRaw(minorHeaders[blockcount-1].Hash(), 1, 0)},
			minorHeaders[blockcount-1:],
		},
		// Ensure protocol limits are honored
		{
			&rpc.P2PRedirectRequest{Data: reqRaw(minorHeaders[blockcount-1].GetParentHash(), uint32(limit), 0)},
			//GetBlockHashesFromHash return hash up to limit which do not include the hash in parameter
			minorHeaders[blockcount-int(limit) : blockcount-1],
		}, {
			// Check that requesting more than available is handled gracefully
			&rpc.P2PRedirectRequest{Data: reqRaw(pm.rootBlockChain.GetHeaderByNumber(2).Hash(), 4, 0)},
			minorHeaders[:3],
		},
	}
	// Run each of the tests and verify the results against the chain
	for i, tt := range tests {
		data, err := serialize.SerializeToBytes(&p2p.GetMinorBlockHeaderListResponse{
			RootTip:         pm.rootBlockChain.CurrentHeader().(*types.RootBlockHeader),
			ShardTip:        minorHeaders[blockcount-1],
			BlockHeaderList: tt.expect,
		})
		assert.NoError(t, err)
		for _, conn := range fakeConnMngr.GetSlaveConns() {
			conn.(*mock_master.MockISlaveConn).EXPECT().GetMinorBlockHeaderList(tt.query).Return(
				data, nil).Times(1)
		}

		go handleMsg(clientPeer)
		res, err := clientPeer.GetMinorBlockHeaderList(tt.query)

		if err != nil {
			t.Errorf("test %d: make message failed: %v", i, err)
		}

		var gRep p2p.GetMinorBlockHeaderListResponse
		assert.NoError(t, serialize.DeserializeFromBytes(res, &gRep))

		rheaders := gRep.BlockHeaderList

		if len(rheaders) != len(tt.expect) {
			t.Errorf("test %d: peer result count is mismatch: got %d, want %d", i, len(rheaders), len(tt.expect))
		}
		for index, header := range rheaders {
			if header.Hash() != tt.expect[index].Hash() {
				t.Errorf("test %d: peer result %d count is mismatch: got %v, want %v", i, index, header.Hash(), tt.expect[index].Hash())
			}
		}
	}
}

func TestGetMinorBlocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	blockcount := minorBlockBatchSize + 15
	fakeConnMngr := newFakeConnManager(2, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, NewFakeSynchronizer(1), fakeConnMngr)
	minorBlocks := generateMinorBlocks(blockcount)
	hashList := make([]common.Hash, blockcount)
	for i, block := range minorBlocks {
		hashList[i] = block.Hash()
	}
	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	// Create a batch of tests for various scenarios
	limit := uint64(minorBlockBatchSize)
	tests := []struct {
		hashList []common.Hash       // The hashList to execute for header retrieval
		expect   []*types.MinorBlock // The hashes of the block whose headers are expected
	}{
		// A single random block should be retrievable by hash
		{
			[]common.Hash{hashList[limit/2]},
			[]*types.MinorBlock{minorBlocks[limit/2]},
		}, {
			hashList[limit/2-2 : limit/2+1],
			minorBlocks[limit/2-2 : limit/2+1],
		},
		// The chain endpoints should be retrievable
		{
			hashList[:1],
			minorBlocks[:1],
		}, {
			hashList[blockcount-1:],
			minorBlocks[blockcount-1:],
		},
		// Ensure protocol limits are honored
		{
			hashList[blockcount-int(limit) : blockcount-1],
			//GetBlockHashesFromHash return hash up to limit which do not include the hash in parameter
			minorBlocks[blockcount-int(limit) : blockcount-1],
		}, {
			// Check that requesting more than available is handled gracefully
			hashList[:3],
			minorBlocks[:3],
		},
	}
	// Run each of the tests and verify the results against the chain
	for i, tt := range tests {
		go handleMsg(clientPeer)
		data, err := serialize.SerializeToBytes(&p2p.GetMinorBlockListResponse{MinorBlockList: tt.expect})
		assert.NoError(t, err)
		for _, conn := range fakeConnMngr.GetSlaveConns() {
			conn.(*mock_master.MockISlaveConn).EXPECT().GetMinorBlocks(gomock.Any()).Return(
				data, nil).Times(1)
		}
		var (
			req = rpc.P2PRedirectRequest{PeerID: "", Branch: 2}
		)
		req.Data, err = serialize.SerializeToBytes(p2p.GetMinorBlockListRequest{MinorBlockHashList: tt.hashList})
		assert.NoError(t, err)

		data, err = clientPeer.GetMinorBlockList(&req)
		assert.NoError(t, err)

		var gReq p2p.GetMinorBlockListResponse
		assert.NoError(t, serialize.DeserializeFromBytes(data, &gReq))
		rheaders := gReq.MinorBlockList

		if err != nil {
			t.Errorf("test %d: make message failed: %v", i, err)
		}

		if len(rheaders) != len(tt.expect) {
			t.Errorf("test %d: peer result count is mismatch: got %d, want %d", i, len(rheaders), len(tt.expect))
		}
		for index, header := range rheaders {
			if header.Hash() != tt.expect[index].Hash() {
				t.Errorf("test %d: peer result %d count is mismatch: got %v, want %v", i, index, header.Hash(), tt.expect[index].Hash())
			}
		}
	}
}

func TestBroadcastMinorBlock(t *testing.T) {
	sync := NewFakeSynchronizer(1)
	ctrl := gomock.NewController(t)
	errc := make(chan error)
	defer ctrl.Finish()
	fakeConnMngr := newFakeConnManager(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, sync, fakeConnMngr)
	minorBlock := generateMinorBlocks(1)[0]
	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	for _, conn := range fakeConnMngr.GetSlaveConns() {
		conn.(*mock_master.MockISlaveConn).EXPECT().
			HandleNewMinorBlock(gomock.Any()).Return(nil).Times(1)
	}
	data, err := serialize.SerializeToBytes(p2p.NewBlockMinor{Block: minorBlock})
	assert.NoError(t, err)
	err = clientPeer.SendNewMinorBlock(2, data)
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	for _, conn := range fakeConnMngr.GetSlaveConns() {
		conn.(*mock_master.MockISlaveConn).EXPECT().
			HandleNewMinorBlock(gomock.Any()).DoAndReturn(func(request *rpc.P2PRedirectRequest) error {
			errc <- nil
			return errors.New("expected error")
		}).Times(1)
	}
	data, err = serialize.SerializeToBytes(p2p.NewBlockMinor{Block: minorBlock})
	assert.NoError(t, err)
	err = clientPeer.SendNewMinorBlock(2, data)
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	if err := waitChanTilErrorOrTimeout(errc, 2); err != nil {
		t.Errorf("got one error: %v", err.Error())
	}
	time.Sleep(2 * time.Second)
	if pm.peers.Peer(peer.id) != nil {
		t.Errorf("peer should be Unregister")
	}
}

func TestBroadcastTransactions(t *testing.T) {
	ctrl := gomock.NewController(t)
	errc := make(chan error)
	defer ctrl.Finish()
	fakeConnMngr := newFakeConnManager(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, NewFakeSynchronizer(1), fakeConnMngr)
	txsBranch, err := newTestTransactionList(10)
	assert.NoError(t, err)
	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	for _, conn := range fakeConnMngr.GetSlaveConns() {
		conn.(*mock_master.MockISlaveConn).EXPECT().
			AddTransactions(gomock.Any()).DoAndReturn(func(request *rpc.P2PRedirectRequest) error {
			errc <- nil
			return nil
		}).AnyTimes()
	}
	err = clientPeer.SendTransactions(txsBranch)
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	if err := waitChanTilErrorOrTimeout(errc, 2); err != nil {
		t.Errorf("got one error: %v", err.Error())
	}
}

func TestBroadcastNewMinorBlockTip(t *testing.T) {
	ctrl := gomock.NewController(t)
	errc := make(chan error, 1)
	defer ctrl.Finish()
	fakeConnMngr := newFakeConnManager(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, NewFakeSynchronizer(1), fakeConnMngr)
	minorBlocks := generateMinorBlocks(30)
	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	for _, conn := range fakeConnMngr.GetSlaveConns() {
		conn.(*mock_master.MockISlaveConn).EXPECT().
			HandleNewTip(gomock.Any()).Return(true, nil).Times(1)
	}
	err = clientPeer.SendNewTip(2, &p2p.Tip{RootBlockHeader: pm.rootBlockChain.CurrentBlock().Header(),
		MinorBlockHeaderList: []*types.MinorBlockHeader{minorBlocks[len(minorBlocks)-2].Header()}})
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	time.Sleep(1 * time.Second)
	if pm.peers.Peer(peer.id) == nil {
		t.Errorf("peer should not be unregister")
	}
	for _, conn := range fakeConnMngr.GetSlaveConns() {
		conn.(*mock_master.MockISlaveConn).EXPECT().
			HandleNewTip(gomock.Any()).DoAndReturn(
			func(*rpc.HandleNewTipRequest) (bool, error) {
				errc <- nil
				return false, errors.New("expected error")
			}).Times(1)
	}
	err = clientPeer.SendNewTip(2, &p2p.Tip{RootBlockHeader: pm.rootBlockChain.CurrentBlock().Header(),
		MinorBlockHeaderList: []*types.MinorBlockHeader{minorBlocks[len(minorBlocks)-1].Header()}})
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	if err := waitChanTilErrorOrTimeout(errc, 2); err != nil {
		t.Errorf("got one error: %v", err.Error())
	}
	time.Sleep(2 * time.Second)
	if pm.peers.Peer(peer.id) != nil {
		t.Errorf("peer should be unregister")
	}
}

func TestBroadcastNewRootBlockTip(t *testing.T) {
	sync := NewFakeSynchronizer(1)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	fakeConnMngr := newFakeConnManager(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, sync, fakeConnMngr)
	peer, err := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	assert.NoError(t, err)

	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()
	blocks := core.GenerateRootBlockChain(pm.rootBlockChain.CurrentBlock(), pm.rootBlockChain.Engine(), 1, nil)
	err = clientPeer.SendNewTip(0, &p2p.Tip{RootBlockHeader: blocks[0].Header(), MinorBlockHeaderList: nil})
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	timeout := time.NewTimer(time.Duration(3 * time.Second))
	defer timeout.Stop()
	select {
	case <-sync.Task:
	case <-timeout.C:
		t.Errorf("synchronize task missed")
	}
}

func waitChanTilErrorOrTimeout(errc chan error, wait time.Duration) error {
	timeout := time.NewTimer(wait * time.Second)
	defer timeout.Stop()
	select {
	case err := <-errc:
		if err != nil {
			return err
		}
	case <-timeout.C:
		return p2p.DiscReadTimeout
	}
	return nil
}

func readOrTimeOut(peer *testPeer) error {
	errc := make(chan error, 1)
	go func() {
		_, err := peer.app.ReadMsg()
		errc <- err
	}()

	timeout := time.NewTimer(1 * time.Second)
	defer timeout.Stop()
	select {
	case err := <-errc:
		if err != nil {
			return err
		}
	case <-timeout.C:
		return p2p.DiscReadTimeout
	}
	return nil
}

func ExpectMsg(r p2p.MsgReader, op p2p.P2PCommandOp, metadata p2p.Metadata, content interface{}) (*p2p.QKCMsg, error) {
	msg, err := r.ReadMsg()
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, msg.Discard()
	}

	payload, err := ioutil.ReadAll(msg.Payload)
	if err != nil {
		return nil, err
	}
	qkcMsg, err := p2p.DecodeQKCMsg(payload)
	if err != nil {
		return nil, err
	}

	if qkcMsg.Op != op {
		return &qkcMsg, fmt.Errorf("incorrect op code: got %d, want %d", qkcMsg.Op, op)
	}

	if qkcMsg.MetaData.Branch != metadata.Branch {
		return &qkcMsg, fmt.Errorf("MetaData miss match: got %d, want %d", qkcMsg.MetaData.Branch, metadata.Branch)
	}

	cmdBytes, err := serialize.SerializeToBytes(content)
	if err != nil {
		return &p2p.QKCMsg{}, err
	}
	contentEnc, err := p2p.Encrypt(metadata, op, qkcMsg.RpcID, cmdBytes)
	if err != nil {
		panic("content encode error: " + err.Error())
	}
	if int(msg.Size) != len(contentEnc) {
		return &qkcMsg, fmt.Errorf("message size mismatch: got %d, want %d", msg.Size, len(contentEnc))
	}

	if !bytes.Equal(payload, contentEnc) {
		return &qkcMsg, fmt.Errorf("message payload mismatch:\ngot:  %x\nwant: %x", common.Bytes2Hex(payload), common.Bytes2Hex(contentEnc))
	}
	return &qkcMsg, nil
}

func handleMsg(peer *Peer) error {
	msg, err := peer.rw.ReadMsg()
	if err != nil {
		return err
	}
	payload, err := ioutil.ReadAll(msg.Payload)
	qkcMsg, err := p2p.DecodeQKCMsg(payload)
	if err != nil {
		return err
	}

	switch {
	case qkcMsg.Op == p2p.GetRootBlockHeaderListResponseMsg:
		var blockHeaderResp p2p.GetRootBlockHeaderListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockHeaderResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- &blockHeaderResp
		}

	case qkcMsg.Op == p2p.GetRootBlockHeaderListWithSkipResponseMsg:
		var blockHeaderResp p2p.GetRootBlockHeaderListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockHeaderResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- &blockHeaderResp
		}

	case qkcMsg.Op == p2p.GetRootBlockListResponseMsg:
		var blockResp p2p.GetRootBlockListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &blockResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- blockResp.RootBlockList
		}

	case qkcMsg.Op == p2p.GetMinorBlockHeaderListResponseMsg:
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- qkcMsg.Data
		}

	case qkcMsg.Op == p2p.GetMinorBlockHeaderListWithSkipResponseMsg:
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- qkcMsg.Data
		}

	case qkcMsg.Op == p2p.GetMinorBlockListResponseMsg:
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- qkcMsg.Data
		}
	default:
		return fmt.Errorf("unknown msg code %d", qkcMsg.Op)
	}
	return nil
}
