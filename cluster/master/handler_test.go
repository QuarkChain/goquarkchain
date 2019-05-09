package master

import (
	"bytes"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/mocks/mock_master"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"io/ioutil"
	"testing"
	"time"
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
	pm, _ := newTestProtocolManagerMust(t, rootBlockHeaderListLimit+15, nil, nil, nil)
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	// Create a batch of tests for various scenarios
	limit := uint64(rootBlockHeaderListLimit)
	tests := []struct {
		query  *p2p.GetRootBlockHeaderListRequest // The hashList to execute for header retrieval
		expect []common.Hash                      // The hashes of the block whose headers are expected
	}{
		// A single random block should be retrievable by hash
		{
			&p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(), Limit: 1, Direction: 0},
			[]common.Hash{pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash()},
		}, {
			&p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(), Limit: 3, Direction: 0},
			[]common.Hash{
				pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 1).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 2).Hash(),
			},
		},
		// The chain endpoints should be retrievable
		{
			&p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.Genesis().Hash(), Limit: 1, Direction: 0},
			[]common.Hash{pm.rootBlockChain.GetBlockByNumber(0).Hash()},
		}, {
			&p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.CurrentBlock().Hash(), Limit: 1, Direction: 0},
			[]common.Hash{pm.rootBlockChain.CurrentBlock().Hash()},
		},
		// Ensure protocol limits are honored
		{
			&p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.CurrentBlock().ParentHash(), Limit: uint32(limit), Direction: 0},
			//GetBlockHashesFromHash return hash up to limit which do not include the hash in parameter
			pm.rootBlockChain.GetBlockHashesFromHash(pm.rootBlockChain.CurrentBlock().Hash(), limit),
		}, {
			// Check that requesting more than available is handled gracefully
			&p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.GetHeaderByNumber(2).Hash(), Limit: 4, Direction: 0},
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
		headers := []*types.RootBlockHeader{}
		for _, hash := range tt.expect {
			headers = append(headers, pm.rootBlockChain.GetHeader(hash).(*types.RootBlockHeader))
		}
		// Send the hash request and verify the response
		go handleMsg(clientPeer)
		rheaders, err := clientPeer.GetRootBlockHeaderList(tt.query.BlockHash, tt.query.Limit, true)
		if err != nil {
			t.Errorf("test %d: make message failed: %v", i, err)
		}

		if len(rheaders) != len(headers) {
			t.Errorf("test %d: peer result count is mismatch: got %d, want %d", i, len(rheaders), len(headers))
		}
		for index, header := range rheaders {
			if header.Hash() != headers[index].Hash() {
				t.Errorf("test %d: peer result %d count is mismatch: got %v, want %v", i, index, header.Hash(), headers[index].Hash())
			}
		}
	}
}

func TestCloseConnWithErr(t *testing.T) {
	chainLength := uint64(1024)
	pm, _ := newTestProtocolManagerMust(t, int(chainLength), nil, NewFakeSynchronizer(1), nil)

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
		{p2p.GetRootBlockHeaderListRequestMsg, &p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.GetBlockByNumber(chainLength / 2).Hash(), Limit: 1, Direction: 1}},             // Wrong direction}
		{p2p.GetRootBlockHeaderListRequestMsg, &p2p.GetRootBlockHeaderListRequest{BlockHash: pm.rootBlockChain.Genesis().Hash(), Limit: uint32(rootBlockHeaderListLimit + 10), Direction: 0}}, // limit larger than expected
		{p2p.GetRootBlockHeaderListRequestMsg, &p2p.GetRootBlockHeaderListRequest{BlockHash: unknown, Limit: 1, Direction: 0}},                                                                // no exist block hash
	}
	// Run each of the tests and verify the results against the chain
	for i, request := range invalidReqs {
		peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)

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
	pm, _ := newTestProtocolManagerMust(t, rootBlockBatchSize+15, nil, NewFakeSynchronizer(1), nil)
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
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
	shardConns := getShardConnForP2P(2, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, nil, func(fullShardId uint32) []ShardConnForP2P {
		return shardConns
	})
	minorBlocks := generateMinorBlocks(blockcount)
	minorHeaders := make([]*types.MinorBlockHeader, blockcount)
	for i, block := range minorBlocks {
		minorHeaders[i] = block.Header()
	}

	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	// Create a batch of tests for various scenarios
	limit := uint64(minorBlockHeaderListLimit)
	tests := []struct {
		query  *p2p.GetMinorBlockHeaderListRequest // The hashList to execute for header retrieval
		expect []*types.MinorBlockHeader           // The hashes of the block whose headers are expected
	}{
		// A single random block should be retrievable by hash
		{
			&p2p.GetMinorBlockHeaderListRequest{BlockHash: minorHeaders[limit/2].Hash(), Limit: 1, Direction: 0},
			[]*types.MinorBlockHeader{minorHeaders[limit/2]},
		}, {
			&p2p.GetMinorBlockHeaderListRequest{BlockHash: minorHeaders[limit/2].Hash(), Limit: 3, Direction: 0},
			minorHeaders[limit/2-2 : limit/2+1],
		},
		// The chain endpoints should be retrievable
		{
			&p2p.GetMinorBlockHeaderListRequest{BlockHash: minorHeaders[0].Hash(), Limit: 1, Direction: 0},
			minorHeaders[:1],
		}, {
			&p2p.GetMinorBlockHeaderListRequest{BlockHash: minorHeaders[blockcount-1].Hash(), Limit: 1, Direction: 0},
			minorHeaders[blockcount-1:],
		},
		// Ensure protocol limits are honored
		{
			&p2p.GetMinorBlockHeaderListRequest{BlockHash: minorHeaders[blockcount-1].GetParentHash(), Limit: uint32(limit), Direction: 0},
			//GetBlockHashesFromHash return hash up to limit which do not include the hash in parameter
			minorHeaders[blockcount-int(limit) : blockcount-1],
		}, {
			// Check that requesting more than available is handled gracefully
			&p2p.GetMinorBlockHeaderListRequest{BlockHash: pm.rootBlockChain.GetHeaderByNumber(2).Hash(), Limit: 4, Direction: 0},
			minorHeaders[:3],
		},
	}
	// Run each of the tests and verify the results against the chain
	for i, tt := range tests {
		go handleMsg(clientPeer)
		for _, conn := range shardConns {
			conn.(*mock_master.MockShardConnForP2P).EXPECT().GetMinorBlockHeaders(tt.query).Return(
				&p2p.GetMinorBlockHeaderListResponse{
					RootTip:         *pm.rootBlockChain.CurrentHeader().(*types.RootBlockHeader),
					ShardTip:        *minorHeaders[blockcount-1],
					BlockHeaderList: tt.expect,
				}, nil).AnyTimes()
		}
		rheaders, err := clientPeer.GetMinorBlockHeaderList(tt.query.BlockHash, tt.query.Limit, 0, true)

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

func TestGetMinorBlocks(t *testing.T) {
	ctrl := gomock.NewController(t)
	blockcount := minorBlockBatchSize + 15
	shardConns := getShardConnForP2P(2, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, nil, func(fullShardId uint32) []ShardConnForP2P {
		return shardConns
	})
	minorBlocks := generateMinorBlocks(blockcount)
	hashList := make([]common.Hash, blockcount)
	for i, block := range minorBlocks {
		hashList[i] = block.Hash()
	}
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
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
		for _, conn := range shardConns {
			conn.(*mock_master.MockShardConnForP2P).EXPECT().GetMinorBlocks(&p2p.GetMinorBlockListRequest{MinorBlockHashList: tt.hashList}).Return(
				&p2p.GetMinorBlockListResponse{MinorBlockList: tt.expect}, nil).AnyTimes()
		}
		rheaders, err := clientPeer.GetMinorBlockList(tt.hashList, 2)

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
	shardConns := getShardConnForP2P(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, sync, func(fullShardId uint32) []ShardConnForP2P {
		return shardConns
	})
	minorBlock := generateMinorBlocks(1)[0]
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	for _, conn := range shardConns {
		conn.(*mock_master.MockShardConnForP2P).EXPECT().
			AddMinorBlock(gomock.Any()).Return(true, nil).Times(1)
	}
	err := clientPeer.SendNewMinorBlock(2, minorBlock)
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	for _, conn := range shardConns {
		conn.(*mock_master.MockShardConnForP2P).EXPECT().
			AddMinorBlock(gomock.Any()).DoAndReturn(func(request *p2p.NewBlockMinor) (bool, error) {
			errc <- nil
			return false, errors.New("expected error")
		}).Times(1)
	}
	err = clientPeer.SendNewMinorBlock(2, minorBlock)
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
	shardConns := getShardConnForP2P(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, nil, func(fullShardId uint32) []ShardConnForP2P {
		return shardConns
	})
	txs := newTestTransactionList(10)
	hashList := make([]common.Hash, 0, len(txs))
	for _, tx := range txs {
		hashList = append(hashList, tx.Hash())
	}
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	for _, conn := range shardConns {
		conn.(*mock_master.MockShardConnForP2P).EXPECT().
			AddTransactions(gomock.Any()).DoAndReturn(func(request *p2p.NewTransactionList) (*rpc.HashList, error) {
			errc <- nil
			return &rpc.HashList{Hashes: hashList}, nil
		}).AnyTimes()
	}
	err := clientPeer.SendTransactions(2, txs)
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	if err := waitChanTilErrorOrTimeout(errc, 2); err != nil {
		t.Errorf("got one error: %v", err.Error())
	}
}

func TestBroadcastNewMinorBlockTip(t *testing.T) {
	ctrl := gomock.NewController(t)
	errc := make(chan error)
	defer ctrl.Finish()
	shardConns := getShardConnForP2P(1, ctrl)
	pm, _ := newTestProtocolManagerMust(t, 15, nil, nil, func(fullShardId uint32) []ShardConnForP2P {
		return shardConns
	})
	minorBlocks := generateMinorBlocks(30)
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	for _, conn := range shardConns {
		conn.(*mock_master.MockShardConnForP2P).EXPECT().HandleNewTip(gomock.Any()).Return(true, nil).Times(1)
	}
	err := clientPeer.SendNewTip(2, &p2p.Tip{RootBlockHeader: pm.rootBlockChain.CurrentBlock().Header(),
		MinorBlockHeaderList: []*types.MinorBlockHeader{minorBlocks[len(minorBlocks)-2].Header()}})
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	for _, conn := range shardConns {
		conn.(*mock_master.MockShardConnForP2P).EXPECT().
			HandleNewTip(gomock.Any()).DoAndReturn(
			func(request *p2p.Tip) (bool, error) {
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
		t.Errorf("peer should be Unregister")
	}
}

func TestBroadcastNewRootBlockTip(t *testing.T) {
	sync := NewFakeSynchronizer(1)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	pm, _ := newTestProtocolManagerMust(t, 15, nil, sync, nil)
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	err := clientPeer.SendNewTip(0, &p2p.Tip{RootBlockHeader: pm.rootBlockChain.CurrentBlock().Header(), MinorBlockHeaderList: nil})
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	timeout := time.NewTimer(time.Duration(2 * time.Second))
	defer timeout.Stop()
	select {
	case <-sync.Task:
		t.Errorf("unexpected task")
	case <-timeout.C:
	}

	blocks := core.GenerateRootBlockChain(pm.rootBlockChain.CurrentBlock(), pm.rootBlockChain.Engine(), 1, nil)
	err = clientPeer.SendNewTip(0, &p2p.Tip{RootBlockHeader: blocks[0].Header(), MinorBlockHeaderList: nil})
	if err != nil {
		t.Errorf("make message failed: %v", err.Error())
	}
	timeout = time.NewTimer(time.Duration(3 * time.Second))
	defer timeout.Stop()
	select {
	case <-sync.Task:
	case <-timeout.C:
		t.Errorf("synchronize task missed")
	}
}

func getShardConnForP2P(n int, ctrl *gomock.Controller) []ShardConnForP2P {
	shardConns := make([]ShardConnForP2P, 0, n)
	for i := 0; i < n; i++ {
		sc := mock_master.NewMockShardConnForP2P(ctrl)
		shardConns = append(shardConns, sc)
	}

	return shardConns
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
	contentEnc, err := p2p.Encrypt(metadata, op, qkcMsg.RpcID, content)
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

func handleMsg(peer *peer) error {
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
			c <- blockHeaderResp.BlockHeaderList
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
		var minorHeaderResp p2p.GetMinorBlockHeaderListResponse

		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorHeaderResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- minorHeaderResp.BlockHeaderList
		}
	case qkcMsg.Op == p2p.GetMinorBlockListResponseMsg:
		var minorBlockResp p2p.GetMinorBlockListResponse
		if err := serialize.DeserializeFromBytes(qkcMsg.Data, &minorBlockResp); err != nil {
			return err
		}
		if c := peer.getChan(qkcMsg.RpcID); c != nil {
			c <- minorBlockResp.MinorBlockList
		}
	default:
		return fmt.Errorf("unknown msg code %d", qkcMsg.Op)
	}
	return nil
}
