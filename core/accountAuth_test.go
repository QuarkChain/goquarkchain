package core

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
)

func TestGenesisAccountStatus(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)

	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	acc2 := account.CreatAddressFromIdentity(id2, 0)

	fakeMoney := uint64(10000000)
	env := setUp([]account.Address{acc1}, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	currStateDB, err := shardState.StateAt(shardState.CurrentBlock().Meta().Root)
	assert.Equal(t, currStateDB.GetBalance(acc1.Recipient).Uint64(), fakeMoney) //check geneis's account's money
	assert.Equal(t, currStateDB.GetAccountStatus(acc1.Recipient), true)         //check status

	assert.Equal(t, currStateDB.GetBalance(acc2.Recipient).Uint64(), uint64(0)) //check other's account's monry
	assert.Equal(t, currStateDB.GetAccountStatus(acc2.Recipient), false)        //check status
}

func TestSendTxFailed(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)

	fakeMoney := uint64(10000000)
	env := setUp([]account.Address{acc1}, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	currState, err := shardState.State()
	checkErr(err)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	assert.Equal(t, err, ErrAuthToAccount)

	tx = createTransferTransaction(shardState, id2.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	assert.Equal(t, err, ErrAuthFromAccount)
}

func TestSendSuperAccountSucc(t *testing.T) {

	superID, err := account.CreatRandomIdentity()
	id1, err := account.CreatRandomIdentity()
	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	superAcc := account.CreatAddressFromIdentity(superID, 0)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	params.SetSuperAccount(superAcc.Recipient)

	fakeMoney := uint64(1000000000000)
	env := setUp([]account.Address{superAcc}, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	currState, err := shardState.State()
	checkErr(err)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	assert.Equal(t, err, ErrAuthFromAccount)

	tx = createTransferTransaction(shardState, superID.GetKey().Bytes(), superAcc, acc2, new(big.Int).SetUint64(1000000), &fakeGas, nil, nil, nil)
	currState.SetGasUsed(currState.GetGasLimit())
	err = shardState.AddTx(tx)
	checkErr(err)

	tNonce := uint64(1)
	tx = createTransferTransaction(shardState, superID.GetKey().Bytes(), superAcc, acc1, new(big.Int).SetUint64(100000), &fakeGas, nil, &tNonce, nil)
	currState.SetGasUsed(currState.GetGasLimit())
	err = shardState.AddTx(tx)
	checkErr(err)

	b2, err := shardState.CreateBlockToMine(nil, &superAcc, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 2)
	assert.Equal(t, b2.Header().Number, uint64(1))
	assert.Equal(t, currState.GetAccountStatus(acc1.Recipient), false)
	assert.Equal(t, currState.GetAccountStatus(acc2.Recipient), false)
	assert.Equal(t, currState.GetAccountStatus(superAcc.Recipient), true)
	// Should succeed
	b2, _, err = shardState.FinalizeAndAddBlock(b2)

	currState, err = shardState.StateAt(b2.Meta().Root)
	assert.Equal(t, currState.GetAccountStatus(acc1.Recipient), true)
	assert.Equal(t, currState.GetAccountStatus(acc2.Recipient), true)
	assert.Equal(t, currState.GetAccountStatus(superAcc.Recipient), true)

	tx = createTransferTransaction(shardState, id1.GetKey().Bytes(), acc1, acc2, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil)
	currState.SetGasUsed(currState.GetGasLimit())
	err = shardState.AddTx(tx)
	checkErr(err)

	tx = createTransferTransaction(shardState, id2.GetKey().Bytes(), acc2, acc1, new(big.Int).SetUint64(0), &fakeGas, nil, nil, nil)
	currState.SetGasUsed(currState.GetGasLimit())
	err = shardState.AddTx(tx)
	checkErr(err)

	b2, err = shardState.CreateBlockToMine(nil, &acc2, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 2)
	assert.Equal(t, b2.Header().Number, uint64(2))
	// Should succeed
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	currState, err = shardState.StateAt(b2.Meta().Root)
	checkErr(err)
	assert.Equal(t, currState.GetAccountStatus(acc1.Recipient), true)
	assert.Equal(t, currState.GetAccountStatus(acc2.Recipient), true)
	assert.Equal(t, currState.GetAccountStatus(superAcc.Recipient), true)

	tx = createTransferTransaction(shardState, superID.GetKey().Bytes(), superAcc, acc1, new(big.Int).SetUint64(100000), &fakeGas, nil, nil, []byte{'1'})
	currState.SetGasUsed(currState.GetGasLimit())
	err = shardState.AddTx(tx)
	checkErr(err)

	b2, err = shardState.CreateBlockToMine(nil, &acc2, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.Header().Number, uint64(3))
	// Should succeed
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	currState, err = shardState.StateAt(b2.Meta().Root)
	checkErr(err)
	assert.Equal(t, currState.GetAccountStatus(acc1.Recipient), false)
	assert.Equal(t, currState.GetAccountStatus(acc2.Recipient), true)
	assert.Equal(t, currState.GetAccountStatus(superAcc.Recipient), true)

}

func TestAsMiner(t *testing.T) {
	superID, err := account.CreatRandomIdentity()
	id1, err := account.CreatRandomIdentity()
	id2, err := account.CreatRandomIdentity()
	checkErr(err)
	superAcc := account.CreatAddressFromIdentity(superID, 0)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	params.SetSuperAccount(superAcc.Recipient)

	fakeMoney := uint64(1000000000000)
	env := setUp([]account.Address{superAcc}, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	fakeGas := uint64(50000)
	tx := createTransferTransaction(shardState, superID.GetKey().Bytes(), superAcc, acc2, new(big.Int).SetUint64(12345), &fakeGas, nil, nil, nil)
	currState, err := shardState.State()
	checkErr(err)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	checkErr(err)

	b2, err := shardState.CreateBlockToMine(nil, &superAcc, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.Header().Number, uint64(1))
	// Should succeed
	b2, _, err = shardState.FinalizeAndAddBlock(b2)
	currState, err = shardState.StateAt(b2.Meta().Root)
	checkErr(err)
	assert.Equal(t, currState.GetAccountStatus(acc1.Recipient), false)
	assert.Equal(t, currState.GetAccountStatus(acc2.Recipient), true)
	assert.Equal(t, currState.GetAccountStatus(superAcc.Recipient), true)

	b2, err = shardState.CreateBlockToMine(nil, &acc2, nil)
	checkErr(err)

	b2, err = shardState.CreateBlockToMine(nil, &acc1, nil)
	assert.Equal(t, err, ErrAccountNotBeMiner)

}
