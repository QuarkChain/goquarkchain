package core

import (
	"encoding/hex"
	"errors"
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	ethParams "github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/assert"
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
	assert.NotNil(t, shardState.CurrentBlock().Header().CoinbaseAmount.GetTokenBalance(genesisTokenID), testShardCoinbaseAmount)
}

func TestInitGenesisState(t *testing.T) {
	env := setUp(nil, nil, nil)

	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	genesisHeader := shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader)
	fakeNonce := uint64(1234)
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil)
	rootBlock = modifyNumber(rootBlock, 0)
	rootBlock.Finalize(nil, nil, common.Hash{})

	newGenesisBlock, err := shardState.InitGenesisState(rootBlock)
	checkErr(err)
	assert.NotEqual(t, newGenesisBlock.Hash(), genesisHeader.Hash())
	// header tip is still the old genesis header
	assert.Equal(t, shardState.CurrentBlock().IHeader().Hash(), genesisHeader.Hash())

	block := newGenesisBlock.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	_, _, err = shardState.FinalizeAndAddBlock(block)
	checkErr(err)

	// extending new_genesis_block doesn't change header_tip due to root chain first consensus
	assert.Equal(t, shardState.CurrentBlock().IHeader().Hash(), genesisHeader.Hash())

	zeroTempHeader := shardState.GetBlockByNumber(0).(*types.MinorBlock).Header()
	assert.Equal(t, zeroTempHeader.Hash(), genesisHeader.Hash())

	// extending the root block will change the header_tip
	newRootBlock := rootBlock.Header().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil)
	newRootBlock.Finalize(nil, nil, common.Hash{})
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

	testGenesisMinorTokenBalance = map[string]*big.Int{
		"QKC": new(big.Int).SetUint64(100000000),
		"QI":  new(big.Int).SetUint64(100000000),
		"BTC": new(big.Int).SetUint64(100000000),
	}
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	fakeData := uint64(100000000)
	env := setUp(&accList[0], &fakeData, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	fakeChan := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChan
	defer shardState.Stop()

	qkcToken := qkcCommon.TokenIDEncode("QKC")
	qiToken := qkcCommon.TokenIDEncode("QI")
	btcToken := qkcCommon.TokenIDEncode("BTC")
	qkcPrices := []uint64{42, 42, 100, 42, 41}
	qiPrices := []uint64{43, 101, 43, 41, 40}

	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.Finalize(nil, nil, common.Hash{})

	_, err := shardState.AddRootBlock(rootBlock)
	checkErr(err)

	// 5 tx per block, make 5 blocks
	for nonce := 0; nonce < 5; nonce++ {
		for accIndex := 0; accIndex < 5; accIndex++ {
			var qkcPrice, qiPrice *big.Int
			if accIndex != 0 {
				qkcPrice = new(big.Int)
				qiPrice = new(big.Int)
			} else {
				qkcPrice = new(big.Int).SetUint64(qkcPrices[nonce])
				qiPrice = new(big.Int).SetUint64(qiPrices[nonce])
			}

			randomIndex := (accIndex + 1) % 5
			fakeGasPrice := qkcPrice.Uint64()
			fakeNonce := uint64(nonce * 2)
			fakeToken := qkcToken

			tempTx := createTransferTransaction(shardState, idList[accIndex].GetKey().Bytes(), accList[accIndex],
				accList[randomIndex], new(big.Int).SetUint64(0), nil, &fakeGasPrice, &fakeNonce,
				nil, &fakeToken, nil)
			err = shardState.AddTx(tempTx)
			checkErr(err)

			randomIndex = (accIndex + 1) % 5
			fakeGasPrice = qiPrice.Uint64()
			fakeNonce = uint64(nonce*2) + 1
			fakeToken = qiToken

			tempTx = createTransferTransaction(shardState, idList[accIndex].GetKey().Bytes(), accList[accIndex],
				accList[randomIndex], new(big.Int).SetUint64(0), nil, &fakeGasPrice, &fakeNonce,
				nil, &fakeToken, nil)
			err = shardState.AddTx(tempTx)
			checkErr(err)
		}
		b, err := shardState.CreateBlockToMine(nil, &accList[1], nil, nil, nil)
		checkErr(err)
		_, _, err = shardState.FinalizeAndAddBlock(b)
		checkErr(err)
		forRe := true
		for forRe == true {
			select {
			case result := <-fakeChan:
				if result == uint64(nonce+1) {
					forRe = false
				}
			case <-time.After(2 * time.Second):
				panic(errors.New("should end here"))

			}

		}

	}

	currentNumber := int(shardState.CurrentBlock().NumberU64())
	assert.Equal(t, currentNumber, 5)
	// for testing purposes, update percentile to take max gas price
	shardState.gasPriceSuggestionOracle.Percentile = 100
	gasPrice, err := shardState.GasPrice(qkcToken)
	assert.NoError(t, err)
	assert.Equal(t, gasPrice, uint64(100))

	gasPrice, err = shardState.GasPrice(qiToken)
	assert.NoError(t, err)
	assert.Equal(t, gasPrice, uint64(43))

	//clear the cache, update percentile to take the second largest gas price
	shardState.gasPriceSuggestionOracle.cache.Purge()
	shardState.gasPriceSuggestionOracle.Percentile = 95
	gasPrice, err = shardState.GasPrice(qkcToken)
	assert.NoError(t, err)
	assert.Equal(t, gasPrice, uint64(42))

	gasPrice, err = shardState.GasPrice(qiToken)
	assert.NoError(t, err)
	assert.Equal(t, gasPrice, uint64(41))

	gasPrice, err = shardState.GasPrice(btcToken)
	assert.NoError(t, err)
	assert.Equal(t, gasPrice, uint64(0))

	gasPrice, err = shardState.GasPrice(1)
	assert.Error(t, err)
	assert.Equal(t, gasPrice, uint64(0))

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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)
	txGen := func(data []byte) *types.Transaction {
		return createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(123456), nil, nil, nil, data, nil, nil)
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
	acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	checkErr(err)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)

	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil, nil, nil)
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
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil, nil, nil)
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil, nil, nil)
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
	b1, err := shardState.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(49999), nil, nil)
	checkErr(err)
	assert.Equal(t, b1.Header().Number, uint64(1))
	assert.Equal(t, len(b1.Transactions()), 0)

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
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

	assert.Equal(t, currentState1.GetBalance(id1.GetRecipient(), testGenesisTokenID).Uint64(), uint64(10000000-ethParams.TxGas-12345))
	assert.Equal(t, currentState1.GetBalance(acc2.Recipient, testGenesisTokenID).Uint64(), uint64(12345))
	// shard miner only receives a percentage of reward because of REWARD_TAX_RATE
	acc3Value := currentState1.GetBalance(acc3.Recipient, testGenesisTokenID)
	should3 := new(big.Int).Add(new(big.Int).SetUint64(ethParams.TxGas/2), new(big.Int).Div(testShardCoinbaseAmount, new(big.Int).SetUint64(2)))
	assert.Equal(t, should3, acc3Value)
	assert.Equal(t, len(re), 3)
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil, nil, nil)

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
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	// Should succeed
	b1, reps, err := shardState.FinalizeAndAddBlock(b1)

	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b1.Header().Hash())

	currentState, err := shardState.State()
	checkErr(err)
	assert.Equal(t, currentState.GetBalance(id1.GetRecipient(), testGenesisTokenID).Uint64(), uint64(10000000-21000-12345))

	assert.Equal(t, currentState.GetBalance(acc2.Recipient, testGenesisTokenID).Uint64(), uint64(12345))

	shouldAcc3 := new(big.Int).Add(new(big.Int).Div(testShardCoinbaseAmount, new(big.Int).SetUint64(2)), new(big.Int).SetUint64(21000/2))
	assert.Equal(t, currentState.GetBalance(acc3.Recipient, testGenesisTokenID).Cmp(shouldAcc3), 0)

	// Check receipts
	assert.Equal(t, len(reps), 3)
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
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, fakeMoey, nil, nil, nil, nil, nil, nil)

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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(1000000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil, nil, nil)

	err = shardState.AddTx(tx)
	assert.Error(t, err)
	assert.Equal(t, len(shardState.txPool.pending), 0)

	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc3, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil, nil, nil)

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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	// xshard tx
	xsGsLmt := shardState.GasLimit()/2 + 1
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &xsGsLmt, nil, nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	assert.Error(t, err)

	fakeGas := uint64(500000)
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 0)
	fakeGasPrice := uint64(2)
	// inshard tx
	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc3, new(big.Int).SetUint64(12345), &fakeGas, &fakeGasPrice, nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)
	b1, err = shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil, nil, nil))
	checkErr(err)
	currState, err := shardState.State()
	checkErr(err)
	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	_, res, err := shardState.FinalizeAndAddBlock(b0)
	checkErr(err)
	assert.Equal(t, len(res), 3)

	currState, err = shardState.State()
	checkErr(err)
	acc1Value := currState.GetBalance(id1.GetRecipient(), testGenesisTokenID)
	acc2Value := currState.GetBalance(acc2.Recipient, testGenesisTokenID)
	acc3Value := currState.GetBalance(acc3.Recipient, testGenesisTokenID)
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
	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, toAddress,
		new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil, nil, nil))
	checkErr(err)
	fakeGas = uint64(40000)
	err = shardState.AddTx(createTransferTransaction(shardState, id2.GetKey().Bytes(), acc2, acc1,
		new(big.Int).SetUint64(54321), &fakeGas, nil, nil, nil, nil, nil))
	checkErr(err)
	//Inshard gas limit is 40000 - 20000
	b1, err := shardState.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(40000),
		new(big.Int).SetUint64(20000), nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 0)
	b1, err = shardState.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(40000),
		new(big.Int), nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 1)
	b1, err = shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 2)

	// # Should succeed
	_, reps, err := shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().Hash(), b1.Header().Hash())
	currState, err = shardState.State()
	checkErr(err)
	acc1Value = currState.GetBalance(id1.GetRecipient(), testGenesisTokenID)
	assert.Equal(t, acc1Value.Uint64(), uint64(1000000-21000-12345+54321))

	acc2Value = currState.GetBalance(id2.GetRecipient(), testGenesisTokenID)
	assert.Equal(t, acc2Value.Uint64(), uint64(1000000-21000+12345-54321))

	acc3Value = currState.GetBalance(acc3.Recipient, testGenesisTokenID)

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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil, nil, nil))
	checkErr(err)
	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	b00 := types.NewMinorBlock(b0.Header(), b0.Meta(), nil, nil, nil)
	_, _, err = shardState.FinalizeAndAddBlock(b00)
	checkErr(err)
	// tx is added back to queue in the end of create_block_to_mine
	assert.Equal(t, len(shardState.txPool.pending), 1)
	assert.Equal(t, len(b1.Transactions()), 1)
	_, _, err = shardState.FinalizeAndAddBlock(b1)
	// b1 is a fork and does not remove the tx from queue
	assert.Equal(t, len(shardState.txPool.pending), 1)

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil, nil, nil))
	checkErr(err)

	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState.FinalizeAndAddBlock(b0)
	checkErr(err)

	b11 := types.NewMinorBlock(b1.Header(), b1.Meta(), nil, nil, nil)
	b11, _, err = shardState.FinalizeAndAddBlock(b11) //# make b1 empty
	checkErr(err)

	b2 := b11.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	_, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)

	// # now b1-b2 becomes the best chain and we expect b0 to be reverted and put the tx back to queue
	b3 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState.FinalizeAndAddBlock(b3)

	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
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
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
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
	rootBlock.AddMinorBlockHeader(shardState.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock = rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(21000 + 9000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &fakeGas, nil, nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
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
	assert.Equal(t, temp.TxHash, tx.Hash())
	assert.Equal(t, temp.From.ToBytes(), acc1.ToBytes())
	assert.Equal(t, temp.To.ToBytes(), acc2.ToBytes())
	assert.Equal(t, temp.Value.Value.Uint64(), uint64(888888))
	assert.Equal(t, temp.GasPrice.Value.Uint64(), uint64(1))

	// Make sure the xshard gas is not used by local block
	id1Value := shardState.currentEvmState.GetGasUsed()
	assert.Equal(t, id1Value.Uint64(), uint64(21000+9000))
	id3Value := shardState.currentEvmState.GetBalance(acc3.Recipient, testGenesisTokenID)
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
	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &fakeGas, nil, nil, nil, nil, nil))
	assert.Error(t, err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, len(b1.Transactions()), 0)
	assert.Equal(t, len(shardState.txPool.pending), 0)
}

func TestXShardTxReceived(t *testing.T) {
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
	rootBlock.AddMinorBlockHeader(shardState0.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err0 := shardState0.AddRootBlock(rootBlock)
	checkErr(err0)

	_, err1 := shardState1.AddRootBlock(rootBlock)
	checkErr(err1)

	// Add one block in shard 0
	b0, err := shardState0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b1Headaer := b1.Header()
	b1Headaer.PrevRootBlockHash = rootBlock.Header().Hash()
	b1 = types.NewMinorBlock(b1Headaer, b1.Meta(), b1.Transactions(), nil, nil)
	fakeGas := uint64(30000)
	fakeGasPrice := uint64(2)
	value := new(big.Int).SetUint64(888888)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, value, &fakeGas, &fakeGasPrice, nil, nil, nil, nil)
	b1.AddTx(tx)
	txList := types.CrossShardTransactionDepositList{}
	crossShardGas := new(serialize.Uint256)
	intrinsic := uint64(21000) + params.GtxxShardCost.Uint64()
	crossShardGas.Value = new(big.Int).SetUint64(tx.EvmTx.Gas() - intrinsic)
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		TxHash:          tx.Hash(),
		From:            acc2,
		To:              acc1,
		Value:           &serialize.Uint256{Value: value},
		GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(fakeGasPrice)},
		GasRemained:     crossShardGas,
		TransferTokenID: tx.EvmTx.TransferTokenID(),
		GasTokenID:      tx.EvmTx.GasTokenID(),
	})
	// Add a x-shard tx from remote peer
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList) // write db
	// Create a root block containing the block with the x-shard tx
	rootBlock = shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	// Add b0 and make sure all x-shard tx's are added
	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)
	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient, shardState0.GetGenesisToken())
	assert.Equal(t, int64(10000000+888888), acc1Value.Int64())

	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient, shardState0.GetGenesisToken())
	acc3Should := new(big.Int).Add(testShardCoinbaseAmount, new(big.Int).SetUint64(9000*2))
	acc3Should = new(big.Int).Div(acc3Should, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3Should.String(), acc3Value.String())

	// X-shard gas used
	assert.Equal(t, uint64(9000), shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64())
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
	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	fakeGas := uint64(30000)
	fakeGasPrice := uint64(2)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(888888), &fakeGas, &fakeGasPrice, nil, nil, nil, nil)
	b1.AddTx(tx)

	// Create a root block containing the block with the x-shard tx
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient, testGenesisTokenID)
	assert.Equal(t, uint64(10000000), acc1Value.Uint64())

	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient, testGenesisTokenID)
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
	shardState0.clusterConfig.Quarkchain.MinMiningGasPrice = new(big.Int)
	fakeShardID = uint32(1)
	shardState1 := createDefaultShardState(env1, &fakeShardID, nil, nil, nil)
	shardState1.clusterConfig.Quarkchain.MinMiningGasPrice = new(big.Int)

	// Add a root block to allow later minor blocks referencing this root block to
	// be broadcasted
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})

	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)
	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)

	// Add one block in shard 0
	b0, err := shardState0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b1Header := b1.Header()
	b1Header.PrevRootBlockHash = rootBlock.Header().Hash()
	b1 = types.NewMinorBlock(b1Header, b1.Meta(), b1.Transactions(), nil, nil)
	gas := uint64(21000) + params.GtxxShardCost.Uint64()
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(888888),
		&gas, nil, nil, nil, nil, nil)
	b1.AddTx(tx)
	crossShardGas := new(serialize.Uint256)
	intrinsic := uint64(21000) + params.GtxxShardCost.Uint64()
	crossShardGas.Value = new(big.Int).SetUint64(tx.EvmTx.Gas() - intrinsic)
	txList := types.CrossShardTransactionDepositList{}
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		TxHash:          tx.Hash(),
		From:            acc2,
		To:              acc1,
		Value:           &serialize.Uint256{Value: new(big.Int).SetUint64(888888)},
		GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(2)},
		GasRemained:     crossShardGas,
		TransferTokenID: tx.EvmTx.TransferTokenID(),
		GasTokenID:      tx.EvmTx.GasTokenID(),
	})
	// Add a x-shard tx from state1
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList)

	// Create a root block containing the block with the x-shard tx
	rootBlock0 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock0.AddMinorBlockHeader(b0.Header())
	rootBlock0.AddMinorBlockHeader(b1.Header())
	rootBlock0.Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(rootBlock0)
	checkErr(err)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b3 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b3Header := b3.Header()
	b3Header.PrevRootBlockHash = rootBlock.Header().Hash()
	b3 = types.NewMinorBlock(b3Header, b3.Meta(), b3.Transactions(), nil, nil)

	txList = types.CrossShardTransactionDepositList{}
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		TxHash:          common.Hash{},
		From:            acc2,
		To:              acc1,
		Value:           &serialize.Uint256{Value: new(big.Int).SetUint64(385723)},
		GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(3)},
		GasRemained:     crossShardGas,
		TransferTokenID: tx.EvmTx.TransferTokenID(),
		GasTokenID:      tx.EvmTx.GasTokenID(),
	})
	// Add a x-shard tx from state1
	shardState0.AddCrossShardTxListByMinorBlockHash(b3.Header().Hash(), txList)

	rootBlock1 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock1.AddMinorBlockHeader(b2.Header())
	rootBlock1.AddMinorBlockHeader(b3.Header())
	rootBlock1.Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(rootBlock1)
	checkErr(err)

	// Test x-shard gas limit when create_block_to_mine
	b6, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(9000), nil, nil)
	checkErr(err)
	assert.Equal(t, rootBlock1.Hash().String(), b6.PrevRootBlockHash().String())

	// There are two x-shard txs: one is root block coinbase with zero gas, and another is from shard 1
	b7, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).Mul(new(big.Int).SetUint64(9000), new(big.Int).SetUint64(2)), nil, nil)
	checkErr(err)

	assert.Equal(t, rootBlock1.Header().Hash().String(), b7.PrevRootBlockHash().String())
	b8, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).Mul(new(big.Int).SetUint64(9000), new(big.Int).SetUint64(3)), nil, nil)
	checkErr(err)
	assert.Equal(t, rootBlock1.Header().Hash().String(), b8.PrevRootBlockHash().String())

	// Add b0 and make sure all x-shard tx's are added
	b4, err := shardState0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, b4.Header().PrevRootBlockHash.String(), rootBlock1.Hash().String())
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)

	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient, testGenesisTokenID)
	assert.Equal(t, 10000000+888888+385723, int(acc1Value.Uint64()))

	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient, testGenesisTokenID)
	acc3ValueShould := new(big.Int).Add(new(big.Int).SetUint64(9000*5), testShardCoinbaseAmount)
	acc3ValueShould = new(big.Int).Div(acc3ValueShould, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3ValueShould.String(), acc3Value.String())

	// Check gas used for receiving x-shard tx
	assert.Equal(t, 18000, int(shardState0.currentEvmState.GetGasUsed().Uint64()))
	assert.Equal(t, 18000, int(shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64()))
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
	b0 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b1 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	b0, _, err = shardState.FinalizeAndAddBlock(b0)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b0.Hash())

	// Fork happens, first come first serve
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b0.Hash())

	// Longer fork happens, override existing one
	b2 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
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
	b0 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &emptyAddress, nil, nil, nil, nil, nil, nil)

	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	evmState, reps, _, _, _, err := shardState1.runBlock(b1)
	temp := types.NewEmptyTokenBalances()
	temp.Add(evmState.GetBlockFee())
	b1.Finalize(reps, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), temp, &types.XShardTxCursorInfo{})
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis.Header())
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b00 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b00, _, err = shardState0.FinalizeAndAddBlock(b00)
	assert.Equal(t, shardState0.CurrentBlock().IHeader().Hash().String(), b00.Hash().String())
	checkErr(err)

	// Create another fork that is much longer (however not confirmed by root_block)
	b3 := b2.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState0.FinalizeAndAddBlock(b3)
	checkErr(err)
	assert.Equal(t, shardState0.CurrentBlock().IHeader().Hash().String(), b00.Hash().String())
	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
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
	b0 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &emptyAddress, nil, nil, nil, nil, nil, nil)

	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	evmState, reps, _, _, _, err := shardState1.runBlock(b1)
	temp := types.NewEmptyTokenBalances()
	temp.Add(evmState.GetBlockFee())
	b1.Finalize(reps, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), temp, &types.XShardTxCursorInfo{})

	// Add one empty root block
	emptyRoot := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(emptyRoot)
	checkErr(err)

	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis.Header())
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})

	rootBlock1 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock1.AddMinorBlockHeader(genesis.Header())
	rootBlock1.AddMinorBlockHeader(b2.Header())
	rootBlock1.AddMinorBlockHeader(b1.Header())
	rootBlock1.Finalize(nil, nil, common.Hash{})

	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b00 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b00, _, err = shardState0.FinalizeAndAddBlock(b00)
	checkErr(err)
	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b00.Hash().String())

	// Create another fork that is much longer (however not confirmed by root_block)
	b3 := b2.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState0.FinalizeAndAddBlock(b3)
	checkErr(err)
	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)

	currBlock := shardState0.CurrentBlock()
	currBlock2 := shardState0.GetBlockByNumber(2)
	currBlock3 := shardState0.GetBlockByNumber(3)
	assert.Equal(t, currBlock.Hash().String(), b00.Header().Hash().String())
	assert.Equal(t, currBlock2.Hash().String(), b00.Header().Hash().String())
	assert.Equal(t, currBlock3, nil)

	b5 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	_, err = shardState0.AddRootBlock(rootBlock1)

	// Add one empty root block
	emptyRoot = rootBlock1.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(emptyRoot)
	checkErr(err)
	rootBlock2 := emptyRoot.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock2.AddMinorBlockHeader(b3.Header())
	rootBlock2.AddMinorBlockHeader(b4.Header())
	rootBlock2.AddMinorBlockHeader(b5.Header())
	rootBlock2.Finalize(nil, nil, common.Hash{})

	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b2.Header().Hash().String())
	_, err = shardState0.AddRootBlock(rootBlock2)
	checkErr(err)

	currBlock = shardState0.CurrentBlock()

	assert.Equal(t, currBlock.Hash().String(), b4.Header().Hash().String())
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
	headers = append(headers, shardState.CurrentBlock().Header())

	for index := 0; index < 13; index++ {
		b := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
		b, _, err = shardState.FinalizeAndAddBlock(b)
		headers = append(headers, b.Header())
	}

	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.ExtendMinorBlockHeaderList(headers)
	rootBlock.Finalize(nil, nil, common.Hash{})

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
	rootBlock1.Finalize(nil, nil, common.Hash{})
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
	rootBlock.Finalize(nil, nil, common.Hash{})

	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b0.Header().Hash().String())
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	b1 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	fakeNonce := uint64(1)
	b2 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil, nil, nil, nil, nil)
	b2Header := b2.Header()
	b2Header.PrevRootBlockHash = rootBlock.Header().Hash()
	b2 = types.NewMinorBlock(b2Header, b2.Meta(), b2.Transactions(), nil, nil)
	fakeNonce = uint64(2)
	b3 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil, nil, nil, nil, nil)
	b3Header := b3.Header()
	b3 = types.NewMinorBlock(b3Header, b2.Meta(), b2.Transactions(), nil, nil)

	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b1.Header().Hash().String())

	// Fork happens, although they have the same height, b2 survives since it confirms root block
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b2.Header().Hash().String())

	// b3 confirms the same root block as b2, so it will not override b2
	b3, _, err = shardState.FinalizeAndAddBlock(b3)
	checkErr(err)

	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b2.Header().Hash().String())
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

	createTime := shardState.CurrentBlock().Header().GetTime() + 8
	// Check new difficulty
	b0, err := shardState.CreateBlockToMine(&createTime, nil, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), uint64(shardState.CurrentBlock().Header().GetDifficulty().Uint64()/uint64(2048)+shardState.CurrentHeader().GetDifficulty().Uint64()))

	createTime = shardState.CurrentBlock().Header().GetTime() + 9
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentBlock().Header().GetDifficulty().Uint64())

	createTime = shardState.CurrentBlock().Header().GetTime() + 17
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64())

	createTime = shardState.CurrentBlock().Header().GetTime() + 24
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentBlock().Header().GetDifficulty().Uint64()-shardState.CurrentBlock().Header().GetDifficulty().Uint64()/uint64(2048))

	createTime = shardState.CurrentBlock().Header().GetTime() + 35
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, b0.Header().Difficulty.Uint64(), shardState.CurrentBlock().Header().GetDifficulty().Uint64()-shardState.CurrentBlock().Header().GetDifficulty().Uint64()/uint64(2048)*2)
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
	blockHeaders = append(blockHeaders, shardState.CurrentBlock().Header())
	blockMetas = append(blockMetas, shardState.CurrentBlock().GetMetaData())
	for index := 0; index < 12; index++ {
		b := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
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
	rootBlock.Finalize(nil, nil, common.Hash{})
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
	assert.Equal(t, recoveredState.CurrentBlock().Hash().String(), blockHeaders[4].Hash().String())
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
	blockHeaders = append(blockHeaders, shardState.CurrentBlock().Header())
	blockMetas = append(blockMetas, shardState.CurrentBlock().GetMetaData())
	for index := 0; index < 12; index++ {
		b := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
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
	assert.Equal(t, recoveredState.CurrentBlock().Hash().String(), recoveredState.CurrentBlock().Hash().String())
	assert.Equal(t, recoveredState.CurrentBlock().Hash().String(), genesis.Hash().String())

	assert.Equal(t, true, recoveredState.confirmedHeaderTip == nil)
	assert.Equal(t, recoveredState.CurrentBlock().GetMetaData().Hash().String(), genesis.(*types.MinorBlock).GetMetaData().Hash().String())

	currState := recoveredState.currentEvmState

	CURRState, err := recoveredState.State()
	checkErr(err)

	roo1, err := currState.Commit(true)
	root2, err := CURRState.Commit(true)
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

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	// Should succeed
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)

	evmState, reps, _, _, _, err := shardState.runBlock(b1)
	checkErr(err)
	temp := types.NewEmptyTokenBalances()
	temp.SetValue(b1.Header().CoinbaseAmount.GetTokenBalance(genesisTokenID), qkcCommon.TokenIDEncode("QKC"))
	b1.Finalize(reps, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), temp, &types.XShardTxCursorInfo{})

	b1Meta := b1.Meta()
	b1Meta.Root = common.Hash{}
	b1 = types.NewMinorBlock(b1.Header(), b1Meta, b1.Transactions(), nil, nil)

	rawdb.DeleteMinorBlock(shardState.db, b1.Hash())
	_, err = shardState.InsertChain([]types.IBlock{b1}, false)
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
	r1.Finalize(nil, nil, common.Hash{})

	_, err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Header().Time + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil, common.Hash{})
	assert.Equal(t, false, reflect.DeepEqual(r1.Header().Hash().String(), r2.Header().Hash().String()))

	_, err = shardState.AddRootBlock(r2)

	checkErr(err)
	assert.Equal(t, shardState.rootTip.Hash().String(), r1.Header().Hash().String())

	m2 := m1.(*types.MinorBlock).CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
	m2Header := m2.Header()
	m2Header.PrevRootBlockHash = r2.Header().Hash()
	m2 = types.NewMinorBlock(m2Header, m2.Meta(), m2.Transactions(), nil, nil)

	// m2 is added
	m2, _, err = shardState.FinalizeAndAddBlock(m2)
	checkErr(err)

	// but m1 should still be the tip
	assert.Equal(t, shardState.GetMinorBlock(m2.Hash()).Hash().String(), m2.Header().Hash().String())
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), m1.IHeader().Hash().String())
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
	m2 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
	m2, _, err = shardState.FinalizeAndAddBlock(m2)
	checkErr(err)

	r1 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r2 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r1.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r1.Finalize(nil, nil, common.Hash{})

	_, err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Header().Time + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil, common.Hash{})
	assert.Equal(t, false, reflect.DeepEqual(r1.Header().Hash().String(), r2.Header().Hash().String()))
	_, err = shardState.AddRootBlock(r2)
	checkErr(err)
	assert.Equal(t, shardState.rootTip.Hash().String(), r1.Header().Hash().String())

	m3, err := shardState.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, m3.Header().PrevRootBlockHash.String(), r1.Header().Hash().String())
	m3, _, err = shardState.FinalizeAndAddBlock(m3)
	checkErr(err)

	r3 := r2.Header().CreateBlockToAppend(nil, nil, &acc1, nil, nil)
	r3.AddMinorBlockHeader(m2.Header())
	r3.Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(r3)
	checkErr(err)

	assert.Equal(t, shardState.rootTip.Hash().String(), r3.Header().Hash().String())
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), m2.Header().Hash().String())
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), m2.Header().Hash().String())
}

func TestTotalTxCount(t *testing.T) {
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil, nil, nil)
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

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.Header().Number, uint64(1))

	// Should succeed
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().IHeader().NumberU64(), uint64(1))
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b2.Header().Hash())
	assert.Equal(t, shardState.CurrentBlock().GetTransactions()[0].Hash(), tx.Hash())

	assert.Equal(t, uint32(1), *shardState.getTotalTxCount(shardState.CurrentBlock().Hash()))

	fakeGas = uint64(50000)
	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	b2, err = shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	assert.Equal(t, uint32(2), *shardState.getTotalTxCount(shardState.CurrentBlock().Hash()))

	fakeGas = uint64(50000)
	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	b2, err = shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	assert.Equal(t, uint32(3), *shardState.getTotalTxCount(shardState.CurrentBlock().Hash()))

}

func TestGetPendingTxFromAddress(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)

	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()

	fakeChan := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChan
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)

	data, _, err := shardState.getPendingTxByAddress(acc1, nil)
	checkErr(err)
	assert.Equal(t, 1, len(data))
	assert.Equal(t, data[0].Value.Value, new(big.Int).SetUint64(12345))
	assert.Equal(t, data[0].TxHash, tx.Hash())
	assert.Equal(t, data[0].FromAddress, acc1)
	assert.Equal(t, data[0].ToAddress, &acc2)
	assert.Equal(t, data[0].BlockHeight, uint64(0))
	assert.Equal(t, data[0].Timestamp, uint64(0))
	assert.Equal(t, data[0].Success, false)

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)

	// Should succeed
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)

	forRe := true
	for forRe == true {
		select {
		case result := <-fakeChan:
			if result == uint64(0+1) {
				forRe = false
			}
		case <-time.After(2 * time.Second):
			panic(errors.New("should end here"))

		}
	}

	data, _, err = shardState.getPendingTxByAddress(acc1, nil)
	checkErr(err)
	assert.Equal(t, 0, len(data))

}

func TestResetToOldChain(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)

	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()

	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	r0 := shardState.CurrentBlock()

	rs1 := r0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rs1, _, err = shardState.FinalizeAndAddBlock(rs1)
	assert.NoError(t, err)

	rr1 := r0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	tHeader := rr1.Header()
	tHeader.SetCoinbase(acc3)
	rr1 = types.NewMinorBlock(tHeader, rr1.Meta(), rr1.Transactions(), nil, nil)
	rr1, _, err = shardState.FinalizeAndAddBlock(rr1)
	assert.NoError(t, err)

	assert.Equal(t, shardState.CurrentBlock(), rs1)

	rr2 := rr1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rr2, _, err = shardState.FinalizeAndAddBlock(rr2)
	assert.NoError(t, err)
	assert.Equal(t, shardState.CurrentBlock(), rr2)

	assert.Equal(t, shardState.GetBlockByNumber(1).Hash(), rr1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(2).Hash(), rr2.Hash())

	err = shardState.reorg(shardState.CurrentBlock(), rs1)
	assert.NoError(t, err)
	assert.Equal(t, shardState.CurrentBlock().Hash(), rs1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(1).Hash(), rs1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(2).Hash(), rr2.Hash())
}

func TestContractCall(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	id2, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	storageKeyStr := ZFill64(hex.EncodeToString(acc1.Recipient[:])) + ZFill64("1")
	storageKeyBytes, err := hex.DecodeString(storageKeyStr)
	assert.NoError(t, err)
	storageKey := crypto.Keccak256Hash(storageKeyBytes)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()

	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	b1, err := shardState.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	assert.NoError(t, err)

	tx0, err := CreateContract(shardState, id1.GetKey(), acc1, acc1.FullShardKey, ContractWithStorage2)
	assert.NoError(t, err)
	err = shardState.AddTx(tx0)
	assert.NoError(t, err)
	b2, err := shardState.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	assert.NoError(t, err)
	_, _, contractReceipt := shardState.GetTransactionReceipt(tx0.Hash())
	assert.Equal(t, uint64(0x1), contractReceipt.Status)
	contractAddress := account.NewAddress(contractReceipt.ContractAddress, contractReceipt.ContractFullShardKey)

	data, err := hex.DecodeString("c2e171d7")
	value, gasPrice, gas := big.NewInt(0), uint64(1), uint64(50000)
	tx3 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, contractAddress,
		value, &gas, &gasPrice, nil, data, nil, nil)
	err = shardState.AddTx(tx3)
	assert.NoError(t, err)
	b3, err := shardState.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	b3, receipts, err := shardState.FinalizeAndAddBlock(b3)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(receipts))
	result, err := shardState.GetStorageAt(contractAddress.Recipient, storageKey, nil)
	assert.NoError(t, err)
	v1 := "000000000000000000000000000000000000000000000000000000000000162e"
	assert.Equal(t, v1, hex.EncodeToString(result.Bytes()))
	_, _, receipt := shardState.GetTransactionReceipt(tx3.Hash())
	assert.Equal(t, uint64(0x1), receipt.Status)
}

func TestXShardRootBlockCoinbase(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1<<16)

	genesis := uint64(10000000)
	shardSize := uint32(64)
	shardId0 := uint32(0)
	shardId := uint32(16)
	env1 := setUp(&acc1, &genesis, &shardSize)
	shardState1 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	shardState1.shardConfig.Genesis.GasLimit = params.GtxxShardCost.Uint64() * 2
	env2 := setUp(&acc2, &genesis, &shardSize)
	shardState2 := createDefaultShardState(env2, &shardId, nil, nil, nil)
	shardState2.shardConfig.Genesis.GasLimit = params.GtxxShardCost.Uint64() * 2
	defer func() {
		shardState1.Stop()
		shardState2.Stop()
	}()

	rootBlock := shardState1.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState2.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)
	_, err = shardState2.AddRootBlock(rootBlock)
	checkErr(err)

	//Create a root block containing the block with the x-shard tx
	rootBlock = shardState1.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	coinbase := types.NewEmptyTokenBalances()
	coinbase.SetValue(big.NewInt(1000000), qkcCommon.TokenIDEncode("QKC"))
	rootBlock.Finalize(coinbase, &acc1, common.Hash{})
	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)
	_, err = shardState2.AddRootBlock(rootBlock)
	checkErr(err)

	//Add b0 and make sure one x-shard tx's are added
	b0, err := shardState1.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState1.FinalizeAndAddBlock(b0)
	checkErr(err)

	//Root block coinbase does not consume xshard gas
	blc, err := shardState1.GetBalance(acc1.Recipient, nil)
	checkErr(err)
	assert.Equal(t, big.NewInt(10000000+1000000), blc.GetTokenBalance(qkcCommon.TokenIDEncode("QKC")))

	//Add b0 and make sure one x-shard tx's are added
	b0, err = shardState2.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState2.FinalizeAndAddBlock(b0)
	checkErr(err)

	//Root block coinbase does not consume xshard gas
	blc, err = shardState2.GetBalance(acc1.Recipient, nil)
	checkErr(err)
	assert.Equal(t, big.NewInt(10000000), blc.GetTokenBalance(qkcCommon.TokenIDEncode("QKC")))
}
func TestXShardSenderGasLimit(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1<<16)
	genesis := uint64(10000000)
	shardSize := uint32(64)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, &genesis, &shardSize)
	shardState1 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer shardState1.Stop()

	rootBlock := shardState1.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState1.AddRootBlock(rootBlock)

	var b0 = shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b0.Header().PrevRootBlockHash = rootBlock.Hash()
	gas := b0.GetMetaData().XShardGasLimit.Value.Uint64() + 1
	gasPrice := uint64(1)
	tx0 := CreateTransferTx(shardState1, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &gas,
		&gasPrice, nil)
	assert.Error(t, shardState1.AddTx(tx0))
	b0.AddTx(tx0)
	_, _, err = shardState1.FinalizeAndAddBlock(b0)
	assert.Error(t, err) //xshard evm tx exceeds xshard gas limit
	xGasLimit := big.NewInt(21000 * 9)
	includeTx := false
	b2, err := shardState1.CreateBlockToMine(nil, nil, nil, xGasLimit, &includeTx)
	checkErr(err)
	b2.Header().PrevRootBlockHash = rootBlock.Hash()
	gas = uint64(21000 * 10)
	tx2 := CreateTransferTx(shardState1, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &gas,
		&gasPrice, nil)
	//assert.Error(t, shardState1.AddTx(tx2)) //No entry for go: pass extra param xshard_gas_limit=opcodes.GTXCOST * 9
	b2.AddTx(tx2)
	_, _, err = shardState1.FinalizeAndAddBlock(b2)
	assert.Error(t, err) //xshard evm tx exceeds xshard gas limit

	b1, err := shardState1.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	gas = b1.GetMetaData().XShardGasLimit.Value.Uint64()
	b1.Header().PrevRootBlockHash = rootBlock.Hash()
	tx1 := CreateTransferTx(shardState1, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &gas,
		&gasPrice, nil)
	b1.AddTx(tx1)
	_, _, err = shardState1.FinalizeAndAddBlock(b1)
	assert.NoError(t, err)
}

func TestXShardGasLimit(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1<<16)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	checkErr(err)

	genesis := uint64(10000000)
	shardSize := uint32(64)
	shardId0 := uint32(0)
	shardId := uint32(16)
	env1 := setUp(&acc1, &genesis, &shardSize)
	shardState1 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	env2 := setUp(&acc2, &genesis, &shardSize)
	shardState2 := createDefaultShardState(env2, &shardId, nil, nil, nil)
	defer func() {
		shardState1.Stop()
		shardState2.Stop()
	}()

	rootBlock := shardState1.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState2.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)
	_, err = shardState2.AddRootBlock(rootBlock)
	checkErr(err)

	//Add one block in shard 1 with 2 x-shard txs
	b1, err := shardState2.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b1.Header().PrevRootBlockHash = rootBlock.Hash()
	gas := params.GtxxShardCost.Uint64() + 21000
	gasPrice := uint64(2)
	value1 := new(big.Int).SetUint64(888888)
	tx0 := CreateTransferTx(shardState1, id1.GetKey().Bytes(), acc2, acc1, value1, &gas, &gasPrice, nil)
	b1.AddTx(tx0)
	value2 := new(big.Int).SetUint64(111111)
	tx1 := CreateTransferTx(shardState1, id1.GetKey().Bytes(), acc2, acc1, value2, &gas, &gasPrice, nil)
	b1.AddTx(tx1)
	//Add a x-shard tx from remote peer
	crossShardGas := new(serialize.Uint256)
	intrinsic := params.GtxxShardCost.Uint64() + 21000
	crossShardGas.Value = new(big.Int).SetUint64(gas - intrinsic)
	shardState1.AddCrossShardTxListByMinorBlockHash(b1.Hash(), types.CrossShardTransactionDepositList{
		TXList: []*types.CrossShardTransactionDeposit{
			{
				TxHash:          tx0.Hash(),
				From:            acc2,
				To:              acc1,
				Value:           &serialize.Uint256{Value: value1},
				GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(gasPrice)},
				GasRemained:     crossShardGas,
				TransferTokenID: tx0.EvmTx.TransferTokenID(),
				GasTokenID:      tx0.EvmTx.GasTokenID(),
			},
			{
				TxHash:          tx1.Hash(),
				From:            acc2,
				To:              acc1,
				Value:           &serialize.Uint256{Value: value2},
				GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(gasPrice)},
				GasRemained:     crossShardGas,
				TransferTokenID: tx1.EvmTx.TransferTokenID(),
				GasTokenID:      tx1.EvmTx.GasTokenID(),
			},
		}})

	//Create a root block containing the block with the x-shard tx
	rootBlock = shardState1.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b1.Header())
	coinbase := types.NewEmptyTokenBalances()
	coinbase.SetValue(big.NewInt(1000000), qkcCommon.TokenIDEncode("QKC"))
	rootBlock.Finalize(coinbase, &acc1, common.Hash{})

	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)
	//Add b0 and make sure one x-shard tx's are added
	xGas := params.GtxxShardCost //xshard gas limit excludes tx1
	b2, err := shardState1.CreateBlockToMine(nil, &acc3, nil, xGas, nil)
	checkErr(err)
	_, _, err = shardState1.FinalizeAndAddBlock(b2) //goquarkchain cannot override xshardGasLimit when validate block
	assert.Error(t, err, "incorrect xshard gas limit, expected %d, actual %d", 6000000, 9000)

	/*
		//Include tx one by one controlled by xshard gas limit.
		//This cannot be tested in goquarkchain because override xshardGasLimit when validate block does not work
		//Root block coinbase does not consume xshard gas
		act1 := getDefaultBalance(acc1, *shardState1)
		assert.Equal(t, 10000000+1000000+888888, int(act1.Uint64()))
		cfg := shardState1.clusterConfig.Quarkchain.GetShardConfigByFullShardID(uint32(shardState1.branch.Value))
		reward := params.GtxxShardCost.Uint64()*gasPrice + cfg.CoinbaseAmount.Uint64()
		rate := getLocalFeeRate(shardState1.clusterConfig.Quarkchain)
		rewardRated := new(big.Int).Mul(new(big.Int).SetUint64(reward), rate.Num())
		rewardRated = new(big.Int).Div(rewardRated, rate.Denom())
		act3 := getDefaultBalance(acc3, *shardState1)
		//Half collected by root
		assert.Equal(t, rewardRated, act3)
		//X-shard gas used
		assert.Equal(t, params.GtxxShardCost, shardState1.currentEvmState.GetXShardReceiveGasUsed())
		//Add b2 and make sure all x-shard tx's are added
		b2, err = shardState1.CreateBlockToMine(nil, &acc3, nil, xGas, nil)
		checkErr(err)
		_, _, err = shardState1.FinalizeAndAddBlock(b2)
		checkErr(err)
		//Root block coinbase does not consume xshard gas
		act1 := getDefaultBalance(acc1, *shardState1)
		assert.Equal(t, 10000000+1000000+888888+111111, int(act1.Uint64()))
		//X-shard gas used
		assert.Equal(t, params.GtxxShardCost, shardState1.currentEvmState.GetXShardReceiveGasUsed())
	*/
	//Add 2 txs at once: goquarkchain can only use default xshard gas limits for block to be added.
	b2, err = shardState1.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	_, _, err = shardState1.FinalizeAndAddBlock(b2)
	act1 := getDefaultBalance(acc1, shardState1)
	assert.Equal(t, 10000000+1000000+888888+111111, int(act1.Uint64()))
	cfg := shardState1.clusterConfig.Quarkchain.GetShardConfigByFullShardID(uint32(shardState1.branch.Value))
	reward := params.GtxxShardCost.Uint64()*gasPrice*2 + cfg.CoinbaseAmount.Uint64()
	rewardRated := afterTax(reward, shardState1)
	act3 := getDefaultBalance(acc3, shardState1)
	//Half collected by root
	assert.Equal(t, rewardRated, act3)
	//X-shard gas used
	assert.Equal(t, params.GtxxShardCost.Uint64()*2, shardState1.currentEvmState.GetXShardReceiveGasUsed().Uint64())

	//Add b3 and make sure no x-shard tx's are added
	b3, err := shardState1.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	_, _, err = shardState1.FinalizeAndAddBlock(b3)
	checkErr(err)
	//Root block coinbase does not consume xshard gas
	act1 = getDefaultBalance(acc1, shardState1)
	assert.Equal(t, 10000000+1000000+888888+111111, int(act1.Uint64()))
	assert.Equal(t, 0, int(shardState1.currentEvmState.GetXShardReceiveGasUsed().Uint64()))

	b4, err := shardState1.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	_, _, err = shardState1.FinalizeAndAddBlock(b4)
	checkErr(err)
	//assert.NotEqual(t, b2.GetMetaData().XShardTxCursorInfo, b3.GetMetaData().XShardTxCursorInfo)
	assert.Equal(t, b3.GetMetaData().XShardTxCursorInfo, b4.GetMetaData().XShardTxCursorInfo)
	assert.Equal(t, 0, int(shardState1.currentEvmState.GetXShardReceiveGasUsed().Uint64()))

	gas1 := params.GtxxShardCost
	xGas1 := new(big.Int).SetUint64(2 * params.GtxxShardCost.Uint64())
	b5, err := shardState1.CreateBlockToMine(nil, &acc3, gas1, xGas1, nil)
	checkErr(err)
	_, _, err = shardState1.FinalizeAndAddBlock(b5) //xshard_gas_limit \\d+ should not exceed total gas_limit
	assert.Error(t, err)                            //gasLimit is not match

	b6, err := shardState1.CreateBlockToMine(nil, &acc3, nil, xGas, nil)
	checkErr(err)
	_, _, err = shardState1.FinalizeAndAddBlock(b6)
	assert.Error(t, err, "incorrect xshard gas limit, expected %d, actual %d", 6000000, 9000)
}

func getDefaultBalance(acc account.Address, shardState *MinorBlockChain) *big.Int {
	blc1, err := shardState.GetBalance(acc.Recipient, nil)
	checkErr(err)
	act1 := blc1.GetTokenBalance(qkcCommon.TokenIDEncode(shardState.clusterConfig.Quarkchain.GenesisToken))
	return act1
}

func afterTax(reward uint64, shardState *MinorBlockChain) *big.Int {
	rate := shardState.getLocalFeeRate()
	rewardRated := new(big.Int).Mul(new(big.Int).SetUint64(reward), rate.Num())
	rewardRated = new(big.Int).Div(rewardRated, rate.Denom())
	return rewardRated
}

func TestXShardTxReceivedDDOSFix(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1<<16)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	checkErr(err)

	genesis := uint64(10000000)
	shardSize := uint32(64)
	shardId0 := uint32(0)
	shardId := uint32(16)
	env1 := setUp(&acc1, &genesis, &shardSize)
	state0 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	env2 := setUp(&acc2, &genesis, &shardSize)
	state1 := createDefaultShardState(env2, &shardId, nil, nil, nil)
	defer func() {
		state0.Stop()
		state1.Stop()
	}()

	rootBlock := state0.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(state0.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(state1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = state0.AddRootBlock(rootBlock)
	checkErr(err)
	_, err = state1.AddRootBlock(rootBlock)
	checkErr(err)

	b0, err := state0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = state0.FinalizeAndAddBlock(b0)
	checkErr(err)

	rHash := rootBlock.Hash()
	b1 := state1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil,
		nil, nil, nil, &rHash)

	value1 := new(big.Int).SetUint64(888888)
	gas := params.GtxxShardCost.Uint64() + 21000
	var gasPrice uint64 = 0
	tx := createTransferTransaction(state1, id1.GetKey().Bytes(), acc2, acc1, value1, &gas, &gasPrice, nil, nil, nil, nil)
	b1.AddTx(tx)

	//Add a x-shard tx from remote peer
	crossShardGas := new(serialize.Uint256)
	intrinsic := params.GtxxShardCost.Uint64() + 21000
	crossShardGas.Value = new(big.Int).SetUint64(gas - intrinsic)
	state0.AddCrossShardTxListByMinorBlockHash(b1.Hash(), types.CrossShardTransactionDepositList{
		TXList: []*types.CrossShardTransactionDeposit{
			{
				TxHash:          tx.Hash(),
				From:            acc2,
				To:              acc1,
				Value:           &serialize.Uint256{Value: value1},
				GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(gasPrice)},
				GasRemained:     crossShardGas,
				TransferTokenID: tx.EvmTx.TransferTokenID(),
				GasTokenID:      tx.EvmTx.GasTokenID(),
			},
		}})

	//Create a root block containing the block with the x-shard tx
	rootBlock = state0.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	//rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = state0.AddRootBlock(rootBlock)
	checkErr(err)
	//Add b0 and make sure all x-shard tx's are adde
	b2, err := state0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b2, _, err = state0.FinalizeAndAddBlock(b2)
	checkErr(err)

	blc1 := getDefaultBalance(acc1, state0)
	assert.Equal(t, int(genesis+value1.Uint64()), int(blc1.Uint64()))
	//Half collected by root
	blc3 := getDefaultBalance(acc3, state0)
	cfg := state0.clusterConfig.Quarkchain.GetShardConfigByFullShardID(uint32(state0.branch.Value))
	assert.Equal(t, afterTax(cfg.CoinbaseAmount.Uint64(), state0), blc3)

	//X-shard gas used (to be fixed)
	assert.Equal(t, uint64(0), state0.currentEvmState.GetXShardReceiveGasUsed().Uint64())
	assert.Equal(t, uint64(0), b2.GetMetaData().GasUsed.Value.Uint64())
	assert.Equal(t, uint64(0), b2.GetMetaData().CrossShardGasUsed.Value.Uint64())

	//Apply the fix
	b2s, err := serialize.SerializeToBytes(b2)
	var b3 types.MinorBlock
	err = serialize.DeserializeFromBytes(b2s, &b3)
	checkErr(err)
	state0.Config().XShardGasDDOSFixRootHeight = 0
	bb3, _, err := state0.FinalizeAndAddBlock(&b3)
	checkErr(err)
	assert.Equal(t, params.GtxxShardCost, bb3.GetMetaData().GasUsed.Value)
	assert.Equal(t, params.GtxxShardCost, bb3.GetMetaData().CrossShardGasUsed.Value)
}

func TestXShardFromRootBlock(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	acc2 := account.CreatAddressFromIdentity(id2, 0)

	genesis := uint64(10000000)
	shardSize := uint32(2)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, &genesis, &shardSize)
	state0 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer state0.Stop()

	//Create a root block containing the block with the x-shard tx
	rootBlock := state0.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(state0.CurrentBlock().Header())
	coinbase := types.NewEmptyTokenBalances()
	coinbase.SetValue(big.NewInt(1000000), qkcCommon.TokenIDEncode(env1.clusterConfig.Quarkchain.GenesisToken))
	rootBlock.Finalize(coinbase, &acc2, common.Hash{})
	_, err = state0.AddRootBlock(rootBlock)
	checkErr(err)

	b0, err := state0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = state0.FinalizeAndAddBlock(b0)
	checkErr(err)
	assert.Equal(t, 1000000, int(getDefaultBalance(acc2, state0).Uint64()))
}

//separate from test_xshard_from_root_block()
func TestXShardFromRootBlockWithCoinbaseIsCode(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)

	genesis := uint64(10000000)
	shardSize := uint32(2)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, &genesis, &shardSize)
	state0 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer state0.Stop()

	olderHeaderTip := state0.CurrentBlock().Header()
	tx, err := CreateContract(state0, id1.GetKey(), acc1, 0, ContractCreationByteCode)
	checkErr(err)
	assert.NoError(t, state0.AddTx(tx))
	b, err := state0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	_, _, err = state0.FinalizeAndAddBlock(b)
	checkErr(err)
	_, _, receipt := state0.GetTransactionReceipt(tx.Hash())
	assert.NotNil(t, receipt.ContractAddress)
	contractAddress := account.NewAddress(receipt.ContractAddress, receipt.ContractFullShardKey)
	//Create a root block containing the block with the x-shard tx
	rootBlock := state0.GetRootTip().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(olderHeaderTip)
	rootBlock.AddMinorBlockHeader(state0.CurrentBlock().Header())
	coinbase := types.NewEmptyTokenBalances()
	coinbase.SetValue(big.NewInt(1000000), qkcCommon.TokenIDEncode(env1.clusterConfig.Quarkchain.GenesisToken))
	rootBlock = rootBlock.Finalize(coinbase, &contractAddress, common.Hash{})
	_, err = state0.AddRootBlock(rootBlock)
	checkErr(err)

	b0, err := state0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = state0.FinalizeAndAddBlock(b0)
	checkErr(err)
	assert.Equal(t, 1000000, int(getDefaultBalance(contractAddress, state0).Uint64()))
}

//goquarkchain only
func TestTransferToContract(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	genesis := uint64(10000000)
	shardSize := uint32(2)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, &genesis, &shardSize)
	state0 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer state0.Stop()
	//need a payable function in contract to receive money
	tx, err := CreateContract(state0, id1.GetKey(), acc1, 0, ContractCreationByteCodePayable)
	checkErr(err)
	assert.NoError(t, state0.AddTx(tx))
	b, err := state0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	_, _, err = state0.FinalizeAndAddBlock(b)
	checkErr(err)
	_, _, receipt := state0.GetTransactionReceipt(tx.Hash())
	assert.NotNil(t, receipt.ContractAddress)
	contractAddress := account.NewAddress(receipt.ContractAddress, receipt.ContractFullShardKey)
	value := new(big.Int).SetUint64(1000000)
	gas := uint64(21000 + 100000) //need extra gas to run contract code
	tx0 := CreateTransferTx(state0, id1.GetKey().Bytes(), acc1, contractAddress, value, &gas, nil, nil)
	assert.NoError(t, state0.AddTx(tx0))
	checkErr(err)
	b0, err := state0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = state0.FinalizeAndAddBlock(b0)
	checkErr(err)
	assert.Equal(t, 1000000, int(getDefaultBalance(contractAddress, state0).Uint64()))
}

func TestGetTxForJsonRpc(t *testing.T) {
	dirname, err := ioutil.TempDir(os.TempDir(), "qkcdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	testDBPath[1] = dirname

	dirname1, err1 := ioutil.TempDir(os.TempDir(), "qkcdb_test_")
	if err1 != nil {
		panic("failed to create test file: " + err.Error())
	}
	testDBPath[2] = dirname1
	defer func() {
		os.RemoveAll(dirname)
		os.RemoveAll(dirname1)
	}()
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
	rootBlock.AddMinorBlockHeader(shardState0.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err0 := shardState0.AddRootBlock(rootBlock)
	checkErr(err0)

	_, err1 = shardState1.AddRootBlock(rootBlock)
	checkErr(err1)

	// Add one block in shard 0
	b0, err := shardState0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b1Headaer := b1.Header()
	b1Headaer.PrevRootBlockHash = rootBlock.Header().Hash()
	b1 = types.NewMinorBlock(b1Headaer, b1.Meta(), b1.Transactions(), nil, nil)
	fakeGas := uint64(30000)
	fakeGasPrice := uint64(2)
	value := new(big.Int).SetUint64(888888)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, value, &fakeGas, &fakeGasPrice, nil, nil, nil, nil)
	b1.AddTx(tx)
	txList := types.CrossShardTransactionDepositList{}
	crossShardGas := new(serialize.Uint256)
	intrinsic := uint64(21000) + params.GtxxShardCost.Uint64()
	crossShardGas.Value = new(big.Int).SetUint64(tx.EvmTx.Gas() - intrinsic)
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		TxHash:          tx.Hash(),
		From:            acc2,
		To:              acc1,
		Value:           &serialize.Uint256{Value: value},
		GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(fakeGasPrice)},
		GasRemained:     crossShardGas,
		TransferTokenID: tx.EvmTx.TransferTokenID(),
		GasTokenID:      tx.EvmTx.GasTokenID(),
	})
	// Add a x-shard tx from remote peer
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList) // write db
	// Create a root block containing the block with the x-shard tx
	rootBlock = shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	// Add b0 and make sure all x-shard tx's are added
	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)
	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient, shardState0.GetGenesisToken())
	assert.Equal(t, int64(10000000+888888), acc1Value.Int64())

	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient, shardState0.GetGenesisToken())
	acc3Should := new(big.Int).Add(testShardCoinbaseAmount, new(big.Int).SetUint64(9000*2))
	acc3Should = new(big.Int).Div(acc3Should, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3Should.String(), acc3Value.String())

	// X-shard gas used
	assert.Equal(t, uint64(9000), shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64())

	b1, _, err = shardState1.FinalizeAndAddBlock(b1)
	checkErr(err)

	//getTxByHash
	tBlock, index := shardState0.GetTransactionByHash(tx.Hash())
	assert.Equal(t, tBlock.Number(), uint64(2))
	assert.Equal(t, len(tBlock.Transactions()), 0)
	assert.Equal(t, index, uint32(1))
	tBlock, index = shardState1.GetTransactionByHash(tx.Hash())
	assert.Equal(t, tBlock.Number(), uint64(1))
	assert.Equal(t, len(tBlock.Transactions()), 1)
	assert.Equal(t, index, uint32(0))

	//getReceiptByHash
	tBlock, index, receipt := shardState0.GetTransactionReceipt(tx.Hash())
	assert.Equal(t, tBlock.Number(), uint64(2))
	assert.Equal(t, len(tBlock.Transactions()), 0)
	assert.Equal(t, index, uint32(1))
	assert.Equal(t, receipt.GasUsed, uint64(9000))
	tBlock, index, receipt = shardState1.GetTransactionReceipt(tx.Hash())
	assert.Equal(t, tBlock.Number(), uint64(1))
	assert.Equal(t, len(tBlock.Transactions()), 1)
	assert.Equal(t, index, uint32(0))
	assert.Equal(t, receipt.GasUsed, uint64(30000))

	tGenesisTokenID := testGenesisTokenID
	//getTxByAddress
	tTxList, _, err := shardState0.GetTransactionByAddress(acc1, &tGenesisTokenID, []byte{}, 20)
	assert.Equal(t, len(tTxList), 1)
	assert.Equal(t, tTxList[0].TxHash, tx.Hash())

	tTxList, _, err = shardState0.GetTransactionByAddress(acc2, &tGenesisTokenID, []byte{}, 20)
	assert.Equal(t, len(tTxList), 1)
	assert.Equal(t, tTxList[0].TxHash, tx.Hash())

	tTxList, _, err = shardState1.GetTransactionByAddress(acc1, &tGenesisTokenID, []byte{}, 20)
	assert.Equal(t, len(tTxList), 1)
	assert.Equal(t, tTxList[0].TxHash, tx.Hash())

	tTxList, _, err = shardState1.GetTransactionByAddress(acc2, &tGenesisTokenID, []byte{}, 20)
	assert.Equal(t, len(tTxList), 1)
	assert.Equal(t, tTxList[0].TxHash, tx.Hash())

	// getAllTx
	tTxList, _, err = shardState0.GetAllTx([]byte{}, 20)
	assert.Equal(t, len(tTxList), 1)
	assert.Equal(t, tTxList[0].TxHash, tx.Hash())

	tTxList, _, err = shardState1.GetAllTx([]byte{}, 20)
	assert.Equal(t, len(tTxList), 1)
	assert.Equal(t, tTxList[0].TxHash, tx.Hash())

	// already compare next with py
	// account is random and key is not match,so not use assert.Equal there
	// but next returns the same except for address and key
}
func TestReorg(t *testing.T) {
	/*
		r0 -> rs1
		r0 -> rr1 -> rr2
	*/

	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)

	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()

	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	r0 := shardState.CurrentBlock()

	rs1 := r0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rs1, _, err = shardState.FinalizeAndAddBlock(rs1)
	assert.NoError(t, err)
	assert.Equal(t, shardState.CurrentBlock(), rs1) //only rs1

	rr1 := r0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	tHeader := rr1.Header()
	tHeader.SetCoinbase(acc3)
	rr1 = types.NewMinorBlock(tHeader, rr1.Meta(), rr1.Transactions(), nil, nil)
	rr1, _, err = shardState.FinalizeAndAddBlock(rr1)
	assert.NoError(t, err)

	assert.Equal(t, shardState.CurrentBlock(), rs1) //rs1's height==rr1's height so rs1

	rr2 := rr1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	rr2, _, err = shardState.FinalizeAndAddBlock(rr2)
	assert.NoError(t, err)
	assert.Equal(t, shardState.CurrentBlock(), rr2) //rr2's height>rs1.height so rr2

	assert.Equal(t, shardState.GetBlockByNumber(1).Hash(), rr1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(2).Hash(), rr2.Hash())

	err = shardState.reorg(shardState.CurrentBlock(), rs1) //rr2->rs1 so rs1
	assert.NoError(t, err)
	assert.Equal(t, shardState.CurrentBlock().Hash(), rs1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(1).Hash(), rs1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(2).Hash(), rr2.Hash())

	err = shardState.reorg(shardState.CurrentBlock(), rr1) //rr2->rr1 so rr1
	assert.NoError(t, err)
	assert.Equal(t, shardState.CurrentBlock().Hash(), rr1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(1).Hash(), rr1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(2).Hash(), rr2.Hash())

	err = shardState.reorg(shardState.CurrentBlock(), rr2) //rr2->rr2 so rr2
	assert.NoError(t, err)
	assert.Equal(t, shardState.CurrentBlock().Hash(), rr2.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(1).Hash(), rr1.Hash())
	assert.Equal(t, shardState.GetBlockByNumber(2).Hash(), rr2.Hash())

}

func TestGetRootChainStakes(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	contractCode := common.Hex2Bytes(`60806040526004361061007b5760003560e01c8063853828b61161004e578063853828b6146101b5578063a69df4b5146101ca578063f83d08ba146101df578063fd8c4646146101e75761007b565b806316934fc4146100d85780632e1a7d4d1461013c578063485d3834146101685780636c19e7831461018f575b336000908152602081905260409020805460ff16156100cb5760405162461bcd60e51b815260040180806020018281038252602681526020018061062e6026913960400191505060405180910390fd5b6100d5813461023b565b50005b3480156100e457600080fd5b5061010b600480360360208110156100fb57600080fd5b50356001600160a01b031661029b565b6040805194151585526020850193909352838301919091526001600160a01b03166060830152519081900360800190f35b34801561014857600080fd5b506101666004803603602081101561015f57600080fd5b50356102cf565b005b34801561017457600080fd5b5061017d61034a565b60408051918252519081900360200190f35b610166600480360360208110156101a557600080fd5b50356001600160a01b0316610351565b3480156101c157600080fd5b506101666103c8565b3480156101d657600080fd5b50610166610436565b6101666104f7565b3480156101f357600080fd5b5061021a6004803603602081101561020a57600080fd5b50356001600160a01b0316610558565b604080519283526001600160a01b0390911660208301528051918290030190f35b8015610297576002820154808201908111610291576040805162461bcd60e51b81526020600482015260116024820152706164646974696f6e206f766572666c6f7760781b604482015290519081900360640190fd5b60028301555b5050565b600060208190529081526040902080546001820154600283015460039093015460ff9092169290916001600160a01b031684565b336000908152602081905260409020805460ff1680156102f3575080600101544210155b6102fc57600080fd5b806002015482111561030d57600080fd5b6002810180548390039055604051339083156108fc029084906000818181858888f19350505050158015610345573d6000803e3d6000fd5b505050565b6203f48081565b336000908152602081905260409020805460ff16156103a15760405162461bcd60e51b81526004018080602001828103825260268152602001806106546026913960400191505060405180910390fd5b6003810180546001600160a01b0319166001600160a01b038416179055610297813461023b565b6103d06105fa565b5033600090815260208181526040918290208251608081018452815460ff16151581526001820154928101929092526002810154928201839052600301546001600160a01b031660608201529061042657600080fd5b61043381604001516102cf565b50565b336000908152602081905260409020805460ff16156104865760405162461bcd60e51b815260040180806020018281038252602b8152602001806106a1602b913960400191505060405180910390fd5b60008160020154116104df576040805162461bcd60e51b815260206004820152601b60248201527f73686f756c642068617665206578697374696e67207374616b65730000000000604482015290519081900360640190fd5b805460ff191660019081178255426203f48001910155565b336000908152602081905260409020805460ff166105465760405162461bcd60e51b815260040180806020018281038252602781526020018061067a6027913960400191505060405180910390fd5b805460ff19168155610433813461023b565b6000806105636105fa565b506001600160a01b03808416600090815260208181526040918290208251608081018452815460ff161580158252600183015493820193909352600282015493810193909352600301549092166060820152906105c75750600091508190506105f5565b60608101516000906001600160a01b03166105e35750836105ea565b5060608101515b604090910151925090505b915091565b6040518060800160405280600015158152602001600081526020016000815260200160006001600160a01b03168152509056fe73686f756c64206f6e6c7920616464207374616b657320696e206c6f636b656420737461746573686f756c64206f6e6c7920736574207369676e657220696e206c6f636b656420737461746573686f756c64206e6f74206c6f636b20616c72656164792d6c6f636b6564206163636f756e747373686f756c64206e6f7420756e6c6f636b20616c72656164792d756e6c6f636b6564206163636f756e7473a265627a7a72315820f2c044ad50ee08e7e49c575b49e8de27cac8322afdb97780b779aa1af44e40d364736f6c634300050b0032`)
	contractAddr := vm.SystemContracts[vm.ROOT_CHAIN_POSW].Address()

	env := &fakeEnv{
		db:            ethdb.NewMemDatabase(),
		clusterConfig: config.NewClusterConfig(),
	}

	env.clusterConfig.Quarkchain.NetworkID = 3
	var chainSize, shardSize uint32 = 2, 1
	env.clusterConfig.Quarkchain.RootChainPoSWContractBytecodeHash = crypto.Keccak256Hash(contractCode)
	env.clusterConfig.Quarkchain.Update(chainSize, shardSize, 10, 1)
	env.clusterConfig.Quarkchain.MinMiningGasPrice = new(big.Int).SetInt64(0)
	env.clusterConfig.Quarkchain.EnableEvmTimeStamp = 1
	env.clusterConfig.Quarkchain.MinTXPoolGasPrice = new(big.Int).SetInt64(0)
	shardConfig := env.clusterConfig.Quarkchain.GetShardConfigByFullShardID(1)
	balance := map[string]*big.Int{env.clusterConfig.Quarkchain.GenesisToken: big.NewInt(10000000)}
	shardConfig.Genesis.Alloc = map[account.Address]config.Allocation{
		account.Address{Recipient: contractAddr, FullShardKey: 0}: {
			Code: contractCode,
		},
		account.Address{Recipient: acc1.Recipient, FullShardKey: 0}: {
			Balances: balance,
		},
	}
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()

	//contract deployed, but no stakes. signer defaults to the recipient
	stakes, signer, err := shardState.GetRootChainStakes(acc1.Recipient, shardState.CurrentHeader().Hash())
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), stakes.Uint64())
	assert.Equal(t, acc1.Recipient, *signer)
	gas := uint64(1000000)
	zero := uint64(0)
	txGen := func(nonce, value *uint64, dat string) *types.Transaction {
		data, _ := hex.DecodeString(dat)
		token := shardState.GetGenesisToken()
		return createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, account.Address{Recipient: contractAddr, FullShardKey: 0},
			new(big.Int).SetUint64(*value), &gas, &zero, nonce, data, &token, &token)
	}

	addStake := func(n, v *uint64) *types.Transaction {
		return txGen(n, v, "")
	}
	setSigner := func(n, v *uint64, a account.Recipient) *types.Transaction {
		return txGen(n, v, "6c19e783000000000000000000000000"+hex.EncodeToString(a[:]))
	}
	withdraw := func(n, v *uint64) *types.Transaction {
		return txGen(n, v, "853828b6")
	}
	unlock := func(n *uint64) *types.Transaction {
		return txGen(n, &zero, "a69df4b5")
	}
	lock := func(n, v *uint64) *types.Transaction {
		return txGen(n, v, "f83d08ba")
	}
	applyTx := func(tx *types.Transaction, timestamp *uint64) bool {
		err := shardState.AddTx(tx)
		assert.NoError(t, err)
		block, err := shardState.CreateBlockToMine(timestamp, nil, nil, nil, nil)
		assert.NoError(t, err)
		_, receipts, err := shardState.FinalizeAndAddBlock(block)
		assert.NoError(t, err)
		for _, r := range receipts {
			if r.Status != uint64(1) {
				return false
			}
		}
		return true
	}
	nonce := uint64(0)
	value := uint64(1234)
	//add stakes and set signer
	tme := shardState.CurrentHeader().GetTime() + 1
	tx0 := addStake(&nonce, &value)
	assert.True(t, applyTx(tx0, &tme))
	randSigner, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	nonce += 1
	value = uint64(4321)
	tx1 := setSigner(&nonce, &value, randSigner.GetRecipient())
	tme += 1
	assert.True(t, applyTx(tx1, &tme))

	stakes, signer, err = shardState.GetRootChainStakes(acc1.Recipient, shardState.CurrentHeader().Hash())
	assert.NoError(t, err)
	assert.Equal(t, uint64(1234+4321), stakes.Uint64())
	assert.Equal(t, randSigner.GetRecipient(), *signer)

	// can't withdraw during locking
	nonce += 1
	tx2 := withdraw(&nonce, &zero)
	tme += 1
	assert.False(t, applyTx(tx2, &tme))

	//unlock should succeed
	nonce += 1
	tx3 := unlock(&nonce)
	tme += 1
	assert.True(t, applyTx(tx3, &tme))
	//but still can't withdraw
	nonce += 1
	tx4 := withdraw(&nonce, &zero)
	tme += 1
	assert.False(t, applyTx(tx4, &tme))
	//and can't add stakes or set signer either
	nonce += 1
	value = uint64(100)
	tx5 := addStake(&nonce, &value)
	tme += 1
	assert.False(t, applyTx(tx5, &tme))
	nonce += 1
	tx6 := setSigner(&nonce, &zero, acc1.Recipient)
	tme += 1
	assert.False(t, applyTx(tx6, &tme))

	//now stakes should be 0 when unlocked
	//if (stake.unlocked) {
	//   return (0, address(0));
	// }
	stakes, signer, err = shardState.GetRootChainStakes(acc1.Recipient, shardState.CurrentHeader().Hash())
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), stakes.Uint64())
	assert.Equal(t, common.Address{}, *signer)
	//4 days passed, should be able to withdraw
	balanceBefore := shardState.currentEvmState.GetBalance(acc1.Recipient, shardState.GetGenesisToken())
	assert.Equal(t, 10000000-1234-4321, int(balanceBefore.Uint64()))
	nonce += 1
	value = uint64(0)
	tx7 := withdraw(&nonce, &value)
	tme += 3600 * 24 * 4
	assert.True(t, applyTx(tx7, &tme))
	balanceAfter := shardState.currentEvmState.GetBalance(acc1.Recipient, shardState.GetGenesisToken())
	assert.Equal(t, 10000000, int(balanceAfter.Uint64()))
	// "should not unlock already-unlocked accounts"
	nonce += 1
	tx8 := unlock(&nonce)
	tme += 1
	assert.False(t, applyTx(tx8, &tme))
	//lock again
	nonce += 1
	value = uint64(42)
	tx9 := lock(&nonce, &value)
	tme += 1
	assert.True(t, applyTx(tx9, &tme))
	balanceAfter = shardState.currentEvmState.GetBalance(acc1.Recipient, shardState.GetGenesisToken())
	assert.Equal(t, 10000000-42, int(balanceAfter.Uint64()))
	//should be able to get stakes
	stakes, signer, err = shardState.GetRootChainStakes(acc1.Recipient, shardState.CurrentHeader().Hash())
	assert.NoError(t, err)
	assert.Equal(t, 42, int(stakes.Uint64()))
	assert.Equal(t, randSigner.GetRecipient(), *signer)
}
