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

package master

import (
	"bytes"
	"fmt"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"io/ioutil"
	"testing"
	"time"
)

// Tests that protocol versions and modes of operations are matched up properly.
func TestProtocolCompatibility(t *testing.T) {
	pm, _, err := newTestProtocolManager(0, nil)
	if pm != nil {
		defer pm.Stop()
	}
	if err != nil {
		t.Errorf("have error %v", err)
	}
}

func TestGetRootBlockHeaders(t *testing.T) {
	pm, _ := newTestProtocolManagerMust(t, rootBlockHeaderListLimit+15, nil)
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	// Create a batch of tests for various scenarios
	limit := uint64(rootBlockHeaderListLimit)
	tests := []struct {
		query  *p2p.GetRootBlockHeaderListRequest // The query to execute for header retrieval
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
	pm, _ := newTestProtocolManagerMust(t, int(chainLength), nil)

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
			t.Errorf("test %d: make message failed: %v", i, err)
		}
		if err == nil {
			t.Errorf("test %d: read msg should be timeout", i)
		}
		if pm.peers.Peer(peer.id) != nil {
			t.Errorf("test %d: peer should be Unregister", i)
		}
	}
}

func TestGetRootBlocks(t *testing.T) {
	pm, _ := newTestProtocolManagerMust(t, rootBlockBatchSize+15, nil)
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

func TestBloadcast(t *testing.T) {
	pm, _ := newTestProtocolManagerMust(t, rootBlockBatchSize+15, nil)
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	defer peer.close()

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
	}
	// Run each of the tests and verify the results against the chain
	for i, request := range tests {
		// Collect the headers to expect in the response
		blocks := make([]*types.RootBlock, 0, len(request.RootBlockHashList))
		for _, hash := range request.RootBlockHashList {
			blocks = append(blocks, pm.rootBlockChain.GetBlock(hash).(*types.RootBlock))
		}
		// Send the hash request and verify the response
		msg, err := p2p.MakeMsg(p2p.GetRootBlockListRequestMsg, peer.getRpcId(), p2p.Metadata{}, request)
		if err != nil {
			t.Errorf("test %d: make message failed: %v", i, err)
		}
		peer.app.WriteMsg(msg)
		response := p2p.GetRootBlockListResponse{RootBlockList: blocks}
		if _, err := ExpectMsg(peer.app, p2p.GetRootBlockListResponseMsg, p2p.Metadata{Branch: 0}, &response); err != nil {
			t.Errorf("test %d: headers mismatch: %v", i, err)
		}
		/*
			rblocks, err := peer.GetRootBlockList(request.RootBlockHashList)
			if len(rblocks) != len(blocks) {
				t.Errorf("test %d: peer result count is mismatch: got %d, want %d", i, len(rblocks), len(blocks))
			}
			for index, block := range rblocks {
				if block.Hash() != blocks[i].Hash() {
					t.Errorf("test %d: peer result %d count is mismatch: got %v, want %v", i, index, block.Hash(), blocks[i].Hash())
				}
			}*/
	}
}

func readOrTimeOut(peer *testPeer) error {
	errc := make(chan error, 1)
	go func() {
		_, err := peer.app.ReadMsg()
		errc <- err
	}()

	timeout := time.NewTimer(1)
	defer timeout.Stop()
	select {
	case err := <-errc:
		if err != nil {
			return err
		}
	case <-timeout.C:
		fmt.Println("return disc Read Time out")
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
	default:
		return fmt.Errorf("unknown msg code %d", qkcMsg.Op)
	}
	return nil
}
