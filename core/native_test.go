package core

import (
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"bou.ke/monkey"
	
	"github.com/QuarkChain/goquarkchain/params"
	ethParams "github.com/ethereum/go-ethereum/params"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/stretchr/testify/assert"
)

var (
	testShardCoinbaseAmount2 = new(big.Int).Mul(new(big.Int).SetUint64(200), jiaozi)
)

func TestNativeTokenTransfer(t *testing.T) {
	dirname, err := ioutil.TempDir(os.TempDir(), "qkcdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	testDBPath[1] = dirname
	QETH := common.TokenIDEncode("QETH")
	QKC := common.TokenIDEncode("QKC")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatEmptyAddress(0)
	acc3 := account.CreatEmptyAddress(0)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(10000000)
	testGenesisMinorTokenBalance["QETH"] = big.NewInt(99999)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	val := big.NewInt(12345)
	gas := uint64(21000)
	gasPrice := uint64(1)
	tx1 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, val, &gas, &gasPrice, nil, nil, nil, &QETH)
	error := shardState.AddTx(tx1)
	if error != nil {
		t.Errorf("addTx error: %v", error)
	}
	b1, _ := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	shardState.FinalizeAndAddBlock(b1)
	assert.Equal(t, shardState.CurrentHeader(), b1.Header())
	tokenBalance, _ := shardState.GetBalance(id1.GetRecipient(), nil)
	tokenBalance2, _ := shardState.GetBalance(acc2.Recipient, nil)
	tokenBalance3, _ := shardState.GetBalance(acc2.Recipient, nil)
	qkcb := tokenBalance.GetTokenBalance(QKC)
	assert.Equal(t, qkcb, big.NewInt(10000000-21000))
	assert.Equal(t, tokenBalance.GetTokenBalance(QETH), big.NewInt(99999-12345))
	assert.Equal(t, tokenBalance2.GetTokenBalance(QETH), big.NewInt(12345))
	reward := new(big.Int).Add(testShardCoinbaseAmount, big.NewInt(21000)).Uint64()
	assert.Equal(t, tokenBalance3.GetTokenBalance(QKC), afterTax(reward, shardState))
	tTxList, _, err := shardState.GetTransactionByAddress(acc1, nil, nil, 0)
	if err != nil {
		t.Errorf("GetTransactionByAddress error :%v", err)
	}
	assert.Equal(t, len(tTxList), 1)
	assert.Equal(t, tTxList[0].Value, serialize.Uint256{Value: big.NewInt(12345)})
	assert.Equal(t, tTxList[0].GasTokenID, QKC)
	assert.Equal(t, tTxList[0].TransferTokenID, QETH)
	tTxList, _, err = shardState.GetTransactionByAddress(acc2, nil, nil, 0)
	if err != nil {
		t.Errorf("GetTransactionByAddress error :%v", err)
	}
	assert.Equal(t, tTxList[0].Value, serialize.Uint256{Value: big.NewInt(12345)})
	assert.Equal(t, tTxList[0].GasTokenID, QKC)
	assert.Equal(t, tTxList[0].TransferTokenID, QETH)
}

func TestNativeTokenTransferValueSuccess(t *testing.T) {
	MALICIOUS0 := common.TokenIDEncode("MALICIOUS0")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(10000000)
	testGenesisMinorTokenBalance["MALICIOUS0"] = big.NewInt(0)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	val := big.NewInt(0)
	gas := uint64(21000)
	gasPrice := uint64(1)
	tx1 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc1, val, &gas, &gasPrice, nil, nil, nil, &MALICIOUS0)
	error := shardState.AddTx(tx1)
	assert.Error(t, error)
}

func TestDisallowedUnknownToken(t *testing.T) {
	MALICIOUS0 := common.TokenIDEncode("MALICIOUS0")
	MALICIOUS1 := common.TokenIDEncode("MALICIOUS1")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(10000000)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	val := big.NewInt(0)
	gas := uint64(21000)
	gasPrice := uint64(1)
	tx1 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc1, val, &gas, &gasPrice, nil, nil, nil, &MALICIOUS0)
	assert.Error(t, shardState.AddTx(tx1))

	tx2 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc1, val, &gas, &gasPrice, nil, nil, nil, &MALICIOUS1)
	assert.Error(t, shardState.AddTx(tx2))
}

func TestNativeTokenGas(t *testing.T) {
	monkey.Patch(PayNativeTokenAsGas, func(a vm.StateDB, b *ethParams.ChainConfig, c uint64, d uint64, gasPrice *big.Int) (uint8, *big.Int, error) {
		return 100, gasPrice, nil
	})
	monkey.Patch(GetGasUtilityInfo, func(a vm.StateDB, b *ethParams.ChainConfig, c uint64, gasPrice *big.Int) (uint8, *big.Int, error) {
		return 100, gasPrice, nil
	})
	defer monkey.UnpatchAll()
	dirname, err := ioutil.TempDir(os.TempDir(), "qkcdb_test_")
	if err != nil {
		panic("failed to create test file: " + err.Error())
	}
	testDBPath[1] = dirname
	qeth := common.TokenIDEncode("QETH")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	id2, _ := account.CreatRandomIdentity()
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	// Miner
	id3, _ := account.CreatRandomIdentity()
	acc3 := account.CreatAddressFromIdentity(id3, 0)
	testGenesisMinorTokenBalance["QETH"] = big.NewInt(10000000)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(10000000)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	tru := true
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, &tru)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	val := big.NewInt(12345)
	gas := uint64(21000)
	tx1 := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, val, &gas, nil, nil, nil, &qeth, &qeth)
	assert.NoError(t, shardState.AddTx(tx1))
	block, _ := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, len(block.Transactions()), 1)
	_, _, _ = shardState.FinalizeAndAddBlock(block)
	assert.Equal(t, shardState.CurrentHeader(), block.Header())
	b1, _ := shardState.GetBalance(acc1.Recipient, nil)
	b2, _ := shardState.GetBalance(acc2.Recipient, nil)
	bb := new(big.Int).Sub(big.NewInt(10000000), big.NewInt(12345))
	assert.Equal(t, b1.GetTokenBalance(qeth), new(big.Int).Sub(bb, big.NewInt(21000)))
	assert.Equal(t, b2.GetTokenBalance(qeth), big.NewInt(12345))
	//after-tax coinbase + tx fee should only be in QKC
	b3, _ := shardState.GetBalance(acc3.Recipient, nil)
	at := afterTax(new(big.Int).Add(shardState.shardConfig.CoinbaseAmount, params.DefaultInShardTxGasLimit).Uint64(), shardState)
	assert.Equal(t, b3.GetTokenBalance(env.clusterConfig.Quarkchain.GetDefaultChainTokenID()), at)
	txList, _, err := shardState.GetTransactionByAddress(acc1, nil, nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, uint64(12345), txList[0].Value.Value.Uint64())
	assert.Equal(t, qeth, txList[0].GasTokenID)
	assert.Equal(t, qeth, txList[0].TransferTokenID)
	txList, _, err = shardState.GetTransactionByAddress(acc2, nil, nil, 0)
	assert.NoError(t, err)
	assert.Equal(t, uint64(12345), txList[0].Value.Value.Uint64())
	assert.Equal(t, qeth, txList[0].GasTokenID)
	assert.Equal(t, qeth, txList[0].TransferTokenID)
}

func TestXshardNativeTokenSent(t *testing.T) {
	QETHXX := common.TokenIDEncode("QETHXX")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatEmptyAddress(1)
	acc3 := account.CreatEmptyAddress(0)
	testGenesisMinorTokenBalance["QETHXX"] = big.NewInt(999999)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(10000000)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	genesisMinorQuarkHash := big.NewInt(10000000).Uint64()
	env1 := getTestEnv(&acc1, &genesisMinorQuarkHash, nil, nil, nil, nil, nil)
	shardId := uint32(1)
	shardState1 := createDefaultShardState(env1, &shardId, nil, nil, nil)
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	shardState.AddRootBlock(rootBlock)
	val := big.NewInt(888888)
	gas := new(big.Int).Add(big.NewInt(9000), big.NewInt(21000)).Uint64()
	QKC := common.TokenIDEncode("QKC")
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, val, &gas, nil, nil, nil, &QKC, &QETHXX)
	shardState.AddTx(tx)
	b1, _ := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	evmState, _ := shardState.State()
	assert.Equal(t, evmState.GetGasUsed(), big.NewInt(0))
	shardState.FinalizeAndAddBlock(b1)
	evmState, _ = shardState.State()
	assert.Equal(t, len(evmState.GetXShardList()), 1)
	deposit := &types.CrossShardTransactionDeposit{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{TxHash: tx.Hash(), From: acc1, To: acc2, Value: &serialize.Uint256{Value: val}, GasPrice: &serialize.Uint256{Value: big.NewInt(1)}, GasTokenID: QKC, TransferTokenID: QETHXX}}
	assert.NotEqual(t, evmState.GetXShardList()[0], deposit)
	balance, _ := shardState.GetBalance(acc1.Recipient, nil)
	balance.GetTokenBalance(QKC)
	assert.Equal(t, balance.GetTokenBalance(QKC), big.NewInt(10000000-21000-9000))
	assert.Equal(t, balance.GetTokenBalance(QETHXX), big.NewInt(999999-888888))
	assert.Equal(t, evmState.GetGasUsed(), big.NewInt(21000))
}

func TestXshardNativeTokenReceived(t *testing.T) {
	QETHXX := common.TokenIDEncode("QETHXX")
	QKC := common.TokenIDEncode("QKC")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 16)
	acc3, _ := account.CreatRandomAccountWithFullShardKey(0)
	testGenesisMinorTokenBalance["QETHXX"] = big.NewInt(999999)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(10000000)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	shardSize := uint32(64)
	env0 := getTestEnv(&acc1, nil, nil, &shardSize, nil, nil, nil)
	env1 := getTestEnv(&acc1, nil, nil, &shardSize, nil, nil, nil)
	shardID := uint32(16)
	shardState0 := createDefaultShardState(env0, nil, nil, nil, nil)
	shardState1 := createDefaultShardState(env1, &shardID, nil, nil, nil)
	// Add a root block to allow later minor blocks referencing this root block to
	// be broadcasted
	rootBlock := shardState0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.EmptyHash)
	_, err := shardState0.AddRootBlock(rootBlock)
	checkErr(err)
	_, err = shardState1.AddRootBlock(rootBlock)
	checkErr(err)
	// Add one block in shard 0
	b0, err := shardState0.CreateBlockToMine(nil, nil, nil, nil, nil)
	checkErr(err)
	b0, _, err = shardState0.FinalizeAndAddBlock(b0)
	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b1Header := b1.Header()
	b1Header.PrevRootBlockHash = rootBlock.Hash()
	b1 = types.NewMinorBlockWithHeader(b1Header, b1.Meta())
	val := new(big.Int).SetUint64(888888)
	gas := uint64(30000)
	gasPrice := uint64(2)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, val, &gas, &gasPrice, nil, nil, &QKC, &QETHXX)
	b1.AddTx(tx)
	// Add a x-shard tx from remote peer
	deposit := types.CrossShardTransactionDeposit{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
		TxHash:          tx.Hash(),
		From:            acc2,
		To:              acc1,
		Value:           &serialize.Uint256{Value: val},
		GasPrice:        &serialize.Uint256{Value: big.NewInt(2)},
		TransferTokenID: QETHXX,
		GasTokenID:      shardState0.GetGenesisToken(),
	}}
	txL := types.CrossShardTransactionDepositList{}
	txL.TXList = append(txL.TXList, &deposit)
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txL)
	//Create a root block containing the block with the x-shard tx
	rootBlock = shardState0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.EmptyHash)
	_, err = shardState0.AddRootBlock(rootBlock)
	checkErr(err)

	//Add b0 and make sure all x-shard tx's are added
	b2, err := shardState0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	checkErr(err)
	b2, _, err = shardState0.FinalizeAndAddBlock(b2)
	checkErr(err)
	acc1Value := shardState0.currentEvmState.GetBalance(acc1.Recipient, QETHXX)
	assert.Equal(t, acc1Value, new(big.Int).Add(big.NewInt(999999), big.NewInt(888888)))
	// Half collected by root
	acc3Value := shardState0.currentEvmState.GetBalance(acc3.Recipient, shardState0.GetGenesisToken())
	acc3Should := new(big.Int).Add(testShardCoinbaseAmount, new(big.Int).SetUint64(9000*2))
	acc3Should = new(big.Int).Div(acc3Should, new(big.Int).SetUint64(2))
	assert.Equal(t, acc3Should.String(), acc3Value.String())
	//X-shard gas used
	assert.Equal(t, shardState0.currentEvmState.GetXShardReceiveGasUsed().Uint64(), uint64(9000))
}

func TestXshardNativeTokenGasSent(t *testing.T) {
	monkey.Patch(PayNativeTokenAsGas, func(a vm.StateDB, b *ethParams.ChainConfig, c uint64, d uint64, gasPrice *big.Int) (uint8, *big.Int, error) {
		return 100, gasPrice, nil
	})
	monkey.Patch(GetGasUtilityInfo, func(a vm.StateDB, b *ethParams.ChainConfig, c uint64, gasPrice *big.Int) (uint8, *big.Int, error) {
		return 100, gasPrice, nil
	})
	defer monkey.UnpatchAll()
	qeth := common.TokenIDEncode("QETHXX")
	qkc := common.TokenIDEncode("QKC")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1)
	acc3 := account.CreatEmptyAddress(0)
	testGenesisMinorTokenBalance["QETHXX"] = big.NewInt(9999999)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(9999999)
	tru := true
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, &tru)
	shardId := uint32(0)
	shardState := createDefaultShardState(env, &shardId, nil, nil, nil)
	testGenesisMinorTokenBalance = make(map[string]*big.Int)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	env1 := getTestEnv(&acc1, nil, nil, nil, nil, nil, nil)
	shardId1 := uint32(1)
	shardState1 := createDefaultShardState(env1, &shardId1, nil, nil, nil)
	rootBlock := shardState.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.EmptyHash)
	shardState.AddRootBlock(rootBlock)
	val := big.NewInt(8888888)
	gas := new(big.Int).Add(big.NewInt(9000), big.NewInt(21000)).Uint64()
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, val, &gas, nil, nil, nil, &qeth, &qeth)
	shardState.AddTx(tx)
	shardState.GetTransactionCount(acc1.Recipient, nil)
	b1, _ := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	evmState, _ := shardState.State()
	assert.Equal(t, evmState.GetGasUsed(), big.NewInt(0))
	shardState.FinalizeAndAddBlock(b1)
	evmState, _ = shardState.State()
	assert.Equal(t, len(evmState.GetXShardList()), 1)
	deposit := &types.CrossShardTransactionDeposit{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{
		TxHash: tx.Hash(), From: acc1, To: acc2, Value: &serialize.Uint256{Value: val}, GasPrice: &serialize.Uint256{Value: big.NewInt(1)},
		GasRemained: &serialize.Uint256{Value: big.NewInt(0)}, GasTokenID: qkc, TransferTokenID: qeth}, RefundRate: 100}
	assert.Equal(t, deposit, evmState.GetXShardList()[0])
	balance, _ := shardState.GetBalance(acc1.Recipient, nil)
	balance3, _ := shardState.GetBalance(acc3.Recipient, nil)
	assert.Equal(t, balance.GetTokenBalance(qeth), big.NewInt(9999999-8888888-21000-9000))
	//Make sure the xshard gas is not used by local block
	assert.Equal(t, evmState.GetGasUsed(), big.NewInt(21000))
	//block coinbase for mining is still in genesis_token + xshard fee
	assert.Equal(t, balance3.GetTokenBalance(qkc), afterTax(shardState.shardConfig.CoinbaseAmount.Uint64()+params.DefaultInShardTxGasLimit.Uint64(), shardState))
}

func TestXshardNativeTokenGasReceived(t *testing.T) {
	QETHXX := common.TokenIDEncode("QETHXX")
	QKC := common.TokenIDEncode("QKC")
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 16)
	acc3, _ := account.CreatRandomAccountWithoutFullShardKey()
	testGenesisMinorTokenBalance["QETHXX"] = big.NewInt(999999)
	testGenesisMinorTokenBalance["QKC"] = big.NewInt(10000000)
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	shardSize := uint32(64)
	env0 := getTestEnv(&acc1, nil, nil, &shardSize, nil, nil, nil)
	env1 := getTestEnv(&acc1, nil, nil, &shardSize, nil, nil, nil)
	shardId := uint32(0)
	shardState0 := createDefaultShardState(env0, &shardId, nil, nil, nil)
	shardId1 := uint32(16)
	shardState1 := createDefaultShardState(env1, &shardId1, nil, nil, nil)
	rootBlock := shardState0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(shardState0.CurrentBlock().Header())
	rootBlock.AddMinorBlockHeader(shardState1.CurrentBlock().Header())
	rootBlock.Finalize(nil, nil, common.EmptyHash)
	shardState0.AddRootBlock(rootBlock)
	shardState1.AddRootBlock(rootBlock)
	b0, _ := shardState0.CreateBlockToMine(nil, nil, nil, nil, nil)
	shardState0.FinalizeAndAddBlock(b0)
	b1 := shardState1.CurrentBlock().CreateBlockToAppend(nil, nil, nil, nil, nil, nil, nil, nil, nil)
	b1Header := b1.Header()
	b1Header.PrevRootBlockHash = rootBlock.Hash()
	b1 = types.NewMinorBlockWithHeader(b1Header, b1.Meta())
	val := big.NewInt(888888)
	gas := new(big.Int).Add(big.NewInt(9000), big.NewInt(21000)).Uint64()
	gasPrice := uint64(2)
	tx := createTransferTransaction(shardState1, id1.GetKey().Bytes(), acc2, acc1, val, &gas, &gasPrice, nil, nil, &QETHXX, &QETHXX)
	b1.AddTx(tx)
	deposit := types.CrossShardTransactionDeposit{CrossShardTransactionDepositV0: types.CrossShardTransactionDepositV0{TxHash: tx.Hash(),
		From: acc1, To: acc2, Value: &serialize.Uint256{Value: val}, GasPrice: &serialize.Uint256{Value: big.NewInt(2)}, GasTokenID: QKC, TransferTokenID: QETHXX}}
	txL := make([]*types.CrossShardTransactionDeposit, 0)
	txL = append(txL, &deposit)
	txList := types.CrossShardTransactionDepositList{TXList: txL}
	shardState0.AddCrossShardTxListByMinorBlockHash(b1.Header().Hash(), txList)
	rootBlock = shardState0.GetRootTip().Header().CreateBlockToAppend(nil, nil, nil, nil, nil)
	rootBlock.AddMinorBlockHeader(b0.Header())
	rootBlock.AddMinorBlockHeader(b1.Header())
	rootBlock.Finalize(nil, nil, common.EmptyHash)
	shardState0.AddRootBlock(rootBlock)
	b2, _ := shardState0.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	shardState0.FinalizeAndAddBlock(b2)
	balance1, _ := shardState0.GetBalance(acc1.Recipient, nil)
	assert.Equal(t, balance1.GetTokenBalance(QETHXX), new(big.Int).Add(big.NewInt(999999), big.NewInt(888888)))
	evmState, _ := shardState0.State()
	assert.Equal(t, evmState.GetGasUsed(), big.NewInt(9000))
	//Half coinbase collected by root + tx fee
	balance3, _ := shardState0.GetBalance(acc3.Recipient, nil)
	assert.Equal(t, balance3.GetTokenBalance(QKC), afterTax(testShardCoinbaseAmount.Uint64()+params.GtxxShardCost.Uint64()*2, shardState0))
}

func TestContractSuicide(t *testing.T) {
	//Kill Call Data: 0x41c0e1b5
	id1, _ := account.CreatRandomIdentity()
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc3 := account.CreatEmptyAddress(0)
	testGenesisMinorTokenBalance["QETHXX"] = big.NewInt(999999)
	testGenesisMinorTokenBalance["QKC"] = testShardCoinbaseAmount2
	defer func() {
		testGenesisMinorTokenBalance = make(map[string]*big.Int)
	}()
	env := getTestEnv(&acc1, nil, nil, nil, nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	// 1. create contract
	BYTECODE := "6080604052348015600f57600080fd5b5060948061001e6000396000f3fe6080604052600436106039576000357c01000000000000000000000000000000000000000000000000000000009004806341c0e1b514603b575b005b348015604657600080fd5b50604d604f565b005b3373ffffffffffffffffffffffffffffffffffffffff16fffea165627a7a7230582034cc4e996685dcadcc12db798751d2913034a3e963356819f2293c3baea4a18c0029"
	tx, _ := CreateContract(shardState, id1.GetKey(), acc1, acc1.FullShardKey, BYTECODE)
	shardState.AddTx(tx)
	b1, _ := shardState.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.Equal(t, len(b1.Transactions()), 1)
	shardState.FinalizeAndAddBlock(b1)
}
