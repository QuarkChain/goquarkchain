package posw_test

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus/posw"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

func TestPoSWFetchPreviousCoinbaseAddress(t *testing.T) {
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
	const POSW_WINDOW_LEN = 2
	tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
	newBlock := tip.CreateBlockToAppend(nil, nil, &acc, nil, nil, nil, nil)
	poswa := posw.NewPoSW(blockchain, blockchain.GetPoSWConfig())
	coinbaseBlkCnt, err := poswa.GetPoSWCoinbaseBlockCnt(newBlock.ParentHash())
	if err != nil {
		t.Fatalf("failed to get PoSW coinbase block count: %v", err)
	}
	//t.Logf("GetPoSWCoinbaseBlockCnt %v", coinbaseBlkCnt)
	if len(coinbaseBlkCnt) != 1 {
		t.Errorf("PoSW coinbase block count: expected %d, actual %d", 1, len(coinbaseBlkCnt))
	}
	newBlock, _, err = blockchain.FinalizeAndAddBlock(newBlock)
	if err != nil {
		t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
	}
	//t.Logf("new block coinbase: %x, height: %d", newBlock.Coinbase(), newBlock.Number())
	var prevAddr account.Recipient
	for i := 0; i < 4; i++ {
		randomAcc, err := account.CreatRandomAccountWithFullShardKey(0)
		if err != nil {
			t.Fatalf("failed to create random account: %v", err)
		}
		tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
		newBlock = tip.CreateBlockToAppend(nil, nil, &randomAcc, nil, nil, nil, nil)
		//t.Logf("new block coinbase: %x, height: %d", newBlock.Coinbase(), newBlock.Number())
		coinbaseBlkCnt, err = poswa.GetPoSWCoinbaseBlockCnt(newBlock.ParentHash())
		if err != nil {
			t.Fatalf("failed to get PoSW coinbase block count: %v", err)
		}
		//t.Logf("GetPoSWCoinbaseBlockCnt %x", coinbaseBlkCnt)
		if len(coinbaseBlkCnt) != 2 {
			t.Errorf("PoSW coinbase block count: expected %d, got %d", 2, len(coinbaseBlkCnt))
		}
		theValue := -1
		for _, v := range coinbaseBlkCnt {
			if theValue > 0 && int(v) != theValue {
				t.Errorf("PoSW coinbase value count should all equal 1: expected %t, got %t", true, false)
			}
			theValue = int(v)
		}
		if theValue != 1 {
			t.Errorf("PoSW coinbase value expected %d, got %d", 1, theValue)
		}
		if len(prevAddr) > 0 {
			if _, ok := coinbaseBlkCnt[prevAddr]; !ok {
				t.Errorf("PoSW coinbase block count should always contain previous block's coinbase: expected %t, got %t", true, ok)
			}
		}
		newBlock, _, err = blockchain.FinalizeAndAddBlock(newBlock)
		if err != nil {
			t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
		}
		prevAddr = randomAcc.Recipient
	}
	//Cached should have certain items
	if l := posw.GetCoinbaseAddrCache(poswa).Len(); l != 1 {
		t.Errorf("len of CoinbaseAddrCache: expected %d, got %d", 1, l)
	}

	if c, ok := posw.GetCoinbaseAddrCache(poswa).Get(2); ok {
		m := c.(map[common.Hash]posw.HeightAndAddrs)
		if l2 := len(m); l2 != 5 {
			t.Errorf("len of CoinbaseAddrCache[2]: expected %d, got %d", 5, l2)
		}
	}
}

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
	poswa := posw.NewPoSW(blockchain, blockchain.GetPoSWConfig())
	sumCnt := func(m map[account.Recipient]uint64) int {
		count := 0
		for _, v := range m {
			count += int(v)
		}
		return count
	}
	for length := 1; length < 5; length++ {
		coinbaseBlkCnt, err := poswa.GetPoSWCoinbaseBlockCnt(newBlock.Hash())
		if err != nil {
			t.Fatalf("failed to get PoSW coinbase block count: %v", err)
		}
		sum := sumCnt(coinbaseBlkCnt)
		if sum != length {
			t.Errorf("sum of PoSW coinbase block count: expected %d, got %d", sum, length)
		}
	}
	//Make sure internal cache state is correct
	if cacheLen := posw.GetCoinbaseAddrCache(poswa).Len(); cacheLen != 4 {
		t.Errorf("cache length: expected %d, got %d", cacheLen, 4)
	}
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
	tx0 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1))
	if _, err := blockchain.ExecuteTx(tx0, &acc1, nil); err != nil {
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
}
