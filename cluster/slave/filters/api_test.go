package filters

import (
	"encoding/json"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"math/big"
	"testing"
	"time"
)

func TestNewHeads(t *testing.T) {
	bak, err := newTestBackend()
	assert.NoError(t, err)
	defer bak.stop()

	chanHeaders := make(chan *types.MinorBlockHeader, 100)
	err = bak.subscribeEvent("newHeads", chanHeaders)
	assert.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	headers, err := bak.cresteMinorBlocks(10)
	assert.NoError(t, err)

	var (
		idx    = 0
		ticker = time.NewTicker(10 * time.Second)
	)
	for {
		select {
		case hd := <-chanHeaders:
			if hd.Hash() != headers[idx].Hash() {
				t.Error("header by subscribe is not match", "actual header: ", hd.Hash().Hex(), "expect header: ", headers[idx].Hash().Hex())
			}
			idx++
			if idx == len(headers) {
				return
			}
		case <-ticker.C:
			assert.Equal(t, idx, len(headers))
			return
		}
	}
}

func TestSyncing(t *testing.T) {
	bak, err := newTestBackend()
	assert.NoError(t, err)
	defer bak.stop()

	tests := []*sync.SyncingResult{
		{
			Syncing: false,
			Status: struct {
				CurrentBlock uint64
				HighestBlock uint64
			}{CurrentBlock: 0, HighestBlock: 100},
		},
		{
			Syncing: true,
			Status: struct {
				CurrentBlock uint64
				HighestBlock uint64
			}{CurrentBlock: 0, HighestBlock: 100},
		},
		{
			Syncing: false,
			Status: struct {
				CurrentBlock uint64
				HighestBlock uint64
			}{CurrentBlock: 100, HighestBlock: 100},
		},
	}

	statuses := make(chan *sync.SyncingResult, len(tests)*2)
	err = bak.subscribeEvent("syncing", statuses)
	assert.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	bak.creatSyncing(tests)

	var (
		idx    = 0
		ticker = time.NewTicker(10 * time.Second)
	)
	for {
		select {
		case dt := <-statuses:
			if dt.Syncing != tests[idx].Syncing || dt.Status.HighestBlock != tests[idx].Status.HighestBlock {
				t.Error("syncing by subscribe not match", "actual: ", dt.Status, "expect: ", tests[idx].Status)
			}
			idx++
			if idx == len(tests) {
				return
			}
		case <-ticker.C:
			assert.Equal(t, idx, len(tests))
		}
	}
}

func TestNewPendingTransactions(t *testing.T) {
	bak, err := newTestBackend()
	assert.NoError(t, err)
	defer bak.stop()

	txdata := []*types.Transaction{
		{
			TxType: 0,
			EvmTx:  types.NewEvmTransaction(0, common.Address{}, new(big.Int), 0, new(big.Int).SetUint64(10000000), 2, 2, 1, 0, []byte{}, 0, 0),
		},
		{
			TxType: 0,
			EvmTx:  types.NewEvmTransaction(1, common.Address{}, new(big.Int), 0, new(big.Int).SetUint64(10000000), 2, 2, 1, 0, []byte{}, 0, 0),
		},
		{
			TxType: 0,
			EvmTx:  types.NewEvmTransaction(2, common.Address{}, new(big.Int), 0, new(big.Int).SetUint64(10000000), 2, 2, 1, 0, []byte{}, 0, 0),
		},
		{
			TxType: 0,
			EvmTx:  types.NewEvmTransaction(3, common.Address{}, new(big.Int), 0, new(big.Int).SetUint64(10000000), 2, 2, 1, 0, []byte{}, 0, 0),
		},
		{
			TxType: 0,
			EvmTx:  types.NewEvmTransaction(4, common.Address{}, new(big.Int), 0, new(big.Int).SetUint64(10000000), 2, 2, 1, 0, []byte{}, 0, 0),
		},
		{
			TxType: 0,
			EvmTx:  types.NewEvmTransaction(5, common.Address{}, new(big.Int), 0, new(big.Int).SetUint64(10000000), 2, 2, 1, 0, []byte{}, 0, 0),
		},
	}

	txhashs := make(chan common.Hash, len(txdata)*2)
	err = bak.subscribeEvent("newPendingTransactions", txhashs)
	assert.NoError(t, err)

	time.Sleep(500 * time.Millisecond)
	bak.createTxs(txdata)

	var (
		idx    = 0
		ticker = time.NewTicker(10 * time.Second)
	)
	for {
		select {
		case dt := <-txhashs:
			if dt != txdata[idx].Hash() {
				t.Error("syncing by subscribe not match", "actual: ", dt.Hex(), "expect: ", txdata[idx].Hash().Hex())
			}
			idx++
			if idx == len(txdata) {
				return
			}
		case <-ticker.C:
			assert.Equal(t, idx, len(txdata))
		}
	}
}

func TestUnmarshalJSONNewFilterArgs(t *testing.T) {
	var (
		fromBlock rpc.BlockNumber = 0x123435
		toBlock   rpc.BlockNumber = 0xabcdef
		address0                  = common.HexToAddress("70c87d191324e6712a591f304b4eedef6ad9bb9d")
		address1                  = common.HexToAddress("9b2055d370f73ec7d8a03e965129118dc8f5bf83")
		topic0                    = common.HexToHash("3ac225168df54212a25c1c01fd35bebfea408fdac2e31ddd6f80a4bbf9a5f1ca")
		topic1                    = common.HexToHash("9084a792d2f8b16a62b882fd56f7860c07bf5fa91dd8a2ae7e809e5180fef0b3")
		topic2                    = common.HexToHash("6ccae1c4af4152f460ff510e573399795dfab5dcf1fa60d1f33ac8fdc1e480ce")
	)

	// default values
	var test0 rpc.FilterQuery
	if err := json.Unmarshal([]byte("{}"), &test0); err != nil {
		t.Fatal(err)
	}
	if test0.FromBlock != nil {
		t.Fatalf("expected nil, got %d", test0.FromBlock)
	}
	if test0.ToBlock != nil {
		t.Fatalf("expected nil, got %d", test0.ToBlock)
	}
	if len(test0.Addresses) != 0 {
		t.Fatalf("expected 0 addresses, got %d", len(test0.Addresses))
	}
	if len(test0.Topics) != 0 {
		t.Fatalf("expected 0 topics, got %d topics", len(test0.Topics))
	}

	// from, to block number
	var test1 rpc.FilterQuery
	vector := fmt.Sprintf(`{"fromBlock":"0x%x","toBlock":"0x%x"}`, fromBlock, toBlock)
	if err := json.Unmarshal([]byte(vector), &test1); err != nil {
		t.Fatal(err)
	}
	if test1.FromBlock.Int64() != fromBlock.Int64() {
		t.Fatalf("expected FromBlock %d, got %d", fromBlock, test1.FromBlock)
	}
	if test1.ToBlock.Int64() != toBlock.Int64() {
		t.Fatalf("expected ToBlock %d, got %d", toBlock, test1.ToBlock)
	}

	// single address
	var test2 rpc.FilterQuery
	vector = fmt.Sprintf(`{"address": "%s"}`, address0.Hex())
	if err := json.Unmarshal([]byte(vector), &test2); err != nil {
		t.Fatal(err)
	}
	if len(test2.Addresses) != 1 {
		t.Fatalf("expected 1 address, got %d address(es)", len(test2.Addresses))
	}
	if test2.Addresses[0] != address0 {
		t.Fatalf("expected address %x, got %x", address0, test2.Addresses[0])
	}

	// multiple address
	var test3 rpc.FilterQuery
	vector = fmt.Sprintf(`{"address": ["%s", "%s"]}`, address0.Hex(), address1.Hex())
	if err := json.Unmarshal([]byte(vector), &test3); err != nil {
		t.Fatal(err)
	}
	if len(test3.Addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d address(es)", len(test3.Addresses))
	}
	if test3.Addresses[0] != address0 {
		t.Fatalf("expected address %x, got %x", address0, test3.Addresses[0])
	}
	if test3.Addresses[1] != address1 {
		t.Fatalf("expected address %x, got %x", address1, test3.Addresses[1])
	}

	// single topic
	var test4 rpc.FilterQuery
	vector = fmt.Sprintf(`{"topics": ["%s"]}`, topic0.Hex())
	if err := json.Unmarshal([]byte(vector), &test4); err != nil {
		t.Fatal(err)
	}
	if len(test4.Topics) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(test4.Topics))
	}
	if len(test4.Topics[0]) != 1 {
		t.Fatalf("expected len(topics[0]) to be 1, got %d", len(test4.Topics[0]))
	}
	if test4.Topics[0][0] != topic0 {
		t.Fatalf("got %x, expected %x", test4.Topics[0][0], topic0)
	}

	// test multiple "AND" topics
	var test5 rpc.FilterQuery
	vector = fmt.Sprintf(`{"topics": ["%s", "%s"]}`, topic0.Hex(), topic1.Hex())
	if err := json.Unmarshal([]byte(vector), &test5); err != nil {
		t.Fatal(err)
	}
	if len(test5.Topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(test5.Topics))
	}
	if len(test5.Topics[0]) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(test5.Topics[0]))
	}
	if test5.Topics[0][0] != topic0 {
		t.Fatalf("got %x, expected %x", test5.Topics[0][0], topic0)
	}
	if len(test5.Topics[1]) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(test5.Topics[1]))
	}
	if test5.Topics[1][0] != topic1 {
		t.Fatalf("got %x, expected %x", test5.Topics[1][0], topic1)
	}

	// test optional topic
	var test6 rpc.FilterQuery
	vector = fmt.Sprintf(`{"topics": ["%s", null, "%s"]}`, topic0.Hex(), topic2.Hex())
	if err := json.Unmarshal([]byte(vector), &test6); err != nil {
		t.Fatal(err)
	}
	if len(test6.Topics) != 3 {
		t.Fatalf("expected 3 topics, got %d", len(test6.Topics))
	}
	if len(test6.Topics[0]) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(test6.Topics[0]))
	}
	if test6.Topics[0][0] != topic0 {
		t.Fatalf("got %x, expected %x", test6.Topics[0][0], topic0)
	}
	if len(test6.Topics[1]) != 0 {
		t.Fatalf("expected 0 topic, got %d", len(test6.Topics[1]))
	}
	if len(test6.Topics[2]) != 1 {
		t.Fatalf("expected 1 topic, got %d", len(test6.Topics[2]))
	}
	if test6.Topics[2][0] != topic2 {
		t.Fatalf("got %x, expected %x", test6.Topics[2][0], topic2)
	}

	// test OR topics
	var test7 rpc.FilterQuery
	vector = fmt.Sprintf(`{"topics": [["%s", "%s"], null, ["%s", null]]}`, topic0.Hex(), topic1.Hex(), topic2.Hex())
	if err := json.Unmarshal([]byte(vector), &test7); err != nil {
		t.Fatal(err)
	}
	if len(test7.Topics) != 3 {
		t.Fatalf("expected 3 topics, got %d topics", len(test7.Topics))
	}
	if len(test7.Topics[0]) != 2 {
		t.Fatalf("expected 2 topics, got %d topics", len(test7.Topics[0]))
	}
	if test7.Topics[0][0] != topic0 || test7.Topics[0][1] != topic1 {
		t.Fatalf("invalid topics expected [%x,%x], got [%x,%x]",
			topic0, topic1, test7.Topics[0][0], test7.Topics[0][1],
		)
	}
	if len(test7.Topics[1]) != 0 {
		t.Fatalf("expected 0 topic, got %d topics", len(test7.Topics[1]))
	}
	if len(test7.Topics[2]) != 0 {
		t.Fatalf("expected 0 topics, got %d topics", len(test7.Topics[2]))
	}
}
