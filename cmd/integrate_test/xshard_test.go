package test

import (
	"encoding/hex"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/master"
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

const GTXCOST = 21000

func assertBalance(t *testing.T, master *master.QKCMasterBackend, acc account.Address, exp *big.Int) {
	ad, err := master.GetPrimaryAccountData(&acc, nil)
	assert.NoError(t, err)
	blc := ad.Balance.GetTokenBalance(testGenesisTokenID)
	result := exp.Cmp(blc)
	assert.Equal(t, 0, result)
	if result != 0 {
		t.Errorf("balance of %x: expected %v, got %v. diff=%v\n", acc, exp, blc, new(big.Int).Sub(blc, exp))
	}
}

type counter struct {
	mst *master.QKCMasterBackend
	exp map[*account.Address]uint64
	t   *testing.T
}

func newCounter(t *testing.T, mstr *master.QKCMasterBackend) counter {
	return counter{
		exp: make(map[*account.Address]uint64),
		t:   t,
		mst: mstr,
	}
}

func (b *counter) assertAll() {
	for acc, value := range b.exp {
		assertBalance(b.t, b.mst, *acc, new(big.Int).SetUint64(value))
	}
}

func (b *counter) sum() uint64 {
	total := uint64(0)
	for _, value := range b.exp {
		total += value
	}
	return total
}

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
	var chainSize, shardSize, slaveSize uint32 = 2, 2, 2
	cfglist := GetClusterConfig(1, chainSize, shardSize, slaveSize, nil, defaultbootNode,
		config.PoWSimulate, true)
	_, cluster := CreateClusterList(1, cfglist)
	c := cluster[0]
	c.clstrCfg.Quarkchain.MinMiningGasPrice = new(big.Int)
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
	gas := uint64(GTXCOST) + params.GtxxShardCost.Uint64() + uint64(12345)
	gasPrice := uint64(1)
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc3, val, &gas, &gasPrice, nil)
	err = slaves[0].AddTx(tx1)
	assert.NoError(t, err)
	b1, err := minorBlockChainA.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	err = c.GetShard(2 | 0).AddMinorBlock(b1)
	assert.NoError(t, err)
	ad, err := master.GetPrimaryAccountData(&acc1, nil)
	assert.NoError(t, err)
	assert.Equal(t, int(1000000-val.Uint64()-gas), int(ad.Balance.GetTokenBalance(testGenesisTokenID).Int64()))
	time.Sleep(100 * time.Millisecond)
	//rb = minorBlockChainA.GetRootTip().CreateBlockToAppend(nil, nil, &acc1, nil, nil)
	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	acc1s1 := acc1.AddressInShard(1)
	ad, err = master.GetPrimaryAccountData(&acc1s1, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1000000, int(ad.Balance.GetTokenBalance(testGenesisTokenID).Uint64()))
	//b2 should include the withdraw of tx1
	b2, err := minorBlockChainB.CreateBlockToMine(nil, &acc4, nil, nil, nil)
	assert.NoError(t, err)
	err = c.GetShard(2 | 1).AddMinorBlock(b2)
	assert.NoError(t, err)
	acc3b, err := master.GetPrimaryAccountData(&acc3, nil)
	assert.NoError(t, err)
	assert.Equal(t, 54321, int(acc3b.Balance.GetTokenBalance(testGenesisTokenID).Uint64()))
	acc11, err := master.GetPrimaryAccountData(&acc1s1, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1012345, int(acc11.Balance.GetTokenBalance(testGenesisTokenID).Uint64()))
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
	var chainSize, shardSize, slaveSize uint32 = 8, 1, 2
	cfglist := GetClusterConfig(1, chainSize, shardSize, slaveSize, nil, defaultbootNode,
		config.PoWSimulate, true)
	_, cluster := CreateClusterList(1, cfglist)
	c := cluster[0]
	alloc := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(100000000)}
	c.clstrCfg.Quarkchain.MinMiningGasPrice = new(big.Int)
	//Enable xshard receipt
	c.clstrCfg.Quarkchain.XShardAddReceiptTimestamp = 1
	for i := 0; i < int(chainSize); i++ {
		fsId := i<<16 | int(shardSize) | 0
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
	b0, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b0)
	assert.NoError(t, err)
	b1, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	err = c.GetShard(1<<16 + 1).AddMinorBlock(b1)
	assert.NoError(t, err)

	val := big.NewInt(1500000)
	gas := uint64(GTXCOST)
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc3, val, &gas, nil, nil)
	err = slaves[0].AddTx(tx1)
	assert.NoError(t, err)
	b00, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b00)
	assert.NoError(t, err)
	ad, err := master.GetPrimaryAccountData(&acc3, nil)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1500000), ad.Balance.GetTokenBalance(testGenesisTokenID).Uint64())
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
	b2, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
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
	b3, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	err = minorBlockChainB.AddBlock(b3)
	assert.NoError(t, err)
	b3n := b3.Header().Number
	result, err = master.GetStorageAt(&contractAddress, storageKeyHash, &b3n)
	assert.NoError(t, err)
	assert.Equal(t, v0, hex.EncodeToString(result.Bytes()))
	ad, err = master.GetPrimaryAccountData(&acc4, nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, int(ad.Balance.GetTokenBalance(testGenesisTokenID).Int64()))
	_, _, receipt, err = master.GetTransactionReceipt(tx2.Hash(), b3.Header().Branch)
	assert.NoError(t, err)
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
	b4, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
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
	b5, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil, nil, nil)
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
	assert.Equal(t, 679498, int(ad.Balance.GetTokenBalance(testGenesisTokenID).Int64()))
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
	acc4 := account.CreatAddressFromIdentity(id2, 1<<16)
	var chainSize, shardSize, slaveSize uint32 = 8, 1, 2
	cfglist := GetClusterConfig(1, chainSize, shardSize, slaveSize, nil, defaultbootNode,
		config.PoWSimulate, true)
	_, cluster := CreateClusterList(1, cfglist)
	c := cluster[0]
	c.clstrCfg.Quarkchain.MinMiningGasPrice = new(big.Int)
	alloc := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(100000000)}
	for i := 0; i < int(chainSize); i++ {
		fsId := i<<16 | int(shardSize) | 0
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
	gas := uint64(GTXCOST) + params.GtxxShardCost.Uint64()
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc4, val, &gas, nil, nil)
	err = slaves[0].AddTx(tx1)
	assert.NoError(t, err)
	b0, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b0)
	assert.NoError(t, err)

	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	b1, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	err = c.GetShard(1<<16 + 1).AddMinorBlock(b1)
	assert.NoError(t, err)
	ad, err := master.GetPrimaryAccountData(&acc4, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(1500000), ad.Balance.GetTokenBalance(testGenesisTokenID).Int64())
	_, _, receipt, err := master.GetTransactionReceipt(tx1.Hash(), b0.Header().Branch)
	assert.NoError(t, err)
	assert.Equal(t, uint64(0x1), receipt.Status)
}

//Test the cross shard transactions are broadcasted to the destination shards
func TestBroadcastCrossShardTransaction1x2(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc3, err := account.CreatRandomAccountWithFullShardKey(2 << 16)
	assert.NoError(t, err)
	acc4, err := account.CreatRandomAccountWithFullShardKey(3 << 16)
	assert.NoError(t, err)
	var chainSize, shardSize, slaveSize uint32 = 8, 1, 2
	cfglist := GetClusterConfig(1, chainSize, shardSize, slaveSize, nil, defaultbootNode,
		config.PoWSimulate, true)
	_, cluster := CreateClusterList(1, cfglist)
	c := cluster[0]
	c.clstrCfg.Quarkchain.MinMiningGasPrice = new(big.Int)
	alloc := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(1000000)}
	for _, fsId := range c.clstrCfg.Quarkchain.GetGenesisShardIds() {
		shardCfg := c.clstrCfg.Quarkchain.GetShardConfigByFullShardID(fsId)
		acc1s := acc1.AddressInShard(fsId)
		shardCfg.Genesis.Alloc[acc1s] = alloc
	}
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()
	master := c.master
	slaves := c.GetSlavelist()
	minorBlockChainA := c.GetShardState(1)
	minorBlockChainB := c.GetShardState(2<<16 + 1)
	minorBlockChainC := c.GetShardState(3<<16 + 1)

	rb, _, err := master.CreateBlockToMine()
	assert.NoError(t, err)
	assert.NoError(t, master.AddRootBlock(rb.(*types.RootBlock)))

	gasLimit := GTXCOST + params.GtxxShardCost.Uint64()
	value1 := big.NewInt(54321)
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc3, value1, &gasLimit, nil, nil)
	assert.NoError(t, slaves[0].AddTx(tx1))
	value2 := big.NewInt(1234)
	nonce, err := minorBlockChainA.GetTransactionCount(acc1.Recipient, nil)
	nonce = nonce + 1
	assert.NoError(t, err)
	tx2 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc4, value2, &gasLimit, nil, &nonce)
	assert.NoError(t, slaves[0].AddTx(tx2))

	b1, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	b2, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)

	assert.NoError(t, c.GetShard(1).AddMinorBlock(b1))
	//expect chain 2 got the CrossShardTransactionList of b1
	xShardTxList := minorBlockChainB.ReadCrossShardTxList(b1.Hash())
	assert.Equal(t, 1, len(xShardTxList.TXList))
	assert.Equal(t, tx1.Hash(), xShardTxList.TXList[0].TxHash)
	assert.Equal(t, acc1, xShardTxList.TXList[0].From)
	assert.Equal(t, acc3, xShardTxList.TXList[0].To)
	assert.Equal(t, value1, xShardTxList.TXList[0].Value.Value)
	//expect chain 3 got the CrossShardTransactionList of b1
	xShardTxList = minorBlockChainC.ReadCrossShardTxList(b1.Hash())
	assert.Equal(t, 1, len(xShardTxList.TXList))
	assert.Equal(t, tx2.Hash(), xShardTxList.TXList[0].TxHash)
	assert.Equal(t, acc1, xShardTxList.TXList[0].From)
	assert.Equal(t, acc4, xShardTxList.TXList[0].To)
	assert.Equal(t, value2, xShardTxList.TXList[0].Value.Value)

	assert.NoError(t, c.GetShard(1).AddMinorBlock(b2))
	//b2 doesn't update tip
	assert.Equal(t, b1.Header().Hash(), minorBlockChainA.CurrentBlock().Hash())
	//expect chain 2 got the CrossShardTransactionList of b1
	xShardTxList = minorBlockChainB.ReadCrossShardTxList(b2.Hash())
	assert.Equal(t, 1, len(xShardTxList.TXList))
	assert.Equal(t, tx1.Hash(), xShardTxList.TXList[0].TxHash)
	assert.Equal(t, acc1, xShardTxList.TXList[0].From)
	assert.Equal(t, acc3, xShardTxList.TXList[0].To)
	assert.Equal(t, value1, xShardTxList.TXList[0].Value.Value)
	//expect chain 3 got the CrossShardTransactionList of b1
	xShardTxList = minorBlockChainC.ReadCrossShardTxList(b2.Hash())
	assert.Equal(t, 1, len(xShardTxList.TXList))
	assert.Equal(t, tx2.Hash(), xShardTxList.TXList[0].TxHash)
	assert.Equal(t, acc1, xShardTxList.TXList[0].From)
	assert.Equal(t, acc4, xShardTxList.TXList[0].To)
	assert.Equal(t, value2, xShardTxList.TXList[0].Value.Value)
	acc1InB := acc1.AddressInShard(2 << 16)
	b3, err := minorBlockChainB.CreateBlockToMine(nil, &acc1InB, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, master.AddMinorBlock(b3.Header().Branch.Value, b3))

	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	assert.NoError(t, master.AddRootBlock(rb.(*types.RootBlock)))
	//b4 should include the withdraw of tx1
	b4, err := minorBlockChainB.CreateBlockToMine(nil, &acc1InB, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, master.AddMinorBlock(b4.Header().Branch.Value, b4))
	ad3, err := master.GetPrimaryAccountData(&acc3, nil)
	assert.NoError(t, err)
	assert.Equal(t, value1, ad3.Balance.GetTokenBalance(testGenesisTokenID))
	//b5 should include the withdraw of tx2
	acc1InC := acc1.AddressInShard(3 << 16)
	b5, err := minorBlockChainC.CreateBlockToMine(nil, &acc1InC, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, master.AddMinorBlock(b5.Header().Branch.Value, b5))
	ad4, err := master.GetPrimaryAccountData(&acc4, nil)
	assert.NoError(t, err)
	assert.Equal(t, value2, ad4.Balance.GetTokenBalance(testGenesisTokenID))
}

//Test the cross shard transactions are broadcasted to the destination shards
func TestBroadcastCrossShardTransaction2x1(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	id2, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 1<<16)
	acc3, err := account.CreatRandomAccountWithFullShardKey(2 << 16)
	assert.NoError(t, err)
	acc4, err := account.CreatRandomAccountWithFullShardKey(1 << 16)
	assert.NoError(t, err)
	acc5, err := account.CreatRandomAccountWithFullShardKey(0)
	assert.NoError(t, err)
	var chainSize, shardSize, slaveSize uint32 = 8, 1, 2
	cfglist := GetClusterConfig(1, chainSize, shardSize, slaveSize, nil, defaultbootNode,
		config.PoWSimulate, true)
	_, cluster := CreateClusterList(1, cfglist)
	c := cluster[0]
	c.clstrCfg.Quarkchain.MinMiningGasPrice = new(big.Int)
	minorCoinbase := big.NewInt(1000000)
	genesis := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(1000000)}
	c.clstrCfg.Quarkchain.Root.CoinbaseAmount = big.NewInt(10)
	for _, fsId := range c.clstrCfg.Quarkchain.GetGenesisShardIds() {
		shardCfg := c.clstrCfg.Quarkchain.GetShardConfigByFullShardID(fsId)
		acc1s := acc1.AddressInShard(fsId)
		shardCfg.Genesis.Alloc[acc1s] = genesis
		shardCfg.CoinbaseAmount = minorCoinbase
	}
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()
	mstr := c.GetMaster()
	slaves := c.GetSlavelist()
	minorBlockChainA := c.GetShardState(1)
	minorBlockChainB := c.GetShardState(1<<16 + 1)
	minorBlockChainC := c.GetShardState(2<<16 + 1)
	shardA := c.GetShard(1)
	shardB := c.GetShard(1<<16 + 1)

	rb, _, err := mstr.CreateBlockToMine()
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	b0, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, shardB.AddMinorBlock(b0))

	assertBalance(t, mstr, acc1, new(big.Int).SetUint64(1000000))
	assertBalance(t, mstr, acc2, new(big.Int).SetUint64(500000))

	gasLimit := GTXCOST + params.GtxxShardCost.Uint64()
	value1 := big.NewInt(54321)
	gasPrice1 := uint64(1)
	tx1 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc3, value1, &gasLimit, &gasPrice1, nil)
	assert.NoError(t, slaves[0].AddTx(tx1))
	value2 := big.NewInt(5555)
	nonce, err := minorBlockChainA.GetTransactionCount(acc1.Recipient, nil)
	nonce = nonce + 1
	gasPrice2 := uint64(3)
	assert.NoError(t, err)
	value3 := big.NewInt(7787)
	tx2 := core.CreateTransferTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, acc3, value2, &gasLimit, &gasPrice2, &nonce)
	assert.NoError(t, slaves[0].AddTx(tx2))
	gasPrice3 := uint64(2)
	tx3 := core.CreateTransferTx(minorBlockChainA, id2.GetKey().Bytes(), acc2, acc3, value3, &gasLimit, &gasPrice3, nil)
	assert.NoError(t, slaves[1].AddTx(tx3))

	b1, err := minorBlockChainA.CreateBlockToMine(nil, &acc5, nil, nil, nil)
	assert.NoError(t, err)
	time.Sleep(1 * time.Second)
	b2, err := minorBlockChainB.CreateBlockToMine(nil, &acc4, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, shardA.AddMinorBlock(b1))
	assert.NoError(t, shardB.AddMinorBlock(b2))

	acc1S2 := acc1.AddressInShard(2 << 16)
	cntr := newCounter(t, mstr)

	cntr.exp[&acc1] = 1000000 - 54321 - 5555 - (params.GtxxShardCost.Uint64()+GTXCOST)*(gasPrice1+gasPrice2)
	cntr.exp[&acc1S2] = 1000000
	cntr.exp[&acc2] = 500000 - 7787 - (params.GtxxShardCost.Uint64()+GTXCOST)*2
	cntr.exp[&acc3] = 0
	cntr.exp[&acc4] = GTXCOST + 500000
	cntr.exp[&acc5] = GTXCOST*2 + 500000
	cntr.assertAll()

	//expect chain 2 got the CrossShardTransactionList of b1
	xShardTxList := minorBlockChainC.ReadCrossShardTxList(b1.Hash())
	assert.Equal(t, 2, len(xShardTxList.TXList))
	assert.Equal(t, tx1.Hash(), xShardTxList.TXList[0].TxHash)
	assert.Equal(t, tx2.Hash(), xShardTxList.TXList[1].TxHash)
	xShardTxList = minorBlockChainC.ReadCrossShardTxList(b2.Hash())
	assert.Equal(t, 1, len(xShardTxList.TXList))
	assert.Equal(t, tx3.Hash(), xShardTxList.TXList[0].TxHash)

	c.clstrCfg.Quarkchain.Root.CoinbaseAddress = acc1
	rb, _, err = mstr.CreateBlockToMine()
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	//b3 should include the deposits of tx1, t2, t3
	b3, err := minorBlockChainC.CreateBlockToMine(nil, &acc1S2, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddMinorBlock(b3.Branch().Value, b3))

	cntr.exp[&acc1S2] += 500000 + params.GtxxShardCost.Uint64()*3
	cntr.exp[&acc3] += 54321 + 5555 + 7787
	cntr.assertAll()

	b4, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddMinorBlock(b4.Branch().Value, b4))

	cntr.exp[&acc1] += c.clstrCfg.Quarkchain.Root.CoinbaseAmount.Uint64() +
		1500000 + // root block tax reward (3 blocks) from minor block tax
		500000 + 31500 + // minor block reward FIXME: 1500000 for python?
		GTXCOST*3 //root block tax reward from tx fee
	cntr.assertAll()

	c.clstrCfg.Quarkchain.Root.CoinbaseAddress = acc3
	rb, _, err = mstr.CreateBlockToMine()
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	//no change
	cntr.assertAll()

	b5, err := minorBlockChainC.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddMinorBlock(b5.Branch().Value, b5))

	cntr.exp[&acc3] += c.clstrCfg.Quarkchain.Root.CoinbaseAmount.Uint64() +
		1000000 + // root block tax reward (1 block) from minor blocks b4+b5
		500000 + 4500 + //FIXME: remove 4500
		params.GtxxShardCost.Uint64()*3 // root block tax reward from tx fee
	cntr.assertAll()

	c.clstrCfg.Quarkchain.Root.CoinbaseAddress = acc4
	rb, _, err = mstr.CreateBlockToMine()
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	//no change
	cntr.assertAll()

	b6, err := minorBlockChainB.CreateBlockToMine(nil, &acc4, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddMinorBlock(b6.Branch().Value, b6))

	cntr.exp[&acc4] += c.clstrCfg.Quarkchain.Root.CoinbaseAmount.Uint64() +
		1000000
	cntr.assertAll()

	total := 3*c.clstrCfg.Quarkchain.Root.CoinbaseAmount.Uint64() +
		6*minorCoinbase.Uint64() +
		2*genesis[c.clstrCfg.Quarkchain.GenesisToken].Uint64() +
		500000 + //post-tax mblock coinbase
		36000 //FIXME remove 36000
	assert.Equal(t, int(total), int(cntr.sum()))
}
