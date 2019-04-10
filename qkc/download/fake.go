package downloader

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

// FakeRootState fake root state for download
type FakeRootState struct {
	Tip types.RootBlockHeader
}

// ContainRootBlockByHash if has this hash
func (r FakeRootState) ContainRootBlockByHash(blockHash common.Hash) bool {
	return false
}

// IsMinorBlockValidated fake func
func (r FakeRootState) IsMinorBlockValidated(hash common.Hash) bool {
	return true
}

// AddValidatedMinorBlockHash fake func
func (r FakeRootState) AddValidatedMinorBlockHash(hash common.Hash) bool {
	return true
}

// FakeShardState fake shard state
type FakeShardState struct {
	HeaderTip types.MinorBlockHeader
}

// FakeMasterServer fake master server
type FakeMasterServer struct {
}

// AddRootBlock add root block
func (m *FakeMasterServer) AddRootBlock(rootBlock *types.RootBlock) error {
	return nil
}

// GetSlaveConnection get slave Connection
func (m *FakeMasterServer) GetSlaveConnection(branch account.Branch) *FakeSlaveConn {
	return &FakeSlaveConn{}
}

// FakeSlaveConn fake slave conn
type FakeSlaveConn struct {
	ShardState *FakeShardState
	Shard      *FakeShard
}

// FakeShard fake shard
type FakeShard struct {
}

//AddBlock shard add block
func (f *FakeShard) AddBlock(block *types.MinorBlock) error {
	return nil
}

func  (r *RootDownloader)testMinor(p *peerConnection, bodys []*types.RootBlock) {
	mapp := make(map[uint32]*types.MinorBlockHeader, 0)
	for _, v := range bodys {
		for _, vv := range v.MinorBlockHeaders() {
			mapp[vv.Branch.Value] = vv
		}
		if _, ok := mapp[1]; ok {
			if _, ok1 := mapp[65537]; ok1 {
				break
			}
		}
	}

	for k, v := range mapp {
		err:=r.MinorDownloader.SyncWithPeer(p,v,k)
		if err!=nil{
			//TODO delete panic
			panic(err)
		}
	}
}

