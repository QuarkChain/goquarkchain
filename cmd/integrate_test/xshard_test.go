//+build integrate_test

package test

import (
	"encoding/hex"
	"math/big"
	"runtime"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/master"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/QuarkChain/goquarkchain/params"
	ethcommon "github.com/ethereum/go-ethereum/common"
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
	c.clstrCfg.Quarkchain.MinTXPoolGasPrice = new(big.Int)
	c.clstrCfg.Quarkchain.EnableEvmTimeStamp = 0
	balance := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(1000000)}
	alloc := config.Allocation{Balances: balance}
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

	rb, _, err := master.CreateBlockToMine(nil)
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
	rb, _, err = master.CreateBlockToMine(nil)
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
	balance := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(100000000)}
	alloc := config.Allocation{Balances: balance}
	c.clstrCfg.Quarkchain.MinMiningGasPrice = new(big.Int)
	c.clstrCfg.Quarkchain.MinTXPoolGasPrice = new(big.Int)
	//Enable xshard receipt
	c.clstrCfg.Quarkchain.EnableEvmTimeStamp = 1
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
	rb, _, err := master.CreateBlockToMine(nil)
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	tx0, err := core.CreateContract(minorBlockChainB, id1.GetKey(), acc2, acc2.FullShardKey, core.ContractWithStorage2)
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
	contractAddress := account.NewAddress(receipt.ContractAddress, receipt.ContractFullShardKey)
	b1n := b1.Header().Number
	result, err := master.GetStorageAt(&contractAddress, storageKeyHash, &b1n)
	assert.NoError(t, err)
	v0 := "0000000000000000000000000000000000000000000000000000000000000000"
	assert.Equal(t, v0, hex.EncodeToString(result.Bytes()))
	//should include b1
	time.Sleep(100 * time.Millisecond)
	rb, _, err = master.CreateBlockToMine(nil)
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
	rb, _, err = master.CreateBlockToMine(nil)
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
	//make sure receipt actually found
	assert.Equal(t, tx2.Hash(), receipt.TxHash)
	assert.Equal(t, uint64(0x0), receipt.Status)
	//call the contract with enough gas
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
	rb, _, err = master.CreateBlockToMine(nil)
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
	assert.Equal(t, tx3.Hash(), receipt.TxHash)
	assert.Equal(t, uint64(0x1), receipt.Status)
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
	c.clstrCfg.Quarkchain.MinTXPoolGasPrice = new(big.Int)
	balance := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(100000000)}
	alloc := config.Allocation{Balances: balance}
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
	rb, _, err := master.CreateBlockToMine(nil)
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

	rb, _, err = master.CreateBlockToMine(nil)
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
	c.clstrCfg.Quarkchain.MinTXPoolGasPrice = new(big.Int)
	balance := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(1000000)}
	alloc := config.Allocation{Balances: balance}
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

	rb, _, err := master.CreateBlockToMine(nil)
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

	rb, _, err = master.CreateBlockToMine(nil)
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
	c.clstrCfg.Quarkchain.MinTXPoolGasPrice = new(big.Int)
	c.clstrCfg.Quarkchain.EnableEvmTimeStamp = 0
	minorCoinbase := big.NewInt(1000000)
	balance := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(1000000)}
	alloc := config.Allocation{Balances: balance}
	c.clstrCfg.Quarkchain.Root.CoinbaseAmount = big.NewInt(10)
	for _, fsId := range c.clstrCfg.Quarkchain.GetGenesisShardIds() {
		shardCfg := c.clstrCfg.Quarkchain.GetShardConfigByFullShardID(fsId)
		acc1s := acc1.AddressInShard(fsId)
		shardCfg.Genesis.Alloc[acc1s] = alloc
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

	rb, _, err := mstr.CreateBlockToMine(nil)
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
	cntr.exp[&acc2] = 500000 - 7787 - (params.GtxxShardCost.Uint64()+GTXCOST)*gasPrice3
	cntr.exp[&acc3] = 0
	cntr.exp[&acc4] = (1000000 + GTXCOST*gasPrice3) / 2
	cntr.exp[&acc5] = (1000000 + GTXCOST*gasPrice1 + GTXCOST*gasPrice2) / 2
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
	rb, _, err = mstr.CreateBlockToMine(nil)
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	//b3 should include the deposits of tx1, t2, t3
	b3, err := minorBlockChainC.CreateBlockToMine(nil, &acc1S2, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddMinorBlock(b3.Branch().Value, b3))

	cntr.exp[&acc1S2] += (1000000 + params.GtxxShardCost.Uint64()*(gasPrice1+gasPrice2+gasPrice3)) / 2
	cntr.exp[&acc3] += 54321 + 5555 + 7787
	cntr.assertAll()

	b4, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddMinorBlock(b4.Branch().Value, b4))

	cntr.exp[&acc1] += c.clstrCfg.Quarkchain.Root.CoinbaseAmount.Uint64() +
		1500000 + // root block tax reward (3 blocks) from minor block tax
		500000 + // minor block reward
		GTXCOST*(gasPrice1+gasPrice2+gasPrice3)/2 //root block tax reward from tx fee
	cntr.assertAll()

	c.clstrCfg.Quarkchain.Root.CoinbaseAddress = acc3
	rb, _, err = mstr.CreateBlockToMine(nil)
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	//no change
	cntr.assertAll()

	b5, err := minorBlockChainC.CreateBlockToMine(nil, &acc3, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddMinorBlock(b5.Branch().Value, b5))

	cntr.exp[&acc3] += c.clstrCfg.Quarkchain.Root.CoinbaseAmount.Uint64() +
		1000000 + // root block tax reward (1 block) from minor blocks b4+b5
		500000 +
		params.GtxxShardCost.Uint64()*3 // root block tax reward from tx fee
	cntr.assertAll()

	c.clstrCfg.Quarkchain.Root.CoinbaseAddress = acc4
	rb, _, err = mstr.CreateBlockToMine(nil)
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
		2*balance[c.clstrCfg.Quarkchain.GenesisToken].Uint64() +
		500000 //post-tax mblock coinbase
	assert.Equal(t, int(total), int(cntr.sum()))
}

//Test the cross shard transactions are broadcasted to the destination shards
func TestCrossShardContractCreate(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id1, 1<<16)
	storageKeyStr := core.ZFill64(hex.EncodeToString(acc2.Recipient[:])) + core.ZFill64("1")
	storageKeyBytes, err := hex.DecodeString(storageKeyStr)
	assert.NoError(t, err)
	storageKeyHash := crypto.Keccak256Hash(storageKeyBytes)
	var chainSize, shardSize, slaveSize uint32 = 8, 1, 2
	cfglist := GetClusterConfig(1, chainSize, shardSize, slaveSize, nil, defaultbootNode,
		config.PoWSimulate, true)
	_, cluster := CreateClusterList(1, cfglist)
	c := cluster[0]
	c.clstrCfg.Quarkchain.MinMiningGasPrice = new(big.Int)
	c.clstrCfg.Quarkchain.MinTXPoolGasPrice = new(big.Int)
	c.clstrCfg.Quarkchain.XShardGasDDOSFixRootHeight = 0
	c.clstrCfg.Quarkchain.EnableEvmTimeStamp = 1
	balance := map[string]*big.Int{c.clstrCfg.Quarkchain.GenesisToken: big.NewInt(1000000)}
	alloc := config.Allocation{Balances: balance}
	c.clstrCfg.Quarkchain.Root.CoinbaseAmount = big.NewInt(10)
	for _, fsId := range c.clstrCfg.Quarkchain.GetGenesisShardIds() {
		shardCfg := c.clstrCfg.Quarkchain.GetShardConfigByFullShardID(fsId)
		acc1s := acc1.AddressInShard(fsId)
		shardCfg.Genesis.Alloc[acc1s] = alloc
	}
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()

	mstr := c.GetMaster()
	slaves := c.GetSlavelist()
	minorBlockChainA := c.GetShardState(1)
	minorBlockChainB := c.GetShardState(1<<16 + 1)
	shardA := c.GetShard(1)
	shardB := c.GetShard(1<<16 + 1)

	time.Sleep(500 * time.Millisecond)
	rb, _, err := mstr.CreateBlockToMine(nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	tx1, err := core.CreateContract(minorBlockChainB, id1.GetKey(), acc2, acc1.FullShardKey, core.ContractWithStorage2)
	assert.NoError(t, slaves[1].AddTx(tx1))

	b1, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, shardB.AddMinorBlock(b1))
	_, _, receipt, err := mstr.GetTransactionReceipt(tx1.Hash(), b1.Header().Branch)
	assert.NoError(t, err)
	assert.Equal(t, 1, int(receipt.Status))

	//should include b1
	rb, _, err = mstr.CreateBlockToMine(nil)
	assert.NoError(t, err)
	assert.NoError(t, mstr.AddRootBlock(rb.(*types.RootBlock)))

	b2, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, shardA.AddMinorBlock(b2))
	//contract should be created
	_, _, receipt, err = mstr.GetTransactionReceipt(tx1.Hash(), b2.Header().Branch)
	assert.NoError(t, err)
	assert.Equal(t, tx1.Hash(), receipt.TxHash)
	assert.Equal(t, 1, int(receipt.Status))
	contractAddress := account.NewAddress(receipt.ContractAddress, receipt.ContractFullShardKey)
	b2n := b2.Header().Number
	result, err := mstr.GetStorageAt(&contractAddress, storageKeyHash, &b2n)
	assert.NoError(t, err)
	assert.Equal(t, "0000000000000000000000000000000000000000000000000000000000000000", hex.EncodeToString(result.Bytes()))

	//	call the contract with enough gas
	data, err := hex.DecodeString("c2e171d7")
	assert.NoError(t, err)
	value, gasPrice, gas := big.NewInt(0), uint64(1), params.GtxxShardCost.Uint64()+700000
	tx2 := core.CreateCallContractTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, contractAddress,
		value, &gas, &gasPrice, nil, data)
	assert.NoError(t, slaves[0].AddTx(tx2))
	b3, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil, nil, nil)
	assert.NoError(t, err)
	assert.NoError(t, shardA.AddMinorBlock(b3))
	_, _, receipt, err = mstr.GetTransactionReceipt(tx2.Hash(), b3.Header().Branch)
	assert.NoError(t, err)
	assert.Equal(t, 1, int(receipt.Status))

	b3n := b3.Header().Number
	result, err = mstr.GetStorageAt(&contractAddress, storageKeyHash, &b3n)
	assert.NoError(t, err)
	assert.Equal(t, "000000000000000000000000000000000000000000000000000000000000162e", hex.EncodeToString(result.Bytes()))
}

func TestPoSWOnRootChain(t *testing.T) {
	stakerkeyB := []byte{0x1}
	stakerkey := account.BytesToIdentityKey(stakerkeyB)
	stakerId, err := account.CreatIdentityFromKey(stakerkey)
	assert.NoError(t, err)
	stakerAddr := account.NewAddress(stakerId.GetRecipient(), 0)
	signerkeyB := []byte{0x2}
	signerkey := account.BytesToIdentityKey(signerkeyB)
	signerId, err := account.CreatIdentityFromKey(signerkey)
	assert.NoError(t, err)

	var chainSize, shardSize, slaveSize uint32 = 2, 1, 2
	contractAddr := vm.SystemContracts[vm.ROOT_CHAIN_POSW].Address()
	contractCode := ethcommon.Hex2Bytes(`60806040526004361061007b5760003560e01c8063853828b61161004e578063853828b6146101b5578063a69df4b5146101ca578063f83d08ba146101df578063fd8c4646146101e75761007b565b806316934fc4146100d85780632e1a7d4d1461013c578063485d3834146101685780636c19e7831461018f575b336000908152602081905260409020805460ff16156100cb5760405162461bcd60e51b815260040180806020018281038252602681526020018061062e6026913960400191505060405180910390fd5b6100d5813461023b565b50005b3480156100e457600080fd5b5061010b600480360360208110156100fb57600080fd5b50356001600160a01b031661029b565b6040805194151585526020850193909352838301919091526001600160a01b03166060830152519081900360800190f35b34801561014857600080fd5b506101666004803603602081101561015f57600080fd5b50356102cf565b005b34801561017457600080fd5b5061017d61034a565b60408051918252519081900360200190f35b610166600480360360208110156101a557600080fd5b50356001600160a01b0316610351565b3480156101c157600080fd5b506101666103c8565b3480156101d657600080fd5b50610166610436565b6101666104f7565b3480156101f357600080fd5b5061021a6004803603602081101561020a57600080fd5b50356001600160a01b0316610558565b604080519283526001600160a01b0390911660208301528051918290030190f35b8015610297576002820154808201908111610291576040805162461bcd60e51b81526020600482015260116024820152706164646974696f6e206f766572666c6f7760781b604482015290519081900360640190fd5b60028301555b5050565b600060208190529081526040902080546001820154600283015460039093015460ff9092169290916001600160a01b031684565b336000908152602081905260409020805460ff1680156102f3575080600101544210155b6102fc57600080fd5b806002015482111561030d57600080fd5b6002810180548390039055604051339083156108fc029084906000818181858888f19350505050158015610345573d6000803e3d6000fd5b505050565b6203f48081565b336000908152602081905260409020805460ff16156103a15760405162461bcd60e51b81526004018080602001828103825260268152602001806106546026913960400191505060405180910390fd5b6003810180546001600160a01b0319166001600160a01b038416179055610297813461023b565b6103d06105fa565b5033600090815260208181526040918290208251608081018452815460ff16151581526001820154928101929092526002810154928201839052600301546001600160a01b031660608201529061042657600080fd5b61043381604001516102cf565b50565b336000908152602081905260409020805460ff16156104865760405162461bcd60e51b815260040180806020018281038252602b8152602001806106a1602b913960400191505060405180910390fd5b60008160020154116104df576040805162461bcd60e51b815260206004820152601b60248201527f73686f756c642068617665206578697374696e67207374616b65730000000000604482015290519081900360640190fd5b805460ff191660019081178255426203f48001910155565b336000908152602081905260409020805460ff166105465760405162461bcd60e51b815260040180806020018281038252602781526020018061067a6027913960400191505060405180910390fd5b805460ff19168155610433813461023b565b6000806105636105fa565b506001600160a01b03808416600090815260208181526040918290208251608081018452815460ff161580158252600183015493820193909352600282015493810193909352600301549092166060820152906105c75750600091508190506105f5565b60608101516000906001600160a01b03166105e35750836105ea565b5060608101515b604090910151925090505b915091565b6040518060800160405280600015158152602001600081526020016000815260200160006001600160a01b03168152509056fe73686f756c64206f6e6c7920616464207374616b657320696e206c6f636b656420737461746573686f756c64206f6e6c7920736574207369676e657220696e206c6f636b656420737461746573686f756c64206e6f74206c6f636b20616c72656164792d6c6f636b6564206163636f756e747373686f756c64206e6f7420756e6c6f636b20616c72656164792d756e6c6f636b6564206163636f756e7473a265627a7a72315820f2c044ad50ee08e7e49c575b49e8de27cac8322afdb97780b779aa1af44e40d364736f6c634300050b0032`)
	cfglist := GetClusterConfig(1, chainSize, shardSize, slaveSize, nil, defaultbootNode,
		config.PoWDoubleSha256, false)
	quarkChain := cfglist[0].Quarkchain
	quarkChain.RootChainPoSWContractBytecodeHash = crypto.Keccak256Hash(contractCode)
	quarkChain.RootSignerPrivateKey = []byte{}
	quarkChain.MinMiningGasPrice = new(big.Int)
	quarkChain.MinTXPoolGasPrice = new(big.Int)
	quarkChain.EnableEvmTimeStamp = 1
	root := quarkChain.Root
	root.DifficultyAdjustmentCutoffTime = 45
	root.DifficultyAdjustmentFactor = 2048
	poswConfig := quarkChain.Root.PoSWConfig
	poswConfig.Enabled = true
	poswConfig.WindowSize = 2
	poswConfig.TotalStakePerBlock = big.NewInt(10000000)
	//should always pass pow check if posw is applied
	poswConfig.DiffDivider = 1000000
	balance := map[string]*big.Int{quarkChain.GenesisToken: big.NewInt(100000000)}
	shardCfg := quarkChain.GetShardConfigByFullShardID(1)
	shardCfg.Genesis.Alloc = map[account.Address]config.Allocation{
		account.Address{Recipient: contractAddr, FullShardKey: 0}: {
			Code: contractCode,
		},
		account.Address{Recipient: stakerAddr.Recipient, FullShardKey: 0}: {
			Balances: balance,
		},
	}
	_, cluster := CreateClusterList(1, cfglist)
	cluster.Start(5*time.Second, true)
	defer cluster.Stop()

	c := cluster[0]
	mstr := c.GetMaster()
	cfg := mstr.GetClusterConfig().Quarkchain.Root

	mineMe := func(block types.IBlock, diff *big.Int) types.IBlock {
		diffCalculator := consensus.EthDifficultyCalculator{
			MinimumDifficulty: big.NewInt(int64(10)),
			AdjustmentCutoff:  45,
			AdjustmentFactor:  2048,
		}
		resultsCh := make(chan types.IBlock)
		engine := doublesha256.New(&diffCalculator, false, nil)
		if err = engine.Seal(nil, block, diff, resultsCh, nil); err != nil {
			t.Fatalf("problem sealing the block: %v", err)
		}
		minedBlock := <-resultsCh
		return minedBlock
	}

	addRootBlock := func(addr account.Address, sign, mine bool) error {
		cfg.CoinbaseAddress = addr
		rootBlock, diff, err := mstr.CreateBlockToMine(nil)
		if err != nil {
			return err
		}
		assert.Equal(t, rootBlock.IHeader().GetDifficulty(), diff)
		rBlock := rootBlock.(*types.RootBlock)
		if sign {
			prvKey, err := crypto.ToECDSA(signerId.GetKey().Bytes())
			if err != nil {
				return err
			}
			err = rBlock.SignWithPrivateKey(prvKey)
			if err != nil {
				return err
			}
		}
		if mine {
			//to pass pow check
			minedBlock := mineMe(rBlock, diff)
			rBlock = minedBlock.(*types.RootBlock)
		}
		return mstr.AddRootBlock(rBlock)
	}

	gas := uint64(1000000)
	zero := uint64(0)
	setSigner := func(nonce, value *uint64, a account.Recipient) *types.Transaction {
		dat := "6c19e783000000000000000000000000" + ethcommon.Bytes2Hex(a[:])
		data := ethcommon.Hex2Bytes(dat)
		return core.CreateCallContractTx(c.GetShardState(1), stakerkey.Bytes(), stakerAddr,
			account.Address{Recipient: contractAddr, FullShardKey: 0},
			new(big.Int).SetUint64(*value), &gas, &zero, nonce, data)
	}

	//add a root block first to init shard chains
	assert.NoError(t, addRootBlock(account.Address{}, false, true))
	//signature mismatch (recovery failed)
	assert.EqualError(t, addRootBlock(stakerAddr, false, false), "invalid proof-of-work")

	//set signer and staking for one block
	value := poswConfig.TotalStakePerBlock.Uint64()
	tx := setSigner(&zero, &value, signerId.GetRecipient())
	assert.NoError(t, c.GetShardState(1).AddTx(tx))
	iBlock, _, err := c.GetShard(1).CreateBlockToMine(nil)
	assert.NoError(t, err)
	minedBlock := mineMe(iBlock, iBlock.IHeader().GetDifficulty())
	//confirmed by a minor block and root block
	assert.NoError(t, c.GetShard(1).AddMinorBlock(minedBlock.(*types.MinorBlock)))
	assert.NoError(t, addRootBlock(account.Address{}, false, true))
	//posw applied: from 1000976 to 1
	assert.NoError(t, addRootBlock(stakerAddr, true, false))
	// 10000000 quota used up; tried posw but no diff change
	assert.EqualError(t, addRootBlock(stakerAddr, true, false), "invalid proof-of-work")
}

func TestGetWorkFromMaster(t *testing.T) {
	var (
		chainSize uint32 = 1
		shardSize uint32 = 1
	)
	cfglist := GetClusterConfig(1, chainSize, shardSize, chainSize, nil, defaultbootNode,
		config.PoWDoubleSha256, true)
	cfglist[0].Quarkchain.Root.PoSWConfig.Enabled = true
	cfglist[0].Quarkchain.Root.ConsensusConfig.RemoteMine = true
	_, clstrList := CreateClusterList(1, cfglist)
	clstrList.Start(5*time.Second, true)
	var (
		mstr = clstrList[0].GetMaster()
	)
	mstr.SetMining(true)
	coinbaseAddr := &account.Recipient{}
	assert.Equal(t, retryTrueWithTimeout(func() bool {
		work, err := mstr.GetWork(account.NewBranch(0), coinbaseAddr)
		if err != nil {
			return false
		}
		return work.Difficulty.Uint64() == uint64(1000000)
	}, 2), true)

	clstrList.Stop()
	time.Sleep(1 * time.Second)
	runtime.GC()
}

func retryTrueWithTimeout(f func() bool, duration int64) bool {
	deadLine := time.Now().Unix() + duration
	for !f() && time.Now().Unix() < deadLine {
		time.Sleep(500 * time.Millisecond) // 0.5 second
	}
	return f()
}
