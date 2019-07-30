package test

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func zfill64(input string) string {
	return strings.Repeat("0", 64-len(input)) + input
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
	storageKeyStr := zfill64(hex.EncodeToString(acc4.Recipient[:])) + zfill64("1")
	fmt.Printf("core=%s\n", storageKeyStr)
	storageKeyHash := crypto.Keccak256Hash([]byte(storageKeyStr))
	fmt.Printf("storageKeyHash=%x\n", storageKeyHash)
	//hexHash := hex.EncodeToString(hex.storageKeyHash)
	//fmt.Printf("hexHash=%x\n", hexHash)
	//storageKey, err := strconv.ParseInt(hexHash, 10, 64)
	//fmt.Printf("storageKey=%d\n", storageKey)
	//assert.NoError(t, err)
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

	//Add a root block first so that later minor blocks referring to this root
	// can be broadcasted to other shards
	master := c.master
	slaves := c.GetSlavelist()
	rb, _, err := master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	tx0, err := createContract(minorBlockChainB, id1.GetKey(), acc2, acc2.FullShardKey, CONTRACT)
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
	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	//call the contract with enough gas
	data, err := hex.DecodeString("c2e171d7")
	assert.NoError(t, err)
	value, gasPrice, gas := big.NewInt(0), uint64(1), uint64(30000+700000)
	tx3 := core.CreateCallContractTx(minorBlockChainA, id2.GetKey().Bytes(), acc3, contractAddress,
		value, &gas, &gasPrice, nil, data)
	err = slaves[0].AddTx(tx3)
	assert.NoError(t, err)
	b4, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b4)
	assert.NoError(t, err)
	//should include b4
	rb, _, err = master.CreateBlockToMine()
	assert.NoError(t, err)
	err = master.AddRootBlock(rb.(*types.RootBlock))
	assert.NoError(t, err)
	//The contract should be called
	b5, err := minorBlockChainB.CreateBlockToMine(nil, &acc2, nil)
	assert.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	err = minorBlockChainB.AddBlock(b5)
	assert.NoError(t, err)
	b5n := b5.Header().Number
	result, err = master.GetStorageAt(&contractAddress, storageKeyHash, &b5n)
	assert.NoError(t, err)
	v1 := "000000000000000000000000000000000000000000000000000000000000162e"
	assert.Equal(t, v1, hex.EncodeToString(result.Bytes()))
	ad, err = master.GetPrimaryAccountData(&acc4, nil)
	assert.NoError(t, err)
	assert.Equal(t, uint64(679498), ad.Balance.Uint64())
	_, _, receipt, err = master.GetTransactionReceipt(tx3.Hash(), b5.Header().Branch)
	assert.NoError(t, err)
	assert.Equal(t, uint64(0x1), receipt.Status)

}

func TestContractCall(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	id2, err := account.CreatRandomIdentity()
	assert.NoError(t, err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)
	acc2 := account.CreatAddressFromIdentity(id2, 0)
	storageKeyStr := zfill64(hex.EncodeToString(acc2.Recipient[:])) + zfill64("1")
	storageKeyBytes, err := hex.DecodeString(storageKeyStr)
	assert.NoError(t, err)
	storageKey := ethCommon.BytesToHash(storageKeyBytes)
	fmt.Printf("storagekey=%x\n", storageKey)
	shardSize := 1
	chainSize := 1
	_, cluster := CreateClusterList(1, uint32(chainSize), uint32(shardSize), 1, nil)
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

	tx0, err := createContract(minorBlockChainA, id1.GetKey(), acc2, acc2.FullShardKey, CONTRACT)
	assert.NoError(t, err)
	err = minorBlockChainA.AddTx(tx0)
	assert.NoError(t, err)
	b0, err := minorBlockChainA.CreateBlockToMine(nil, &acc1, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b0)
	assert.NoError(t, err)
	_, _, receipt := minorBlockChainA.GetTransactionReceipt(tx0.Hash())
	assert.Equal(t, uint64(0x1), receipt.Status)
	contractAddress := account.NewAddress(receipt.ContractAddress, receipt.ContractFullShardId)
	fmt.Printf("contractAddress=%x\n", contractAddress)

	data, err := hex.DecodeString("c2e171d7")
	value, gasPrice, gas := big.NewInt(0), uint64(1), uint64(30000+700000)
	tx3 := core.CreateCallContractTx(minorBlockChainA, id1.GetKey().Bytes(), acc1, contractAddress,
		value, &gas, &gasPrice, nil, data)
	err = minorBlockChainA.AddTx(tx3)
	assert.NoError(t, err)
	b1, err := minorBlockChainA.CreateBlockToMine(nil, &acc2, nil)
	assert.NoError(t, err)
	err = c.GetShard(1).AddMinorBlock(b1)
	assert.NoError(t, err)
	b5n := b1.Header().Number
	code, err := minorBlockChainA.GetCode(contractAddress.Recipient, &b5n)
	assert.NoError(t, err)
	fmt.Printf("code=%x\n", code)

	result, err := minorBlockChainA.GetStorageAt(contractAddress.Recipient, storageKey, &b5n)
	assert.NoError(t, err)
	v1 := "000000000000000000000000000000000000000000000000000000000000162e"
	assert.Equal(t, v1, hex.EncodeToString(result.Bytes()))
	_, _, receipt = minorBlockChainA.GetTransactionReceipt(tx3.Hash())
	assert.NoError(t, err)
	assert.Equal(t, uint64(0x1), receipt.Status)

}

const CONTRACT = "6080604052348015600f57600080fd5b5060c68061001e6000396000f3fe6080604052600436106039576000357c010000000000000000000000000000000000000000000000000000000090048063c2e171d714603e575b600080fd5b348015604957600080fd5b5060506052565b005b61162e600160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000208190555056fea165627a7a72305820fe440b2cadff2d38365becb4339baa8c7b29ce933a2ad1b43f49feea0e1f7a7e0029"

func createContract(mBlockChain *core.MinorBlockChain, key account.Key, fromAddress account.Address,
	toFullShardKey uint32, bytecode string) (*types.Transaction, error) {

	z := big.NewInt(0)
	one := big.NewInt(1)
	nonce, err := mBlockChain.GetTransactionCount(fromAddress.Recipient, nil)
	if err != nil {
		return nil, err
	}
	bytecodeb, err := hex.DecodeString(bytecode)

	if err != nil {
		return nil, err
	}
	evmTx := types.NewEvmContractCreation(nonce, z, 1000000, one, fromAddress.FullShardKey, toFullShardKey,
		mBlockChain.Config().NetworkID, 0, bytecodeb)

	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(key.Bytes()))
	if err != nil {
		return nil, err
	}
	evmTx, err = types.SignTx(evmTx, types.MakeSigner(evmTx.NetworkId()), prvKey)
	if err != nil {
		return nil, err
	}
	return &types.Transaction{TxType: types.EvmTx, EvmTx: evmTx}, nil
}
