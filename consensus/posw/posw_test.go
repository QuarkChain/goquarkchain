package posw_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
)

func TestPoSWCoinbaseAddrsCntByDiffLen(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc := account.CreatAddressFromIdentity(id1, 0)
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()
	fullShardID := blockchain.Config().Chains[0].ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.PoswConfig.WindowSize = 3

	var newBlock *types.MinorBlock
	for i := 0; i < 4; i++ {
		randomAcc, err := account.CreatRandomAccountWithFullShardKey(0)
		if err != nil {
			t.Fatalf("failed to create random account: %v", err)
		}
		tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
		newBlock = tip.CreateBlockToAppend(nil, nil, &randomAcc, nil, nil, nil, nil)
		newBlock, _, err = blockchain.FinalizeAndAddBlock(newBlock)
		if err != nil {
			t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
		}
	}
	//should pass if export PoSW from minorBlockChain.
	/* poswa := core.GetPoSW(blockchain).(*posw.PoSW)
	sumCnt := func(m map[account.Recipient]uint64) int {
		count := 0
		for _, v := range m {
			count += int(v)
		}
		return count
	}
	length := int(shardConfig.PoswConfig.WindowSize - 1)
	for i := 0; i < 5; i++ {
		coinbaseBlkCnt, err := poswa.GetPoSWCoinbaseBlockCnt(newBlock.Hash())
		if err != nil {
			t.Fatalf("failed to get PoSW coinbase block count: %v", err)
		}
		sum := sumCnt(coinbaseBlkCnt)
		if sum != length {
			t.Errorf("sum of PoSW coinbase block count: expected %d, got %d", length, sum)
		}
	}

	//Make sure internal cache state is correct
	if cacheLen := posw.GetCoinbaseAddrCache(poswa).Len(); cacheLen != 4 {
		t.Errorf("cache length: expected %d, got %d", length, cacheLen)
	} */
}

func TestPoSWCoinBaseSendUnderLimit(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc1)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()

	fullShardID := blockchain.Config().Chains[0].ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.CoinbaseAmount = big.NewInt(8)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(2)
	shardConfig.PoswConfig.WindowSize = 4

	//Add a root block to have all the shards initialized, also include the genesis from
	// another shard to allow x-shard tx TO that shard
	rootBlk := blockchain.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	var sId uint32 = 1
	blockchain2, err := core.CreateFakeMinorCanonicalPoSWShardId(acc1, &sId)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	rootBlk.AddMinorBlockHeader(blockchain2.CurrentBlock().Header())

	added, err := blockchain.AddRootBlock(rootBlk.Finalize(nil, nil))
	if err != nil || !added {
		t.Fatalf("failed to add root block: %v", err)
	}
	tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
	newBlock := tip.CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	newBlock, _, err = blockchain.FinalizeAndAddBlock(newBlock)
	if err != nil {
		t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
	}
	evmState, err := blockchain.State()
	if err != nil {
		t.Fatalf("failed to get State: %v", err)
	}
	disallowMap := evmState.GetSenderDisallowMap()
	lenDislmp := len(disallowMap)
	if lenDislmp != 2 {
		t.Errorf("len of Sender Disallow map: expect %d, got %d", 2, lenDislmp)
	}
	balance := evmState.GetBalance(acc1.Recipient)
	balanceExp := new(big.Int).Div(shardConfig.CoinbaseAmount, big.NewInt(2)) // tax rate is 0.5
	if balance.Cmp(balanceExp) != 0 {
		t.Errorf("Balance: expected %v, got %v", balanceExp, balance)
	}
	coinbaseBytes := make([]byte, 20)
	coinbase := account.BytesToIdentityRecipient(coinbaseBytes)
	bn2 := big.NewInt(2)
	disallowMapExp := map[account.Recipient]*big.Int{
		coinbase:       bn2,
		acc1.Recipient: bn2,
	}
	if !reflect.DeepEqual(disallowMap, disallowMapExp) {
		t.Errorf("disallowMap: expected %x, got %x", disallowMapExp, disallowMap)
	}
	// Try to send money from that account
	id2, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id2)
	}
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	tx0 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(1))
	if _, err = blockchain.ExecuteTx(tx0, &acc1, nil); err != nil {
		t.Errorf("tx failed: %v", err)
	}
	//Create a block including that tx, receipt should also report error
	if err := blockchain.AddTx(tx0); err != nil {
		t.Errorf("add tx failed: %v", err)
	}
	var mb *types.MinorBlock
	if mb, err = blockchain.CreateBlockToMine(nil, &acc2, nil); err != nil {
		t.Fatalf("create block failed: %v", err)
	}
	if mb, _, err = blockchain.FinalizeAndAddBlock(mb); err != nil {
		t.Fatalf("finalize and add block failed: %v", err)
	}
	var blc *big.Int
	if blc, err = blockchain.GetBalance(acc1.Recipient, nil); err != nil {
		t.Fatalf("get balance failed: %v", err)
	}
	balanceExp = new(big.Int).Sub(balanceExp, big.NewInt(1))
	if balanceExp.Cmp(blc) != 0 {
		t.Errorf("Balance: expected %v, got %v", balanceExp, blc)
	}

	disallowMapExp1 := map[account.Recipient]*big.Int{
		coinbase:       bn2,
		acc1.Recipient: bn2,
		acc2.Recipient: bn2,
	}
	evmState1, err := blockchain.State()
	if err != nil {
		t.Fatalf("failed to get State: %v", err)
	}
	disallowMap1 := evmState1.GetSenderDisallowMap()
	if !reflect.DeepEqual(disallowMap1, disallowMapExp1) {
		t.Errorf("disallowMap: expected %x, got %x", disallowMapExp1, disallowMap1)
	}
	tx1 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(2))
	if _, err := blockchain.ExecuteTx(tx1, &acc1, nil); err == nil {
		t.Error("tx should fail")
	}
	//Create a block including that tx, receipt should also report error
	if err := blockchain.AddTx(tx1); err != nil {
		t.Fatalf("error adding tx %v", tx1)
	}
	var mb1 *types.MinorBlock
	if mb1, err = blockchain.CreateBlockToMine(nil, &acc2, nil); err != nil {
		t.Fatalf("error creating block %v", err)
	}
	if _, _, err = blockchain.FinalizeAndAddBlock(mb1); err != nil {
		t.Fatalf("error finalize and add block %v", err)
	}
	var tb1 *big.Int
	if tb1, err = blockchain.GetBalance(acc1.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	}
	if tb1.Cmp(balanceExp) != 0 {
		t.Errorf("Balance: expected %v, got %v", balanceExp, tb1)
	}

	if tb2, err := blockchain.GetBalance(acc2.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	} else if tb2.Cmp(shardConfig.CoinbaseAmount) != 0 {
		t.Errorf("Balance: expected %v, got %v", shardConfig.CoinbaseAmount, tb2)
	}

	/* 	disallowMapExp2 := map[account.Recipient]*big.Int{
	   		acc1.Recipient: bn2,
	   		acc2.Recipient: big.NewInt(4),
	   	}
	   	evmState2, err := blockchain.State()
	   	if err != nil {
	   		t.Fatalf("failed to get State: %v", err)
	   	}
	   	disallowMap2 := evmState2.GetSenderDisallowMap()
	   	if !reflect.DeepEqual(disallowMap2, disallowMapExp2) {
	   		//got map[48fac366c8b0dbac09414ba26892feb7e47228f4:4 e519ef8de42916f1354c85f8e8bfa7e8deda9b02:2 0000000000000000000000000000000000000000:2]
	   		t.Errorf("disallowMap: expected %x, got %x", disallowMapExp2, disallowMap2)
	   	} */
	tx2 := core.CreateFreeTx(blockchain, id2.GetKey().Bytes(), acc2, account.Address{}, new(big.Int).SetUint64(5))
	if _, err := blockchain.ExecuteTx(tx2, &acc2, nil); err == nil {
		t.Error("tx should fail")
	}
	tx3 := core.CreateFreeTx(blockchain, id2.GetKey().Bytes(), acc2, account.Address{}, new(big.Int).SetUint64(4))
	if _, err := blockchain.ExecuteTx(tx3, &acc2, nil); err != nil {
		t.Error("tx should succeed")
	}
}
func TestPoSWCoinbaseSendEqualLocked(t *testing.T) {

	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	// t.Logf("account1=%x", acc1.Recipient)
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc1)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()

	fullShardID := blockchain.Config().Chains[0].ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.CoinbaseAmount = big.NewInt(10)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(2)
	shardConfig.PoswConfig.WindowSize = 4

	//Add a root block to have all the shards initialized, also include the genesis from
	// another shard to allow x-shard tx TO that shard
	rootBlk := blockchain.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	var sId uint32 = 1
	blockchain2, err := core.CreateFakeMinorCanonicalPoSWShardId(acc1, &sId)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	rootBlk.AddMinorBlockHeader(blockchain2.CurrentBlock().Header())

	added, err := blockchain.AddRootBlock(rootBlk.Finalize(nil, nil))
	if err != nil || !added {
		t.Fatalf("failed to add root block: %v", err)
	}
	if minorBlock, err := blockchain.CreateBlockToMine(nil, &acc1, nil); err != nil {
		t.Fatalf("failed to CreateBlockToMine: %v", err)
	} else {
		if _, _, err = blockchain.FinalizeAndAddBlock(minorBlock); err != nil {
			t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
		}
	}
	if evmState, err := blockchain.State(); err != nil {
		t.Fatalf("error get state: %v", err)
	} else {
		if sdMap := evmState.GetSenderDisallowMap(); len(sdMap) != 2 {
			t.Errorf("len of sender disallow map: expected %d, got %d", 2, len(sdMap))
		}
		balance := evmState.GetBalance(acc1.Recipient)
		balanceExp := new(big.Int).Div(shardConfig.CoinbaseAmount, big.NewInt(2))
		if balanceExp.Cmp(balance) != 0 {
			t.Errorf("balance: expected %v, got %v", balanceExp, balance)
		}
		coinbaseBytes := make([]byte, 20)
		coinbase := account.BytesToIdentityRecipient(coinbaseBytes)
		bn2 := big.NewInt(2)
		disallowMapExp := map[account.Recipient]*big.Int{
			coinbase:       bn2,
			acc1.Recipient: bn2,
		}
		disallowMap := evmState.GetSenderDisallowMap()
		if !reflect.DeepEqual(disallowMap, disallowMapExp) {
			t.Errorf("disallowMap: expected %x, got %x", disallowMapExp, disallowMap)
		}
	}
	//Try to send money from that account, the expected locked tokens are 4
	tx0 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(1))

	if err = blockchain.AddTx(tx0); err != nil {
		t.Fatalf("add tx failed: %v", err)
	}
	if minorBlock, err := blockchain.CreateBlockToMine(nil, &acc1, nil); err != nil {
		t.Fatalf("CreateBlockToMine failed: %v", err)
	} else {
		if _, rs, err := blockchain.FinalizeAndAddBlock(minorBlock); err != nil {
			t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
		} else {
			if rs[0].Status != uint64(1) {
				t.Errorf("tx status wrong: expected 1, got %d", rs[2].Status)
			}
		}
	}
	if tb1, err := blockchain.GetBalance(acc1.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	} else {
		balanceExp := new(big.Int).Sub(shardConfig.CoinbaseAmount, big.NewInt(1))
		if tb1.Cmp(balanceExp) != 0 {
			t.Errorf("Balance: expected %v, got %v", balanceExp, tb1)
		}
	}
}
