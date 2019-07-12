package test

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func tipGen(geneAcc *account.Account, shrd *shard.ShardBackend) *types.MinorBlock {
	iBlock, err := shrd.CreateBlockToMine()
	if err != nil {
		utils.Fatalf("failed to create minor block to mine: %v", err)
	}
	return iBlock.(*types.MinorBlock)
}

func assertTrueWithTimeout(f func() bool, duration int64) bool {
	deadLine := time.Now().Unix() + duration
	for !f() && time.Now().Unix() < deadLine {
		time.Sleep(500 * time.Millisecond) // 0.5 second
	}
	return f()
}

func TestSingleCluster(t *testing.T) {
	_, clstrList := CreateClusterList(1, 1, 1, 1, nil)
	assert.Equal(t, len(clstrList), 1)
}

func TestThreeClusters(t *testing.T) {
	_, clstrList := CreateClusterList(3, 1, 1, 1, nil)
	assert.Equal(t, len(clstrList), 3)
}

func TestCreateShardAtDifferentHeight(t *testing.T) {
	var (
		id1          = uint32(0<<16 | 1 | 0)
		id2          = uint32(1<<16 | 1 | 0)
		geneRHeights = map[uint32]uint32{
			id1: 1,
			id2: 2,
		}
		shardSize uint32 = 2
	)

	_, clstrList := CreateClusterList(1, 2, shardSize, 1, geneRHeights)
	clstrList.Start()
	defer clstrList.Stop()

	rBlock := clstrList[0].CreateAndInsertBlocks([]uint32{id1, id2}, 0)
	assert.Equal(t, rBlock.NumberU64(), uint64(1))
	assert.Equal(t, rBlock.MinorBlockHeaders().Len(), 0)

	assert.NotEqual(t, clstrList[0].GetShard(id1), (*shard.ShardBackend)(nil))
	assert.Equal(t, clstrList[0].GetShard(id2), (*shard.ShardBackend)(nil))

	/*shardState, err := clstrList[0].GetShardState(id1)
	if err != nil {
		t.Error("failed to get shard state", "fullshardId", id1, "err", err)
	}*/
}

func TestGetPrimaryAccountData(t *testing.T) {
	geneAcc, clstrList := CreateClusterList(1, 1, 2, 2, nil)
	fullShardId, _ := geneAcc.QKCAddress.GetFullShardID(2)
	clstrList.Start()
	defer clstrList.Stop()

	mstr := clstrList[0].GetMaster()
	mstr.SetMining(true)

	// check account nonce
	accData, err := mstr.GetAccountData(&geneAcc.QKCAddress, nil)
	if err != nil {
		t.Error("failed to get account data", "address", geneAcc.Address(), "err", err)
	}
	assert.Equal(t, accData[fullShardId].TransactionCount, uint64(0))

	tx := createTx(geneAcc.QKCAddress)
	if err := mstr.AddTransaction(tx); err != nil {
		t.Error("failed to add tx", "err", err)
	}

	// create and add master|shards blocks
	clstrList[0].CreateAndInsertBlocks([]uint32{fullShardId}, 0)

	// check account nonce
	accData, err = mstr.GetAccountData(&geneAcc.QKCAddress, nil)
	if err != nil {
		t.Error("failed to get account data", "address", geneAcc.Address(), "err", err)
	}
	assert.Equal(t, accData[fullShardId].TransactionCount, uint64(1))
}

func TestAddTransaction(t *testing.T) {
	var shardSize uint32 = 2
	geneAcc, clstrList := CreateClusterList(2, 1, shardSize, 1, nil)
	clstrList.Start()
	defer clstrList.Stop()

	var (
		id0            = uint32(0<<16 | shardSize | 0)
		id1            = uint32(0<<16 | shardSize | 1)
		mstr0          = clstrList[0].GetMaster()
		mstr1          = clstrList[1].GetMaster()
		shard0         = clstrList[0].GetShard(id0)
		fullShardId, _ = geneAcc.QKCAddress.GetFullShardID(2)
	)

	// send tx in shard 0
	tx0 := createTx(geneAcc.QKCAddress)
	if err := mstr0.AddTransaction(tx0); err != nil {
		t.Error("failed to add transaction", "err", err)
	}
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		state0, err := shard0.MinorBlockChain.GetShardStatus()
		if err != nil {
			return false
		}
		return state0.PendingTxCount == uint32(1)
	}, 1), true)

	// send the same tx in shard 1
	addr := geneAcc.QKCAddress.AddressInShard(id1)
	tx1 := createTx(addr)
	if err := mstr0.AddTransaction(tx1); err != nil {
		t.Error("failed to add transaction", "err", err)
	}
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		state0, err := shard0.MinorBlockChain.GetShardStatus()
		if err != nil {
			return false
		}
		return state0.PendingTxCount == uint32(1)
	}, 1), true)

	// check another cluster' pending pool
	// time.Sleep(10 * time.Second)
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		state0, err := clstrList[1].GetShard(fullShardId).MinorBlockChain.GetShardStatus()
		if err != nil {
			return false
		}
		return state0.PendingTxCount == uint32(1)
	}, 10), true)

	rBlock := clstrList[0].CreateAndInsertBlocks([]uint32{id0, id1}, 0)

	// verify address account and nonce
	accdata, err := mstr0.GetAccountData(&geneAcc.QKCAddress, nil)
	assert.Equal(t, accdata[fullShardId].TransactionCount, uint64(1))

	// sleep 10 seconds so that another can sync blocks
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		rBlockTip, err := mstr1.GetRootBlockByNumber(nil)
		if err != nil {
			return false
		}
		return rBlock.Hash() == rBlockTip.Hash()
	}, 10), true)

	// verify tx hash in another cluster
	mBlock, _, err := mstr1.GetTransactionByHash(tx0.Hash(), account.NewBranch(fullShardId))
	if err != nil {
		t.Error("failed to get minor block in another cluster", "err", err)
	}
	if mBlock.Transaction(tx0.Hash()).Hash() != tx0.Hash() {
		t.Error("tx hash is diffierent", "expected", mBlock.Transaction(tx0.Hash()).Hash(), "actual", tx0.Hash())
	}

	// verify address account and nonce in another cluster
	accdata1, err := mstr1.GetAccountData(&geneAcc.QKCAddress, nil)
	assert.Equal(t, accdata1[fullShardId].TransactionCount, uint64(1))
	assert.Equal(t, accdata1[fullShardId].Balance.Uint64() == accdata[fullShardId].Balance.Uint64(), true)
}

func TestAddMinorBlockRequestList(t *testing.T) {
	var shardSize uint32 = 2
	_, clstrList := CreateClusterList(2, 1, shardSize, 1, nil)
	clstrList.Start()
	defer clstrList.Stop()

	var (
		id0    = uint32(0<<16 | shardSize | 0)
		id1    = uint32(0<<16 | shardSize | 1)
		mstr0  = clstrList[0].GetMaster()
		mstr1  = clstrList[1].GetMaster()
		shard0 = clstrList[0].GetShard(id0)
		shard1 = clstrList[1].GetShard(id1)
	)

	rBlock := clstrList[0].CreateAndInsertBlocks([]uint32{id0, id1}, 0)
	assert.Equal(t, shard1.MinorBlockChain.GetBlock(rBlock.Hash()), (*types.MinorBlock)(nil))

	rBlockTip0, err := mstr0.GetRootBlockByNumber(nil)
	if err != nil {
		t.Error("failed to get root block by number", "err", err)
	}
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return rBlock.Hash() == rBlockTip0.Hash()
	}, 1), true)

	assert.Equal(t, assertTrueWithTimeout(func() bool {
		rBlockTip := shard0.MinorBlockChain.GetRootTip()
		return rBlockTip.Hash() == rBlock.Hash()
	}, 1), true)

	// check neighbor cluster is the same height
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		rBlockTip1, err := mstr1.GetRootBlockByNumber(nil)
		if err != nil {
			return false
		}
		return rBlockTip1.Hash() == rBlock.Hash()
	}, 10), true)
}

func TestAddRootBlockRequestList(t *testing.T) {
	// fullShardId, _ := geneAcc.QKCAddress.GetFullShardID(2)
	var shardSIze uint32 = 2
	geneAcc, clstrList := CreateClusterList(2, 1, shardSIze, 1, nil)
	clstrList.Start()
	defer clstrList.Stop()

	var (
		mstr0     = clstrList[0].GetMaster()
		mstr1     = clstrList[1].GetMaster()
		id0       = uint32(0<<16 | shardSIze | 0)
		id1       = uint32(0<<16 | shardSIze | 1)
		shard0    = clstrList[0].GetShard(id0)
		shard1    = clstrList[0].GetShard(id1)
		maxBlocks = shard0.Config.MaxBlocksPerShardInOneRootBlock() - 1
		b0        *types.MinorBlock
	)

	for i := 0; i < int(maxBlocks); i++ {
		b0 = tipGen(geneAcc, shard0)
		if err := shard0.AddMinorBlock(b0); err != nil {
			t.Error("failed to add minor block", "fullShardId", b0.Header().Branch.Value, "err", err)
		}
	}
	// minor block is downloaded
	assert.Equal(t, b0.Header().Number, uint64(maxBlocks))

	b1 := tipGen(geneAcc, shard1)
	if err := shard1.AddMinorBlock(b1); err != nil {
		t.Error("failed to add minor block", "fullShardId", b1.Header().Branch.Value, "err", err)
	}

	// create and add root/minor block
	rBlock0 := clstrList[0].CreateAndInsertBlocks(nil, 10)
	// make sure the root block tip of local cluster is changed
	rBlockTip0, err := mstr0.GetRootBlockByNumber(nil)
	if err != nil {
		t.Error("failed to get root block by number", "err", err)
	}
	assert.Equal(t, rBlockTip0.Header().Hash(), rBlock0.Header().Hash())
	// make sure the root tip of cluster 1 is changed
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return mstr0.GetRootTip().Hash() == rBlock0.Header().Hash()
	}, 2), true)

	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return shard0.MinorBlockChain.GetMinorTip().Hash() == b0.Hash()
	}, 1), true)
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return shard1.MinorBlockChain.GetMinorTip().Hash() == b1.Hash()
	}, 1), true)

	// check neighbor cluster is the same height
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		rBlockTip1, err := mstr1.GetRootBlockByNumber(nil)
		if err != nil {
			return false
		}
		return rBlock0.Hash() == rBlockTip1.Hash()
	}, 10), true)
}

func TestGetRootBlockHeaderSyncWithFork(t *testing.T) {
	_, clstrList := CreateClusterList(2, 1, 1, 1, nil)
	clstrList.Start()
	defer clstrList.Stop()

	var (
		mstr0         = clstrList[0].GetMaster()
		mstr1         = clstrList[1].GetMaster()
		rootBlockList = make([]*types.RootBlock, 0, 10)
	)
	for i := 0; i < 10; i++ {
		iBlock, err := mstr0.CreateBlockToMine()
		if err != nil {
			assert.Error(t, err)
		}
		rBlock := iBlock.(*types.RootBlock)
		if err := mstr0.AddRootBlock(rBlock); err != nil {
			assert.Error(t, err)
		}
		rootBlockList = append(rootBlockList, rBlock)
	}
	for i := 0; i < 2; i++ {
		if err := mstr1.AddRootBlock(rootBlockList[i]); err != nil {
			assert.Error(t, err)
		}
	}
	for i := 0; i < 3; i++ {
		iBlock, err := mstr1.CreateBlockToMine()
		if err != nil {
			assert.Error(t, err)
		}
		rBlock := iBlock.(*types.RootBlock)
		if err := mstr1.AddRootBlock(rBlock); err != nil {
			assert.Error(t, err)
		}
	}
}

func TestShardGenesisFork(t *testing.T) {
	var (
		shardSize    uint32 = 2
		id0                 = uint32(0<<16 | shardSize | 0)
		id1                 = uint32(0<<16 | shardSize | 1)
		geneRHeights        = map[uint32]uint32{
			id0: 0,
			id1: 1,
		}
		mHeader0, mHeader1 *types.MinorBlockHeader
	)
	_, clstrList := CreateClusterList(2, 1, shardSize, 1, geneRHeights)
	clstrList.Start()
	defer clstrList.Stop()

	root0 := clstrList[0].CreateAndInsertBlocks([]uint32{id0}, 3)
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		genesis0 := clstrList[0].GetShard(id1).MinorBlockChain.GetBlockByNumber(0)
		if root0 == nil || common.IsNil(genesis0) {
			return false
		}
		mHeader0 = genesis0.IHeader().(*types.MinorBlockHeader)
		return root0.Hash() == mHeader0.PrevRootBlockHash
	}, 1), true)

	// root1 := clstrList[1].CreateAndInsertBlocks([]uint32{id0, id1}, 3)
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		rootHeight := uint64(1)
		root1, _ := clstrList[1].GetMaster().GetRootBlockByNumber(&rootHeight)
		genesis1 := clstrList[1].GetShard(id1).MinorBlockChain.GetBlockByNumber(0)
		if root1 == nil || common.IsNil(genesis1) {
			return false
		}
		mHeader1 = genesis1.IHeader().(*types.MinorBlockHeader)
		return root1.Hash() == mHeader1.PrevRootBlockHash
	}, 10), true)

	assert.Equal(t, mHeader0.Hash() == mHeader1.Hash(), true)

	root2 := clstrList[1].CreateAndInsertBlocks([]uint32{id0, id1}, 3)
	// after minered check roottip
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return clstrList[1].GetMaster().GetRootTip().Number == uint32(2)
	}, 1), true)

	// check the two cluster genesis root hash is the same
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		iBlock := clstrList[0].GetShard(id1).MinorBlockChain.GetBlockByNumber(0)
		if common.IsNil(iBlock) {
			return false
		}
		mHeader := iBlock.IHeader().(*types.MinorBlockHeader)
		return mHeader.Hash() == mHeader1.Hash()
	}, 1), true)

	assert.Equal(t, assertTrueWithTimeout(func() bool {
		mHeaderTip := clstrList[0].GetShard(id1).MinorBlockChain.GetRootTip()
		return mHeaderTip.Hash() == root2.Hash()
	}, 10), true)
}

func TestBroadcastCrossShardTransactions(t *testing.T) {
	var (
		chainSize uint32 = 2
		shardSize uint32 = 2
		//id0              = uint32(0<<16 | shardSize | 0)
		//id1              = uint32(0<<16 | shardSize | 1)
	)
	_, clstrList := CreateClusterList(1, chainSize, shardSize, chainSize, nil)
	clstrList.Start()
	defer clstrList.Stop()
	// root := clstrList[0].CreateAndInsertBlocks([]uint32{id0, id1}, 3)
}

func TestGetWorkFromSlave(t *testing.T) {
	var (
		chainSize uint32 = 2
		shardSize uint32 = 2
		//id0              = uint32(0<<16 | shardSize | 0)
		//id1              = uint32(0<<16 | shardSize | 1)
	)
	_, clstrList := CreateClusterList(2, chainSize, shardSize, chainSize, nil)
	clstrList.Start()
	defer clstrList.Stop()

	mstr0 := clstrList[0].GetMaster()
	mstr0.GetRootTip()
}

func TestGetMinorBlockHeadersWithSkip(t *testing.T) {

}
