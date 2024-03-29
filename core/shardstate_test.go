package core

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io/ioutil"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/state"
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
	if shardState.rootTip.NumberU64() != 0 {
		t.Errorf("rootTip number mismatch have:%v want:0", shardState.rootTip.NumberU64())
	}
	if shardState.CurrentBlock().NumberU64() != 0 {
		t.Errorf("minorHeader number mismatch have:%v want:%v", shardState.CurrentBlock().NumberU64(), 0)
	}
	rootBlock := shardState.GetRootBlockByHash(shardState.rootTip.Hash())
	assert.NotNil(t, rootBlock)
	// make sure genesis minor block has the right coinbase after-tax
	assert.NotNil(t, shardState.CurrentBlock().CoinbaseAmount().GetTokenBalance(genesisTokenID), testShardCoinbaseAmount)
}

func TestInitGenesisState(t *testing.T) {
	env := setUp(nil, nil, nil)

	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	genesisHeader := shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader)
	fakeNonce := uint64(1234)
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil)
	rootBlock = modifyNumber(rootBlock, 0)
	rootBlock.Finalize(nil, nil, common.Hash{})

	newGenesisBlock, err := shardState.InitGenesisState(rootBlock)
	checkErr(err)
	assert.NotEqual(t, newGenesisBlock.Hash(), genesisHeader.Hash())
	// header tip is still the old genesis header
	assert.Equal(t, shardState.CurrentBlock().Hash(), genesisHeader.Hash())

	block := newGenesisBlock.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)

	_, _, err = shardState.FinalizeAndAddBlock(block)
	checkErr(err)

	// extending new_genesis_block doesn't change header_tip due to root chain first consensus
	assert.Equal(t, shardState.CurrentBlock().Hash(), genesisHeader.Hash())

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
	assert.Equal(t, currentMinorHeader.Hash(), newGenesisBlock.Hash())
	assert.Equal(t, currentZero.Hash(), newGenesisBlock.Hash())

}

func TestGasPrice(t *testing.T) {
	monkey.Patch(PayNativeTokenAsGas, func(a vm.StateDB, b *ethParams.ChainConfig, c uint64, d uint64, gasPrice *big.Int) (uint8, *big.Int, error) {
		return 100, gasPrice, nil
	})
	monkey.Patch(GetGasUtilityInfo, func(a vm.StateDB, b *ethParams.ChainConfig, c uint64, gasPrice *big.Int) (uint8, *big.Int, error) {
		return 100, gasPrice, nil
	})
	defer monkey.UnpatchAll()
	addContractAddrBalance = true
	defer func() {
		addContractAddrBalance = false
	}()
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
			//checkErr(err)
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
	assert.NoError(t, err)
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)
	txGen := func(data []byte) *types.Transaction {
		return createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(123456), nil, nil, nil, data, nil, nil)
	}
	tx := txGen([]byte{})
	estimate, err := shardState.EstimateGas(tx, &acc1)
	checkErr(err)

	assert.Equal(t, estimate, uint32(21000))

	newTx := txGen([]byte("12123478123412348125936583475758"))
	estimate, err = shardState.EstimateGas(newTx, &acc1)
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})

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
	assert.Equal(t, block.Time(), uint64(0))
	assert.Equal(t, i, uint32(0))

	// tx claims to use more gas than the limit and thus not included
	b1, err := shardState.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(49999), nil, nil)
	checkErr(err)
	assert.Equal(t, b1.NumberU64(), uint64(1))
	assert.Equal(t, len(b1.Transactions()), 0)

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.NumberU64(), uint64(1))

	// Should succeed
	b2, re, err := shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().NumberU64(), uint64(1))
	assert.Equal(t, shardState.CurrentBlock().Hash(), b2.Hash())
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
	assert.Equal(t, curBlock.Hash(), block.Hash())

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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	assert.Equal(t, block.Time(), uint64(0))
	assert.Equal(t, i, uint32(0))
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	// Should succeed
	b1, reps, err := shardState.FinalizeAndAddBlock(b1)

	assert.Equal(t, shardState.CurrentBlock().Hash(), b1.Hash())

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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	assert.Equal(t, shardState.CurrentBlock().Hash(), b1.Hash())
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	assert.Equal(t, shardState.getBlockCountByHeight(1), 1)
	_, _, err = shardState.FinalizeAndAddBlock(b22)
	checkErr(err)
	assert.Equal(t, shardState.getBlockCountByHeight(1), 2)
}

func TestXShardTxSent(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	env.clusterConfig.Quarkchain.EnableEvmTimeStamp = 15695676000
	id := uint32(0)
	shardState := createDefaultShardState(env, &id, nil, nil, nil)
	env1 := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	id = uint32(1)
	shardState1 := createDefaultShardState(env1, &id, nil, nil, nil)

	// Add a root block to update block gas limit so that xshard tx can be included
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	rootBlock := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	b1Headaer.PrevRootBlockHash = rootBlock.Hash()
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
		CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
			TxHash:          tx.Hash(),
			From:            acc2,
			To:              acc1,
			Value:           &serialize.Uint256{Value: value},
			GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(fakeGasPrice)},
			GasRemained:     crossShardGas,
			TransferTokenID: tx.EvmTx.TransferTokenID(),
			GasTokenID:      tx.EvmTx.GasTokenID(),
		},
		RefundRate: 100,
	})
	// Add a x-shard tx from remote peer
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Hash(), txList) // write db
	// Create a root block containing the block with the x-shard tx
	rootBlock = shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	rootBlock := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	rootBlock := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	b1Header.PrevRootBlockHash = rootBlock.Hash()
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
		CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
			TxHash:          tx.Hash(),
			From:            acc2,
			To:              acc1,
			Value:           &serialize.Uint256{Value: new(big.Int).SetUint64(888888)},
			GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(2)},
			GasRemained:     crossShardGas,
			TransferTokenID: tx.EvmTx.TransferTokenID(),
			GasTokenID:      tx.EvmTx.GasTokenID(),
		}, RefundRate: 100,
	})
	// Add a x-shard tx from state1
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Hash(), txList)

	// Create a root block containing the block with the x-shard tx
	rootBlock0 := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	b3Header.PrevRootBlockHash = rootBlock.Hash()
	b3 = types.NewMinorBlock(b3Header, b3.Meta(), b3.Transactions(), nil, nil)

	txList = types.CrossShardTransactionDepositList{}
	txList.TXList = append(txList.TXList, &types.CrossShardTransactionDeposit{
		CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
			TxHash:          common.Hash{},
			From:            acc2,
			To:              acc1,
			Value:           &serialize.Uint256{Value: new(big.Int).SetUint64(385723)},
			GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(3)},
			GasRemained:     crossShardGas,
			TransferTokenID: tx.EvmTx.TransferTokenID(),
			GasTokenID:      tx.EvmTx.GasTokenID(),
		}, RefundRate: 100,
	})
	// Add a x-shard tx from state1
	shardState0.AddCrossShardTxListByMinorBlockHash(b3.Hash(), txList)

	rootBlock1 := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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

	assert.Equal(t, rootBlock1.Hash().String(), b7.PrevRootBlockHash().String())
	b8, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).Mul(new(big.Int).SetUint64(9000), new(big.Int).SetUint64(3)), nil, nil)
	checkErr(err)
	assert.Equal(t, rootBlock1.Hash().String(), b8.PrevRootBlockHash().String())

	// Add b0 and make sure all x-shard tx's are added
	b4, err := shardState0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, b4.PrevRootBlockHash().String(), rootBlock1.Hash().String())
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
	assert.Equal(t, shardState.CurrentBlock().Hash(), b0.Hash())

	// Fork happens, first come first serve
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().Hash(), b0.Hash())

	// Longer fork happens, override existing one
	b2 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	assert.Equal(t, shardState.CurrentBlock().Hash(), b2.Hash())
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
	rootBlock := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis.Header())
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b00 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b00, _, err = shardState0.FinalizeAndAddBlock(b00)
	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b00.Hash().String())
	checkErr(err)

	// Create another fork that is much longer (however not confirmed by root_block)
	b3 := b2.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState0.FinalizeAndAddBlock(b3)
	checkErr(err)
	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b00.Hash().String())
	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)
	assert.Equal(t, true, b4.NumberU64() > b00.NumberU64())
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
	emptyRoot := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	_, err = shardState0.AddRootBlock(emptyRoot)
	checkErr(err)

	rootBlock := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis.Header())
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})

	rootBlock1 := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	assert.Equal(t, currBlock.Hash().String(), b00.Hash().String())
	assert.Equal(t, currBlock2.Hash().String(), b00.Hash().String())
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

	assert.Equal(t, shardState0.CurrentBlock().Hash().String(), b2.Hash().String())
	_, err = shardState0.AddRootBlock(rootBlock2)
	checkErr(err)

	currBlock = shardState0.CurrentBlock()

	assert.Equal(t, currBlock.Hash().String(), b4.Hash().String())
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

	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.ExtendMinorBlockHeaderList(headers, uint64(time.Now().Unix()+9999))
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.Finalize(nil, nil, common.Hash{})

	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b0.Hash().String())
	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	b1 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	fakeNonce := uint64(1)
	b2 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil, nil, nil, nil, nil)
	b2Header := b2.Header()
	b2Header.PrevRootBlockHash = rootBlock.Hash()
	b2 = types.NewMinorBlock(b2Header, b2.Meta(), b2.Transactions(), nil, nil)
	fakeNonce = uint64(2)
	b3 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil, nil, nil, nil, nil)
	b3Header := b3.Header()
	b3 = types.NewMinorBlock(b3Header, b2.Meta(), b2.Transactions(), nil, nil)

	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b1.Hash().String())

	// Fork happens, although they have the same height, b2 survives since it confirms root block
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b2.Hash().String())

	// b3 confirms the same root block as b2, so it will not override b2
	b3, _, err = shardState.FinalizeAndAddBlock(b3)
	checkErr(err)

	assert.Equal(t, shardState.CurrentBlock().Hash().String(), b2.Hash().String())
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
	assert.Equal(t, DBb1.Time(), b11Header.Time)

	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.ExtendMinorBlockHeaderList(blockHeaders[:5], uint64(time.Now().Unix()+9999))
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
	tempBlock := recoveredState.GetMinorBlock(b1.Hash())

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
		rootBlock = shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	temp.SetValue(b1.CoinbaseAmount().GetTokenBalance(genesisTokenID), qkcCommon.TokenIDEncode("QKC"))
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

	r1 := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	r2 := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	r1.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r1.Finalize(nil, nil, common.Hash{})

	_, err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Time() + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil, common.Hash{})
	assert.Equal(t, false, reflect.DeepEqual(r1.Hash().String(), r2.Hash().String()))

	_, err = shardState.AddRootBlock(r2)

	checkErr(err)
	assert.Equal(t, shardState.rootTip.Hash().String(), r1.Hash().String())

	m2 := m1.(*types.MinorBlock).CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
	m2Header := m2.Header()
	m2Header.PrevRootBlockHash = r2.Hash()
	m2 = types.NewMinorBlock(m2Header, m2.Meta(), m2.Transactions(), nil, nil)

	// m2 is added
	m2, _, err = shardState.FinalizeAndAddBlock(m2)
	checkErr(err)

	// but m1 should still be the tip
	assert.Equal(t, shardState.GetMinorBlock(m2.Hash()).Hash().String(), m2.Hash().String())
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), m1.Hash().String())
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

	r1 := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	r2 := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	r1.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r1.Finalize(nil, nil, common.Hash{})

	_, err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Time() + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil, common.Hash{})
	assert.Equal(t, false, reflect.DeepEqual(r1.Hash().String(), r2.Hash().String()))
	_, err = shardState.AddRootBlock(r2)
	checkErr(err)
	assert.Equal(t, shardState.rootTip.Hash().String(), r1.Hash().String())

	m3, err := shardState.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, m3.PrevRootBlockHash().String(), r1.Hash().String())
	m3, _, err = shardState.FinalizeAndAddBlock(m3)
	checkErr(err)

	r3 := r2.Header().CreateBlockToAppend(nil, nil, &acc1, nil, nil)
	r3.AddMinorBlockHeader(m2.Header())
	r3.Finalize(nil, nil, common.Hash{})
	_, err = shardState.AddRootBlock(r3)
	checkErr(err)

	assert.Equal(t, shardState.rootTip.Hash().String(), r3.Hash().String())
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), m2.Hash().String())
	assert.Equal(t, shardState.CurrentBlock().Hash().String(), m2.Hash().String())
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})

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
	assert.Equal(t, block.Time(), uint64(0))
	assert.Equal(t, i, uint32(0))

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.NumberU64(), uint64(1))

	// Should succeed
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().NumberU64(), uint64(1))
	assert.Equal(t, shardState.CurrentBlock().Hash(), b2.Hash())
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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})

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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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

	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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

	rootBlock := shardState1.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState2.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.Hash{})
	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)
	_, err = shardState2.AddRootBlock(rootBlock)
	checkErr(err)

	//Create a root block containing the block with the x-shard tx
	rootBlock = shardState1.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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

	rootBlock := shardState1.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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

	rootBlock := shardState1.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
			{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
				TxHash:          tx0.Hash(),
				From:            acc2,
				To:              acc1,
				Value:           &serialize.Uint256{Value: value1},
				GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(gasPrice)},
				GasRemained:     crossShardGas,
				TransferTokenID: tx0.EvmTx.TransferTokenID(),
				GasTokenID:      tx0.EvmTx.GasTokenID(),
			}, RefundRate: 100,
			},
			{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
				TxHash:          tx1.Hash(),
				From:            acc2,
				To:              acc1,
				Value:           &serialize.Uint256{Value: value2},
				GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(gasPrice)},
				GasRemained:     crossShardGas,
				TransferTokenID: tx1.EvmTx.TransferTokenID(),
				GasTokenID:      tx1.EvmTx.GasTokenID(),
			}, RefundRate: 100},
		}})

	//Create a root block containing the block with the x-shard tx
	rootBlock = shardState1.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	rate := shardState.Config().LocalFeeRate
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

	rootBlock := state0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
			{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
				TxHash:          tx.Hash(),
				From:            acc2,
				To:              acc1,
				Value:           &serialize.Uint256{Value: value1},
				GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(gasPrice)},
				GasRemained:     crossShardGas,
				TransferTokenID: tx.EvmTx.TransferTokenID(),
				GasTokenID:      tx.EvmTx.GasTokenID(),
			}, RefundRate: 100,
			},
		}})

	//Create a root block containing the block with the x-shard tx
	rootBlock = state0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	rootBlock := state0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	rootBlock := state0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	rootBlock := shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	b1Headaer.PrevRootBlockHash = rootBlock.Hash()
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
		CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
			TxHash:          tx.Hash(),
			From:            acc2,
			To:              acc1,
			Value:           &serialize.Uint256{Value: value},
			GasPrice:        &serialize.Uint256{Value: new(big.Int).SetUint64(fakeGasPrice)},
			GasRemained:     crossShardGas,
			TransferTokenID: tx.EvmTx.TransferTokenID(),
			GasTokenID:      tx.EvmTx.GasTokenID(),
		}, RefundRate: 100,
	})
	// Add a x-shard tx from remote peer
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Hash(), txList) // write db
	// Create a root block containing the block with the x-shard tx
	rootBlock = shardState0.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
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
	assert.Equal(t, receipt.GasUsed, uint64(21000))

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
	rootBlock := shardState.rootTip.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
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
	contractAddr := vm.SystemContracts[vm.ROOT_CHAIN_POSW].Address()
	contractCode := common.Hex2Bytes(vm.RootChainPoSWContractBytecode)
	env := &fakeEnv{
		db:            ethdb.NewMemDatabase(),
		clusterConfig: config.NewClusterConfig(),
	}

	runtimeStart := bytes.LastIndex(contractCode, common.Hex2Bytes("608060405260"))
	// # get rid of the constructor argument
	contractCode = contractCode[runtimeStart : len(contractCode)-32]
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
		{Recipient: contractAddr, FullShardKey: 0}: {
			Code: contractCode,
		},
		{Recipient: acc1.Recipient, FullShardKey: 0}: {
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

func TestSigToAddr(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)

	signerId, err := account.CreatRandomIdentity()
	assert.NoError(t, err)

	prvKey, err := crypto.ToECDSA(signerId.GetKey().Bytes())
	assert.NoError(t, err)

	genesis := uint64(10000000)
	shardSize := uint32(2)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, &genesis, &shardSize)
	state0 := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer state0.Stop()

	var rootBlock *types.RootBlock
	var recovered *account.Recipient

	rootBlock = state0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock = rootBlock.Finalize(nil, nil, common.Hash{})
	err = rootBlock.SignWithPrivateKey(prvKey)
	assert.NoError(t, err)
	recovered, err = sigToAddr(rootBlock.Header().SealHash().Bytes(), rootBlock.Header().Signature)
	assert.NoError(t, err)
	assert.Equal(t, signerId.GetRecipient(), *recovered)
	_, err = state0.AddRootBlock(rootBlock)
	assert.NoError(t, err)
}

func TestProcMintMNT(t *testing.T) {
	mintMNTAddr := common.HexToAddress(vm.MintMNTAddr)
	minter := common.HexToAddress(strings.Repeat("00", 19) + "34")
	tokenIDB := common.Hex2Bytes(strings.Repeat("00", 28) + strings.Repeat("11", 4))
	tokenID := new(big.Int).SetBytes(tokenIDB).Uint64()
	amount := common.Hex2Bytes(strings.Repeat("00", 30) + strings.Repeat("22", 2))
	data := common.Hex2Bytes(strings.Repeat("00", 12))
	data = append(data, minter.Bytes()...)
	data = append(data, tokenIDB...)
	data = append(data, amount...)

	runContract := func(codeAddr common.Address, statedb *state.StateDB) (ret []byte, gasRemained uint64, balance *big.Int, err error) {
		contract := vm.NewContract(vm.AccountRef(codeAddr), vm.AccountRef(codeAddr), new(big.Int), 34001)
		evm := vm.NewEVM(vm.Context{}, statedb, &params.DefaultConstantinople, vm.Config{})
		evm.StateDB.SetQuarkChainConfig(config.NewQuarkChainConfig())
		ret, err = vm.RunPrecompiledContract(vm.PrecompiledContractsByzantium[mintMNTAddr], data, contract, evm)
		gasRemained = contract.Gas
		balance = evm.StateDB.GetBalance(minter, tokenID)
		return
	}
	statedb, err := state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	assert.NoError(t, err)
	sysContractAddr := common.HexToAddress(vm.NonReservedNativeTokenContractAddr)
	ret, gasRemained, balance, err := runContract(sysContractAddr, statedb)
	assert.NoError(t, err)
	assert.Equal(t, 34001-34000, int(gasRemained))
	assert.Equal(t, 32, len(ret))
	assert.Equal(t, 1, int(new(big.Int).SetBytes(ret).Uint64()))
	assert.Equal(t, new(big.Int).SetBytes(amount), balance)

	//# Mint again with exactly the same parameters
	ret, gasRemained, balance, err = runContract(sysContractAddr, statedb)
	assert.NoError(t, err)
	assert.Equal(t, 34001-9000, int(gasRemained))
	assert.Equal(t, 32, len(ret))
	assert.Equal(t, 1, int(new(big.Int).SetBytes(ret).Uint64()))
	assert.Equal(t, new(big.Int).Mul(new(big.Int).SetBytes(amount), new(big.Int).SetUint64(2)), balance)

	randomAcc, err := account.CreatRandomAccountWithoutFullShardKey()
	assert.NoError(t, err)
	randomAddr := randomAcc.Recipient
	statedb, err = state.New(common.Hash{}, state.NewDatabase(ethdb.NewMemDatabase()))
	ret, gasRemained, balance, err = runContract(randomAddr, statedb)

	assert.EqualError(t, err, "invalid sender")
	assert.Equal(t, 0, int(gasRemained))
	assert.Equal(t, 0, len(ret))
	assert.Equal(t, new(big.Int), balance)
}

func TestPayNativeTokenAsGasContractAPI(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	genesis := uint64(10000000)
	shardSize := uint32(2)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, &genesis, &shardSize)
	shardState := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer shardState.Stop()

	evmState := shardState.currentEvmState
	evmState.SetQuarkChainConfig(env1.clusterConfig.Quarkchain)
	tokenID := uint64(123)
	//# contract not deployed yet
	refundPercentage, gasPrice, err := GetGasUtilityInfo(evmState, shardState.ethChainConfig, tokenID, new(big.Int).SetUint64(1))
	assert.Equal(t, ErrContractNotFound, err)
	assert.Equal(t, 0, int(refundPercentage))
	assert.Nil(t, gasPrice)
	runtimeBytecode := common.Hex2Bytes(vm.GeneralNativeTokenContractBytecode)
	runtimeStart := bytes.LastIndex(runtimeBytecode, common.Hex2Bytes("608060405260"))
	// # get rid of the constructor argument
	runtimeBytecode = runtimeBytecode[runtimeStart : len(runtimeBytecode)-32]
	contractAddr := vm.SystemContracts[vm.GENERAL_NATIVE_TOKEN].Address()
	evmState.SetCode(contractAddr, runtimeBytecode)
	//# Set caller
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(0)), common.BytesToHash(contractAddr.Bytes()))
	//# Set supervisor
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(1)), common.BytesToHash(acc1.Recipient.Bytes()))
	//# Set min gas reserve for maintenance
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(3)), common.BigToHash(big.NewInt(30000)))
	//# Set min starting gas for use as gas
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(4)), common.BigToHash(big.NewInt(1)))
	_, err = evmState.Commit(true)
	assert.NoError(t, err)

	ctx := vm.Context{
		CanTransfer:                       CanTransfer,
		Transfer:                          Transfer,
		TransferFailureByPoswBalanceCheck: TransferFailureByPoswBalanceCheck,
		TransferTokenID:                   shardState.GetGenesisToken(),
		BlockNumber:                       new(big.Int),
	}
	evm := vm.NewEVM(ctx, evmState, shardState.ethChainConfig, vm.Config{})
	call := func(data string, value *big.Int) ([]byte, error) {
		ret, _, err := evm.Call(vm.AccountRef(acc1.Recipient), contractAddr, common.Hex2Bytes(data), 1000000, value)
		return ret, err
	}
	toStr := func(input uint64) string {
		b := qkcCommon.EncodeToByte32(input)
		return common.Bytes2Hex(b)
	}
	formattedTokenID := toStr(tokenID)
	//# propose a new exchange rate for token id 123 with ratio 1 / 30000, which will fail because no registration
	supply := int64(100000)
	_, err = call("735e0e19"+formattedTokenID+toStr(1)+toStr(30000), big.NewInt(supply))
	assert.Error(t, err)
	// # register and re-propose, should succeed
	tk := evm.TransferTokenID
	evm.TransferTokenID = tokenID
	evm.StateDB.AddBalance(acc1.Recipient, new(big.Int).SetUint64(1), tokenID)
	_, err = call("bf03314a", new(big.Int).SetUint64(1))
	assert.NoError(t, err)
	evm.TransferTokenID = tk
	_, err = call("735e0e19"+formattedTokenID+toStr(1)+toStr(30000), big.NewInt(supply))
	assert.NoError(t, err)
	//# set the refund rate to 60
	_, err = call("6d27af8c"+formattedTokenID+toStr(60), new(big.Int))
	assert.NoError(t, err)

	//# get the gas utility information by calling the get_gas_utility_info function
	gasPriceInNativeToken := big.NewInt(60000)
	refundPercentage, gasPrice, err = GetGasUtilityInfo(evmState, shardState.ethChainConfig, tokenID, gasPriceInNativeToken)
	assert.Equal(t, uint8(60), refundPercentage)
	assert.Equal(t, big.NewInt(2), gasPrice)

	data, err := ConvertToDefaultChainTokenGasPrice(evmState, shardState.ethChainConfig, tokenID, new(big.Int).SetInt64(60000))
	assert.NoError(t, err)
	assert.Equal(t, data.Uint64(), uint64(2))

	//# exchange the Qkc with the native token
	refundPercentage, gasPrice, err = PayNativeTokenAsGas(evmState, shardState.ethChainConfig, tokenID, 3, gasPriceInNativeToken)
	assert.Equal(t, uint8(60), refundPercentage)
	assert.Equal(t, big.NewInt(2), gasPrice)
	// # check the balance of the gas reserve. amount of native token (60000) * exchange rate (1 / 30000) = 2 QKC
	ret, err := call("13dee215"+formattedTokenID+strings.Repeat("0", 24)+common.Bytes2Hex(acc1.Recipient.Bytes()), new(big.Int))
	assert.NoError(t, err)
	assert.Equal(t, uint64(supply-3*2), new(big.Int).SetBytes(ret).Uint64())
	// # check the balance of native token.
	ret, err = call("21a2b36e"+formattedTokenID+strings.Repeat("0", 24)+common.Bytes2Hex(acc1.Recipient.Bytes()), new(big.Int))
	assert.NoError(t, err)
	gasPriceInNativeTokenMul3Add1 := new(big.Int).Add(new(big.Int).Mul(gasPriceInNativeToken, new(big.Int).SetUint64(3)), new(big.Int).SetUint64(1))
	assert.Equal(t, gasPriceInNativeTokenMul3Add1, new(big.Int).SetBytes(ret))
	//# give the contract real native token and withdrawing should work
	evm.StateDB.AddBalance(contractAddr, new(big.Int).Mul(gasPriceInNativeToken, new(big.Int).SetUint64(3)), tokenID)
	//withdraw_native_token
	_, err = call("f9c94eb7"+formattedTokenID, new(big.Int))
	assert.NoError(t, err)
	suBalance := evm.StateDB.GetBalance(acc1.Recipient, tokenID)
	assert.Equal(t, gasPriceInNativeTokenMul3Add1, suBalance)
	coBalance := evm.StateDB.GetBalance(contractAddr, tokenID)
	assert.Equal(t, 0, int(coBalance.Uint64()))
	//# check again the balance of native token.
	ret, err = call("21a2b36e"+formattedTokenID+strings.Repeat("0", 24)+common.Bytes2Hex(acc1.Recipient.Bytes()), new(big.Int))
	assert.NoError(t, err)
	assert.Equal(t, 0, int(new(big.Int).SetBytes(ret).Uint64()))
}

func TestPayNativeTokenAsGasEndToEnd(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	shardSize := uint32(2)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, nil, &shardSize)
	ids := env1.clusterConfig.Quarkchain.GetGenesisShardIds()
	genesisBalance := new(big.Int).Mul(big.NewInt(100), config.QuarkashToJiaozi)
	for _, v := range ids {
		shardConfig := env1.clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		balance := make(map[string]*big.Int)
		balance[env1.clusterConfig.Quarkchain.GenesisToken] = genesisBalance
		balance["QI"] = genesisBalance
		alloc := config.Allocation{Balances: balance}
		adalloc := make(map[account.Address]config.Allocation)
		addr := acc1.AddressInShard(v)
		adalloc[addr] = alloc
		shardConfig.Genesis.Alloc = adalloc
	}
	shardState := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer shardState.Stop()

	evmState := shardState.currentEvmState
	evmState.SetQuarkChainConfig(env1.clusterConfig.Quarkchain)
	tokenID := qkcCommon.TokenIDEncode("QI")
	runtimeBytecode := common.Hex2Bytes(vm.GeneralNativeTokenContractBytecode)
	runtimeStart := bytes.LastIndex(runtimeBytecode, common.Hex2Bytes("608060405260"))
	// # get rid of the constructor argument
	runtimeBytecode = runtimeBytecode[runtimeStart : len(runtimeBytecode)-32]
	contractAddr := vm.SystemContracts[vm.GENERAL_NATIVE_TOKEN].Address()
	evmState.SetCode(contractAddr, runtimeBytecode)
	//# Set caller
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(0)), common.BytesToHash(contractAddr.Bytes()))
	//# Set supervisor
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(1)), common.BytesToHash(acc1.Recipient.Bytes()))
	//# Set min gas reserve for maintenance
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(3)), common.BigToHash(big.NewInt(30000)))
	//# Set min starting gas for use as gas
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(4)), common.BigToHash(big.NewInt(1)))
	_, err = evmState.Commit(true)
	assert.NoError(t, err)

	ctx := vm.Context{
		CanTransfer:                       CanTransfer,
		Transfer:                          Transfer,
		TransferFailureByPoswBalanceCheck: TransferFailureByPoswBalanceCheck,
		TransferTokenID:                   shardState.GetGenesisToken(),
		BlockNumber:                       new(big.Int),
	}
	evm := vm.NewEVM(ctx, evmState, shardState.ethChainConfig, vm.Config{})
	call := func(data string, value *big.Int) ([]byte, error) {
		ret, _, err := evm.Call(vm.AccountRef(acc1.Recipient), contractAddr, common.Hex2Bytes(data), 1000000, value)
		evmState.SubRefund(evmState.GetRefund())
		return ret, err
	}
	toStr := func(input uint64) string {
		b := qkcCommon.EncodeToByte32(input)
		return common.Bytes2Hex(b)
	}
	formattedTokenID := toStr(tokenID)
	//unrequire_registered_token
	_, err = call("764a27ef"+toStr(0), new(big.Int))
	assert.NoError(t, err)
	//# propose a new exchange rate (1/2) with 1 ether of QKC as reserve
	supply := config.QuarkashToJiaozi
	_, err = call("735e0e19"+formattedTokenID+toStr(1)+toStr(2), supply)
	assert.NoError(t, err)
	//# set the refund rate to 80
	_, err = call("6d27af8c"+formattedTokenID+toStr(80), new(big.Int))
	assert.NoError(t, err)

	//# 1) craft a tx using native token for gas, with gas price as 10
	gas := uint64(1000000)
	gasPrice := uint64(10)
	nonce := uint64(0)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc1, new(big.Int), &gas, &gasPrice, &nonce,
		nil, &tokenID, &genesisTokenID)
	accCoinbase, err := account.CreatRandomAccountWithFullShardKey(0)
	assert.NoError(t, err)
	block, err := shardState.CreateBlockToMine(nil, &accCoinbase, nil, nil, nil)
	assert.NoError(t, err)
	_, _, err = shardState.FinalizeAndAddBlock(block)
	assert.NoError(t, err)
	_, receipt, _, err := ApplyTransaction(shardState.ethChainConfig, shardState, new(GasPool).AddGas(shardState.GasLimit()),
		evmState, shardState.CurrentHeader(), tx, new(uint64), *shardState.GetVMConfig())
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), receipt.Status)
	//# native token balance should update accordingly
	b := evmState.GetBalance(acc1.Recipient, tokenID)
	assert.Equal(t, new(big.Int).Sub(genesisBalance, new(big.Int).SetUint64(gas*gasPrice)), b)
	b = evmState.GetBalance(contractAddr, tokenID)
	assert.Equal(t, new(big.Int).SetUint64(gas*gasPrice), b)
	//query_native_token_balance
	ret, err := call("21a2b36e"+formattedTokenID+strings.Repeat("0", 24)+common.Bytes2Hex(acc1.Recipient.Bytes()), new(big.Int))
	assert.NoError(t, err)
	assert.Equal(t, new(big.Int).SetUint64(gas*gasPrice), new(big.Int).SetBytes(ret))
	//# qkc balance should update accordingly:
	//# should have 100 ether - 1 ether + refund
	senderBalance := new(big.Int).Add(new(big.Int).Sub(genesisBalance, supply), new(big.Int).SetUint64((gas-21000)*(gasPrice/2)*8/10))
	assert.Equal(t, senderBalance, evmState.GetBalance(acc1.Recipient, genesisTokenID))
	contractRemainingQKC := new(big.Int).Sub(supply, new(big.Int).SetUint64(gas*(gasPrice/2)))
	assert.Equal(t, contractRemainingQKC, evmState.GetBalance(contractAddr, genesisTokenID))
	//query_gas_reserve_balance
	ret, err = call("13dee215"+formattedTokenID+strings.Repeat("0", 24)+common.Bytes2Hex(acc1.Recipient.Bytes()), new(big.Int))
	assert.NoError(t, err)
	assert.Equal(t, contractRemainingQKC, new(big.Int).SetBytes(ret))
	//# burned QKC for gas conversion
	assert.Equal(t, new(big.Int).SetUint64((gas-21000)*(gasPrice/2)*2/10), evmState.GetBalance(common.BytesToAddress([]byte{0}), genesisTokenID))
	//# miner fee with 50% tax
	assert.Equal(t, new(big.Int).SetUint64(21000*(gasPrice/2)/2), evmState.GetBalance(accCoinbase.Recipient, genesisTokenID))
	//# 2) craft a tx that will use up gas reserve, should fail validation
	gasPrice = 2000000000000
	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc1, new(big.Int), &gas, &gasPrice, &nonce, nil, &tokenID, &genesisTokenID)
	assert.Error(t, ValidateTransaction(evmState, shardState.ethChainConfig, tx, &acc1))
}

func TestMintNewNativeToken(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	shardSize := uint32(2)
	shardId0 := uint32(0)
	env1 := setUp(&acc1, nil, &shardSize)
	ids := env1.clusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		shardConfig := env1.clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		balance := make(map[string]*big.Int)
		balance[env1.clusterConfig.Quarkchain.GenesisToken] = new(big.Int).Mul(big.NewInt(100),
			config.QuarkashToJiaozi)
		alloc := config.Allocation{Balances: balance}
		adalloc := make(map[account.Address]config.Allocation)
		addr := acc1.AddressInShard(v)
		adalloc[addr] = alloc
		shardConfig.Genesis.Alloc = adalloc
	}
	shardState := createDefaultShardState(env1, &shardId0, nil, nil, nil)
	defer shardState.Stop()

	evmState := shardState.currentEvmState
	evmState.SetQuarkChainConfig(env1.clusterConfig.Quarkchain)
	runtimeBytecode := common.Hex2Bytes(vm.NonReservedNativeTokenContractBytecode)
	runtimeStart := bytes.LastIndex(runtimeBytecode, common.Hex2Bytes("608060405260"))
	// # get rid of the constructor argument
	runtimeBytecode = runtimeBytecode[runtimeStart : len(runtimeBytecode)-64]
	contractAddr := vm.SystemContracts[vm.NON_RESERVED_NATIVE_TOKEN].Address()
	evmState.SetCode(contractAddr, runtimeBytecode)
	evmState.SetState(contractAddr, common.BigToHash(big.NewInt(0)), common.BytesToHash(acc1.Recipient.Bytes()))
	_, err = evmState.Commit(true)
	assert.NoError(t, err)
	ctx := vm.Context{
		CanTransfer:                       CanTransfer,
		Transfer:                          Transfer,
		TransferFailureByPoswBalanceCheck: TransferFailureByPoswBalanceCheck,
		TransferTokenID:                   shardState.GetGenesisToken(),
		BlockNumber:                       new(big.Int),
		Time:                              new(big.Int).SetUint64(evmState.GetTimeStamp()),
	}
	evm := vm.NewEVM(ctx, evmState, shardState.ethChainConfig, vm.Config{})
	call := func(data string) ([]byte, error) {
		ret, _, err := evm.Call(vm.AccountRef(acc1.Recipient), contractAddr, common.Hex2Bytes(data), 1000000, new(big.Int))
		return ret, err
	}
	exec := func(data string, value *big.Int, delay uint64) bool {
		evm := vm.NewEVM(ctx, evmState, shardState.ethChainConfig, vm.Config{})
		evm.Time.SetUint64(evm.Time.Uint64() + delay)
		_, _, err := evm.Call(vm.AccountRef(acc1.Recipient), contractAddr, common.Hex2Bytes(data), 1000000, value)
		return err == nil
	}
	toStr := func(input uint64) string {
		b := qkcCommon.EncodeToByte32(input)
		return common.Bytes2Hex(b)
	}
	big2Str := func(bi *big.Int) string {
		b := common.LeftPadBytes(bi.Bytes(), 32)
		return common.Bytes2Hex(b)
	}
	bytes2Uint64 := func(b []byte) uint64 {
		return new(big.Int).SetBytes(b).Uint64()
	}
	getAuctionState := func() (tokenId, round, endTime uint64, newPrice *big.Int, bidder common.Address) {
		ret, err := call("08bfc300")
		assert.NoError(t, err)
		tokenId = bytes2Uint64(ret[:32])
		newPrice = new(big.Int).SetBytes(ret[32:64])
		bidder = common.BytesToAddress(ret[64:96])
		round = bytes2Uint64(ret[96:128])
		endTime = bytes2Uint64(ret[128:])
		return
	}
	//# set auction parameters: minimum bid price: 20 QKC, minimum increment: 5%, duration: one week
	_, err = call("3c69e3d2" + toStr(20) + toStr(5) + toStr(3600*24*7))
	assert.NoError(t, err)
	//	resume_auction
	succ := exec("32353fbd", new(big.Int), 1)
	assert.True(t, succ)

	// # token id to bid and win
	tokenID := toStr(uint64(9999999))
	//bid_new_token
	price := new(big.Int).Mul(big.NewInt(25), config.QuarkashToJiaozi)
	succ = exec("6aecd9d7"+tokenID+big2Str(price)+toStr(0), new(big.Int).Mul(big.NewInt(26), config.QuarkashToJiaozi), 1)
	assert.True(t, succ)
	tokenId, round, _, newPrice, bidder := getAuctionState()
	assert.Equal(t, 9999999, int(tokenId))
	assert.Equal(t, 0, int(round))
	assert.Equal(t, price, newPrice)
	assert.Equal(t, acc1.Recipient, bidder)
	//# End before ending time, should fail
	succ = exec("fe67a54b", new(big.Int), 3600*24*3)
	assert.False(t, succ)
	//# 7 days passed, this round of auction ends
	succ = exec("fe67a54b", new(big.Int), 3600*24*7)
	assert.True(t, succ)
	tokenId, _, _, _, _ = getAuctionState()
	assert.Equal(t, 0, int(tokenId))
	//get_native_token_info
	output, err := call("9ea41be7" + tokenID)
	assert.NoError(t, err)
	assert.NotEqual(t, uint64(0), new(big.Int).SetBytes(output[:32]).Uint64())
	assert.Equal(t, common.Bytes2Hex(acc1.Recipient.Bytes()), common.Bytes2Hex(output[44:64]))
	assert.Equal(t, uint64(0), new(big.Int).SetBytes(output[64:96]).Uint64())
	//mint_new_token
	amount := uint64(1000)
	_, err = call("0f2dc31a" + tokenID + toStr(amount))
	assert.NoError(t, err)
	output, err = call("9ea41be7" + tokenID)
	assert.NoError(t, err)
	assert.Equal(t, amount, new(big.Int).SetBytes(output[64:96]).Uint64())
}

func TestBlocksWithIncorrectVersion(t *testing.T) {
	env := getTestEnv(nil, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.Header().Version = 1
	_, err := shardState.AddRootBlock(rootBlock.Finalize(nil, nil, common.Hash{}))
	if err != nil {
		t.Errorf("incorrect minor block version err:%v", err)
	}
	rootBlock.Header().Version = 0
	_, err = shardState.AddRootBlock(rootBlock.Finalize(nil, nil, common.Hash{}))
	if err != nil {
		t.Errorf("incorrect minor block version err:%v", err)
	}
	shardBlock, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	shardBlock.Header().Version = 1
	_, _, err = shardState.FinalizeAndAddBlock(shardBlock)
	if err != nil {
		t.Errorf("incorrect minor block version err:%v", err)
	}
	shardBlock.Header().Version = 0
	_, _, err = shardState.FinalizeAndAddBlock(shardBlock)
	if err != nil {
		t.Errorf("incorrect minor block version err:%v", err)
	}
}

func TestEnablEvmTimestampWithContractCall(t *testing.T) {
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatEmptyAddress(0)
	a := big.NewInt(10000000).Uint64()
	env := getTestEnv(&acc1, &a, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	shardState.AddRootBlock(rootBlock)
	val := big.NewInt(12345)
	gas := uint64(50000)
	tx1 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, val, &gas, nil, nil, []byte("1234"), nil, nil)
	error := shardState.AddTx(tx1)
	if error != nil {
		t.Errorf("addTx error: %v", error)
	}
	b1, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	env.clusterConfig.Quarkchain.EnableEvmTimeStamp = b1.Header().GetTime() + uint64(100)
	b2, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	assert.Equal(t, len(b2.Transactions()), 0)

	_, _, err := shardState.FinalizeAndAddBlock(b1)
	if err != nil {
		t.Logf("smart contract tx is not allowed before evm is enabled ")
	}
}

func TestEnableEvmTimestampWithContractCreate(t *testing.T) {
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	a := big.NewInt(10000000).Uint64()
	env := getTestEnv(&acc1, &a, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	shardState.AddRootBlock(rootBlock)
	tx, err := CreateContract(shardState, id1.GetKey(), acc1, 0, "")
	if err != nil {
		t.Errorf("CreateContract err:%v", err)
	}
	assert.NoError(t, shardState.AddTx(tx))
	b1, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	env.clusterConfig.Quarkchain.EnableEvmTimeStamp = b1.Header().GetTime() + uint64(100)
	b2, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	assert.Equal(t, len(b2.Transactions()), 0)
	_, _, err = shardState.FinalizeAndAddBlock(b1)
	if err != nil {
		t.Logf("smart contract tx is not allowed before evm is enabled ")
	}
}

// no need to test_enable_tx_timestamp
// go not support account whiteList

func TestFailedTransactionGas(t *testing.T) {
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatEmptyAddress(0)
	testGenesisMinorTokenBalance = map[string]*big.Int{
		"QKC": new(big.Int).SetUint64(100000000),
		"QI":  new(big.Int).SetUint64(100000000),
		"BTC": new(big.Int).SetUint64(100000000),
	}
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	//Create failed contract with revert operation
	/**
	pragma solidity ^0.5.1;
	        contract RevertContract {
	            constructor() public {
	                revert();
	            }
	        }
	*/
	//FAILED_TRANSACTION_COST := 54416

	tx, _ := CreateContract(shardState, id1.GetKey(), acc1, acc1.FullShardKey, "6080604052348015600f57600080fd5b50600080fdfe")
	assert.NoError(t, shardState.AddTx(tx))
	b1, _ := shardState.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	_, _, err := shardState.FinalizeAndAddBlock(b1)
	if err != nil {
		t.Logf("smart contract tx is not allowed before evm is enabled ")
	}
	assert.Equal(t, shardState.CurrentHeader(), b1.Header())
	//Check receipts and make sure the transaction is failed
	//evmState, _ := shardState.State()
	//state.evm_state.receipts  ?

}

func TestIncorrectCoinbaseAmount(t *testing.T) {
	QKC := qkcCommon.TokenIDEncode("QKC")
	env := getTestEnv(nil, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, common.Hash{})
	shardState.AddRootBlock(rootBlock)
	b, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	evmState, _, _, _, _, _ := shardState.runBlock(b)
	b.Finalize(nil, emptyHash, evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), shardState.getCoinbaseAmount(b.Header().Number), evmState.GetTxCursorInfo())
	shardState.FinalizeAndAddBlock(b)
	b, _ = shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	wrong_coinbase := shardState.getCoinbaseAmount(b.Header().Number)
	m := make(map[uint64]*big.Int)
	m[QKC] = new(big.Int).Add(new(big.Int).SetUint64(QKC), big.NewInt(1))
	b.Finalize(nil, emptyHash, evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), wrong_coinbase, evmState.GetTxCursorInfo())
	_, _, err := shardState.FinalizeAndAddBlock(b)
	if err != nil {
		t.Errorf("AddBlock err:%v", err)
	}
}

func TestShardCoinbaseDecay(t *testing.T) {
	env := getTestEnv(nil, nil, nil, nil, nil, nil, nil)
	QKC := qkcCommon.TokenIDEncode(env.clusterConfig.Quarkchain.GenesisToken)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	coinbase := shardState.getCoinbaseAmount(shardState.shardConfig.EpochInterval)
	m := make(map[uint64]*big.Int)
	a := new(big.Rat).Mul(env.clusterConfig.Quarkchain.BlockRewardDecayFactor, env.clusterConfig.Quarkchain.RewardTaxRate)
	m[QKC] = new(big.Int).Mul(shardState.shardConfig.CoinbaseAmount, a.Num())
	m[QKC] = new(big.Int).Div(m[QKC], a.Denom())
	assert.Equal(t, m, coinbase.GetBalanceMap())
	coinbase = shardState.getCoinbaseAmount(shardState.shardConfig.EpochInterval + 1)
	m1 := make(map[uint64]*big.Int)
	a1 := new(big.Rat).Mul(env.clusterConfig.Quarkchain.BlockRewardDecayFactor, env.clusterConfig.Quarkchain.RewardTaxRate)
	m1[QKC] = new(big.Int).Mul(shardState.shardConfig.CoinbaseAmount, a1.Num())
	m1[QKC] = new(big.Int).Div(m1[QKC], a1.Denom())
	assert.Equal(t, m1, coinbase.GetBalanceMap())
	coinbase = shardState.getCoinbaseAmount(shardState.shardConfig.EpochInterval * 2)
	m2 := make(map[uint64]*big.Int)
	sq := new(big.Rat).Mul(env.clusterConfig.Quarkchain.BlockRewardDecayFactor, env.clusterConfig.Quarkchain.BlockRewardDecayFactor)
	a2 := new(big.Rat).Mul(sq, env.clusterConfig.Quarkchain.RewardTaxRate)
	m2[QKC] = new(big.Int).Mul(shardState.shardConfig.CoinbaseAmount, a2.Num())
	m2[QKC] = new(big.Int).Div(m2[QKC], a2.Denom())
	assert.Equal(t, m2, coinbase.GetBalanceMap())
}
func (m *MinorBlockChain) getTip() *types.MinorBlock {
	return m.GetMinorBlock(m.CurrentHeader().Hash())
}
func TestShardReorgByAddingRootBlock(t *testing.T) {
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	id2, _ := account.CreatRandomIdentity()
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	a := big.NewInt(10000000).Uint64()
	env := getTestEnv(&acc1, &a, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	genesis := shardState.CurrentBlock().Header()
	b1 := shardState.getTip().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil, nil, nil)
	b2 := shardState.getTip().CreateBlockToAppend(nil, nil, &acc2, nil, nil, nil, nil, nil, nil)
	shardState.FinalizeAndAddBlock(b1)
	shardState.FinalizeAndAddBlock(b2)
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis)
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, emptyHash)
	rootBlock2 := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock2.AddMinorBlockHeader(genesis)
	rootBlock2.AddMinorBlockHeader(b2.Header())
	rootBlock2.Finalize(nil, nil, emptyHash)
	shardState.AddRootBlock(rootBlock)
	assert.Equal(t, b1.Header(), shardState.CurrentHeader())
	assert.Equal(t, b1.Header(), shardState.CurrentHeader())

	r2Header := rootBlock2.Header()
	r2Header.ToTalDifficulty = new(big.Int).Add(r2Header.ToTalDifficulty, r2Header.Difficulty)
	r2Header.Difficulty = new(big.Int).Mul(r2Header.Difficulty, new(big.Int).SetUint64(2))
	rootBlock22 := types.NewRootBlock(r2Header, rootBlock2.MinorBlockHeaders(), nil)

	_, err := shardState.AddRootBlock(rootBlock22)
	if err != nil {
		t.Errorf("AddRootBlock err:%v", err)
	}
	assert.Equal(t, shardState.CurrentHeader().Hash(), b2.Header().Hash())
	assert.Equal(t, shardState.rootTip.Hash(), rootBlock22.Header().Hash())
	assert.Equal(t, shardState.currentEvmState.IntermediateRoot(false).String(), b2.Meta().Root.String())

}

func TestSkipUnderPricedTxToBlock(t *testing.T) {
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatEmptyAddress(0)
	a := big.NewInt(10000000).Uint64()
	env := getTestEnv(&acc1, &a, nil, nil, nil, nil, nil)
	env.clusterConfig.Quarkchain.MinMiningGasPrice = big.NewInt(10)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	// Add a root block to have all the shards initialized
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil, emptyHash)
	shardState.AddRootBlock(rootBlock)
	//Under-priced
	gas := big.NewInt(50000).Uint64()
	val := big.NewInt(12345)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, val, &gas, nil, nil, []byte("1234"), nil, nil)
	error := shardState.AddTx(tx)
	if error != nil {
		t.Errorf("addTx error: %v", error)
	}
	b1, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 0)

	//# Qualified
	gasPrice := big.NewInt(11).Uint64()
	tx1 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, val, &gas, &gasPrice, nil, []byte("1234"), nil, nil)
	error = shardState.AddTx(tx1)
	if error != nil {
		t.Errorf("addTx error: %v", error)
	}
	b2, _ := shardState.CreateBlockToMine(nil, nil, nil, nil, nil)
	assert.Equal(t, len(b2.Transactions()), 1)
}

func TestXshardGasLimitFromMultipleShards(t *testing.T) {
	QKC := qkcCommon.TokenIDEncode("QKC")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 16)
	acc3 := account.CreatAddressFromIdentity(id1, 8)
	a := big.NewInt(10000000).Uint64()
	shardSize := uint32(64)
	env0 := getTestEnv(&acc1, &a, nil, &shardSize, nil, nil, nil)
	env1 := getTestEnv(&acc1, &a, nil, &shardSize, nil, nil, nil)
	shardId0 := uint32(0)
	shardState0 := createDefaultShardState(env0, &shardId0, nil, nil, nil)
	shardId1 := uint32(16)
	shardState1 := createDefaultShardState(env1, &shardId1, nil, nil, nil)
	shardId2 := uint32(8)
	shardState2 := createDefaultShardState(env1, &shardId2, nil, nil, nil)

	// Add
	//	Add a root block to allow later minor blocks referencing this root block to
	//	be broadcasted
	rootBlock := shardState0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState2.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, emptyHash)
	shardState0.AddRootBlock(rootBlock)
	shardState1.AddRootBlock(rootBlock)
	shardState2.AddRootBlock(rootBlock)
	//	Add one block in shard 1 with 2 x-shard txs
	b1_ := shardState1.getTip().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b1_header := b1_.Header()
	b1_header.PrevRootBlockHash = rootBlock.Header().Hash()
	b1 := types.NewMinorBlockWithHeader(b1_header, b1_.Meta())
	val := big.NewInt(888888)
	gas := new(big.Int).Add(big.NewInt(21000), big.NewInt(9000)).Uint64()
	gasPrice := uint64(2)
	tx0 := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc1, acc2, val, &gas, &gasPrice, nil, nil, nil, nil)
	b1.AddTx(tx0)
	val1 := big.NewInt(111111)
	gas1 := new(big.Int).Add(big.NewInt(21000), big.NewInt(9000)).Uint64()
	gasPrice1 := uint64(2)
	tx1 := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, val1, &gas1, &gasPrice1, nil, nil, nil, nil)
	b1.AddTx(tx1)
	//	# Add a x-shard tx from remote peer
	deposit := types.CrossShardTransactionDeposit{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{TxHash: tx0.Hash(), From: acc2, To: acc1, Value: &serialize.Uint256{Value: big.NewInt(888888)}, GasPrice: &serialize.Uint256{Value: big.NewInt(2)}, GasTokenID: QKC, TransferTokenID: QKC}}
	deposit2 := types.CrossShardTransactionDeposit{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{TxHash: tx1.Hash(), From: acc2, To: acc1, Value: &serialize.Uint256{Value: big.NewInt(111111)}, GasPrice: &serialize.Uint256{Value: big.NewInt(2)}, GasTokenID: QKC, TransferTokenID: QKC}}
	txL := make([]*types.CrossShardTransactionDeposit, 0)
	txL = append(txL, &deposit, &deposit2)
	txList := types.CrossShardTransactionDepositList{TXList: txL}
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList)
	//	# Add one block in shard 1 with 2 x-shard txs
	b2_ := shardState2.getTip().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b2_header := b2_.Header()
	b2_header.PrevRootBlockHash = rootBlock.Header().Hash()
	b2 := types.NewMinorBlockWithHeader(b2_header, b2_.Meta())
	val2 := big.NewInt(12345)
	gas2 := new(big.Int).Add(big.NewInt(21000), big.NewInt(9000)).Uint64()
	gasPrice2 := uint64(2)
	tx3 := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, val2, &gas2, &gasPrice2, nil, []byte("1234"), nil, nil)
	b2.AddTx(tx3)
	//	# Add a x-shard tx from remote peer
	deposit = types.CrossShardTransactionDeposit{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{TxHash: tx3.Hash(), From: acc3, To: acc1, Value: &serialize.Uint256{Value: big.NewInt(12345)}, GasPrice: &serialize.Uint256{Value: big.NewInt(2)}, GasTokenID: QKC, TransferTokenID: QKC}}
	txL = make([]*types.CrossShardTransactionDeposit, 0)
	txL = append(txL, &deposit)
	txList = types.CrossShardTransactionDepositList{TXList: txL}
	shardState0.AddCrossShardTxListByMinorBlockHash(b2.Header().Hash(), txList)
	//	# Create a root block containing the block with the x-shard tx
	rootBlock = shardState0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b2.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	coinbase := types.NewEmptyTokenBalances()
	coinbase.SetValue(big.NewInt(1000000), qkcCommon.TokenIDEncode("QKC"))
	rootBlock.Finalize(coinbase, &acc1, emptyHash)
	shardState0.AddRootBlock(rootBlock)

	//	# Add b0 and make sure one x-shard tx's are added
	xShardGasLimit := big.NewInt(9000)
	b2, _ = shardState0.CreateBlockToMine(nil, nil, nil, xShardGasLimit, nil)
	shardState0.xShardGasLimit = new(big.Int).Set(xShardGasLimit)
	shardState0.FinalizeAndAddBlock(b2)
	//	# Root block coinbase does not consume xshard gas
	tb := shardState0.currentEvmState.GetBalance(acc1.Recipient, shardState0.GetGenesisToken())

	assert.Equal(t, tb, big.NewInt(10000000+1000000+12345))
	//	# X-shard gas used
	evmState0 := shardState0.currentEvmState
	assert.Equal(t, evmState0.GetXShardReceiveGasUsed(), big.NewInt(9000))
	//	# Add b2 and make sure all x-shard tx's are added
	xShardGasLimit1 := big.NewInt(9000)
	b2, _ = shardState0.CreateBlockToMine(nil, nil, nil, xShardGasLimit1, nil)
	shardState0.FinalizeAndAddBlock(b2)
	//	# Root block coinbase does not consume xshard gas
	tb = shardState0.currentEvmState.GetBalance(acc1.Recipient, shardState0.GetGenesisToken())
	assert.Equal(t, tb, big.NewInt(10000000+1000000+12345+888888))
	//	# Add b3 and make sure no x-shard tx's are added
	xShardGasLimit1 = big.NewInt(9000)
	b3, _ := shardState0.CreateBlockToMine(nil, nil, nil, xShardGasLimit1, nil)
	shardState0.FinalizeAndAddBlock(b3)
	//	# Root block coinbase does not consume xshard gas
	tb = shardState0.currentEvmState.GetBalance(acc1.Recipient, shardState0.GetGenesisToken())
	assert.Equal(t, tb, big.NewInt(10000000+1000000+12345+888888+111111))
}
