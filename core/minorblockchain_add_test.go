package core

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common"
	ethParams "github.com/ethereum/go-ethereum/params"
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
	if rootBlock == nil {
		t.Errorf("rootBlock is nil")
	}
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
	FakeQkcConfig := config.NewQuarkChainConfig()
	genesisManager := NewGenesis(FakeQkcConfig)
	gensisBlock, err := genesisManager.CreateMinorBlock(rootBlock, 2, env.db)
	checkErr(err)

	newGenesisBlock, err := shardState.InitGenesisState(rootBlock, gensisBlock)
	checkErr(err)
	if shardState.CurrentBlock().IHeader().Hash() != genesisHeader.Hash() {
		t.Errorf("genesis hash is not match have %v want %v", shardState.CurrentBlock().IHeader().Hash(), genesisHeader.Hash())
	}

	block := newGenesisBlock.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)

	_, _, err = shardState.FinalizeAndAddBlock(block)
	checkErr(err)

	if !reflect.DeepEqual(shardState.CurrentBlock().IHeader().Hash(), genesisHeader.Hash()) {
		t.Errorf("genesis header is not match")
	}

	zeroTempHeader := shardState.GetBlockByNumber(0).(*types.MinorBlock).Header()
	if !reflect.DeepEqual(zeroTempHeader.Hash(), genesisHeader.Hash()) {
		t.Errorf("genesis header is not match zeroTempHeader")
	}

	newRootBlock := rootBlock.Header().CreateBlockToAppend(nil, nil, nil, &fakeNonce, nil)
	newRootBlock.Finalize(nil, nil)
	err = shardState.AddRootBlock(newRootBlock)
	checkErr(err)

	currentMinorHeader := shardState.CurrentBlock().IHeader()
	currentZero := shardState.GetBlockByNumber(0).IHeader().(*types.MinorBlockHeader)
	if !reflect.DeepEqual(currentMinorHeader.Hash(), newGenesisBlock.Header().Hash()) {
		t.Errorf("header is not match-1")
	}
	if !reflect.DeepEqual(currentZero.Hash(), newGenesisBlock.Header().Hash()) {
		t.Errorf("header is not match-2")
	}

}

func TestGasPrice(t *testing.T) {
	idList := make([]account.Identity, 0)
	for index := 0; index < 5; index++ {
		temp, err := account.CreatRandomIdentity()
		if err != nil {
			t.Errorf("create random identity err:%v", err)
		}
		idList = append(idList, temp)
	}

	accList := make([]account.Address, 0)
	for _, v := range idList {
		accList = append(accList, account.CreatAddressFromIdentity(v, 0))
	}

	fakeData := uint64(10000000)
	env := setUp(&accList[0], &fakeData, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	fakeChan := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChan
	defer shardState.Stop()
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.Finalize(nil, nil)

	err := shardState.AddRootBlock(rootBlock)
	checkErr(err)

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
		forRe := false
		for forRe == true {
			select {
			case result := <-fakeChan:
				if result == uint64(blockIndex+1) {
					forRe = true
				}
			case <-time.After(2 * time.Second):
				panic(errors.New("should end here"))

			}
		}
		b, err := shardState.CreateBlockToMine(nil, &accList[1], nil)
		checkErr(err)
		_, _, err = shardState.FinalizeAndAddBlock(b)
		checkErr(err)
	}

	currentNumber := int(shardState.CurrentBlock().NumberU64())

	if currentNumber != 3 {
		panic(errors.New("number is not match"))
	}

	shardState.gasPriceSuggestionOracle.Percentile = 100
	gasPrice := shardState.GasPrice()
	if *gasPrice != 42 {
		panic(errors.New("gas price is not match"))
	}
	shardState.gasPriceSuggestionOracle.Percentile = 50
	gasPrice2 := shardState.GasPrice()
	if *gasPrice2 != 42 {
		panic("gas price is not match -2")
	}

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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)
	txGen := func(data []byte) *types.Transaction {
		return createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(123456), nil, nil, nil, data)
	}
	tx := txGen([]byte{})
	estimate, err := shardState.EstimateGas(tx, acc1)
	checkErr(err)

	if estimate != 21000 {
		panic(errors.New("estimate!=21000"))
	}

	newTx := txGen([]byte("12123478123412348125936583475758"))
	estimate, err = shardState.EstimateGas(newTx, acc1)
	checkErr(err)
	if estimate != 23176 {
		panic(errors.New("should 23176"))
	}

}

func TestEcecuteTx(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	checkErr(err)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)

	defer shardState.Stop()
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil)
	currentEvmState, err := shardState.State()
	checkErr(err)

	currentEvmState.SetGasUsed(currentEvmState.GetGasLimit())
	err = shardState.ExecuteTx(tx, acc1, nil)
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
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil)
	err = shardState.AddTx(tx)
	if err == nil {
		panic(errors.New("need err"))
	}
	err = shardState.ExecuteTx(tx, acc1, nil)
	if err == nil {
		panic(errors.New("need err"))
	}
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)

	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	currState, err := shardState.State()
	checkErr(err)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	checkErr(err)

	block, i := shardState.GetTransactionByHash(tx.Hash())
	if block == nil {
		panic(errors.New("block is nil"))
	}
	if !reflect.DeepEqual(block.GetTransactions()[0].Hash(), tx.Hash()) {
		t.Errorf("tx is not match")
	}
	if block.Header().Time != 0 {
		t.Errorf("time is must zero")
	}
	if i != 0 {
		t.Errorf("i should ==0")
	}

	b1, err := shardState.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(49999))
	checkErr(err)
	if b1.Header().Number != 1 {
		panic(errors.New("header should equal 1"))
	}

	if len(b1.Transactions()) != 0 {
		panic(errors.New("len should equal zero"))
	}
	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	if len(b2.Transactions()) != 1 {
		panic(errors.New("len should equal 1"))
	}
	if b2.Header().Number != 1 {
		panic(errors.New("header should equal 2"))
	}
	b2, re, err := shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	if shardState.CurrentBlock().IHeader().NumberU64() != 1 {
		t.Errorf("current block number should equal 1")
	}

	if !reflect.DeepEqual(shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b2.Header().Hash()) {
		panic(errors.New("should equal here"))
	}

	if !reflect.DeepEqual(shardState.CurrentBlock().GetTransactions()[0].Hash(), tx.Hash()) {
		panic(errors.New("tx is nil"))
	}
	currentState1, err := shardState.State()
	checkErr(err)

	if currentState1.GetBalance(id1.GetRecipient()).Uint64() != (10000000 - ethParams.TxGas - 12345) {
		t.Errorf("should equal")
	}
	if currentState1.GetBalance(acc2.Recipient).Uint64() != 12345 {
		t.Errorf("shoule equal")
	}
	acc3Value := currentState1.GetBalance(acc3.Recipient)
	should3 := new(big.Int).Add(new(big.Int).SetUint64(ethParams.TxGas/2), new(big.Int).Div(testShardCoinBaseAmount, new(big.Int).SetUint64(2)))
	if acc3Value.Cmp(should3) != 0 {
		t.Errorf("!=")
	}

	if len(re) != 1 {
		t.Errorf("len(re)!=1")
	}

	if re[0].Status != 1 {
		t.Errorf("!=1")
	}
	if re[0].GasUsed != 21000 {
		t.Errorf("!=21000")
	}

	block, i = shardState.GetTransactionByHash(tx.Hash())
	if block == nil {
		panic(errors.New("block is nil"))
	}

	currrrrrrrrr := shardState.CurrentBlock()

	if currrrrrrrrr.Hash() != b2.Hash() {
		panic(errors.New("sssss"))
	}

	if !reflect.DeepEqual(currrrrrrrrr.IHeader().Hash(), block.Header().Hash()) {
		panic(errors.New("bbbbbbbbbbb"))
	}
	if !reflect.DeepEqual(currrrrrrrrr.GetMetaData().Hash(), block.Meta().Hash()) {
		panic(errors.New("bbbbbbbbbbb"))
	}

	for k, v := range currrrrrrrrr.GetTransactions() {
		if v.Hash() != block.Transactions()[k].Hash() {
			panic(errors.New("tx is not match"))
		}
	}
	if i != 0 {
		panic(errors.New("should equal zero"))
	}

	blockR, indexR, reR := shardState.GetTransactionReceipt(tx.Hash())
	if blockR == nil {
		panic(errors.New("block is nil"))
	}
	if blockR.Hash() != b2.Hash() {
		panic(errors.New("hash is not match"))
	}
	if indexR != 0 {
		panic(errors.New("indexR!=0"))
	}

	if reR[0].Status != 1 {
		panic(errors.New("!=1"))
	}
	if reR[0].GasUsed != 21000 {
		panic(errors.New("!=21000"))
	}

	s, err := shardState.State()
	if s.GetFullShardKey(acc2.Recipient) != acc2.FullShardKey {
		t.Errorf("shardId is not match")
	}

}

func TestDuplicatedTx(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	fakeMoney := uint64(10000000)
	env := setUp(&acc1, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), nil, nil, nil, nil)

	err = shardState.AddTx(tx)
	checkErr(err)
	err = shardState.AddTx(tx)
	if err == nil {
		panic(errors.New("should err"))
	}
	pending, err := shardState.txPool.Pending()
	if len(pending) != 1 {
		panic(errors.New("pending length is 1"))
	}

	block, i := shardState.GetTransactionByHash(tx.Hash())
	if block == nil {
		panic(errors.New("block is nil"))
	}
	if len(block.Transactions()) != 1 {
		panic(errors.New("len should 1"))
	}

	if block.Transactions()[0].Hash() != tx.Hash() {
		panic(errors.New("tx hash is not match"))
	}

	if block.Header().Time != 0 {
		panic(errors.New("header time not 0"))
	}
	if i != 0 {
		panic(errors.New("i!=0"))
	}
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	if len(b1.Transactions()) != 1 {
		panic(errors.New("b1.txList should 1"))
	}
	b1, reps, err := shardState.FinalizeAndAddBlock(b1)

	if !reflect.DeepEqual(shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b1.Header().Hash()) {
		panic(errors.New("should equal"))
	}

	currentState, err := shardState.State()
	checkErr(err)
	if currentState.GetBalance(id1.GetRecipient()).Uint64() != (10000000 - 21000 - 12345) {
		panic(errors.New("id1 balance is not match"))
	}
	if currentState.GetBalance(acc2.Recipient).Uint64() != 12345 {
		panic(errors.New("acc2 balance is not match"))
	}

	shouldAcc3 := new(big.Int).Add(new(big.Int).Div(testShardCoinBaseAmount, new(big.Int).SetUint64(2)), new(big.Int).SetUint64(21000/2))

	if currentState.GetBalance(acc3.Recipient).Cmp(shouldAcc3) != 0 {
		panic(errors.New("shouldAcc3 not match"))
	}

	if len(reps) != 1 {
		panic(errors.New("reps is not 1"))
	}
	if reps[0].Status != 1 {
		panic(errors.New("reps.statue is not 1"))
	}
	if reps[0].GasUsed != 21000 {
		panic(errors.New("!=21000"))
	}

	block, _ = shardState.GetTransactionByHash(tx.Hash())
	if block == nil {
		panic(errors.New("block is nil"))
	}

	err = shardState.AddTx(tx)

	if err == nil {
		panic(errors.New("should false"))
	}

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
	fakeValue := new(big.Int).Mul(JIAOZI, new(big.Int).SetUint64(100))
	fakeMoey := new(big.Int).Sub(fakeValue, new(big.Int).SetUint64(1))
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, fakeMoey, nil, nil, nil, nil)

	err = shardState.AddTx(tx)
	if err == nil {
		panic(errors.New("should false"))
	}

	pending, err := shardState.txPool.Pending()
	checkErr(err)
	if len(pending) != 0 {
		panic(errors.New("len should 0"))
	}
}

func TestAddNonNeighborTxFail(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2, err := account.CreatRandomAccountWithFullShardKey(3)
	acc3, err := account.CreatRandomAccountWithFullShardKey(8)
	fakeMoney := uint64(10000000)
	fakeShardSize := uint32(64)
	env := setUp(&acc1, &fakeMoney, &fakeShardSize)

	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(1000000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil)

	err = shardState.AddTx(tx)
	if err == nil {
		panic(errors.New("flag==true"))
	}
	if len(shardState.txPool.pending) != 0 {
		panic(errors.New("should 1"))
	}

	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc3, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil)

	err = shardState.AddTx(tx)
	checkErr(err)
	pending, err := shardState.txPool.Pending()
	checkErr(err)
	if len(pending) != 1 {
		panic(errors.New("should 1"))
	}
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
	env.clusterConfig.Quarkchain.MaxNeighbors = 4294967295
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(500000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	if len(b1.GetTransactions()) != 0 {
		panic("should =0")
	}
	fakeGasPrice := uint64(2)
	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc3, new(big.Int).SetUint64(12345), &fakeGas, &fakeGasPrice, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)
	b1, err = shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	if len(b1.Transactions()) != 1 {
		panic(errors.New("should 1"))
	}
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil))
	checkErr(err)
	currState, err := shardState.State()
	checkErr(err)
	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	_, res, err := shardState.FinalizeAndAddBlock(b0)
	checkErr(err)
	if len(res) != 1 {
		panic(errors.New("should equal 1"))
	}

	currState, err = shardState.State()
	checkErr(err)
	acc1Value := currState.GetBalance(id1.GetRecipient())
	acc2Value := currState.GetBalance(acc2.Recipient)
	acc3Value := currState.GetBalance(acc3.Recipient)

	if acc1Value.Uint64() != 1000000 {
		panic("should equal 1000000")
	}
	if acc2Value.Uint64() != 1000000 {
		panic("should equal 1000000")
	}

	should333 := new(big.Int).Add(testShardCoinBaseAmount, new(big.Int).SetUint64(21000))
	should3 := new(big.Int).Div(should333, new(big.Int).SetUint64(2))
	if acc3Value.Cmp(should3) != 0 {
		panic(errors.New("not equal"))
	}

	currOB := currState.GetOrNewStateObject(acc2.Recipient)
	if currOB.FullShardKey() != acc2.FullShardKey {
		panic(errors.New("fullShardKey is not match"))
	}

	forRe := false
	for forRe == true {
		select {
		case result := <-fakeChan:
			if result == shardState.CurrentBlock().NumberU64() {
				forRe = true
			}
		case <-time.After(2 * time.Second):
			panic(errors.New("should end here"))

		}
	}
	toAddress := account.Address{Recipient: acc2.Recipient, FullShardKey: acc2.FullShardKey + 2}
	fakeGas := uint64(50000)
	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, toAddress, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil))
	checkErr(err)
	fakeGas = uint64(40000)
	err = shardState.AddTx(createTransferTransaction(shardState, id2.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(54321), &fakeGas, nil, nil, nil))
	checkErr(err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(40000))
	if err != nil {
		panic(err)
	}
	if len(b1.GetTransactions()) != 1 {
		panic(errors.New("block length should 1"))
	}
	b1, err = shardState.CreateBlockToMine(nil, &acc3, nil)
	if len(b1.Transactions()) != 2 {
		panic(errors.New("block len should 2"))
	}

	_, reps, err := shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	if shardState.CurrentHeader().Hash() != b1.Header().Hash() {
		panic(errors.New("hash is not match"))
	}
	currState, err = shardState.State()
	checkErr(err)
	acc1Value = currState.GetBalance(id1.GetRecipient())
	if acc1Value.Uint64() != (1000000 - 21000 - 12345 + 54321) {
		panic(errors.New("balance is not match"))
	}

	acc2Value = currState.GetBalance(id2.GetRecipient())
	if acc2Value.Uint64() != (1000000 - 21000 + 12345 - 54321) {
		panic(errors.New("balance id2 is not match"))
	}

	acc3Value = currState.GetBalance(acc3.Recipient)

	shouldAcc3Value := new(big.Int).Mul(testShardCoinBaseAmount, new(big.Int).SetUint64(2))
	shouldAcc3Value = new(big.Int).Add(shouldAcc3Value, new(big.Int).SetUint64(63000))
	shouldAcc3Value = new(big.Int).Div(shouldAcc3Value, new(big.Int).SetUint64(2))
	if shouldAcc3Value.Cmp(acc3Value) != 0 {
		panic(errors.New("should equal"))
	}

	if len(reps) != 2 {
		panic(errors.New("should 2"))
	}
	if reps[0].Status != 1 {
		panic(errors.New("should 1"))
	}
	if reps[0].CumulativeGasUsed != 21000 {
		panic(errors.New("should 21000"))
	}
	if reps[1].Status != 1 {
		panic(errors.New("should 1"))
	}

	if reps[1].CumulativeGasUsed != 42000 {
		panic(errors.New("should 42000"))
	}

	block, i := shardState.GetTransactionByHash(b1.GetTransactions()[0].Hash())
	if block.Hash() != b1.Hash() {
		panic(errors.New("block is not match"))
	}
	if i != 0 {
		panic(errors.New("i!=0"))
	}

	block, i = shardState.GetTransactionByHash(b1.GetTransactions()[1].Hash())
	if block.Hash() != b1.Hash() {
		panic(errors.New("block is not match"))
	}

	if i != 1 {
		panic(errors.New("i!=1"))
	}

	if currState.GetFullShardKey(acc2.Recipient) != acc2.FullShardKey {
		panic(errors.New("fullShardKey is not match"))
	}
}

func TestForkDoesNotConfirmTx(t *testing.T) {
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
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(1000000), nil, nil, nil, nil))
	checkErr(err)
	b0, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	b00 := types.NewMinorBlock(b0.Header(), b0.Meta(), nil, nil, nil)
	_, _, err = shardState.FinalizeAndAddBlock(b00)
	checkErr(err)
	if len(shardState.txPool.pending) != 1 {
		panic(errors.New("len should 1"))
	}
	if len(b1.Transactions()) != 1 {
		panic(errors.New("b1.lentx is 1"))
	}
	_, _, err = shardState.FinalizeAndAddBlock(b1)
	if len(shardState.txPool.pending) != 1 {
		panic(errors.New("should 1"))
	}

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	_, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)

	time.Sleep(1 * time.Second)

	forRe := false
	for forRe == true {
		select {
		case result := <-fakeChain:
			if result == 2 {
				pending, err := shardState.txPool.Pending()
				if err != nil {
					panic(err)
				}
				if len(pending) != 0 {
					panic(errors.New("should 0"))
				}
				forRe = true
			}

		case <-time.After(2 * time.Second):
			panic(errors.New("should end here"))

		}
	}

}

//TODO one can run ,all can not run
func TestRevertForkPutTxBackToQueue(t *testing.T) {
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

	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
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
	b11, _, err = shardState.FinalizeAndAddBlock(b11)
	checkErr(err)

	b2 := b11.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	_, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)

	b3 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState.FinalizeAndAddBlock(b3)

	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState.FinalizeAndAddBlock(b4)

	forRe := false
	for forRe == true {
		select {
		case result := <-fakeChain:
			if result == b4.NumberU64() {
				pending, err := shardState.txPool.Pending()
				if err != nil {
					panic(err)
				}
				if len(pending) != 0 {
					panic(errors.New("should 0"))
				}
				forRe = true
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
	if shardState.getBlockCountByHeight(1) != 1 {
		panic(errors.New("not equal"))
	}
	_, _, err = shardState.FinalizeAndAddBlock(b22)
	checkErr(err)
	if shardState.getBlockCountByHeight(1) != 2 {
		panic(errors.New("not equal"))
	}
}

func TestXShardTxSent(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

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

	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.AddMinorBlockHeader(shardState1.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock = rootBlock.Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(21000 + 9000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &fakeGas, nil, nil, nil)
	err = shardState.AddTx(tx)
	checkErr(err)

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	ifEqual(1, len(b1.GetTransactions()))
	currentState, err := shardState.State()
	checkErr(err)
	ifEqual(currentState.GetGasUsed().Uint64(), uint64(0))
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)

	currentState, err = shardState.State()
	checkErr(err)
	ifEqual(len(shardState.currentEvmState.GetXShardList()), int(1))
	temp := shardState.currentEvmState.GetXShardList()[0]
	ifEqual(temp.TxHash, tx.Hash())
	ifEqual(temp.From.ToBytes(), acc1.ToBytes())
	ifEqual(temp.To.ToBytes(), acc2.ToBytes())
	ifEqual(temp.Value.Value.Uint64(), uint64(888888))
	ifEqual(temp.GasPrice.Value.Uint64(), uint64(1))

	id1Value := shardState.currentEvmState.GetGasUsed()
	ifEqual(id1Value.Uint64(), uint64(21000+9000))
	id3Value := shardState.currentEvmState.GetBalance(acc3.Recipient)
	shouldID3Value := new(big.Int).Add(testShardCoinBaseAmount, new(big.Int).SetUint64(21000))
	shouldID3Value = new(big.Int).Div(shouldID3Value, new(big.Int).SetUint64(2))
	ifEqual(id3Value.Uint64(), shouldID3Value.Uint64())
}

func TestXShardTxInsufficientGas(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1)
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	id := uint32(0)
	shardState := createDefaultShardState(env, &id, nil, nil, nil)

	fakeGas := uint64(21000)
	err = shardState.AddTx(createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(888888), &fakeGas, nil, nil, nil))
	if err == nil {
		panic(errors.New("should err"))
	}

	b1, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	ifEqual(len(b1.Transactions()), 0)
	ifEqual(len(shardState.txPool.pending), 0)
}

func TestXShardTxReceiver(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

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
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.AddMinorBlockHeader(shardState1.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.Finalize(nil, nil)
	err0 := shardState0.AddRootBlock(rootBlock)
	checkErr(err0)

	err1 := shardState1.AddRootBlock(rootBlock)
	checkErr(err1)

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
		TxHash:   tx.Hash(),
		From:     acc2,
		To:       acc1,
		Value:    &serialize.Uint256{Value: new(big.Int).SetUint64(888888)},
		GasPrice: &serialize.Uint256{Value: new(big.Int).SetUint64(2)},
	})
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList) // write db
	rootBlock = shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil)
	err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)
	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient)
	ifEqual(acc1Value.Uint64(), uint64(10000000+888888))

	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient)
	acc3Should := new(big.Int).Add(testShardCoinBaseAmount, new(big.Int).SetUint64(9000*2))
	acc3Should = new(big.Int).Div(acc3Should, new(big.Int).SetUint64(2))
	ifEqual(acc3Value.String(), acc3Should.String())

	ifEqual(shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64(), uint64(9000))
}

func TestXShardTxReceivedExcludeNonNeighbor(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	if err != nil {
		panic(err)
	}

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

	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil)
	err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient)
	ifEqual(uint64(10000000), acc1Value.Uint64())

	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient)
	acc3Should := new(big.Int).Div(testShardCoinBaseAmount, new(big.Int).SetUint64(2))
	ifEqual(acc3Should.Uint64(), acc3Value.Uint64())
	ifEqual(shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64(), uint64(0))
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

	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.AddMinorBlockHeader(shardState1.CurrentHeader().(*types.MinorBlockHeader))
	rootBlock.Finalize(nil, nil)

	err = shardState0.AddRootBlock(rootBlock)

	checkErr(err)
	err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)

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
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList)

	rootBlock0 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock0.AddMinorBlockHeader(b0.Header())
	rootBlock0.AddMinorBlockHeader(b1.Header())
	rootBlock0.Finalize(nil, nil)
	//fmt.Println("????", shardState0.CurrentBlock().Hash().String(), shardState0.CurrentHeader().Hash().String(), b0.Hash().String())
	err = shardState0.AddRootBlock(rootBlock0)

	checkErr(err)
	//fmt.Println("????", shardState0.CurrentBlock().Hash().String(), shardState0.CurrentHeader().Hash().String())
	//fmt.Println("before b2", shardState0.CurrentBlock().NumberU64())
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	//fmt.Println("b2.Number", b2.NumberU64())
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
	shardState0.AddCrossShardTxListByMinorBlockHash(b3.Header().Hash(), txList)

	rootBlock1 := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock1.AddMinorBlockHeader(b2.Header())
	rootBlock1.AddMinorBlockHeader(b3.Header())
	rootBlock1.Finalize(nil, nil)
	err = shardState0.AddRootBlock(rootBlock1)
	checkErr(err)

	b5, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(0))
	checkErr(err)

	ifEqual(b5.PrevRootBlockHash().String(), rootBlock0.Hash().String())

	b6, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).SetUint64(9000))
	checkErr(err)
	ifEqual(rootBlock0.Hash().String(), b6.PrevRootBlockHash().String())

	b7, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).Mul(new(big.Int).SetUint64(9000), new(big.Int).SetUint64(2)))
	checkErr(err)

	ifEqual(rootBlock1.Header().Hash().String(), b7.PrevRootBlockHash().String())
	b8, err := shardState0.CreateBlockToMine(nil, &acc3, new(big.Int).Mul(new(big.Int).SetUint64(9000), new(big.Int).SetUint64(3)))
	checkErr(err)
	ifEqual(rootBlock1.Header().Hash().String(), b8.PrevRootBlockHash().String())

	b4, err := shardState0.CreateBlockToMine(nil, &acc3, nil)
	ifEqual(b4.Header().PrevRootBlockHash.String(), rootBlock1.Hash().String())
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)

	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient)
	ifEqual(acc1Value.Uint64(), uint64(10000000+888888+385723))

	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient)
	acc3ValueShould := new(big.Int).Add(new(big.Int).SetUint64(9000*5), testShardCoinBaseAmount)
	acc3ValueShould = new(big.Int).Div(acc3ValueShould, new(big.Int).SetUint64(2))
	ifEqual(acc3ValueShould.String(), acc3Value.String())

	ifEqual(shardState0.currentEvmState.GetGasUsed().Uint64(), uint64(18000))
	ifEqual(shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64(), uint64(18000))
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
	ifEqual(shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b0.Hash())

	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)
	ifEqual(shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b0.Hash())
	b2 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	ifEqual(shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b2.Hash())
}

func TestRootChainFirstConsensus(t *testing.T) {
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
	b0 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &emptyAddress, nil, nil, nil, nil)

	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	evmState, reps, err := shardState1.runBlock(b1)
	b1.Finalize(reps, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), evmState.GetBlockFee())
	rootBlock := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(genesis.Header())
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil)
	err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b00 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b00, _, err = shardState0.FinalizeAndAddBlock(b00)
	ifEqual(shardState0.CurrentBlock().IHeader().Hash().String(), b00.Hash().String())
	checkErr(err)

	b3 := b2.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState0.FinalizeAndAddBlock(b3)
	checkErr(err)
	ifEqual(shardState0.CurrentBlock().IHeader().Hash().String(), b00.Hash().String())
	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)
	ifEqual(true, b4.Header().Number > b00.Header().Number)
	ifEqual(shardState0.CurrentBlock().Hash().String(), b00.Hash().String())
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
	b0 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	b2 := shardState0.CurrentBlock().CreateBlockToAppend(nil, nil, &emptyAddress, nil, nil, nil, nil)

	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)

	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	evmState, reps, err := shardState1.runBlock(b1)
	b1.Finalize(reps, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), evmState.GetBlockFee())

	emptyRoot := shardState0.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState0.AddRootBlock(emptyRoot)
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

	err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	b00 := b0.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b00, _, err = shardState0.FinalizeAndAddBlock(b00)
	checkErr(err)
	ifEqual(shardState0.CurrentBlock().Hash().String(), b00.Hash().String())

	b3 := b2.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b3, _, err = shardState0.FinalizeAndAddBlock(b3)
	checkErr(err)
	b4 := b3.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)
	b4, _, err = shardState0.FinalizeAndAddBlock(b4)
	checkErr(err)

	currBlock := shardState0.CurrentBlock()
	currBlock2 := shardState0.GetBlockByNumber(2)
	currBlock3 := shardState0.GetBlockByNumber(3)
	ifEqual(currBlock.Hash().String(), b00.Header().Hash().String())
	ifEqual(currBlock2.Hash().String(), b00.Header().Hash().String())
	ifEqual(currBlock3, nil)

	b5 := b1.CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil)

	err = shardState0.AddRootBlock(rootBlock1)

	emptyRoot = rootBlock1.Header().CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)
	err = shardState0.AddRootBlock(emptyRoot)
	checkErr(err)
	rootBlock2 := emptyRoot.Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock2.AddMinorBlockHeader(b3.Header())
	rootBlock2.AddMinorBlockHeader(b4.Header())
	rootBlock2.AddMinorBlockHeader(b5.Header())
	rootBlock2.Finalize(nil, nil)

	ifEqual(shardState0.CurrentBlock().Hash().String(), b2.Header().Hash().String())

	//fmt.Println("AddRootBlock2")
	err = shardState0.AddRootBlock(rootBlock2)
	//fmt.Println("AddRootBlock2-end")
	checkErr(err)

	currHeader := shardState0.CurrentHeader().(*types.MinorBlockHeader)

	ifEqual(currHeader.Hash().String(), b4.Header().Hash().String())
	ifEqual(shardState0.rootTip.Hash().String(), rootBlock2.Hash().String())
	ifEqual(shardState0.GetBlockByNumber(2).Hash().String(), b3.Hash().String())
	ifEqual(shardState0.GetBlockByNumber(3).Hash().String(), b4.Hash().String())
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

	err = shardState.AddRootBlock(rootBlock)

	unConfirmedHeaderList := shardState.GetUnconfirmedHeaderList()
	for k, v := range unConfirmedHeaderList {
		if v.Hash() != headers[k].Hash() {
			panic(errors.New("not match"))
		}
	}

	rootBlock1 := types.NewRootBlock(rootBlock.Header(), headers[:13], nil)
	rootBlock1.Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock1)
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

	ifEqual(shardState.CurrentHeader().Hash().String(), b0.Header().Hash().String())
	err = shardState.AddRootBlock(rootBlock)
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
	ifEqual(shardState.CurrentHeader().Hash().String(), b1.Header().Hash().String())

	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
	ifEqual(shardState.CurrentHeader().Hash().String(), b2.Header().Hash().String())

	b3, _, err = shardState.FinalizeAndAddBlock(b3)
	checkErr(err)

	ifEqual(shardState.CurrentHeader().Hash().String(), b2.Header().Hash().String())
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
	env.clusterConfig.Quarkchain.NetworkID = 1

	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, diffCalc, nil, nil)

	createTime := shardState.CurrentHeader().GetTime() + 8
	b0, err := shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	ifEqual(b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64()/uint64(2048)+shardState.CurrentHeader().GetDifficulty().Uint64())

	createTime = shardState.CurrentHeader().GetTime() + 9
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	ifEqual(b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64())

	createTime = shardState.CurrentHeader().GetTime() + 17
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	ifEqual(b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64())

	createTime = shardState.CurrentHeader().GetTime() + 24
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	ifEqual(b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64()-shardState.CurrentHeader().GetDifficulty().Uint64()/uint64(2048))

	createTime = shardState.CurrentHeader().GetTime() + 35
	b0, err = shardState.CreateBlockToMine(&createTime, nil, nil)
	checkErr(err)
	ifEqual(b0.Header().Difficulty.Uint64(), shardState.CurrentHeader().GetDifficulty().Uint64()-shardState.CurrentHeader().GetDifficulty().Uint64()/uint64(2048)*2)
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
		//fmt.Println("index", index, b.Header().Number)
		b, _, err = shardState.FinalizeAndAddBlock(b)

		checkErr(err)
		blockHeaders = append(blockHeaders, b.Header())
		blockMetas = append(blockMetas, b.Meta())
	}
	//fmt.Println("end")
	b1 := shardState.GetBlockByNumber(3).(*types.MinorBlock)
	b11Header := b1.Header()
	b11Header.Time = b1.Header().GetTime() + 1
	b1 = types.NewMinorBlock(b11Header, b1.Meta(), b1.Transactions(), nil, nil)
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	//fmt.Println("b1???")
	checkErr(err)

	DBb1 := shardState.GetBlockByHash(b1.Hash())
	ifEqual(DBb1.IHeader().GetTime(), b11Header.Time)

	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.ExtendMinorBlockHeaderList(blockHeaders[:5])
	rootBlock.Finalize(nil, nil)
	err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	ifEqual(shardState.CurrentBlock().Hash().String(), blockHeaders[12].Hash().String())

	fakeClusterConfig := config.NewClusterConfig()
	Engine := new(consensus.FakeEngine)

	recoveredState, err := NewMinorBlockChain(env.db, nil, ethParams.TestChainConfig, fakeClusterConfig, Engine, vm.Config{}, nil, 2|0, nil)

	//fmt.Println("hahha")
	err = recoveredState.InitFromRootBlock(rootBlock)
	fmt.Println(">>>>>>>>>>", recoveredState.CurrentBlock().NumberU64())
	//s := recoveredState.GetBlockByNumber(10)
	//fmt.Println("SSSSS", s.NumberU64())
	checkErr(err)
	tempBlock := recoveredState.GetBlockByHash(b1.Header().Hash())

	ifEqual(tempBlock.Hash().String(), b1.Hash().String())
	ifEqual(recoveredState.rootTip.Hash().String(), rootBlock.Hash().String())
	ifEqual(recoveredState.CurrentHeader().Hash().String(), blockHeaders[4].Hash().String())
	ifEqual(recoveredState.confirmedHeaderTip.Hash().String(), blockHeaders[4].Hash().String())
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

	var rootBlock *types.RootBlock
	for index := 0; index < 3; index++ {
		rootBlock = shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
		err := shardState.AddRootBlock(rootBlock)
		checkErr(err)

	}

	fakeClusterConfig := config.NewClusterConfig()
	Engine := new(consensus.FakeEngine)
	recoveredState, err := NewMinorBlockChain(env.db, nil, ethParams.TestChainConfig, fakeClusterConfig, Engine, vm.Config{}, nil, 2|0, nil)
	checkErr(err)
	err = recoveredState.InitFromRootBlock(rootBlock)
	checkErr(err)

	genesis := shardState.GetBlockByNumber(0)
	ifEqual(recoveredState.rootTip.Hash().String(), rootBlock.Hash().String())
	ifEqual(recoveredState.CurrentBlock().Hash().String(), recoveredState.CurrentHeader().Hash().String())
	ifEqual(recoveredState.CurrentHeader().Hash().String(), genesis.Hash().String())

	ifEqual(true, recoveredState.confirmedHeaderTip == nil)
	ifEqual(recoveredState.CurrentBlock().GetMetaData().Hash().String(), genesis.(*types.MinorBlock).GetMetaData().Hash().String())

	currState := recoveredState.currentEvmState

	CURRState, err := recoveredState.State()
	checkErr(err)

	roo1, err := currState.Commit(true)
	root2, err := CURRState.Commit(true)
	ifEqual(genesis.(*types.MinorBlock).GetMetaData().Root.String(), roo1.String())
	ifEqual(roo1.String(), root2.String())
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
	b1, _, err = shardState.FinalizeAndAddBlock(b1)
	checkErr(err)

	evmState, reps, err := shardState.runBlock(b1)
	checkErr(err)
	b1.Finalize(reps, evmState.IntermediateRoot(true), evmState.GetGasUsed(), evmState.GetXShardReceiveGasUsed(), b1.Header().CoinbaseAmount.Value)

	b1Meta := b1.Meta()
	b1Meta.Root = common.Hash{}
	b1 = types.NewMinorBlock(b1.Header(), b1Meta, b1.Transactions(), nil, nil)

	rawdb.DeleteMinorBlock(shardState.db, b1.Hash())
	_, _, err = shardState.InsertChain([]types.IBlock{b1})
	ifEqual(errors.New("meta hash is not match"), err)

}

func TestNotUpdateTipOnRootFork(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, nil, nil, nil)

	m1 := shardState.GetBlockByNumber(0)

	r1 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r2 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r1.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r1.Finalize(nil, nil)

	err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Header().Time + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil)
	ifEqual(false, reflect.DeepEqual(r1.Header().Hash().String(), r2.Header().Hash().String()))

	err = shardState.AddRootBlock(r2)

	checkErr(err)
	ifEqual(shardState.rootTip.Hash().String(), r1.Header().Hash().String())

	m2 := m1.(*types.MinorBlock).CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	m2Header := m2.Header()
	m2Header.PrevRootBlockHash = r2.Header().Hash()
	m2 = types.NewMinorBlock(m2Header, m2.Meta(), m2.Transactions(), nil, nil)

	m2, _, err = shardState.FinalizeAndAddBlock(m2)
	checkErr(err)

	ifEqual(shardState.GetBlockByHash(m2.Hash()).Hash().String(), m2.Header().Hash().String())
	ifEqual(shardState.CurrentHeader().Hash().String(), m1.IHeader().Hash().String())
}

func TestAddRootBlockRevertHeaderTip(t *testing.T) {

	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	newGenesisMinorQuarkash := uint64(10000000)
	env := setUp(&acc1, &newGenesisMinorQuarkash, nil)
	fakeShardID := uint32(0)
	shardState := createDefaultShardState(env, &fakeShardID, nil, nil, nil)

	m1 := shardState.GetBlockByNumber(0)
	m2 := shardState.CurrentBlock().CreateBlockToAppend(nil, nil, &acc1, nil, nil, nil, nil)
	m2, _, err = shardState.FinalizeAndAddBlock(m2)
	checkErr(err)

	r1 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r2 := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil)
	r1.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r1.Finalize(nil, nil)

	err = shardState.AddRootBlock(r1)
	checkErr(err)

	r2.AddMinorBlockHeader(m1.IHeader().(*types.MinorBlockHeader))
	r2Header := r2.Header()
	r2Header.Time = r1.Header().Time + 1
	r2 = types.NewRootBlock(r2Header, r2.MinorBlockHeaders(), nil)
	r2.Finalize(nil, nil)
	ifEqual(false, reflect.DeepEqual(r1.Header().Hash().String(), r2.Header().Hash().String()))
	err = shardState.AddRootBlock(r2)
	checkErr(err)
	ifEqual(shardState.rootTip.Hash().String(), r1.Header().Hash().String())

	m3, err := shardState.CreateBlockToMine(nil, &acc1, nil)
	checkErr(err)
	ifEqual(m3.Header().PrevRootBlockHash.String(), r1.Header().Hash().String())
	m3, _, err = shardState.FinalizeAndAddBlock(m3)
	checkErr(err)

	r3 := r2.Header().CreateBlockToAppend(nil, nil, &acc1, nil, nil)
	r3.AddMinorBlockHeader(m2.Header())
	r3.Finalize(nil, nil)
	err = shardState.AddRootBlock(r3)
	checkErr(err)

	ifEqual(shardState.rootTip.Hash().String(), r3.Header().Hash().String())
	ifEqual(shardState.CurrentHeader().Hash().String(), m2.Header().Hash().String())
	ifEqual(shardState.CurrentBlock().Hash().String(), m2.Header().Hash().String())

}
