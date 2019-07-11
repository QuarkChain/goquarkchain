package test

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
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
		time.Sleep(10 * time.Millisecond) // 0.001 second
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

func TestAddRootBlockRequestList(t *testing.T) {
	// fullShardId, _ := geneAcc.QKCAddress.GetFullShardID(2)
	var shardSIze uint32 = 2
	geneAcc, clstrList := CreateClusterList(1, 1, shardSIze, 1, nil)
	clstrList.Start()
	defer clstrList.Stop()

	var (
		mstr0 = clstrList[0].GetMaster()
		// mstr1          = clstrList[1].GetMaster()
		id0            = uint32(0<<16 | shardSIze | 0)
		id1            = uint32(0<<16 | shardSIze | 1)
		shard0         = clstrList[0].GetShard(id0)
		shard1         = clstrList[0].GetShard(id1)
		fullShardId, _ = geneAcc.QKCAddress.GetFullShardID(1)
		b1             *types.MinorBlock
	)

	uncfmdHderList := make([]*types.MinorBlockHeader, 0, 1)

	for i := 0; i < 7; i++ {
		b1 = tipGen(geneAcc, shard0)
		if err := shard0.AddMinorBlock(b1); err != nil {
			t.Error("failed to add minor block", "fullShardId", b1.Header().Branch.Value, "err", err)
		}
		fmt.Println("========= b1", b1.Hash().Hex(), b1.Number(), b1.Time())
		uncfmdHderList = append(uncfmdHderList, b1.Header())
	}

	b2 := tipGen(geneAcc, shard1)
	fmt.Println("========= b2", b2.Hash().Hex(), b2.Number(), b2.Time())
	if err := shard1.AddMinorBlock(b2); err != nil {
		t.Error("failed to add minor block", "fullShardId", b2.Header().Branch.Value, "err", err)
	}
	uncfmdHderList = append(uncfmdHderList, b2.Header())

	// create and add root/minor block
	rBlock0 := clstrList[0].CreateAndInsertBlocks(fullShardId)
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

	// minor block is downloaded
	assert.Equal(t, b1.Header().Number, 7)
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return shard0.MinorBlockChain.GetMinorTip().Hash() == b1.Hash()
	}, 1), true)
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

	geneAcc, clstrList := CreateClusterList(1, 2, shardSize, 1, geneRHeights)
	fullShardId, _ := geneAcc.QKCAddress.GetFullShardID(2)
	clstrList.Start()
	defer clstrList.Stop()

	rBlock := clstrList[0].CreateAndInsertBlocks(fullShardId)
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
	clstrList[0].CreateAndInsertBlocks(fullShardId)

	// check account nonce
	accData, err = mstr.GetAccountData(&geneAcc.QKCAddress, nil)
	if err != nil {
		t.Error("failed to get account data", "address", geneAcc.Address(), "err", err)
	}
	assert.Equal(t, accData[fullShardId].TransactionCount, uint64(1))
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
