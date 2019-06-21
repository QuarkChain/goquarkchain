package core

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	ethParams "github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/assert"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
	"time"
)

func TestShardStateSimple(t *testing.T) {
	env := setUp(nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	if shardState.rootTip.Number != 0 {
		t.Errorf("rootTip number mismatch have:%v want:%v", shardState.rootTip.Number, 0)
	}
	if shardState.CurrentBlock().IHeader().NumberU64() != 0 {
		t.Errorf("minorHeader number mismatch have:%v want:%v", shardState.CurrentBlock().IHeader().NumberU64(), 0)
	}
	rootBlock := shardState.GetRootBlockByHash(shardState.rootTip.Hash())
	assert.NotNil(t, rootBlock)
	// make sure genesis minor block has the right coinbase after-tax
	assert.NotNil(t, shardState.CurrentBlock().Header().CoinbaseAmount.Value, testShardCoinbaseAmount)
}

func TestInitGenesisState(t *testing.T) {
	env := setUp(nil, nil, nil)

	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	genesisHeader := shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader)
	fakeNonce := uint64(1234)
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil)
	rootBlock = modifyNumber(rootBlock, 0)
	rootBlock.Finalize(nil, nil)

	newGenesisBlock, err := shardState.InitGenesisState(rootBlock)
	checkErr(err)
	assert.NotEqual(t, newGenesisBlock.Hash(), genesisHeader.Hash())
	// header tip is still the old genesis header
	assert.Equal(t, shardState.CurrentBlock().IHeader().Hash(), genesisHeader.Hash())

	block := newGenesisBlock.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)

	_, _, err = shardState.FinalizeAndAddBlock(block)
	checkErr(err)

	// extending new_genesis_block doesn't change header_tip due to root chain first consensus
	assert.Equal(t, shardState.CurrentBlock().IHeader().Hash(), genesisHeader.Hash())

	zeroTempHeader := shardState.GetBlockByNumber(0).(*types.MinorBlock).Header()
	assert.Equal(t, zeroTempHeader.Hash(), genesisHeader.Hash())

	// extending the root block will change the header_tip
	newRootBlock := rootBlock.Header().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil)
	newRootBlock.Finalize(nil, nil)
	_, err = shardState.AddRootBlock(newRootBlock)
	checkErr(err)

	// ideally header_tip should be block.header but we don't track tips on fork chains for the moment
	// and thus it reverted all the way back to genesis
	currentMinorHeader := shardState.CurrentBlock().IHeader()
	currentZero := shardState.GetBlockByNumber(0).IHeader().(*types.MinorBlockHeader)
	assert.Equal(t, currentMinorHeader.Hash(), newGenesisBlock.Header().Hash())
	assert.Equal(t, currentZero.Hash(), newGenesisBlock.Header().Hash())

}

func TestGasPrice(t *testing.T) {
	idList := make([]account.Identity, 0)
	for index := 0; index < 5; index++ {
		temp, err := account.CreatRandomIdentity()
		checkErr(err)
		idList = append(idList, temp)
	}

	accList := make([]account.Address, 0)
	for _, v := range idList {
		accList = append(accList, account.CreatAddressFromIdentity(v, 0))
	}

	fakeData := uint64(10000000)
	env := setUp(&accList[0], &fakeData, nil)
	for _, v := range env.clusterConfig.Quarkchain.GetGenesisShardIds() {
		for _, vv := range accList {
			addr := vv.AddressInShard(v)
			shardConfig := env.clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
			shardConfig.Genesis.Alloc[addr] = new(big.Int).SetUint64(100000000000000)
		}
	}
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	fakeChan := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChan
	defer shardState.Stop()

	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.Finalize(nil, nil)

	_, err := shardState.AddRootBlock(rootBlock)
	checkErr(err)

	// 5 tx per block, make 3 blocks
	for blockIndex := 0; blockIndex < 3; blockIndex++ {
		for txIndex := 0; txIndex < 5; txIndex++ {
			randomIndex := rand.Int() % 1
			fakeValue := uint64(0)
			fakeGasPrice := uint64(42)
			if txIndex != 0 {
				fakeGasPrice = 0
			}
			tempTx := createTransferTransaction(shardState, idList[txIndex].GetKey().Bytes(), accList[txIndex], accList[randomIndex], new(big.Int).SetUint64(fakeValue), nil, &fakeGasPrice, nil, nil)
			err = shardState.AddTx(tempTx)
			checkErr(err)
		}
		b, err := shardState.CreateBlockToMine(nil, &accList[1], nil)
		checkErr(err)
		_, _, err = shardState.FinalizeAndAddBlock(b)
		checkErr(err)
		forRe := true
		for forRe == true {
			select {
			case result := <-fakeChan:
				if result == uint64(blockIndex+1) {
					forRe = false
				}
			case <-time.After(2 * time.Second):
				panic(errors.New("should end here"))

			}
		}

	}

	currentNumber := int(shardState.CurrentBlock().NumberU64())
	assert.Equal(t, currentNumber, 3)
	// for testing purposes, update percentile to take max gas price
	shardState.gasPriceSuggestionOracle.Percentile = 100
	gasPrice, err := shardState.GasPrice()
	assert.NoError(t, err)
	assert.Equal(t, gasPrice, uint64(42))

	// results should be cached (same header). updating oracle shouldn't take effect
	shardState.gasPriceSuggestionOracle.Percentile = 50
	gasPrice2, err := shardState.GasPrice()
	assert.NoError(t, err)
	assert.Equal(t, gasPrice2, uint64(42))

}

func TestEstimateGas(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 2)
	acc2, err := account.CreatRandomAccountWithFullShardKey(2)
	checkErr(err)

	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)
	txGen := func(data []byte) *types.Transaction {
		return createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(123456), nil, nil, nil, data)
	}
	tx := txGen([]byte{})
	estimate, err := shardState.EstimateGas(tx, acc1)
	checkErr(err)

	assert.Equal(t, estimate, uint32(21000))

	newTx := txGen([]byte("12123478123412348125936583475758"))
	estimate, err = shardState.EstimateGas(newTx, acc1)
	checkErr(err)
	assert.Equal(t, estimate, uint32(23176))
}

func TestExecuteTx(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	//acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	checkErr(err)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)

	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc1, new(big.Int).SetUint64(12345), nil, nil, nil, nil)
	currentEvmState, err := shardState.State()
	checkErr(err)

	// adding this line to make sure `execute_tx` would reset `gas_used`
	currentEvmState.SetGasUsed(currentEvmState.GetGasLimit())
	_, err = shardState.ExecuteTx(tx, &acc1, nil)
	checkErr(err)
}

func TestAddTxIncorrectFromShardID(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 1)
	acc2, err := account.CreatRandomAccountWithFullShardKey(1)
	checkErr(err)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// state is shard 0 but tx from shard 1
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	assert.Error(t, err)
	_, err = shardState.ExecuteTx(tx, &acc1, nil)
	assert.Error(t, err)
}

func TestOneTx(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)

	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc1, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	currState, err := shardState.State()
	checkErr(err)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	checkErr(err)

	block, i := shardState.GetTransactionByHash(tx.Hash())
	assert.NotNil(t, block)
	assert.Equal(t, block.GetTransactions()[0].Hash(), tx.Hash())
	assert.Equal(t, block.Header().Time, uint64(0))
	assert.Equal(t, i, uint32(0))

	// tx claims to use more gas than the limit and thus not included
	b1, err := shardState.CreateBlockToMine(nil, &acc1, new(big.Int).SetUint64(49999))
	checkErr(err)
	assert.Equal(t, b1.Header().Number, uint64(1))
	assert.Equal(t, len(b1.Transactions()), 0)

	b2, err := shardState.CreateBlockToMine(nil, &acc1, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.Header().Number, uint64(1))

	// Should succeed
	b2, re, err := shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().IHeader().NumberU64(), uint64(1))
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b2.Header().Hash())
	assert.Equal(t, shardState.CurrentBlock().GetTransactions()[0].Hash(), tx.Hash())

	currentState1, err := shardState.State()
	checkErr(err)

	assert.Equal(t, currentState1.GetBalance(id1.GetRecipient()).Uint64(), uint64(10000000-ethParams.TxGas-12345))
	assert.Equal(t, currentState1.GetBalance(acc2.Recipient).Uint64(), uint64(12345))
	// shard miner only receives a percentage of reward because of REWARD_TAX_RATE
	acc3Value := currentState1.GetBalance(acc3.Recipient)
	should3 := new(big.Int).Add(new(big.Int).SetUint64(ethParams.TxGas/2), new(big.Int).Div(testShardCoinbaseAmount, new(big.Int).SetUint64(2)))
	assert.Equal(t, should3, acc3Value)
	assert.Equal(t, len(re), 1)
	assert.Equal(t, re[0].Status, uint64(1))
	assert.Equal(t, re[0].GasUsed, uint64(21000))

	block, i = shardState.GetTransactionByHash(tx.Hash())
	assert.NotNil(t, block)

	curBlock := shardState.CurrentBlock()

	assert.Equal(t, curBlock.Hash(), b2.Hash())
	assert.Equal(t, curBlock.IHeader().Hash(), block.Header().Hash())

	assert.Equal(t, curBlock.GetMetaData().Hash(), block.Meta().Hash())

	for k, v := range curBlock.GetTransactions() {
		assert.Equal(t, v.Hash(), block.Transactions()[k].Hash())
	}
	assert.Equal(t, i, uint32(0))

	// Check receipts
	blockR, indexR, reR := shardState.GetTransactionReceipt(tx.Hash())
	assert.NotNil(t, blockR)
	assert.Equal(t, blockR.Hash(), b2.Hash())
	assert.Equal(t, indexR, uint32(0))
	assert.Equal(t, reR.Status, uint64(1))
	assert.Equal(t, reR.GasUsed, uint64(21000))

	s, err := shardState.State()
	// Check Account has full_shard_key
	assert.Equal(t, s.GetFullShardKey(acc2.Recipient), acc2.FullShardKey)

}

func TestDuplicatedTx(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil)

	err = shardState.AddTx(tx)
	checkErr(err)
	err = shardState.AddTx(tx)
	assert.Error(t, err) //# already in tx_queue
	pending, err := shardState.txPool.Pending()
	assert.Equal(t, len(pending), 1)

	block, i := shardState.GetTransactionByHash(tx.Hash())
	assert.NotNil(t, block)
	assert.Equal(t, len(block.Transactions()), 1)
	assert.Equal(t, block.Transactions()[0].Hash(), tx.Hash())
	assert.Equal(t, block.Header().Time, uint64(0))
	assert.Equal(t, i, uint32(0))
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	// Should succeed
	b1, reps, err := shardState.FinalizeAndAddBlock(b1)

	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b1.Header().Hash())

	currentState, err := shardState.State()
	checkErr(err)
	assert.Equal(t, currentState.GetBalance(id1.GetRecipient()).Uint64(), uint64(10000000-21000-12345))

	assert.Equal(t, currentState.GetBalance(acc2.Recipient).Uint64(), uint64(12345))

	shouldAcc3 := new(big.Int).Add(new(big.Int).Div(testShardCoinbaseAmount, new(big.Int).SetUint64(2)), new(big.Int).SetUint64(21000/2))
	assert.Equal(t, currentState.GetBalance(acc3.Recipient).Cmp(shouldAcc3), 0)

	// Check receipts
	assert.Equal(t, len(reps), 1)
	assert.Equal(t, reps[0].Status, uint64(1))
	assert.Equal(t, reps[0].GasUsed, uint64(21000))

	block, _ = shardState.GetTransactionByHash(tx.Hash())
	assert.NotNil(t, block)

	// tx already confirmed
	err = shardState.AddTx(tx)
	assert.Error(t, err)

}

func TestAddInvalidTxFail(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	fakeValue := new(big.Int).Mul(jiaozi, new(big.Int).SetUint64(100))
	fakeMoey := new(big.Int).Sub(fakeValue, new(big.Int).SetUint64(1))
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, fakeMoey, nil, nil, nil, nil)

	err = shardState.AddTx(tx)
	assert.Error(t, err)

	pending, err := shardState.txPool.Pending()
	checkErr(err)
	assert.Equal(t, len(pending), 0)
}

func TestAddNonNeighborTxFail(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(3) // not acc1's neighbor
	acc3, err := account.CreatRandomAccountWithFullShardKey(8) // acc1's neighbor
	fakeMoney := uint64(10000000)
	fakeShardSize := uint32(64)
	env := setUp(&acc1, &fakeMoney, &fakeShardSize)

	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(1000000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil)

	err = shardState.AddTx(tx)
	assert.Error(t, err)
	assert.Equal(t, len(shardState.txPool.pending), 0)

	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc3, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil)

	err = shardState.AddTx(tx)
	checkErr(err)
	pending, err := shardState.txPool.Pending()
	checkErr(err)
	assert.Equal(t, len(pending), 1)
}

func TestExceedingXShardLimit(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(1)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	defaultNeighbors := env.clusterConfig.Quarkchain.MaxNeighbors
	// a huge number to make xshard tx limit become 0 so that no xshard tx can be
	// included in the block
	env.clusterConfig.Quarkchain.MaxNeighbors = 4294967295
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(500000)
	// xshard tx
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 0)
	fakeGasPrice := uint64(2)
	// inshard tx
	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc3, new(big.Int).SetUint64(12345), &fakeGas, &fakeGasPrice, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)
	b1, err = shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 1)

	env.clusterConfig.Quarkchain.MaxNeighbors = defaultNeighbors
}

func TestTwoTxInOneBlock(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 2)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)

	fakeQuarkHash := uint64(2000000 + 21000)
	env := setUp(&acc1, &fakeQuarkHash, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	fakeChan := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChan
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil))
	checkErr(err)
	currState, err := shardState.State()
	checkErr(err)
	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	_, res, err := shardState.FinalizeAndAddBlock(b0)
	checkErr(err)
	assert.Equal(t, len(res), 1)

	currState, err = shardState.State()
	checkErr(err)
	acc1Value := currState.GetBalance(id1.GetRecipient())
	acc2Value := currState.GetBalance(acc2.Recipient)
	acc3Value := currState.GetBalance(acc3.Recipient)
	assert.Equal(t, acc1Value.Uint64(), uint64(1000000))
	assert.Equal(t, acc2Value.Uint64(), uint64(1000000))

	should333 := new(big.Int).Add(testShardCoinbaseAmount, new(big.Int).SetUint64(21000))
	should3 := new(big.Int).Div(should333, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3Value.Cmp(should3), 0)

	//Check Account has full_shard_key
	currOB := currState.GetOrNewStateObject(acc2.Recipient)
	assert.Equal(t, currOB.FullShardKey(), acc2.FullShardKey)
	forRe := true
	for forRe == true {
		select {
		case result := <-fakeChan:
			if result == shardState.CurrentBlock().NumberU64() {
				forRe = false
			}
		case <-time.After(2 * time.Second):
			panic(errors.New("should end here"))

		}
	}
	// set a different full shard id
	toAddress := account.Address{Recipient: acc2.Recipient, FullShardKey: acc2.FullShardKey + 2}
	fakeGas := uint64(50000)
	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, toAddress, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil))
	checkErr(err)
	fakeGas = uint64(40000)
	err = shardState.AddTx(createTransferTransaction(shardState, id2.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(54321), &fakeGas, nil, nil, nil))
	checkErr(err)

	// # Should succeed
	b1, err := shardState.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(40000))
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 1)
	b1, err = shardState.CreateBlockToMine(nil, &acc3, nil)
	assert.Equal(t, len(b1.Transactions()), 2)

	_, reps, err := shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentHeader().Hash(), b1.Header().Hash())
	currState, err = shardState.State()
	checkErr(err)
	acc1Value = currState.GetBalance(id1.GetRecipient())
	assert.Equal(t, acc1Value.Uint64(), uint64(1000000-21000-12345+54321))

	acc2Value = currState.GetBalance(id2.GetRecipient())
	assert.Equal(t, acc2Value.Uint64(), uint64(1000000-21000+12345-54321))

	acc3Value = currState.GetBalance(acc3.Recipient)

	//# 2 block rewards: 3 tx, 2 block rewards
	shouldAcc3Value := new(big.Int).Mul(testShardCoinbaseAmount, new(big.Int).SetUint64(2))
	shouldAcc3Value = new(big.Int).Add(shouldAcc3Value, new(big.Int).SetUint64(63000))
	shouldAcc3Value = new(big.Int).Div(shouldAcc3Value, new(big.Int).SetUint64(2))
	assert.Equal(t, shouldAcc3Value.Cmp(acc3Value), 0)

	// Check receipts

	assert.Equal(t, len(reps), 2)
	assert.Equal(t, reps[1].CumulativeGasUsed, uint64(42000))
	assert.Equal(t, reps[1].Status, uint64(1))
	assert.Equal(t, reps[1].GasUsed, uint64(21000))
	assert.Equal(t, reps[0].Status, uint64(1))

	block, i := shardState.GetTransactionByHash(b1.GetTransactions()[0].Hash())
	assert.Equal(t, block.Hash(), b1.Hash())
	assert.Equal(t, i, uint32(0))

	block, i = shardState.GetTransactionByHash(b1.GetTransactions()[1].Hash())

	assert.Equal(t, block.Hash(), b1.Hash())
	assert.Equal(t, i, uint32(1))

	//Check acc2 full_shard_key doesn't change
	assert.Equal(t, currState.GetFullShardKey(acc2.Recipient), acc2.FullShardKey)

}

func TestForkDoesNotConfirmTx(t *testing.T) {
	// "Tx should only be confirmed and removed from tx queue by the best chain"
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(2021000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)

	fakeChain := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChain
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil))
	checkErr(err)
	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	b00 := types.NewMinorBlock(b0.Header(), b0.Meta(), nil, nil, nil)
	_, _, err = shardState.FinalizeAndAddBlock(b00)
	checkErr(err)
	// tx is added back to queue in the end of create_block_to_mine
	assert.Equal(t, len(shardState.txPool.pending), 1)
	assert.Equal(t, len(b1.Transactions()), 1)
	_, _, err = shardState.FinalizeAndAddBlock(b1)
	// b1 is a fork and does not remove the tx from queue
	assert.Equal(t, len(shardState.txPool.pending), 1)

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	_, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)

	forRe := true
	for forRe == true {
		select {
		case result := <-fakeChain:
			if result == 2 {
				pending, err := shardState.txPool.Pending()
				checkErr(err)
				assert.Equal(t, len(pending), 0)
				forRe = false
			}

		case <-time.After(1 * time.Second):
			panic(errors.New("should end here"))
		}
	}
}

func TestRevertForkPutTxBackToQueue(t *testing.T) {
	//  "Tx in the reverted chain should be put back to the queue"
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(2021000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()

	fakeChain := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChain
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil))
	checkErr(err)

	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	b0, _, err = shardState.FinalizeAndAddBlock(b0)
	checkErr(err)

	b11 := types.NewMinorBlock(b1.Header(), &types.MinorBlockMeta{}, nil, nil, nil)
	b11, _, err = shardState.FinalizeAndAddBlock(b11) //# make b1 empty
	checkErr(err)

	b2 := b11.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	_, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)

	// # now b1-b2 becomes the best chain and we expect b0 to be reverted and put the tx back to queue
	b3 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState.FinalizeAndAddBlock(b3)

	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState.FinalizeAndAddBlock(b4)

	forRe := true
	for forRe == true {
		select {
		case result := <-fakeChain:
			if result == b4.NumberU64() {
				pending, err := shardState.txPool.Pending()
				checkErr(err)
				// # b0-b3-b4 becomes the best chain
				assert.Equal(t, len(pending), 0)
				forRe = false
			}

		case <-time.After(2 * time.Second):
			panic(errors.New("should end here"))

		}
	}

}

func TestStaleBlockCount(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	checkErr(err)
	fakeQuarkash := uint64(10000000)
	env := setUp(&acc1, &fakeQuarkash, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)

	tempHeader := b2.Header()
	tempHeader.Time += uint64(1)
	b22 := types.NewMinorBlock(tempHeader, b2.Meta(), nil, nil, nil)

	_, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.getBlockCountByHeight(1), uint64(1))
	_, _, err = shardState.FinalizeAndAddBlock(b22)
	checkErr(err)
	assert.Equal(t, shardState.getBlockCountByHeight(1), uint64(2))
}

func TestXShardTxSent(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	id := uint32(0)
	shardState := createDefaultShardState(env, &id, nil, nil, nil)
	env1 := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	id = uint32(1)
	shardState1 := createDefaultShardState(env1, &id, nil, nil, nil)

	// Add a root block to update block gas limit so that xshard tx can be included
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.AddMinorBlockHeader(shardState1.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock = rootBlock.Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(21000 + 9000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &fakeGas, nil, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	assert.Equal(t, 1, len(b1.GetTransactions()))
	currentState, err := shardState.State()
	checkErr(err)
	assert.Equal(t, currentState.GetGasUsed().Uint64(), uint64(0))
	// Should succeed
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)

	currentState, err = shardState.State()
	checkErr(err)
	assert.Equal(t, len(shardState.currentEvmState.GetXShardList()), int(1))
	temp := shardState.currentEvmState.GetXShardList()[0]
	assert.Equal(t, temp.TxHash, tx.EvmTx.Hash())
	assert.Equal(t, temp.From.ToBytes(), acc1.ToBytes())
	assert.Equal(t, temp.To.ToBytes(), acc2.ToBytes())
	assert.Equal(t, temp.Value.Value.Uint64(), uint64(888888))
	assert.Equal(t, temp.GasPrice.Value.Uint64(), uint64(1))

	// Make sure the xshard gas is not used by local block
	id1Value := shardState.currentEvmState.GetGasUsed()
	assert.Equal(t, id1Value.Uint64(), uint64(21000+9000))
	id3Value := shardState.currentEvmState.GetBalance(acc3.Recipient)
	shouldID3Value := new(big.Int).Add(testShardCoinbaseAmount, new(big.Int).SetUint64(21000))
	shouldID3Value = new(big.Int).Div(shouldID3Value, new(big.Int).SetUint64(2))
	// GTXXSHARDCOST is consumed by remote shard
	assert.Equal(t, id3Value.Uint64(), shouldID3Value.Uint64())
}

func TestXShardTxInsufficientGas(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	id := uint32(0)
	shardState := createDefaultShardState(env, &id, nil, nil, nil)

	fakeGas := uint64(21000)
	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &fakeGas, nil, nil, nil))
	assert.Error(t, err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 0)
	assert.Equal(t, len(shardState.txPool.pending), 0)
}

func TestXShardTxReceiver(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 16)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(10000000)
	fakeShardSize := uint32(64)
	env := setUp(&acc1, &newGenesisMinorQuarkash, &fakeShardSize)
	env1 := setUp(&acc1, &newGenesisMinorQuarkash, &fakeShardSize)
	fakeID := uint32(0)
	shardState0 := createDefaultShardState(env, &fakeID, nil, nil, nil)
	fakeID = uint32(16)
	shardState1 := createDefaultShardState(env1, &fakeID, nil, nil, nil)
	// Add a root block to allow later minor blocks referencing this root block to
	// be broadcasted
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.AddMinorBlockHeader(shardState1.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.Finalize(nil, nil)
	_, err0 := shardState0.AddRootBlock(rootBlock)
	checkErr(err0)

	_, err1 := shardState1.AddRootBlock(rootBlock)
	checkErr(err1)

	// Add one block in shard 0
	b0, err := shardState0.CreateBlockToMine(nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b1Headaer := b1.Header()
	b1Headaer.PrevRootBlockHash = rootBlock.Header().Hash()
	b1 = types.NewMinorBlock(b1Headaer, b1.Meta(), b1.Transactions(), nil, nil)
	fakeGas := uint64(30000)
	fakeGasPrice := uint64(2)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(888888), &fakeGas, &fakeGasPrice, nil, nil)
	b1.AddTx(tx)
	txList := types.CrossShardTransactionDepositList{}
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		TxHash:   tx.EvmTx.Hash(),
		From:     acc2,
		To:       acc1,
		Value:    &serialize.Uint256{Value: new(big.Int).SetUint64(888888)},
		GasPrice: &serialize.Uint256{Value: new(big.Int).SetUint64(2)},
	})
	// Add a x-shard tx from remote peer
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList) // write db
	// Create a root block containing the block with the x-shard tx
	rootBlock = shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil)
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	// Add b0 and make sure all x-shard tx's are added
	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)
	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient)
	assert.Equal(t, acc1Value.Uint64(), uint64(10000000+888888))

	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient)
	acc3Should := new(big.Int).Add(testShardCoinbaseAmount, new(big.Int).SetUint64(9000*2))
	acc3Should = new(big.Int).Div(acc3Should, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3Value.String(), acc3Should.String())

	// X-shard gas used
	assert.Equal(t, shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64(), uint64(9000))
}

func TestXShardTxReceivedExcludeNonNeighbor(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 3)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(10000000)
	fakeShardSize := uint32(64)
	env0 := setUp(&acc1, &newGenesisMinorQuarkash, &fakeShardSize)
	env1 := setUp(&acc1, &newGenesisMinorQuarkash, &fakeShardSize)

	fakeShardID := uint32(0)
	shardState0 := createDefaultShardState(env0, &fakeShardID, nil, nil, nil)
	fakeShardID = uint32(3)
	shardState1 := createDefaultShardState(env1, &fakeShardID, nil, nil, nil)
	b0 := shardState0.CurrentBlock()
	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)

	fakeGas := uint64(30000)
	fakeGasPrice := uint64(2)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(888888), &fakeGas, &fakeGasPrice, nil, nil)
	b1.AddTx(tx)

	// Create a root block containing the block with the x-shard tx
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil)
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient)
	assert.Equal(t, uint64(10000000), acc1Value.Uint64())

	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient)
	acc3Should := new(big.Int).Div(testShardCoinbaseAmount, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3Should.Uint64(), acc3Value.Uint64())
	// No xshard tx is processed on the receiving side due to non-neighbor
	assert.Equal(t, shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64(), uint64(0))
}

func TestXShardForTwoRootBlocks(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(10000000)

	env0 := setUp(&acc1, &newGenesisMinorQuarkash, nil)

	env1 := setUp(&acc1, &newGenesisMinorQuarkash, nil)

	fakeShardID := uint32(0)
	shardState0 := createDefaultShardState(env0, &fakeShardID, nil, nil, nil)
	fakeShardID = uint32(1)
	shardState1 := createDefaultShardState(env1, &fakeShardID, nil, nil, nil)

	// Add a root block to allow later minor blocks referencing this root block to
	// be broadcasted
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.AddMinorBlockHeader(shardState1.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.Finalize(nil, nil)

	_, err = shardState0.AddRootBlock(rootBlock)

	checkErr(err)
	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)

	// Add one block in shard 0
	b0, err := shardState0.CreateBlockToMine(nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b1Header := b1.Header()
	b1Header.PrevRootBlockHash = rootBlock.Header().Hash()
	b1 = types.NewMinorBlock(b1Header, b1.Meta(), b1.Transactions(), nil, nil)
	fakeGas := uint64(30000)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(888888), &fakeGas, nil, nil, nil)
	b1.AddTx(tx)

	txList := types.CrossShardTransactionDepositList{}
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		TxHash:   tx.EvmTx.Hash(),
		From:     acc2,
		To:       acc1,
		Value:    &serialize.Uint256{Value: new(big.Int).SetUint64(888888)},
		GasPrice: &serialize.Uint256{Value: new(big.Int).SetUint64(2)},
	})
	// Add a x-shard tx from state1
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList)

	// Create a root block containing the block with the x-shard tx
	rootBlock0 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock0.AddMinorBlockHeader(b0.Header())
	rootBlock0.AddMinorBlockHeader(b1.Header())
	rootBlock0.Finalize(nil, nil)
	_, err = shardState0.AddRootBlock(rootBlock0)
	checkErr(err)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b3 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b3Header := b3.Header()
	b3Header.PrevRootBlockHash = rootBlock.Header().Hash()
	b3 = types.NewMinorBlock(b3Header, b3.Meta(), b3.Transactions(), nil, nil)

	txList = types.CrossShardTransactionDepositList{}
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		TxHash:   common.Hash{},
		From:     acc2,
		To:       acc1,
		Value:    &serialize.Uint256{Value: new(big.Int).SetUint64(385723)},
		GasPrice: &serialize.Uint256{Value: new(big.Int).SetUint64(3)},
	})
	// Add a x-shard tx from state1
	shardState0.AddCrossShardTxListByMinorBlockHash(b3.Header().Hash(), txList)

	rootBlock1 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock1.AddMinorBlockHeader(b2.Header())
	rootBlock1.AddMinorBlockHeader(b3.Header())
	rootBlock1.Finalize(nil, nil)
	_, err = shardState0.AddRootBlock(rootBlock1)
	checkErr(err)

	// Test x-shard gas limit when create_block_to_mine
	b5, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(0))
	checkErr(err)

	assert.Equal(t, b5.PrevRootBlockHash().String(), rootBlock0.Hash().String())

	// Current algorithm allows at least one root block to be included
	b6, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(9000))
	checkErr(err)
	assert.Equal(t, rootBlock0.Hash().String(), b6.PrevRootBlockHash().String())

	// There are two x-shard txs: one is root block coinbase with zero gas, and another is from shard 1
	b7, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).Mul(new(big.Int).SetUint64(9000), new(big.Int).SetUint64(2)))
	checkErr(err)

	assert.Equal(t, rootBlock1.Header().Hash().String(), b7.PrevRootBlockHash().String())
	b8, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).Mul(new(big.Int).SetUint64(9000), new(big.Int).SetUint64(3)))
	checkErr(err)
	assert.Equal(t, rootBlock1.Header().Hash().String(), b8.PrevRootBlockHash().String())

	// Add b0 and make sure all x-shard tx's are added
	b4, err := shardState0.CreateBlockToMine(nil, &acc3, nil)
	assert.Equal(t, b4.Header().PrevRootBlockHash.String(), rootBlock1.Hash().String())
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)

	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient)
	assert.Equal(t, acc1Value.Uint64(), uint64(10000000+888888+385723))

	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient)
	acc3ValueShould := new(big.Int).Add(new(big.Int).SetUint64(9000*5), testShardCoinbaseAmount)
	acc3ValueShould = new(big.Int).Div(acc3ValueShould, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3ValueShould.String(), acc3Value.String())

	// Check gas used for receiving x-shard tx
	assert.Equal(t, shardState0.currentEvmState.GetGasUsed().Uint64(), uint64(18000))
	assert.Equal(t, shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64(), uint64(18000))
}

func TestForkResolve(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)

	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeID := uint32(0)
	shardState := createDefaultShardState(env, &fakeID, nil, nil, nil)
	b0 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b1 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)

	b0, _, err = shardState.FinalizeAndAddBlock(b0)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b0.Hash())

	// Fork happens, first come first serve
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b0.Hash())

	// Longer fork happens, override existing one
	b2 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b2.Hash())
}

func TestRootChainFirstConsensus(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)

	env0 := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	env1 := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeID := uint32(0)
	shardState0 := createDefaultShardState(env0, &fakeID, nil, nil, nil)
	fakeID = uint32(1)
	shardState1 := createDefaultShardState(env1, &fakeID, nil, nil, nil)
	genesis := shardState0.CurrentBlock()
	emptyAddress := account.CreatEmptyAddress(0)
	// Add one block and prepare a fork
	b0 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &emptyAddress, nil, nil, nil, nil)

	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	evmState, reps, err := shardState1.runBlock(b1)
	b1.Finalize(reps, evmState.IntermediateRoot(), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), evmState.GetBlockFee())
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis.Header())
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil)
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b00 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b00, _, err = shardState0.FinalizeAndAddBlock(b00)
	assert.Equal(t, shardState0.CurrentBlock().IHeader().Hash().String(), b00.Hash().String())
	checkErr(err)

	// Create another fork that is much longer (however not confirmed by root_block)
	b3 := b2.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState0.FinalizeAndAddBlock(b3)
	checkErr(err)
	assert.Equal(t, shardState0.CurrentBlock().IHeader().Hash().String(), b00.Hash().String())
	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)
	assert.Equal(t, true, b4.Header().Number > b00.Header().Number)
	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b00.Hash().String())
}

func TestShardStateAddRootBlock(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)

	env0 := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	env1 := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeID := uint32(0)
	shardState0 := createDefaultShardState(env0, &fakeID, nil, nil, nil)
	fakeID = uint32(1)
	shardState1 := createDefaultShardState(env1, &fakeID, nil, nil, nil)
	genesis := shardState0.CurrentBlock()
	emptyAddress := account.CreatEmptyAddress(0)
	// Add one block and prepare a fork
	b0 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &emptyAddress, nil, nil, nil, nil)

	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	evmState, reps, err := shardState1.runBlock(b1)
	b1.Finalize(reps, evmState.IntermediateRoot(), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), evmState.GetBlockFee())

	// Add one empty root block
	emptyRoot := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState0.AddRootBlock(emptyRoot)
	checkErr(err)

	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis.Header())
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil)

	rootBlock1 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock1.AddMinorBlockHeader(genesis.Header())
	rootBlock1.AddMinorBlockHeader(b2.Header())
	rootBlock1.AddMinorBlockHeader(b1.Header())
	rootBlock1.Finalize(nil, nil)

	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b00 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b00, _, err = shardState0.FinalizeAndAddBlock(b00)
	checkErr(err)
	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b00.Hash().String())

	// Create another fork that is much longer (however not confirmed by root_block)
	b3 := b2.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState0.FinalizeAndAddBlock(b3)
	checkErr(err)
	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)

	currBlock := shardState0.CurrentBlock()
	currBlock2 := shardState0.GetBlockByNumber(2)
	currBlock3 := shardState0.GetBlockByNumber(3)
	assert.Equal(t, currBlock.Hash().String(), b00.Header().Hash().String())
	assert.Equal(t, currBlock2.Hash().String(), b00.Header().Hash().String())
	assert.Equal(t, currBlock3, nil)

	b5 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)

	_, err = shardState0.AddRootBlock(rootBlock1)

	// Add one empty root block
	emptyRoot = rootBlock1.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	_, err = shardState0.AddRootBlock(emptyRoot)
	checkErr(err)
	rootBlock2 := emptyRoot.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock2.AddMinorBlockHeader(b3.Header())
	rootBlock2.AddMinorBlockHeader(b4.Header())
	rootBlock2.AddMinorBlockHeader(b5.Header())
	rootBlock2.Finalize(nil, nil)

	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b2.Header().Hash().String())
	_, err = shardState0.AddRootBlock(rootBlock2)
	checkErr(err)

	currHeader := shardState0.CurrentHeader().(*types.MinorBlockHeader)

	assert.Equal(t, currHeader.Hash().String(), b4.Header().Hash().String())
	assert.Equal(t, shardState0.rootTip.Hash().String(), rootBlock2.Hash().String())
	assert.Equal(t, shardState0.GetBlockByNumber(2).Hash().String(), b3.Hash().String())
	assert.Equal(t, shardState0.GetBlockByNumber(3).Hash().String(), b4.Hash().String())
}

func TestShardStateAddRootBlockTooManyMinorBlocks(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	fakeShardSize := uint32(1)
	env := setUp(&acc1, &newGenesisMinorQuarkash, &fakeShardSize)

	fakeID := uint32(0)
	shardState := createDefaultShardState(env, &fakeID, nil, nil, nil)
	headers := make([]*types.MinorBlockHeader, 0)
	headers = append(headers, shardState.CurrentHeader().(*types.MinorBlockHeader))

	for index := 0; index < 13; index++ {
		b := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
		b, _, err = shardState.FinalizeAndAddBlock(b)
		headers = append(headers, b.Header())
	}

	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.ExtendMinorBlockHeaderList(headers)
	rootBlock.Finalize(nil, nil)

	// Too many blocks
	_, err = shardState.AddRootBlock(rootBlock)
	assert.Error(t, err)
	unConfirmedHeaderList := shardState.GetUnconfirmedHeaderList()
	for k, v := range unConfirmedHeaderList {
		if v.Hash() != headers[k].Hash() {
			panic(errors.New("not match"))
		}
	}

	// 10 blocks is okay
	rootBlock1 := types.NewRootBlock(rootBlock.Header(), headers[:13], nil)
	rootBlock1.Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock1)
	checkErr(err)
}

func TestShardStateForkResolveWithHigherRootChain(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, nil, nil, nil)

	b0 := shardState.CurrentBlock()
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.Finalize(nil, nil)

	assert.Equal(t, shardState.CurrentHeader().Hash().String(), b0.Header().Hash().String())
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	b1 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	fakeNonce := uint64(1)
	b2 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil, nil, nil)
	b2Header := b2.Header()
	b2Header.PrevRootBlockHash = rootBlock.Header().Hash()
	b2 = types.NewMinorBlock(b2Header, b2.Meta(), b2.Transactions(), nil, nil)
	fakeNonce = uint64(2)
	b3 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil, nil, nil)
	b3Header := b3.Header()
	b3 = types.NewMinorBlock(b3Header, b2.Meta(), b2.Transactions(), nil, nil)

	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentHeader().Hash().String(), b1.Header().Hash().String())

	// Fork happens, although they have the same height, b2 survives since it confirms root block
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, shardState.CurrentHeader().Hash().String(), b2.Header().Hash().String())

	// b3 confirms the same root block as b2, so it will not override b2
	b3, _, err = shardState.FinalizeAndAddBlock(b3)
	checkErr(err)

	assert.Equal(t, shardState.CurrentHeader().Hash().String(), b2.Header().Hash().String())
}

func TestShardStateDifficulty(t *testing.T) {
	env := setUp(nil, nil, nil)
	for _, v := range env.clusterConfig.Quarkchain.GetGenesisShardIds() {
		shardConfig := env.clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		shardConfig.Genesis.Difficulty = 10000
	}
	env.clusterConfig.Quarkchain.SkipMinorDifficultyCheck = false
	diffCalc := &consensus.EthDifficultyCalculator{
		AdjustmentCutoff:  9,
		AdjustmentFactor:  2048,
		MinimumDifficulty: new(big.Int).SetUint64(1),
	}
	env.clusterConfig.Quarkchain.NetworkID = 1 // other network ids will skip difficulty check

	fakeShardID := uint32(0)

	flagEngine := true
	shardState := createDefaultShardState(env, &fakeShardID, diffCalc, nil, &flagEngine)

	createTime := shardState.CurrentHeader().GetTime() + 8
	// Check new difficulty
	b0, err := shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), uint64(shardState.CurrentHeader().GetDifficulty().Uint64()/uint64(2048)+shardState.CurrentHeader().GetDifficulty().Uint64()))

	createTime = shardState.CurrentHeader().GetTime() + 9
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64())

	createTime = shardState.CurrentHeader().GetTime() + 17
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64())

	createTime = shardState.CurrentHeader().GetTime() + 24
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64()-shardState.CurrentHeader().GetDifficulty().Uint64()/uint64(2048))

	createTime = shardState.CurrentHeader().GetTime() + 35
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64()-shardState.CurrentHeader().GetDifficulty().Uint64()/uint64(2048)*2)
}

func TestShardStateRecoveryFromRootBlock(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, nil, nil, nil)

	blockHeaders := make([]*types.MinorBlockHeader, 0)
	blockMetas := make([]*types.MinorBlockMeta, 0)
	blockHeaders = append(blockHeaders, shardState.CurrentHeader().(*types.MinorBlockHeader))
	blockMetas = append(blockMetas, shardState.CurrentBlock().GetMetaData())
	for index := 0; index < 12; index++ {
		b := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
		b, _, err = shardState.FinalizeAndAddBlock(b)

		checkErr(err)
		blockHeaders = append(blockHeaders, b.Header())
		blockMetas = append(blockMetas, b.Meta())
	}

	// add a fork
	b1 := shardState.GetBlockByNumber(3).(*types.MinorBlock)
	b11Header := b1.Header()
	b11Header.Time = b1.Header().GetTime() + 1
	b1 = types.NewMinorBlock(b11Header, b1.Meta(), b1.Transactions(), nil, nil)
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)

	DBb1 := shardState.GetMinorBlock(b1.Hash())
	assert.Equal(t, DBb1.IHeader().GetTime(), b11Header.Time)

	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.ExtendMinorBlockHeaderList(blockHeaders[:5])
	rootBlock.Finalize(nil, nil)
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	assert.Equal(t, shardState.CurrentBlock().Hash().String(), blockHeaders[12].Hash().String())

	fakeClusterConfig := config.NewClusterConfig()
	Engine := new(consensus.FakeEngine)

	recoveredState, err := NewMinorBlockChain(env.db, nil, ethParams.TestChainConfig, fakeClusterConfig, Engine, vm.Config{}, nil, 2|0)
	// forks are pruned
	err = recoveredState.InitFromRootBlock(rootBlock)
	checkErr(err)
	tempBlock := recoveredState.GetMinorBlock(b1.Header().Hash())

	assert.Equal(t, tempBlock.Hash().String(), b1.Hash().String())
	assert.Equal(t, recoveredState.rootTip.Hash().String(), rootBlock.Hash().String())
	assert.Equal(t, recoveredState.CurrentHeader().Hash().String(), blockHeaders[4].Hash().String())
	assert.Equal(t, recoveredState.confirmedHeaderTip.Hash().String(), blockHeaders[4].Hash().String())
}

func TestShardStateTecoveryFromGenesis(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, nil, nil, nil)

	blockHeaders := make([]*types.MinorBlockHeader, 0)
	blockMetas := make([]*types.MinorBlockMeta, 0)
	blockHeaders = append(blockHeaders, shardState.CurrentHeader().(*types.MinorBlockHeader))
	blockMetas = append(blockMetas, shardState.CurrentBlock().GetMetaData())
	for index := 0; index < 12; index++ {
		b := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
		b, _, err := shardState.FinalizeAndAddBlock(b)
		checkErr(err)
		blockHeaders = append(blockHeaders, b.Header())
		blockMetas = append(blockMetas, b.Meta())
	}

	// Add a few empty root blocks
	var rootBlock *types.RootBlock
	for index := 0; index < 3; index++ {
		rootBlock = shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
		_, err := shardState.AddRootBlock(rootBlock)
		checkErr(err)

	}

	fakeClusterConfig := config.NewClusterConfig()
	Engine := new(consensus.FakeEngine)
	recoveredState, err := NewMinorBlockChain(env.db, nil, ethParams.TestChainConfig, fakeClusterConfig, Engine, vm.Config{}, nil, 2|0)
	checkErr(err)
	// expect to recover from genesis
	err = recoveredState.InitFromRootBlock(rootBlock)
	checkErr(err)

	genesis := shardState.GetBlockByNumber(0)
	assert.Equal(t, recoveredState.rootTip.Hash().String(), rootBlock.Hash().String())
	assert.Equal(t, recoveredState.CurrentBlock().Hash().String(), recoveredState.CurrentHeader().Hash().String())
	assert.Equal(t, recoveredState.CurrentHeader().Hash().String(), genesis.Hash().String())

	assert.Equal(t, true, recoveredState.confirmedHeaderTip == nil)
	assert.Equal(t, recoveredState.CurrentBlock().GetMetaData().Hash().String(), genesis.(*types.MinorBlock).GetMetaData().Hash().String())

	currState := recoveredState.currentEvmState

	CURRState, err := recoveredState.State()
	checkErr(err)

	roo1, err := currState.Commit()
	root2, err := CURRState.Commit()
	assert.Equal(t, genesis.(*types.MinorBlock).GetMetaData().Root.String(), roo1.String())
	assert.Equal(t, roo1.String(), root2.String())
}

func TestAddBlockReceiptRootNotMatch(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc3 := account.CreatEmptyAddress(0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	// Should succeed
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)

	evmState, reps, err := shardState.runBlock(b1)
	checkErr(err)
	b1.Finalize(reps, evmState.IntermediateRoot(), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), b1.Header().CoinbaseAmount.Value)

	b1Meta := b1.Meta()
	b1Meta.Root = common.Hash{}
	b1 = types.NewMinorBlock(b1.Header(), b1Meta, b1.Transactions(), nil, nil)

	rawdb.DeleteMinorBlock(shardState.db, b1.Hash())
	_, err = shardState.InsertChain([]types.IBlock{b1})
	assert.Equal(t, ErrMetaHash, err)

}

func TestNotUpdateTipOnRootFork(t *testing.T) {
	//  block's hash_prev_root_block must be on the same chain with root_tip to update tip.
	//
	//                 +--+
	//              a. |r1|
	//                /+--+
	//               /   |
	//        +--+  /  +--+    +--+
	//        |r0|<----|m1|<---|m2| c.
	//        +--+  \  +--+    +--+
	//               \   |      |
	//                \+--+     |
	//              b. |r2|<----+
	//                 +--+
	//
	//        Initial state: r0 <- m1
	//        Then adding r1, r2, m2 should not make m2 the tip because r1 is the root tip and r2 and r1
	//        are not on the same root chain.
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, nil, nil, nil)

	// m1 is the genesis block
	m1 := shardState.GetBlockByNumber(0)

	r1 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r2 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r1.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r1.Finalize(nil, nil)

	_, err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Header().Time + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil)
	assert.Equal(t, false, reflect.DeepEqual(r1.Header().Hash().String(), r2.Header().Hash().String()))

	_, err = shardState.AddRootBlock(r2)

	checkErr(err)
	assert.Equal(t, shardState.rootTip.Hash().String(), r1.Header().Hash().String())

	m2 := m1.(*types.MinorBlock).CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	m2Header := m2.Header()
	m2Header.PrevRootBlockHash = r2.Header().Hash()
	m2 = types.NewMinorBlock(m2Header, m2.Meta(), m2.Transactions(), nil, nil)

	// m2 is added
	m2, _, err = shardState.FinalizeAndAddBlock(m2)
	checkErr(err)

	// but m1 should still be the tip
	assert.Equal(t, shardState.GetMinorBlock(m2.Hash()).Hash().String(), m2.Header().Hash().String())
	assert.Equal(t, shardState.CurrentHeader().Hash().String(), m1.IHeader().Hash().String())
}

func TestAddRootBlockRevertHeaderTip(t *testing.T) {
	// block's hash_prev_root_block must be on the same chain with root_tip to update tip.
	//
	//                 +--+
	//                 |r1|<-------------+
	//                /+--+              |
	//               /   |               |
	//        +--+  /  +--+    +--+     +--+
	//        |r0|<----|m1|<---|m2| <---|m3|
	//        +--+  \  +--+    +--+     +--+
	//               \   |       \
	//                \+--+.     +--+
	//                 |r2|<-----|r3| (r3 includes m2)
	//                 +--+      +--+
	//
	//        Initial state: r0 <- m1 <- m2
	//        Adding r1, r2, m3 makes r1 the root_tip, m3 the header_tip
	//        Adding r3 should change the root_tip to r3, header_tip to m2
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, nil, nil, nil)

	// m1 is the genesis block
	m1 := shardState.GetBlockByNumber(0)
	m2 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	m2, _, err = shardState.FinalizeAndAddBlock(m2)
	checkErr(err)

	r1 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r2 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r1.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r1.Finalize(nil, nil)

	_, err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Header().Time + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil)
	assert.Equal(t, false, reflect.DeepEqual(r1.Header().Hash().String(), r2.Header().Hash().String()))
	_, err = shardState.AddRootBlock(r2)
	checkErr(err)
	assert.Equal(t, shardState.rootTip.Hash().String(), r1.Header().Hash().String())

	m3, err := shardState.CreateBlockToMine(nil, &acc1, nil)
	checkErr(err)
	assert.Equal(t, m3.Header().PrevRootBlockHash.String(), r1.Header().Hash().String())
	m3, _, err = shardState.FinalizeAndAddBlock(m3)
	checkErr(err)

	r3 := r2.Header().CreateBlockToAppend(nil, nil, &acc1, nil, nil)
	r3.AddMinorBlockHeader(m2.Header())
	r3.Finalize(nil, nil)
	_, err = shardState.AddRootBlock(r3)
	checkErr(err)

	assert.Equal(t, shardState.rootTip.Hash().String(), r3.Header().Hash().String())
	assert.Equal(t, shardState.CurrentHeader().Hash().String(), m2.Header().Hash().String())
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), m2.Header().Hash().String())

}
