package posw_test

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus/posw"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
	"testing"
)

//type PoSWForTest interface {
//	GetPoSWCoinbaseBlockCnt(headerHash common.Hash, length uint32) (map[account.Recipient]uint32, error)
//}

func TestPoSWFetchPreviousCoinbaseAddress(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc := account.CreatAddressFromIdentity(id1, 0)
	blockchain, err := core.CreateFakeMinorCanonical(acc)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()
	const POSW_WINDOW_LEN = 2
	tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
	newBlock := tip.CreateBlockToAppend(nil, nil, &acc, nil, nil, nil, nil)
	poswa := posw.NewPoSW(blockchain)
	coinbaseBlkCnt, err := poswa.GetPoSWCoinbaseBlockCnt(newBlock.ParentHash(), POSW_WINDOW_LEN)
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
		coinbaseBlkCnt, err = poswa.GetPoSWCoinbaseBlockCnt(newBlock.ParentHash(), POSW_WINDOW_LEN)
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
	cache := poswa.GetCoinbaseAddrCache()
	//Cached should have certain items
	for k,v := range cache {
		t.Logf("CoinbaseAddrCache k=%d v=%x \n",k, v)
	}
	if len(cache) != 1 {
		t.Errorf("len of CoinbaseAddrCache: expected %d, got %d",1, len(cache))
	}
	if len(cache[2]) != 5 {
		t.Errorf("len of CoinbaseAddrCache[2]: expected %d, got %d",5, len(cache[2]))
	}
}

func TestPoSWCoinbaseAddrsCntByDiffLen(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc := account.CreatAddressFromIdentity(id1, 0)
	blockchain, err := core.CreateFakeMinorCanonical(acc)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()
	var newBlock *types.MinorBlock
	for i:=0; i<4; i++ {
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
	poswa := posw.NewPoSW(blockchain)
	sumCnt := func(m map[account.Recipient]uint32) int{
		count := 0
		for _,v := range m {
			count += int(v)
		}
		return count
	}
	for length:=1; length< 5; length++ {
		coinbaseBlkCnt, err := poswa.GetPoSWCoinbaseBlockCnt(newBlock.Hash(), uint32(length))
		if err != nil {
			t.Fatalf("failed to get PoSW coinbase block count: %v", err)
		}
		sum := sumCnt(coinbaseBlkCnt)
		if sum != length {
			t.Errorf("sum of PoSW coinbase block count: expected %d, got %d", sum, length)
		}
	}
	//Make sure internal cache state is correct
	cacheLen := len(poswa.GetCoinbaseAddrCache())
	if cacheLen != 4 {
		t.Errorf("cache length: expected %d, got %d", cacheLen, 4)
	}
}

func TestPoSWCoinBaseSendUnderLimit(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	blockchain, err := core.CreateFakeMinorCanonical(acc1)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()

	fullShardId := blockchain.Config().GetGenesisShardIds()[0]
	t.Logf("fullshardId %d", fullShardId)
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardId)
	shardConfig.CoinbaseAmount = big.NewInt(8)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(2)
	shardConfig.PoswConfig.WindowSize = 4

	//Add a root block to have all the shards initialized, also include the genesis from
	// another shard to allow x-shard tx TO that shard
	rootBlk := blockchain.GetRootTip().CreateBlockToAppend(nil,nil,nil,nil,nil)
	var sId uint32 = 1
	blockchain2, err := core.CreateFakeMinorCanonicalShardId(acc1, &sId)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	rootBlk.AddMinorBlockHeader(blockchain2.CurrentBlock().Header())

	added, err := blockchain.AddRootBlock(rootBlk.Finalize(nil, nil))
	if err != nil || !added{
		t.Fatalf("failed to add root block: %v", err)
	}
	tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
	newBlock := tip.CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	newBlock, _, err = blockchain.FinalizeAndAddBlock(newBlock)
	if err != nil {
		t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
	}
	//poswa := posw.NewPoSW(blockchain)
	//disallowMap := poswa.BuildSenderDisallowMap()
	state, err := blockchain.State()
	if err != nil {
		t.Fatalf("failed to get State: %v", err)
	}
	disallowMap := state.GetSenderDisallowMap()
	t.Logf("disallowMap=%x", disallowMap)
	lenDislmp:= len(disallowMap)
	if lenDislmp != 2 {
		t.Errorf("len of Sender Disallow map: expect %d, got %d", 2, lenDislmp)
	}
	balance := state.GetBalance(acc1.Recipient)
	balanceEx := new(big.Int).Div(shardConfig.CoinbaseAmount, big.NewInt(2))
	if balance != balanceEx {
		t.Errorf("Balance: expected %v, got %v", balanceEx, balance)
	}
}


