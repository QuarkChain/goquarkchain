package core

import (
	"encoding/hex"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	ethParams "github.com/ethereum/go-ethereum/params"
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
	currStateDB, err := shardState.StateAt(shardState.CurrentBlock().Meta().Root)
	checkErr(err)
	assert.Equal(t, currStateDB.GetAccountStatus(acc1.Recipient), true)  //check status
	assert.Equal(t, currStateDB.GetAccountStatus(acc2.Recipient), false) //check status
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
	currState, err := shardState.StateAt(shardState.CurrentBlock().Meta().Root)
	assert.Equal(t, currState.GetAccountStatus(acc2.Recipient), false)    //check status
	assert.Equal(t, currState.GetAccountStatus(acc1.Recipient), false)    //check status
	assert.Equal(t, currState.GetAccountStatus(superAcc.Recipient), true) //check status
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

	tx = createTransferTransaction(shardState, superID.GetKey().Bytes(), superAcc, acc1, new(big.Int).SetUint64(100000), &fakeGas, nil, nil, []byte{1})
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

func TestContractCall(t *testing.T) {
	var (
		id1, _ = account.CreatRandomIdentity()
		addr1  = account.CreatAddressFromIdentity(id1, 0)
		db     = ethdb.NewMemDatabase()
		// this code generates a log
		code          = common.Hex2Bytes("60606040525b7f24ec1d3ff24c2f6ff210738839dbc339cd45a5294d85c79361016243157aae7b60405180905060405180910390a15b600a8060416000396000f360606040526008565b00")
		clusterConfig = config.NewClusterConfig()
		gspec         = &Genesis{
			qkcConfig: clusterConfig.Quarkchain,
		}
		rootBlock = gspec.CreateRootBlock()
		signer    = types.NewEIP155Signer(uint32(gspec.qkcConfig.NetworkID))
	)
	superID, err := account.CreatRandomIdentity()
	superAcc := account.CreatAddressFromIdentity(superID, 0)
	params.SetSuperAccount(superAcc.Recipient)
	prvKey1, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	clusterConfig.Quarkchain.SkipRootDifficultyCheck = true
	clusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	clusterConfig.Quarkchain.SkipRootCoinbaseCheck = true
	ids := clusterConfig.Quarkchain.GetGenesisShardIds()
	addr0 := account.NewAddress(account.BytesToIdentityRecipient(common.Address{0}.Bytes()), 0)
	for _, v := range ids {
		addr := addr1.AddressInShard(v)
		shardConfig := clusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		shardConfig.Genesis.Alloc[addr] = big.NewInt(1000000)
		addr = addr0.AddressInShard(v)
		shardConfig.Genesis.Alloc[addr] = big.NewInt(0)
		addr = superAcc.AddressInShard(v)
		shardConfig.Genesis.Alloc[addr] = big.NewInt(100000000000000)

	}
	engine := &consensus.FakeEngine{}
	genesis := gspec.MustCommitMinorBlock(db, rootBlock, clusterConfig.Quarkchain.Chains[0].ShardSize|0)
	blockchain, _ := NewMinorBlockChain(db, nil, ethParams.TestChainConfig, clusterConfig, engine, vm.Config{}, nil, config.NewClusterConfig().Quarkchain.Chains[0].ShardSize|0)
	genesis, err = blockchain.InitGenesisState(rootBlock)
	if err != nil {
		panic(err)
	}
	defer blockchain.Stop()
	tempHash := common.Hash{}
	chain, _ := GenerateMinorBlockChain(ethParams.TestChainConfig, clusterConfig.Quarkchain, genesis, engine, db, 2, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {
		if i == 1 {
			tx, err := types.SignTx(types.NewEvmContractCreation(gen.TxNonce(addr1.Recipient), new(big.Int), 1000000, new(big.Int), 0, 0, 3, 0, code), signer, prvKey1)
			if err != nil {
				t.Fatalf("failed to create tx: %v", err)
			}
			tt := transEvmTxToTx(tx)
			tempHash = tt.Hash()
			gen.AddTx(config, tt)
		}
	})

	if _, err := blockchain.InsertChain(toMinorBlocks(chain)); err != nil {
		t.Fatalf("failed to insert chain: %v", err)
	}
	data, index, re := blockchain.GetTransactionReceipt(tempHash)
	assert.Equal(t, data.Transactions()[index].Hash(), tempHash)
	currState, err := blockchain.StateAt(blockchain.CurrentBlock().Meta().Root)
	checkErr(err)
	assert.Equal(t, true, currState.GetAccountStatus(re.ContractAddress))

	contractAddr := account.Address{
		Recipient:    re.ContractAddress,
		FullShardKey: addr1.FullShardKey,
	}
	tx := createTransferTransaction(blockchain, id1.GetKey().Bytes(), addr1, contractAddr, new(big.Int), nil, nil, nil, nil)
	err = blockchain.AddTx(tx)
	checkErr(err) //can send tx

	b2, err := blockchain.CreateBlockToMine(nil, &addr0, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.Header().Number, uint64(3))

	// Should succeed
	b2, res, err := blockchain.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, len(res), 1)

	fakeGas := uint64(30000)
	tx = createTransferTransaction(blockchain, superID.GetKey().Bytes(), superAcc, contractAddr, new(big.Int), &fakeGas, nil, nil, []byte{1})
	err = blockchain.AddTx(tx)
	checkErr(err) //can send tx

	b2, err = blockchain.CreateBlockToMine(nil, &addr0, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.Header().Number, uint64(4))

	// Should succeed
	b2, res, err = blockchain.FinalizeAndAddBlock(b2)
	checkErr(err)
	assert.Equal(t, len(res), 1)
	currState, err = blockchain.StateAt(b2.Meta().Root)
	assert.Equal(t, currState.GetAccountStatus(contractAddr.Recipient), false)

}
