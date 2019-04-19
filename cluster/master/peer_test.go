package master

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/p2p"
	"github.com/ethereum/go-ethereum/common"
	"testing"
)

func TestPeerGetRootBlockHeaders(t *testing.T) {
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

func TestPeerGetRootBlocks(t *testing.T) {
	pm, _ := newTestProtocolManagerMust(t, rootBlockBatchSize+15, nil)
	peer, _ := newTestPeer("peer", int(qkcconfig.P2PProtocolVersion), pm, true)
	clientPeer := newTestClientPeer(int(qkcconfig.P2PProtocolVersion), peer.app)
	defer peer.close()

	// Create a batch of tests for various scenarios
	limit := uint64(rootBlockBatchSize)
	tests := []*p2p.GetRootBlockListRequest{
		// A single random block should be retrievable by hash
		{[]common.Hash{pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash()}},
		{ // multi hash retrievable
			[]common.Hash{
				pm.rootBlockChain.GetBlockByNumber(limit / 2).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 1).Hash(),
				pm.rootBlockChain.GetBlockByNumber(limit/2 - 2).Hash(),
			},
		},
		// The chain endpoints should be retrievable
		{[]common.Hash{pm.rootBlockChain.Genesis().Hash()}},
		{[]common.Hash{pm.rootBlockChain.CurrentBlock().Hash()}},
		// Ensure protocol limits are honored
		{pm.rootBlockChain.GetBlockHashesFromHash(pm.rootBlockChain.CurrentBlock().Hash(), limit)},
	}
	// Run each of the tests and verify the results against the chain
	for i, request := range tests {
		// Collect the headers to expect in the response
		blocks := make([]*types.RootBlock, 0, len(request.RootBlockHashList))
		for _, hash := range request.RootBlockHashList {
			blocks = append(blocks, pm.rootBlockChain.GetBlock(hash).(*types.RootBlock))
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
