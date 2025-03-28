package posw_test

import (
	"math/big"
	"reflect"
	"runtime/debug"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

var (
	testGenesisTokenID = qkcCommon.TokenIDEncode("QKC")
)

func appendNewBlock(blockchain *core.MinorBlockChain, acc1 account.Address, t *testing.T) (*types.MinorBlock, types.Receipts) {
	newBlock, err := blockchain.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to CreateBlockToMine: %v", err)
	}
	resultsCh := make(chan types.IBlock)
	if balance, err := blockchain.GetBalance(newBlock.Coinbase().Recipient, nil); err != nil {
		t.Fatalf("failed to get balance: %v", err)
	} else {
		stakePreBlock := blockchain.DecayByHeightAndTime(newBlock.NumberU64(), newBlock.Time())
		adjustedDiff, err := core.GetPoSW(blockchain).PoSWDiffAdjust(newBlock.Header(), balance.GetTokenBalance(testGenesisTokenID), stakePreBlock)
		if err != nil {
			t.Fatalf("failed to adjust posw diff: %v", err)
		}
		if err = blockchain.Engine().Seal(nil, newBlock, adjustedDiff, 1, resultsCh, nil); err != nil {
			t.Fatalf("problem sealing the block: %v", err)
		}
	}
	minedBlock := <-resultsCh
	block, rs, err := blockchain.FinalizeAndAddBlock(minedBlock.(*types.MinorBlock))
	if err != nil {
		t.Fatalf("failed to FinalizeAndAddBlock: %v, %v", err, string(debug.Stack()))
	}
	return block, rs
}

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

	for i := 0; i < 4; i++ {
		randomAcc, err := account.CreatRandomAccountWithFullShardKey(0)
		if err != nil {
			t.Fatalf("failed to create random account: %v", err)
		}
		appendNewBlock(blockchain, randomAcc, t)
	}
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
	rootBlk := blockchain.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	appendNewBlock(blockchain, acc1, t)
	evmState, err := blockchain.State()
	if err != nil {
		t.Fatalf("failed to get State: %v", err)
	}
	disallowMap := evmState.GetSenderDisallowMap()
	lenDislmp := len(disallowMap)
	if lenDislmp != 2 {
		t.Errorf("len of Sender Disallow map: expect %d, got %d", 2, lenDislmp)
	}
	balance := evmState.GetBalance(acc1.Recipient, testGenesisTokenID)
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
	tx0 := core.CreateTransferTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{},
		new(big.Int).SetUint64(1), nil, nil, nil)
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
	appendNewBlock(blockchain, acc2, t)
	blc := types.NewEmptyTokenBalances()
	if blc, err = blockchain.GetBalance(acc1.Recipient, nil); err != nil {
		t.Fatalf("get balance failed: %v", err)
	}
	b := blc.GetTokenBalance(testGenesisTokenID)
	balanceExp1 := new(big.Int).Sub(balanceExp, big.NewInt(1))
	if balanceExp1.Cmp(b) != 0 {
		t.Errorf("Balance: expected %v, got %v", balanceExp1, b)
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
	tx1 := core.CreateTransferTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{},
		new(big.Int).SetUint64(2), nil, nil, nil)
	if ret, _ := blockchain.ExecuteTx(tx1, &acc1, nil); ret != nil {
		t.Error("tx should fail")
	}
	//Create a block including that tx, receipt should also report error
	// txPool.AddLocal(tx) will be called and no state available. so posw disallow check error will not happen here.
	if err := tryAddTx(blockchain, tx1); err != nil {
		t.Fatalf("error adding tx %v", err)
	}
	var mb1 *types.MinorBlock
	if mb1, err = blockchain.CreateBlockToMine(nil, &acc2, nil, nil, nil); err != nil {
		t.Fatalf("error creating block %v", err)
	}
	if _, _, err = blockchain.FinalizeAndAddBlock(mb1); err == nil {
		t.Error("finalize and add block should fail due to SENDER NOT ALLOWED")
	}
	tb1 := types.NewEmptyTokenBalances()
	if tb1, err = blockchain.GetBalance(acc1.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	}
	if tb1.GetTokenBalance(testGenesisTokenID).Cmp(balanceExp1) != 0 {
		t.Errorf("Balance: expected %v, got %v", balanceExp1, tb1)
	}

	if tb2, err := blockchain.GetBalance(acc2.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	} else if tb2.GetTokenBalance(testGenesisTokenID).Cmp(balanceExp) != 0 { //acc2 only mined 1 block successfully
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

	tx2 := core.CreateTransferTx(blockchain, id2.GetKey().Bytes(), acc2, account.Address{},
		new(big.Int).SetUint64(3), nil, nil, nil)
	if ret, _ := blockchain.ExecuteTx(tx2, &acc2, nil); ret != nil {
		t.Error("tx should fail")
	}
	//ok to transfer 1 because 1+2(disallow)<4(balance)
	tx3 := core.CreateTransferTx(blockchain, id2.GetKey().Bytes(), acc2, account.Address{},
		new(big.Int).SetUint64(1), nil, nil, nil)
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
	rootBlk := blockchain.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil,
		nil)
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
	appendNewBlock(blockchain, acc1, t)
	if evmState, err := blockchain.State(); err != nil {
		t.Fatalf("error get state: %v", err)
	} else {
		if sdMap := evmState.GetSenderDisallowMap(); len(sdMap) != 2 {
			t.Errorf("len of sender disallow map: expected %d, got %d", 2, len(sdMap))
		}
		balance := evmState.GetBalance(acc1.Recipient, testGenesisTokenID)
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
	tx0 := core.CreateTransferTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(1),
		nil, nil, nil)

	if err = tryAddTx(blockchain, tx0); err != nil {
		t.Fatalf("add tx failed: %v", err)
	}

	_, rs := appendNewBlock(blockchain, acc1, t)
	if rs[0].Status != uint64(1) {
		t.Errorf("tx status wrong: expected 1, got %d", rs[2].Status)
	}
	if tb1, err := blockchain.GetBalance(acc1.Recipient, nil); err != nil {
		t.Fatalf("get balance error %v", err)
	} else {
		balanceExp := new(big.Int).Sub(shardConfig.CoinbaseAmount, big.NewInt(1))
		if tb1.GetTokenBalance(testGenesisTokenID).Cmp(balanceExp) != 0 {
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
	var quarkash uint64 = 1000000
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc1, nil, &quarkash)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()
	chainConfig := blockchain.Config().Chains[0]
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.CoinbaseAmount = big.NewInt(10)
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(500000)
	shardConfig.PoswConfig.WindowSize = 4

	//Add a root block to have all the shards initialized, also include the genesis from
	// another shard to allow x-shard tx TO that shard
	rootBlk := blockchain.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil,
		nil)
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
	appendNewBlock(blockchain, acc1, t)
	if evmState, err := blockchain.State(); err != nil {
		t.Fatalf("error get state: %v", err)
	} else {
		if sdMap := evmState.GetSenderDisallowMap(); len(sdMap) != 2 {
			t.Errorf("len of sender disallow map: expected %d, got %d", 2, len(sdMap))
		}
		balance := evmState.GetBalance(acc1.Recipient, testGenesisTokenID)
		balanceExp := new(big.Int).Add(big.NewInt(1000000), new(big.Int).Div(shardConfig.CoinbaseAmount,
			big.NewInt(2)))
		if balanceExp.Cmp(balance) != 0 {
			t.Errorf("balance: expected %v, got %v", balanceExp, balance)
		}
		coinbaseBytes := make([]byte, 20)
		coinbase := account.BytesToIdentityRecipient(coinbaseBytes)
		bn2 := big.NewInt(500000)
		disallowMapExp := map[account.Recipient]*big.Int{
			coinbase:       bn2,
			acc1.Recipient: bn2,
		}
		disallowMap := evmState.GetSenderDisallowMap()
		if !reflect.DeepEqual(disallowMap, disallowMapExp) {
			t.Errorf("disallowMap: expected %x, got %x", disallowMapExp, disallowMap)
		}
	}
	//Try to send money from that account, the expected locked tokens are 2 * 500000
	tx0 := core.CreateTransferTx(blockchain, id1.GetKey().Bytes(), acc1, account.Address{}, new(big.Int).SetUint64(100),
		nil, nil, nil)
	if err = tryAddTx(blockchain, tx0); err != nil { //addTx will not check posw sender disallow map
		t.Fatalf("add tx failed: %v", err)
	}

	acc2 := account.CreatAddressFromIdentity(id1, 1)
	var tx1 *types.Transaction
	var gas1 uint64 = 30000
	var gasPrice uint64 = 1
	nonce := tx0.EvmTx.Nonce() + 1
	tx1 = core.CreateTransferTx(blockchain, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(2), &gas1, &gasPrice, &nonce)
	if err = tryAddTx(blockchain, tx1); err != nil {
		t.Fatalf("add tx failed: %v", err)
	}

	block, _ := appendNewBlock(blockchain, acc1, t)
	if txl := len(block.Transactions()); txl != 2 {
		t.Errorf("tx len expected 2, got %d", txl)
	}
	if _, _, r0 := blockchain.GetTransactionReceipt(tx0.Hash()); r0.Status != 0 {
		t.Errorf("tx0 should fail")
	}
	if _, _, r1 := blockchain.GetTransactionReceipt(tx1.Hash()); r1.Status != 0 {
		t.Errorf("tx1 should fail")
	}

	if evmState, err := blockchain.State(); err != nil {
		t.Fatalf("error get state: %v", err)
	} else { //only one block succeed.
		balance := evmState.GetBalance(acc1.Recipient, testGenesisTokenID)
		balanceExp := new(big.Int).SetUint64(1000000 + 10 - gas1/2)
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
	for i := range accb {
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
	defer blockchain.Stop()

	chainConfig := blockchain.Config().Chains[0]
	blockchain.Config().SkipRootDifficultyCheck = true
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
		if balanceExp.Cmp(balance.GetTokenBalance(testGenesisTokenID)) != 0 {
			t.Errorf("balance: expected: %v, got %v", balanceExp, balance)
		}
	}
	reci0 := account.BytesToIdentityRecipient(make([]byte, 20))
	genesis := account.NewAddress(reci0, 0)
	if balance, err := blockchain.GetBalance(reci0, nil); err != nil {
		t.Fatalf("failed to get balance %v", err)
	} else {
		balanceExp := big.NewInt(0)
		if balanceExp.Cmp(balance.GetTokenBalance(testGenesisTokenID)) != 0 {
			t.Errorf("balance: expected: %v, got %v", balanceExp, balance)
		}
	}
	diff := big.NewInt(1000)
	//Genesis already has 1 block but zero stake, so no change to block diff

	appendNewBlock(blockchain, genesis, t)
	// Total stake * block PoSW is 256, so acc should pass the check no matter
	//  how many blocks he mined before

	for i := 0; i < 4; i++ {
		var newBlock *types.MinorBlock
		tip := blockchain.GetMinorBlock(blockchain.CurrentBlock().Hash())
		for n := 0; n < 4; n++ {
			nonce := uint64(n)
			newBlock = tip.CreateBlockToAppend(nil, diff, &acc, &nonce, nil, nil, nil, nil, nil)
			if err := blockchain.Validator().ValidateSeal(newBlock.IHeader(), true); err != nil {
				t.Errorf("validate block error %v", err)
			}
		}
		if _, _, err = blockchain.FinalizeAndAddBlock(newBlock); err != nil {
			t.Errorf("failed to FinalizeAndAddBlock: %v", err)
		}
	}
}

func TestPoSWWindowEdgeCases(t *testing.T) {
	accb := make([]byte, 20)
	for i := range accb {
		accb[i] = 1
	}
	reci := account.BytesToIdentityRecipient(accb)
	acc := account.NewAddress(reci, 0)
	var alloc uint64 = 500
	var shardId uint32 = 0
	blockchain, err := core.CreateFakeMinorCanonicalPoSW(acc, &shardId, &alloc)
	if err != nil {
		t.Fatalf("failed to create fake minor chain: %v", err)
	}
	defer blockchain.Stop()

	chainConfig := blockchain.Config().Chains[0]
	blockchain.Config().SkipRootDifficultyCheck = true
	fullShardID := chainConfig.ChainID<<16 | chainConfig.ShardSize | 0
	shardConfig := blockchain.Config().GetShardConfigByFullShardID(fullShardID)
	shardConfig.CoinbaseAmount = big.NewInt(0)
	shardConfig.ConsensusType = config.PoWDoubleSha256
	shardConfig.PoswConfig.TotalStakePerBlock = big.NewInt(500)
	shardConfig.PoswConfig.WindowSize = 2
	shardConfig.PoswConfig.DiffDivider = 1000

	// Use 0 to denote blocks mined by others, 1 for blocks mined by acc,
	// stake * state per block = 1 for acc, 0 <- [curr], so current block
	// should enjoy the diff adjustment

	diff := big.NewInt(1000)
	tip := blockchain.GetMinorBlock(blockchain.CurrentBlock().Hash())
	newBlock := tip.CreateBlockToAppend(nil, diff, &acc, nil, nil, nil, nil, nil, nil)
	if _, _, err = blockchain.FinalizeAndAddBlock(newBlock); err != nil {
		t.Fatalf("failed to FinalizeAndAddBlock: %v", err)
	}
	//Make sure stakes didn't change
	if balance, err := blockchain.GetBalance(reci, nil); err != nil {
		t.Fatalf("failed to get balance %v", err)
	} else {
		balanceExp := big.NewInt(int64(alloc))
		if balanceExp.Cmp(balance.GetTokenBalance(testGenesisTokenID)) != 0 {
			t.Errorf("balance: expected: %v, got %v", balanceExp, balance)
		}
	}
	//0 <- 1 <- [curr], the window already has one block with PoSW benefit,
	// mining new blocks should fail
	tip1 := blockchain.GetMinorBlock(blockchain.CurrentBlock().Hash())
	newBlock1 := tip1.CreateBlockToAppend(nil, diff, &acc, nil, nil, nil, nil, nil, nil)
	if _, _, err = blockchain.FinalizeAndAddBlock(newBlock1); err == nil {
		t.Error("Should fail due to PoSW")
	}
}
