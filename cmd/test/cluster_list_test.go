// +build integrationTest

package test

import (
	"bytes"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/shard"
	"github.com/QuarkChain/goquarkchain/cmd/utils"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"math/big"
	"reflect"
	"testing"
	"time"
)

func tipGen(geneAcc *account.Account, shrd *shard.ShardBackend) *types.MinorBlock {
	iBlock, _, err := shrd.CreateBlockToMine()
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
	assert.Equal(t, len(clstrList)-1, 1)
}

func TestThreeClusters(t *testing.T) {
	_, clstrList := CreateClusterList(3, 1, 1, 1, nil)
	assert.Equal(t, len(clstrList)-1, 3)
}

func TestShardGenesisForkFork(t *testing.T) {
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
	clstrList.Start(5*time.Second, true)
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
		return clstrList[1].GetMaster().GetCurrRootHeader().Number == uint32(2)
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

func TestGetMinorBlockHeadersWithSkip(t *testing.T) {
	var (
		numCluster                  = 2
		chainSize, shardSize uint32 = 1, 2
	)
	_, clstrList := CreateClusterList(numCluster, chainSize, shardSize, chainSize, nil)
	clstrList.Start(5*time.Second, true)
	defer clstrList.Stop()
	clstrList.PeerList()

	var (
		id0       = uint32(0<<16 | shardSize | 0)
		peer1     = clstrList.GetPeerByIndex(1)
		mBHeaders = make([]*types.MinorBlockHeader, 0, 10)
	)

	for i := 0; i < 3; i++ {
		iBlock, _, err := clstrList[0].GetShard(id0).CreateBlockToMine()
		if err != nil {
			t.Errorf("failed to create block, fullShardId: %d, err: %v", id0, err)
		}
		mBlock := iBlock.(*types.MinorBlock)
		if err := clstrList[0].GetShard(id0).InsertMinedBlock(iBlock); err != nil {
			t.Errorf("failed to insert minered block, fullShardId:%d, err: %v", id0, err)
		}
		mBHeaders = append(mBHeaders, mBlock.Header())
	}
	assert.Equal(t, mBHeaders[len(mBHeaders)-1].Number, uint64(3))

	clstrList[0].CreateAndInsertBlocks(nil, 6)

	// TODO reverse == false can't use.
	// TODO skip parameter need to be added.
	mHeaders, err := peer1.GetMinorBlockHeaderList(mBHeaders[2].Hash(), 3, id0, true)
	if err != nil {
		t.Errorf("failed to get minor block header list by peer, err: %v", err)
	}
	assert.Equal(t, len(mHeaders), 3)
	assert.Equal(t, mHeaders[2].Hash(), mBHeaders[0].Hash())
	assert.Equal(t, mHeaders[1].Hash(), mBHeaders[1].Hash())
	assert.Equal(t, mHeaders[0].Hash(), mBHeaders[2].Hash())
}

func TestCreateShardAtDifferentHeight(t *testing.T) {
	var (
		shardSize    uint32 = 2
		id0                 = uint32(0<<16 | shardSize | 0)
		id1                 = uint32(0<<16 | shardSize | 1)
		id2                 = uint32(1<<16 | shardSize | 0)
		id3                 = uint32(1<<16 | shardSize | 1)
		geneRHeights        = map[uint32]uint32{
			id0: 1,
			id1: 2,
		}
	)

	_, clstrList := CreateClusterList(1, 2, shardSize, 1, geneRHeights)
	clstrList.Start(0, true)
	defer clstrList.Stop()

	rBlock := clstrList[0].CreateAndInsertBlocks([]uint32{id2, id3}, 0)
	assert.Equal(t, rBlock.NumberU64(), uint64(1))
	assert.Equal(t, rBlock.MinorBlockHeaders().Len(), 4)

	assert.Equal(t, assertTrueWithTimeout(func() bool {
		shrd := clstrList[0].GetShard(id0)
		if shrd == nil {
			return false
		}
		return clstrList[0].GetShard(id0) != (*shard.ShardBackend)(nil)
	}, 1), true)

	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return clstrList[0].GetShard(id1) == (*shard.ShardBackend)(nil)
	}, 1), true)

	/*shardState, err := clstrList[0].GetShardState(id1)
	if err != nil {
		t.Error("failed to get shard state", "fullshardId", id1, "err", err)
	}*/
}

func TestGetPrimaryAccountData(t *testing.T) {
	geneAcc, clstrList := CreateClusterList(1, 1, 2, 2, nil)
	fullShardId, _ := geneAcc.QKCAddress.GetFullShardID(2)
	clstrList.Start(0, true)
	defer clstrList.Stop()

	mstr := clstrList[0].GetMaster()
	mstr.SetMining(true)

	// check account nonce
	accData, err := mstr.GetAccountData(&geneAcc.QKCAddress, nil)
	if err != nil {
		t.Error("failed to get account data", "address", geneAcc.Address(), "err", err)
	}
	assert.Equal(t, accData[fullShardId].TransactionCount, uint64(0))

	tx := createTx(geneAcc.QKCAddress, nil)
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
	clstrList.Start(5*time.Second, true)
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
	tx0 := createTx(geneAcc.QKCAddress, nil)
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
	tx1 := createTx(addr, nil)
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
	clstrList.Start(5*time.Second, true)
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
	clstrList.Start(5*time.Second, true)
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
		return mstr0.GetCurrRootHeader().Hash() == rBlock0.Header().Hash()
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
	clstrList.Start(5*time.Second, true)
	defer clstrList.Stop()

	var (
		mstr0         = clstrList[0].GetMaster()
		mstr1         = clstrList[1].GetMaster()
		rootBlockList = make([]*types.RootBlock, 0, 10)
	)
	for i := 0; i < 10; i++ {
		iBlock, _, err := mstr0.CreateBlockToMine()
		if err != nil {
			assert.Error(t, err)
		}
		rBlock := iBlock.(*types.RootBlock)
		if err := mstr0.AddRootBlock(rBlock); err != nil {
			assert.Error(t, err)
		}
		rootBlockList = append(rootBlockList, rBlock)
	}
	clstrList[0].CreateAndInsertBlocks(nil, 10)

	for i := 0; i < 2; i++ {
		if err := mstr1.AddRootBlock(rootBlockList[i]); err != nil {
			assert.Error(t, err)
		}
	}
	for i := 0; i < 3; i++ {
		iBlock, _, err := mstr1.CreateBlockToMine()
		if err != nil {
			assert.Error(t, err)
		}
		rBlock := iBlock.(*types.RootBlock)
		if err := mstr1.AddRootBlock(rBlock); err != nil {
			assert.Error(t, err)
		}
	}
}

func TestBroadcastCrossShardTransactions(t *testing.T) {
	var (
		chainSize uint32 = 2
		shardSize uint32 = 2
		id0              = uint32(0<<16 | shardSize | 0)
		id1              = uint32(0<<16 | shardSize | 1)
	)
	geneAcc, clstrList := CreateClusterList(1, chainSize, shardSize, chainSize, nil)
	clstrList.Start(0, true)
	defer clstrList.Stop()
	// root := clstrList[0].CreateAndInsertBlocks([]uint32{id0, id1}, 3)
	var (
		toAddr, _ = account.CreatRandomAccountWithFullShardKey(id1)
		mstr      = clstrList[0].GetMaster()
		shrd0     = clstrList[0].GetShard(id0)
		shrd1     = clstrList[0].GetShard(id1)
	)
	// create a root block
	clstrList[0].CreateAndInsertBlocks(nil, 3)

	tx := createTx(geneAcc.QKCAddress, &toAddr)
	err := mstr.AddTransaction(tx)
	assert.NoError(t, err)

	iB0, _, err := shrd0.CreateBlockToMine()
	assert.NoError(t, err)
	iB1, _, err := shrd0.CreateBlockToMine()
	assert.NoError(t, err)

	b0 := iB0.(*types.MinorBlock)
	b1 := iB1.(*types.MinorBlock)
	b1.Header().Time += 1
	assert.Equal(t, b0.Hash(), b1.Hash())
	if err := shrd0.InsertMinedBlock(iB0); err != nil {
		t.Error("failed to insert mined block", "fullShardId", id0, "err", err)
	}
	// clstrList[0].CreateAndInsertBlocks([]uint32{id0, id1}, 3)

	txDepositList := clstrList[0].GetShard(id1).MinorBlockChain.ReadCrossShardTxList(b1.Hash())
	xshardList := txDepositList.TXList
	assert.Equal(t, len(xshardList), 1)
	assert.Equal(t, xshardList[0].TxHash, tx.EvmTx.Hash())
	assert.Equal(t, xshardList[0].From, geneAcc.QKCAddress)
	assert.Equal(t, xshardList[0].To, toAddr)
	assert.Equal(t, xshardList[0].Value.Value.Uint64(), uint64(100))

	if err := shrd0.InsertMinedBlock(iB1); err != nil {
		t.Error("failed to insert mined block", "fullShardId", id0, "err", err)
	}
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return shrd0.MinorBlockChain.CurrentBlock().Hash() == iB0.Hash()
	}, 3), true)

	txDepositList = clstrList[0].GetShard(id1).MinorBlockChain.ReadCrossShardTxList(b0.Hash())
	xshardList = txDepositList.TXList
	assert.Equal(t, len(xshardList), 1)
	assert.Equal(t, xshardList[0].TxHash, tx.EvmTx.Hash())
	assert.Equal(t, xshardList[0].From, geneAcc.QKCAddress)
	assert.Equal(t, xshardList[0].To, toAddr)
	assert.Equal(t, xshardList[0].Value.Value.Uint64(), uint64(100))

	iB2, _, err := shrd1.CreateBlockToMine()
	assert.NoError(t, err)
	b2 := iB2.(*types.MinorBlock)
	_, err = mstr.AddMinorBlock(b2.Branch().Value, b2)
	// push one root block.
	clstrList[0].CreateAndInsertBlocks(nil, 3)

	iB3, _, err := shrd1.CreateBlockToMine()
	assert.NoError(t, err)
	b3 := iB3.(*types.MinorBlock)

	_, err = mstr.AddMinorBlock(b0.Branch().Value, b0)
	assert.NoError(t, err)
	_, err = mstr.AddMinorBlock(b1.Branch().Value, b1)
	assert.NoError(t, err)
	_, err = mstr.AddMinorBlock(b2.Branch().Value, b2)
	assert.NoError(t, err)
	_, err = mstr.AddMinorBlock(b3.Branch().Value, b3)
	assert.NoError(t, err)

	clstrList[0].CreateAndInsertBlocks(nil, 3)
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		accData, err := mstr.GetAccountData(&toAddr, nil)
		if err != nil {
			return false
		}
		return accData[id1].Balance.Uint64() == uint64(100)
	}, 1), true)
}

func TestGetWorkFromSlave(t *testing.T) {
	var (
		chainSize uint32 = 2
		shardSize uint32 = 2
		id0              = uint32(0<<16 | shardSize | 0)
		// id1              = uint32(0<<16 | shardSize | 1)
	)
	_, clstrList := CreateClusterList(1, chainSize, shardSize, chainSize, nil)
	clstrList.Start(5*time.Second, true)
	defer clstrList.Stop()
	var (
		slave = clstrList[0].GetSlavelist()[0]
		mstr  = clstrList[0].GetMaster()
	)
	slave.SetMining(true)

	assert.Equal(t, assertTrueWithTimeout(func() bool {
		work, err := mstr.GetWork(account.NewBranch(id0))
		if err != nil {
			t.Log("failed to get work from slave", "slave id", slave.GetConfig().ID)
			return false
		}
		fmt.Println(work.Number, work.Difficulty.Uint64())
		return work.Difficulty.Uint64() == uint64(10)
	}, 3), true)
	// TODO need to change remote type test.
}

func TestShardSynchronizerWithFork(t *testing.T) {
	var (
		chainSize uint32 = 2
		shardSize uint32 = 2
		id0              = uint32(0<<16 | shardSize | 0)
		// id1              = uint32(0<<16 | shardSize | 1)
	)
	_, clstrList := CreateClusterList(2, chainSize, shardSize, chainSize, nil)
	clstrList.Start(5*time.Second, false)
	defer clstrList.Stop()
	clstrList.PeerList()

	var (
		mstr      = clstrList[0].GetMaster()
		mstr1     = clstrList[1].GetMaster()
		shard00   = clstrList[0].GetShard(id0)
		shard10   = clstrList[1].GetShard(id0)
		blockList = make([]*types.MinorBlock, 0, 13)
	)
	for i := 0; i < 13; i++ {
		iBlock, _, err := shard00.CreateBlockToMine()
		assert.NoError(t, err)
		mBlock := iBlock.(*types.MinorBlock)
		blockList = append(blockList, mBlock)
		_, err = mstr.AddMinorBlock(mBlock.Branch().Value, mBlock)
		assert.NoError(t, err)
	}
	assert.Equal(t, shard00.GetTip(), uint64(13))

	for i := 0; i < 12; i++ {
		iBlock, _, err := shard10.CreateBlockToMine()
		assert.NoError(t, err)
		mBlock := iBlock.(*types.MinorBlock)
		_, err = mstr1.AddMinorBlock(mBlock.Branch().Value, mBlock)
		assert.NoError(t, err)
	}
	assert.Equal(t, shard10.GetTip(), uint64(12))
	clstrList.Start(5*time.Second, true)
	clstrList.PeerList()

	iBlock, _, err := shard00.CreateBlockToMine()
	assert.NoError(t, err)
	mBlock := iBlock.(*types.MinorBlock)
	blockList = append(blockList, mBlock)
	_, err = mstr.AddMinorBlock(mBlock.Branch().Value, mBlock)
	assert.NoError(t, err)

	for _, blk := range blockList {
		assert.Equal(t, assertTrueWithTimeout(func() bool {
			if shard10.GetMinorBlock(blk.Hash(), nil) != nil {
				return true
			}
			return false
		}, 1), true)
		assert.Equal(t, assertTrueWithTimeout(func() bool {
			mBlock, err := mstr1.GetMinorBlockByHash(blk.Hash(), blk.Branch())
			if err != nil || mBlock == nil {
				return false
			}
			return true
		}, 1), true)
	}

	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return shard00.GetTip() == shard10.GetTip()
	}, 1), true)
}

func TestBroadcastCrossShardTransactionsToNeighborOnly(t *testing.T) {
	var (
		chainSize uint32 = 2
		shardSize uint32 = 64
		id0              = uint32(0<<16 | shardSize | 0)
		// id1              = uint32(0<<16 | shardSize | 1)
	)
	_, clstrList := CreateClusterList(1, chainSize, shardSize, 4, nil)
	clstrList.Start(5*time.Second, false)
	defer clstrList.Stop()
	clstrList.PeerList()

	var (
		mstr  = clstrList[0].GetMaster()
		shrd0 = clstrList[0].GetShard(id0)
	)
	clstrList[0].CreateAndInsertBlocks(nil, 2)
	iBlock, _, err := shrd0.CreateBlockToMine()
	assert.NoError(t, err)
	mBlock := iBlock.(*types.MinorBlock)
	_, err = mstr.AddMinorBlock(mBlock.Branch().Value, mBlock)
	assert.NoError(t, err)

	nborShards := make(map[int]bool)
	for i := 0; i < 6; i++ {
		nborShards[1<<uint32(i)] = true
	}
	for shardId := 0; shardId < 64; shardId++ {
		shrdI := clstrList[0].GetShard(shardSize | uint32(shardId))
		xshardTxList := shrdI.MinorBlockChain.ReadCrossShardTxList(mBlock.Hash())
		if nborShards[shardId] {
			assert.NotNil(t, xshardTxList)
		} else {
			assert.Nil(t, xshardTxList)
		}
	}
}

func TestHandleGetMinorBlockListRequestWithTotalDiff(t *testing.T) {
	_, cluster := CreateClusterList(2, 2, 2, 2, nil)
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()

	calCoinBase := func(rootBlock *types.RootBlock) *big.Int {
		ret := new(big.Int).Set(cluster[0].clstrCfg.Quarkchain.Root.CoinbaseAmount)
		rewardTaxRate := cluster[0].clstrCfg.Quarkchain.RewardTaxRate
		ratio := big.NewRat(1, 1)
		ratio.Sub(ratio, rewardTaxRate)
		ratio.Quo(ratio, rewardTaxRate)

		minorBlockFee := new(big.Int)
		for _, header := range rootBlock.MinorBlockHeaders() {
			minorBlockFee.Add(minorBlockFee, header.CoinbaseAmount.Value)
		}
		minorBlockFee.Mul(minorBlockFee, ratio.Num())
		minorBlockFee.Div(minorBlockFee, ratio.Denom())
		ret.Add(ret, minorBlockFee)
		return ret
	}
	tipNumber := cluster[0].master.GetTip()
	rb0, err := cluster[0].master.GetRootBlockByNumber(&tipNumber)
	assert.NoError(t, err)
	var z uint64 = 0
	block0, err := cluster[0].master.GetRootBlockByNumber(&z)
	assert.NoError(t, err)
	//Cluster 0 generates a root block of height 1 with 1e6 difficulty
	coinbaseAmount := calCoinBase(block0)
	rb1 := rb0.Header().CreateBlockToAppend(nil, big.NewInt(1000000), nil, nil,
		nil).Finalize(coinbaseAmount, nil)
	//Cluster 0 broadcasts the root block to cluster 1
	err = cluster[0].master.AddRootBlock(rb1)
	assert.NoError(t, err)
	assert.Equal(t, cluster[0].master.CurrentBlock().Hash(), rb1.Header().Hash())
	//Make sure the root block tip of cluster 1 is changed
	assertTrueWithTimeout(func() bool {
		return cluster[1].master.CurrentBlock().Hash() == rb1.Hash()
	}, 2)
	//Cluster 1 generates a minor block and broadcasts to cluster 0
	b1 := tipGen(nil, cluster[1].GetShard(2))
	assertTrueWithTimeout(func() bool {
		success, err := cluster[1].master.AddMinorBlock(b1.Header().Branch.Value, b1)
		assert.NoError(t, err)
		return success
	}, 2)
	//Make sure another cluster received the new minor block
	assertTrueWithTimeout(func() bool {
		b := cluster[1].GetShardState(2).GetBlock(b1.Hash())
		return b != nil
	}, 2)

	assertTrueWithTimeout(func() bool {
		b, err := cluster[0].master.GetMinorBlockByHash(b1.Hash(), b1.Header().Branch)
		assert.NoError(t, err)
		return b != nil
	}, 2)
	//Cluster 1 generates a new root block with higher total difficulty
	rb2 := rb0.Header().CreateBlockToAppend(nil, big.NewInt(3000000), nil, nil,
		nil).Finalize(coinbaseAmount, nil)
	err = cluster[1].master.AddRootBlock(rb2)
	assert.NoError(t, err)
	assert.Equal(t, cluster[1].master.CurrentBlock().Hash(), rb2.Header().Hash())
	//Generate a minor block b2
	b2 := tipGen(nil, cluster[1].GetShard(2))
	assertTrueWithTimeout(func() bool {
		success, err := cluster[1].master.AddMinorBlock(b2.Header().Branch.Value, b2)
		assert.NoError(t, err)
		return success
	}, 2)
	//Make sure another cluster received the new minor block
	assertTrueWithTimeout(func() bool {
		b := cluster[1].GetShardState(2).GetBlock(b2.Hash())
		return b != nil
	}, 2)

	assertTrueWithTimeout(func() bool {
		b, err := cluster[0].master.GetMinorBlockByHash(b1.Hash(), b2.Header().Branch)
		assert.NoError(t, err)
		return b != nil
	}, 2)
}

func TestNewBlockHeaderPool(t *testing.T) {
	_, cluster := CreateClusterList(1, 2, 2, 2, nil)
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()
	b1 := tipGen(nil, cluster[0].GetShard(2))
	assertTrueWithTimeout(func() bool {
		success, err := cluster[0].master.AddMinorBlock(b1.Header().Branch.Value, b1)
		assert.NoError(t, err)
		return success
	}, 2)
	// Update config to force checking diff
	cluster[0].clstrCfg.Quarkchain.SkipMinorDifficultyCheck = false
	b2 := b1.CreateBlockToAppend(nil, big.NewInt(12345), nil, nil, nil,
		nil, nil)
	shard := cluster[0].slavelist[0].GetShard(b2.Header().Branch.Value)
	_ = shard.HandleNewTip(nil, b2.Header(), "")
	//Also the block should not exist in new block pool
	inPool := func(bHash ethCommon.Hash) bool {
		v := reflect.ValueOf(*shard)
		f := v.FieldByName("mBPool")
		p := f.FieldByName("BlockPool")
		for _, e := range p.MapKeys() {
			if i, ok := p.MapIndex(e).Interface().(ethCommon.Hash); ok {
				if bytes.Compare(bHash[:], i[:]) > 0 {
					return true
				}
			}
		}
		return false
	}
	assert.True(t, !inPool(b2.Header().Hash()))
}

//Test the broadcast is only done to the neighbors
func TestGetRootBlockHeadersWithSkip(t *testing.T) {
	_, cluster := CreateClusterList(2, 2, 2, 2, nil)
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()
	//Add a root block first so that later minor blocks referring to this root
	//can be broadcasted to other shards
	master := cluster[0].master
	rootBlockHeaderList := []types.IHeader{master.GetCurrRootHeader()}
	for i := 0; i < 10; i++ {
		rootBlock, _, err := master.CreateBlockToMine()
		assert.NoError(t, err)
		err = master.AddRootBlock(rootBlock.(*types.RootBlock))
		assert.NoError(t, err)
		rootBlockHeaderList = append(rootBlockHeaderList, rootBlock.IHeader())
	}
	assert.Equal(t, rootBlockHeaderList[len(rootBlockHeaderList)-1].NumberU64(), uint64(10))
	assertTrueWithTimeout(func() bool {
		return cluster[1].master.GetTip() == 10
	}, 2)

	peer := cluster.GetPeerByIndex(1)
	//# Test Case 1 ###################################################
	blockHeaders, err := peer.GetRootBlockHeaderList(rootBlockHeaderList[2].Hash(), 3, true)
	assert.NoError(t, err)
	assert.Equal(t, len(blockHeaders), 3)
	assert.Equal(t, blockHeaders[0].Hash(), rootBlockHeaderList[2].Hash())
	assert.Equal(t, blockHeaders[1].Hash(), rootBlockHeaderList[1].Hash())
	assert.Equal(t, blockHeaders[2].Hash(), rootBlockHeaderList[0].Hash())

	// TODO reverse == false can't use.
	// TODO skip parameter need to be added.
}

func TestGetRootBlockHeaderSyncFromGenesis(t *testing.T) {

	_, clstrList := CreateClusterList(2, 1, 1, 1, nil)
	clstrList.Start(5*time.Second, true)
	defer clstrList.Stop()

	var (
		mstr0         = clstrList[0].GetMaster()
		mstr1         = clstrList[1].GetMaster()
		rootBlockList = make([]*types.RootBlock, 0, 10)
	)
	rootBlockList = append(rootBlockList, mstr0.CurrentBlock())

	for index := 0; index < 10; index++ {
		rBlock, _, err := mstr0.CreateBlockToMine()
		assert.NoError(t, err)
		err = mstr0.AddRootBlock(rBlock.(*types.RootBlock))
		assert.NoError(t, err)
		rootBlockList = append(rootBlockList, rBlock.(*types.RootBlock))
	}
	b0 := rootBlockList[len(rootBlockList)-1]
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return mstr1.CurrentBlock().Hash() == b0.Hash()
	}, 1), true)
	//TODO test synchronisze.status
}

func TestGetRootBlockHeaderSyncFromHeight3(t *testing.T) {
	_, clstrList := CreateClusterList(2, 1, 1, 1, nil)
	clstrList.Start(5*time.Second, false)
	defer clstrList.Stop()

	var (
		mstr0         = clstrList[0].GetMaster()
		mstr1         = clstrList[1].GetMaster()
		rootBlockList = make([]*types.RootBlock, 0, 10)
	)
	for index := 0; index < 10; index++ {
		rBlock, _, err := mstr0.CreateBlockToMine()
		assert.NoError(t, err)
		err = mstr0.AddRootBlock(rBlock.(*types.RootBlock))
		assert.NoError(t, err)
		rootBlockList = append(rootBlockList, rBlock.(*types.RootBlock))
	}
	for index := 0; index < 3; index++ {
		err := mstr1.AddRootBlock(rootBlockList[index])
		assert.NoError(t, err)
	}
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return mstr1.CurrentBlock().Hash() == rootBlockList[2].Hash()
	}, 3), true)
	clstrList.Start(5*time.Second, true)
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return mstr1.CurrentBlock().Hash() == rootBlockList[len(rootBlockList)-1].Hash()
	}, 3), true)
}

func TestGetRootBlockHeaderSyncWithStaleness(t *testing.T) {
	_, clstrList := CreateClusterList(2, 1, 1, 1, nil)
	clstrList.Start(5*time.Second, false)
	defer clstrList.Stop()

	var (
		mstr0         = clstrList[0].GetMaster()
		mstr1         = clstrList[1].GetMaster()
		rootBlockList = make([]*types.RootBlock, 0, 10)
		rBlock        types.IBlock
		err           error
	)

	for index := 0; index < 10; index++ {
		rBlock, _, err = mstr0.CreateBlockToMine()
		assert.NoError(t, err)
		err = mstr0.AddRootBlock(rBlock.(*types.RootBlock))
		assert.NoError(t, err)
		rootBlockList = append(rootBlockList, rBlock.(*types.RootBlock))
	}
	assert.Equal(t, mstr0.CurrentBlock().Hash(), rBlock.Hash())
	for index := 0; index < 8; index++ {
		rBlock, _, err = mstr1.CreateBlockToMine()
		assert.NoError(t, err)
		err = mstr1.AddRootBlock(rBlock.(*types.RootBlock))
		assert.NoError(t, err)
	}
	assert.Equal(t, mstr1.CurrentBlock().Hash(), rBlock.Hash())

	clstrList.Start(5*time.Second, true)
	b0 := rootBlockList[len(rootBlockList)-1]
	assert.Equal(t, assertTrueWithTimeout(func() bool {
		return mstr1.CurrentBlock().Hash() == b0.Hash()
	}, 1), true)
}

func TestGetRootBlockHeaderSyncWithMultipleLookup(t *testing.T) {}

func TestGetRootBlockHeaderSyncWithStartEqualEnd(t *testing.T) {}

func TestGetRootBlockHeaderSyncWithBestAncestor(t *testing.T) {}
