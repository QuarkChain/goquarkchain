package core

import (
	"encoding/hex"
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
	"time"
)

// contract code
//pragma solidity >=0.4.10 <0.7.0;
//
//contract C {
//function f() public payable {
//uint256 _id = 0x420042;
//log3(
//bytes32(msg.value),
//bytes32(0x50cb9fe53daa9737b786ab3646f04d0150dc50ef4e75f59509d83667ad5adb20),
//bytes32(uint256(msg.sender)),
//bytes32(_id)
//);
//}
//}
func TestGetLog(t *testing.T) {
	id1, err := account.CreatRandomIdentity()
	acc3, err := account.CreatRandomAccountWithFullShardKey(0)
	checkErr(err)
	acc1 := account.CreatAddressFromIdentity(id1, 0)

	fakeMoney := uint64(100000000000000000)
	env := setUp([]account.Address{acc1, acc3}, &fakeMoney, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	defer shardState.Stop()

	fakeChan := make(chan uint64, 100)
	shardState.txPool.fakeChanForReset = fakeChan
	// Add a root block to have all the shards initialized
	rootBlock := shardState.rootTip.CreateBlockToAppend(nil, nil, nil, nil, nil).Finalize(nil, nil)

	_, err = shardState.AddRootBlock(rootBlock)
	checkErr(err)

	data := common.FromHex("6080604052348015600f57600080fd5b5060d88061001e6000396000f3fe6080604052600436106039576000357c01000000000000000000000000000000000000000000000000000000009004806326121ff014603e575b600080fd5b60446046565b005b6000624200429050806001023373ffffffffffffffffffffffffffffffffffffffff166001027f50cb9fe53daa9737b786ab3646f04d0150dc50ef4e75f59509d83667ad5adb20600102346001026040518082815260200191505060405180910390a35056fea165627a7a72305820eb8d6b105e05bbc4bc155f248007b36f41e06228c6981f30f35e534c87ed92500029")
	evmtx := types.NewEvmContractCreation(0, new(big.Int), 1000000, new(big.Int).SetUint64(10000000000), 0, 0, 3, 0, data)
	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	evmtx, err = types.SignTx(evmtx, types.MakeSigner(3), prvKey)
	if err != nil {
		panic(err)
	}
	tx := &types.Transaction{
		EvmTx:  evmtx,
		TxType: types.EvmTx,
	}
	currState, err := shardState.State()
	checkErr(err)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	checkErr(err)

	b2, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	assert.Equal(t, len(b2.Transactions()), 1)
	assert.Equal(t, b2.Header().Number, uint64(1))

	// Should succeed
	b2, re, err := shardState.FinalizeAndAddBlock(b2)
	checkErr(err)
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
	assert.Equal(t, shardState.CurrentBlock().IHeader().NumberU64(), uint64(1))
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b2.Header().Hash())
	assert.Equal(t, shardState.CurrentBlock().GetTransactions()[0].Hash(), tx.Hash())
	contractAddr := re[0].ContractAddress

	//second contract
	data = common.FromHex("26121ff0")
	evmtx = types.NewEvmTransaction(1, contractAddr, new(big.Int), 1000000, new(big.Int).SetUint64(10000000000), 0, 0, 3, 0, data)

	evmtx, err = types.SignTx(evmtx, types.MakeSigner(3), prvKey)
	if err != nil {
		panic(err)
	}
	tx = &types.Transaction{
		EvmTx:  evmtx,
		TxType: types.EvmTx,
	}
	currState, err = shardState.State()
	checkErr(err)
	currState.SetGasUsed(currState.GetGasLimit())

	err = shardState.AddTx(tx)
	checkErr(err)

	b3, err := shardState.CreateBlockToMine(nil, &acc3, nil)
	checkErr(err)
	assert.Equal(t, len(b3.Transactions()), 1)
	assert.Equal(t, b3.Header().Number, uint64(2))

	// Should succeed
	b3, re, err = shardState.FinalizeAndAddBlock(b3)
	checkErr(err)
	assert.Equal(t, shardState.CurrentBlock().IHeader().NumberU64(), uint64(2))
	assert.Equal(t, shardState.CurrentBlock().IHeader().(*types.MinorBlockHeader).Hash(), b3.Header().Hash())
	assert.Equal(t, shardState.CurrentBlock().GetTransactions()[0].Hash(), tx.Hash())

	address := make([]common.Address, 0)
	address = append(address, contractAddr)
	filter := NewRangeFilter(shardState, 0, 2, address, nil) //address is match
	logs, err := filter.Logs()
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 1)

	assert.Equal(t, len(logs[0].Topics), 3)
	topics := make([][]common.Hash, 0)
	topics = append(topics, logs[0].Topics)
	filter = NewRangeFilter(shardState, 0, 2, nil, topics) //topics is match
	logs, err = filter.Logs()
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 1)

	topic := make([]common.Hash, 0)
	topic = append(topic, logs[0].Topics[0])
	topics = make([][]common.Hash, 0)
	topics = append(topics, topic)
	filter = NewRangeFilter(shardState, 0, 2, nil, topics) // topics match one
	logs, err = filter.Logs()
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 1)

	topic = make([]common.Hash, 0)
	topic = append(topic, common.HexToHash("2324242424"))
	topics = make([][]common.Hash, 0)
	topics = append(topics, topic)
	filter = NewRangeFilter(shardState, 0, 2, nil, topics) // topics not match
	logs, err = filter.Logs()
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 0)

	address = make([]common.Address, 0)
	address = append(address, acc1.Recipient)
	filter = NewRangeFilter(shardState, 0, 2, address, nil) // address is not match
	logs, err = filter.Logs()
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 0)

	address1 := make([]common.Address, 0)
	filter = NewRangeFilter(shardState, 0, 2, address1, nil) // no limit
	logs, err = filter.Logs()
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 1)

	filter = NewRangeFilter(shardState, 0, 2, nil, nil) // no limit
	logs, err = filter.Logs()
	assert.NoError(t, err)
	assert.Equal(t, len(logs), 1)
}
