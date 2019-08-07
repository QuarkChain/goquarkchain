//+build integration test

package test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/params"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

//Test the cross shard transactions are broadcasted to the destination shards
func TestBroadcastCrossShardTransactionsWithExtraGas(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	id2, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(1)
	assert.NoError(t, err)
	acc4, err := account.CreatRandomAccountWithFullShardKey(1)
	assert.NoError(t, err)
	chainSize := 2
	shardSize := 2
	_, cluster := CreateClusterList(1, uint32(chainSize), uint32(shardSize), 2, nil)
	c := cluster[0]
	alloc := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(1000000)}
	for _, fsId := range c.clstrCfg.Quarkchain.GetGenesisShardIds() {
		shardCfg := c.clstrCfg.Quarkchain.GetShardConfigByFullShardID(fsId)
		acc1s := acc1.AddressInShard(fsId)
		shardCfg.Genesis.Alloc[acc1s] = alloc
	}
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()

	master := c.GetMaster()
	slaves := c.GetSlavelist()
	minorBlockChainA := c.GetShardState(2 | 0)
	minorBlockChainB := c.GetShardState(2 | 1)
	//genesisToken := minorBlockChainA.Config().GenesisToken

	rb, _, err := master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	val := big.NewInt(54321)
	gas := uint64(21000) + params.GtxxShardCost.Uint64() + uint64(12345)
	gasPrice := uint64(1)
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc3, val, &gas, &gasPrice, nil)
	err = slaves[0].AddTx(tx1)
	assert.NoError(t, err)
	b1, err := minorBlockChainA.CreateBlockToMine(nil, &acc2, nil)
	assert.NoError(t, err)
	err = c.GetShard(2 | 0).AddMinorBlock(b1)
	assert.NoError(t, err)
	ad, err := master.GetPrimaryAccountData(&acc1, nil)
	assert.NoError(t, err)
	assert.Equal(t, int(1000000-val.Uint64()-gas), int(ad.Balance.Int64()))
	time.Sleep(100 * time.Millisecond)
	//rb = minorBlockChainA.GetRootTip().CreateBlockToAppend(nil, nil, &acc1, nil, nil)
	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	acc1s1 := acc1.AddressInShard(1)
	ad, err = master.GetPrimaryAccountData(&acc1s1, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1000000, int(ad.Balance.Uint64()))
	//b2 should include the withdraw of tx1
	b2, err := minorBlockChainB.CreateBlockToMine(nil, &acc4, nil)
	assert.NoError(t, err)
	err = c.GetShard(2 | 1).AddMinorBlock(b2)
	assert.NoError(t, err)
	acc3b, err := master.GetPrimaryAccountData(&acc3, nil)
	assert.NoError(t, err)
	assert.Equal(t, 54321, int(acc3b.Balance.Uint64()))
	acc11, err := master.GetPrimaryAccountData(&acc1s1, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1012345, int(acc11.Balance.Uint64()))
}

func TestCrossShardContractCall(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	id2, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1<<16)
	acc3 := account.CreatAddressFromIdentity(id2, 0)
	acc4 := account.CreatAddressFromIdentity(id2, 1<<16)
	storageKeyStr := core.ZFill64(hex.EncodeToString(acc4.Recipient[:])) + core.ZFill64("1")
	storageKeyBytes, err := hex.DecodeString(storageKeyStr)
	assert.NoError(t, err)
	storageKeyHash := crypto.Keccak256Hash(storageKeyBytes)
	shardSize := 1
	chainSize := 8
	_, cluster := CreateClusterList(1, uint32(chainSize), uint32(shardSize), 2, nil)
	c := cluster[0]
	alloc := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(100000000)}
	//Enable xshard receipt
	c.clstrCfg.Quarkchain.XShardAddReceiptTimestamp = 1
	for i := 0; i < chainSize; i++ {
		fsId := i<<16 | shardSize | 0
		shardCfg := c.clstrCfg.Quarkchain.GetShardConfigByFullShardID(uint32(fsId))
		shardCfg.CoinbaseAmount = big.NewInt(1000000)
		shardCfg.Genesis.Alloc[acc1] = alloc
		shardCfg.Genesis.Alloc[acc2] = alloc
	}
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()
	minorBlockChainA := c.GetShardState(1)
	minorBlockChainB := c.GetShardState(1<<16 + 1)

	//Add a root block first so that later minor blocks referring to this root
	// can be broadcasted to other shards
	master := c.master
	slaves := c.GetSlavelist()
	rb, _, err := master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	tx0, err := core.CreateContract(minorBlockChainB, id1.GetKey(), acc2, acc2.FullShardKey, core.CONTRACT)
	assert.NoError(t, err)
	err = slaves[1].AddTx(tx0)
	assert.NoError(t, err)
	b0, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b0)
	assert.NoError(t, err)
	b1, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil)
	assert.NoError(t, err)
	err = c.GetShard(1<<16 + 1).AddMinorBlock(b1)
	assert.NoError(t, err)

	val := big.NewInt(1500000)
	gas := uint64(21000)
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc3, val, &gas, nil, nil)
	err = slaves[0].AddTx(tx1)
	assert.NoError(t, err)
	b00, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b00)
	assert.NoError(t, err)
	ad, err := master.GetPrimaryAccountData(&acc3, nil)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1500000), ad.Balance.Uint64())
	_, _, receipt, err := master.GetTransactionReceipt(tx0.Hash(), b1.Header().Branch)
	assert.NoError(t, err)
	assert.Equal(t, uint64(0x1), receipt.Status)
	contractAddress := account.NewAddress(receipt.ContractAddress, receipt.ContractFullShardId)
	b1n := b1.Header().Number
	result, err := master.GetStorageAt(&contractAddress, storageKeyHash, &b1n)
	assert.NoError(t, err)
	v0 := "0000000000000000000000000000000000000000000000000000000000000000"
	assert.Equal(t, v0, hex.EncodeToString(result.Bytes()))
	//should include b1
	time.Sleep(100 * time.Millisecond)
	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	// call the contract with insufficient gas
	data, err := hex.DecodeString("c2e171d7")
	assert.NoError(t, err)
	value, gasPrice, gas := big.NewInt(0), uint64(1), uint64(30000+500)
	tx2 := core.CreateCallContractTx(minorBlockChainA, id2.GetKey().Bytes(), acc3, contractAddress,
		value, &gas, &gasPrice, nil, data)
	err = slaves[0].AddTx(tx2)
	assert.NoError(t, err)
	b2, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b2)
	assert.NoError(t, err)
	//should include b2
	time.Sleep(100 * time.Millisecond)
	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	//The contract should be called
	b3, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil)
	assert.NoError(t, err)
	err = minorBlockChainB.AddBlock(b3)
	assert.NoError(t, err)
	b3n := b3.Header().Number
	result, err = master.GetStorageAt(&contractAddress, storageKeyHash, &b3n)
	assert.NoError(t, err)
	assert.Equal(t, v0, hex.EncodeToString(result.Bytes()))
	ad, err = master.GetPrimaryAccountData(&acc4, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, int(ad.Balance.Int64()))
	fmt.Printf("srch receipt %x\n", tx2.Hash())
	_, _, receipt, err = master.GetTransactionReceipt(tx2.Hash(), b3.Header().Branch)
	assert.NoError(t, err)
	fmt.Printf("find receipt %x, %v\n", receipt.TxHash, receipt)
	//TODO enable xshard receipt retrieve
	//make sure receipt actually found
	//assert.Equal(t, tx2.Hash(), receipt.TxHash)
	//assert.Equal(t, uint64(0x0), receipt.Status)
	//call the contract with enough gas
	assert.NoError(t, err)
	value, gasPrice, gas = big.NewInt(0), uint64(1), uint64(30000+700000)
	tx3 := core.CreateCallContractTx(minorBlockChainA, id2.GetKey().Bytes(), acc3, contractAddress,
		value, &gas, &gasPrice, nil, data)
	err = slaves[0].AddTx(tx3)
	assert.NoError(t, err)
	b4, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b4)
	assert.NoError(t, err)
	//should include b4
	time.Sleep(100 * time.Millisecond)
	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	//The contract should be called
	b5, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil)
	assert.NoError(t, err)
	err = minorBlockChainB.AddBlock(b5)
	assert.NoError(t, err)
	b5n := b5.Header().Number
	result, err = master.GetStorageAt(&contractAddress, storageKeyHash, &b5n)
	assert.NoError(t, err)
	v1 := "000000000000000000000000000000000000000000000000000000000000162e"
	assert.Equal(t, v1, hex.EncodeToString(result.Bytes()))
	ad, err = master.GetPrimaryAccountData(&acc4, nil)
	assert.NoError(t, err)
	assert.Equal(t, 679498, int(ad.Balance.Int64()))
	_, _, receipt, err = master.GetTransactionReceipt(tx3.Hash(), b5.Header().Branch)
	assert.NoError(t, err)
	//TODO enable xshard receipt retrieve
	//assert.Equal(t, tx3.Hash(), receipt.TxHash)
	//assert.Equal(t, uint64(0x1), receipt.Status)
}

func TestCrossShardTransfer(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	id2, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1<<16)
	//acc3 := account.CreatAddressFromIdentity(id2, 0)
	acc4 := account.CreatAddressFromIdentity(id2, 1<<16)
	shardSize := 1
	chainSize := 8
	_, cluster := CreateClusterList(1, uint32(chainSize), uint32(shardSize), 2, nil)
	c := cluster[0]
	alloc := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(100000000)}
	for i := 0; i < chainSize; i++ {
		fsId := i<<16 | shardSize | 0
		shardCfg := c.clstrCfg.Quarkchain.GetShardConfigByFullShardID(uint32(fsId))
		shardCfg.CoinbaseAmount = big.NewInt(1000000)
		shardCfg.Genesis.Alloc[acc1] = alloc
		shardCfg.Genesis.Alloc[acc2] = alloc
	}
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()
	minorBlockChainA := c.GetShardState(1)
	minorBlockChainB := c.GetShardState(1<<16 + 1)

	master := c.master
	slaves := c.GetSlavelist()
	rb, _, err := master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	val := big.NewInt(1500000)
	gas := uint64(21000) + params.GtxxShardCost.Uint64()
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc4, val, &gas, nil, nil)
	err = slaves[0].AddTx(tx1)
	assert.NoError(t, err)
	b0, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b0)
	assert.NoError(t, err)

	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	b1, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil)
	assert.NoError(t, err)
	err = c.GetShard(1<<16 + 1).AddMinorBlock(b1)
	assert.NoError(t, err)
	ad, err := master.GetPrimaryAccountData(&acc4, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(1500000), ad.Balance.Int64())
	_, _, receipt, err := master.GetTransactionReceipt(tx1.Hash(), b0.Header().Branch)
	assert.NoError(t, err)
	assert.Equal(t, uint64(0x1), receipt.Status)
}
