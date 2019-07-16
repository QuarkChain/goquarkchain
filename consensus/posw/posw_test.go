package posw_test

import (
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

func TestPoSWCoinbaseAddrsCntByDiffLen(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc := account.CreatAddressFromIdentity(id1, 0)
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc, nil, nil)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()
	chainConfig := blockchain.Config().Chains[0]
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
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
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc1, nil, nil)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()

	chainConfig := blockchain.Config().Chains[0]
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.CoinbaseAmount = big.NewInt(8)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(2)
	shardConfig.PoswConfig.WindowSize = 4

	//Add a root block to have all the shards initialized, also include the genesis from
	// another shard to allow x-shard tx TO that shard
	rootBlk := blockchain.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	var sId uint32 = 1
	blockchain2, err := core.CreateFakeMinorCanonicalPoSW(acc1, &sId, nil)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	rootBlk.AddMinorBlockHeader(blockchain2.CurrentBlock().Header())

	added, err := blockchain.AddRootBlock(rootBlk.Finalize(nil, nil, common.Hash{}))
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
	tx0 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(1), nil, nil)
	if _, err = blockchain.ExecuteTx(tx0, &acc1, nil); err != nil {
		t.Errorf("tx failed: %v", err)
	}
	//Create a block including that tx, receipt should also report error
	if err := tryAddTx(blockchain, tx0); err != nil {
		t.Errorf("add tx failed: %v", err)
	}
	id2, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id2)
	}
	acc2 := account.CreatAddressFromIdentity(id2, 0)
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
	balanceExp1 := new(big.Int).Sub(balanceExp, big.NewInt(1))
	if balanceExp1.Cmp(blc) != 0 {
		t.Errorf("Balance: expected %v, got %v", balanceExp1, blc)
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
	tx1 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(2), nil, nil)
	if _, err := blockchain.ExecuteTx(tx1, &acc1, nil); err == nil {
		t.Error("tx should fail")
	}
	//Create a block including that tx, receipt should also report error
	if err := tryAddTx(blockchain, tx1); err != nil { //txPool.AddLocal(tx) will be called and no state available. so posw disallow check error will not happen here.
		t.Fatalf("error adding tx %v", err)
	}
	var mb1 *types.MinorBlock
	if mb1, err = blockchain.CreateBlockToMine(nil, &acc2, nil); err != nil {
		t.Fatalf("error creating block %v", err)
	}
	if _, _, err = blockchain.FinalizeAndAddBlock(mb1); err == nil {
		t.Error("finalize and add block should fail due to SENDER NOT ALLOWED")
	}
	var tb1 *big.Int
	if tb1, err = blockchain.GetBalance(acc1.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	}
	if tb1.Cmp(balanceExp1) != 0 {
		t.Errorf("Balance: expected %v, got %v", balanceExp1, tb1)
	}

	if tb2, err := blockchain.GetBalance(acc2.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	} else if tb2.Cmp(balanceExp) != 0 { //acc2 only mined 1 block successfully
		t.Errorf("Balance: expected %v, got %v", balanceExp, tb2)
	}

	disallowMapExp2 := map[account.Recipient]*big.Int{
		coinbase:       bn2,
		acc1.Recipient: bn2,
		acc2.Recipient: bn2,
	}
	evmState2, err := blockchain.State()
	if err != nil {
		t.Fatalf("failed to get State: %v", err)
	}
	disallowMap2 := evmState2.GetSenderDisallowMap()
	if !reflect.DeepEqual(disallowMap2, disallowMapExp2) {
		t.Errorf("disallowMap: expected %x, got %x", disallowMapExp2, disallowMap2)
	}

	tx2 := core.CreateFreeTx(blockchain, id2.GetKey().Bytes(), acc2, account.Address{}, new(big.Int).SetUint64(3), nil, nil)
	if _, err := blockchain.ExecuteTx(tx2, &acc2, nil); err == nil {
		t.Error("tx should fail")
	}
	//ok to transfer 1 because 1+2(disallow)<4(balance)
	tx3 := core.CreateFreeTx(blockchain, id2.GetKey().Bytes(), acc2, account.Address{}, new(big.Int).SetUint64(1), nil, nil)
	if _, err := blockchain.ExecuteTx(tx3, &acc2, nil); err != nil {
		t.Errorf("tx should succeed but get: %v", err)
	}
}
func TestPoSWCoinbaseSendEqualLocked(t *testing.T) {

	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	// t.Logf("account1=%x", acc1.Recipient)
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc1, nil, nil)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()

	chainConfig := blockchain.Config().Chains[0]
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.CoinbaseAmount = big.NewInt(10)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(2)
	shardConfig.PoswConfig.WindowSize = 4

	//Add a root block to have all the shards initialized, also include the genesis from
	// another shard to allow x-shard tx TO that shard
	rootBlk := blockchain.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	var sId uint32 = 1
	blockchain2, err := core.CreateFakeMinorCanonicalPoSW(acc1, &sId, nil)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	rootBlk.AddMinorBlockHeader(blockchain2.CurrentBlock().Header())

	added, err := blockchain.AddRootBlock(rootBlk.Finalize(nil, nil, common.Hash{}))
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
	tx0 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(1), nil, nil)

	if err = tryAddTx(blockchain, tx0); err != nil {
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

func TestPoSWCoinbaseSendAboveLocked(t *testing.T) {

	id1, err := account.CreatRandomIdentity()
	if err != nil {
		t.Fatalf("error create id %v", id1)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	// t.Logf("account1=%x", acc1.Recipient)
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc1, nil, nil)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()
	chainConfig := blockchain.Config().Chains[0]
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.CoinbaseAmount = big.NewInt(10)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(2)
	shardConfig.PoswConfig.WindowSize = 4

	//Add a root block to have all the shards initialized, also include the genesis from
	// another shard to allow x-shard tx TO that shard
	rootBlk := blockchain.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	var sId uint32 = 1
	blockchain2, err := core.CreateFakeMinorCanonicalPoSW(acc1, &sId, nil)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	rootBlk.AddMinorBlockHeader(blockchain2.CurrentBlock().Header())

	added, err := blockchain.AddRootBlock(rootBlk.Finalize(nil, nil, common.Hash{}))
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
	tx0 := core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(2), nil, nil)

	if err = tryAddTx(blockchain, tx0); err != nil {
		t.Fatalf("add tx failed: %v", err)
	}
	acc2 := account.CreatAddressFromIdentity(id1, 1)
	var tx1 *types.Transaction
	var nonce uint64 = 1
	var gas1 uint64 = 30000
	tx1 = core.CreateFreeTx(blockchain, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(2), &gas1, &nonce)
	if err = tryAddTx(blockchain, tx1); err != nil {
		t.Fatalf("add tx failed: %v", err)
	}

	if minorBlock, err := blockchain.CreateBlockToMine(nil, &acc1, nil); err != nil {
		t.Fatalf("failed to CreateBlockToMine: %v", err)
	} else {
		//fmt.Printf("gaslimit=%v\n", minorBlock.GasLimit())
		if size := len(minorBlock.Transactions()); size != 2 {
			t.Errorf("tx len in block: expected %d, got %d", 2, size)
		}
		if _, _, err = blockchain.FinalizeAndAddBlock(minorBlock); err == nil {
			t.Fatalf("FinalizeAndAddBlock should fail due to posw")
		}
	}
	if _, _, r0 := blockchain.GetTransactionReceipt(tx0.Hash()); r0 != nil {
		t.Errorf("tx0 should fail")
	}
	if _, _, r1 := blockchain.GetTransactionReceipt(tx1.Hash()); r1 != nil {
		t.Errorf("tx1 should fail")
	}

	if evmState, err := blockchain.State(); err != nil {
		t.Fatalf("error get state: %v", err)
	} else { //only one block succeed.
		balance := evmState.GetBalance(acc1.Recipient)
		balanceExp := new(big.Int).Div(shardConfig.CoinbaseAmount, big.NewInt(2))
		if balanceExp.Cmp(balance) != 0 {
			t.Errorf("balance: expected %v, got %v", balanceExp, balance)
		}
	}
}

func tryAddTx(blockchain *core.MinorBlockChain, tx *types.Transaction) error {
	var err error
	if err = blockchain.AddTx(tx); err != nil {
		time.Sleep(time.Duration(2) * time.Second)
		return blockchain.AddTx(tx)
	}
	return nil
}

func TestPoSWValidateMinorBlockSeal(t *testing.T) {
	accb := make([]byte, 20)
	for i, _ := range accb {
		accb[i] = 1
	}
	reci := account.BytesToIdentityRecipient(accb)
	acc := account.NewAddress(reci, 0)
	var alloc uint64 = 256
	var shardId uint32 = 0
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc, &shardId, &alloc)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	chainConfig := blockchain.Config().Chains[0]
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.ConsensusType = config.PoWDoubleSha256
	shardConfig.CoinbaseAmount = big.NewInt(10)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(1)
	shardConfig.PoswConfig.WindowSize = 256
	shardConfig.PoswConfig.DiffDivider = 1000
	if balance, err := blockchain.GetBalance(reci, nil); err != nil {
		t.Fatalf("failed to get balance %v", err)
	} else {
		balanceExp := big.NewInt(int64(alloc))
		if balanceExp.Cmp(balance) != 0 {
			t.Errorf("balance: expected: %v, got %v", balanceExp, balance)
		}
	}
	reci0 := account.BytesToIdentityRecipient(make([]byte, 20))
	genesis := account.NewAddress(reci0, 0)
	if balance, err := blockchain.GetBalance(reci0, nil); err != nil {
		t.Fatalf("failed to get balance %v", err)
	} else {
		balanceExp := big.NewInt(0)
		if balanceExp.Cmp(balance) != 0 {
			t.Errorf("balance: expected: %v, got %v", balanceExp, balance)
		}
	}
	diff := big.NewInt(1000)
	//Genesis already has 1 block but zero stake, so no change to block diff

	tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
	newBlock := tip.CreateBlockToAppend(nil, diff, &genesis, nil, nil, nil, nil)
	if _, _, err = blockchain.FinalizeAndAddBlock(newBlock); err != nil {
		t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
	}

	// Total stake * block PoSW is 256, so acc should pass the check no matter
	//  how many blocks he mined before

	for i := 0; i < 4; i++ {
		for n := 0; n < 4; n++ {
			nonce := uint64(n)
			tip := blockchain.GetMinorBlock(blockchain.CurrentHeader().Hash())
			newBlock := tip.CreateBlockToAppend(nil, diff, &acc, &nonce, nil, nil, nil)
			if err := blockchain.Validator().ValidatorSeal(newBlock.IHeader()); err != nil {
				t.Errorf("validate block error %v", err)
			}
		}
		if _, _, err = blockchain.FinalizeAndAddBlock(newBlock); err != nil {
			t.Errorf("failed to FinalizeAndAddBlock: %v", err)
		}
	}
}
