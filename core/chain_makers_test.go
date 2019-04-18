// Modified from go-ethereum under GNU Lesser General Public License

package core

import (
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
	"math/big"
)

func ExampleGenerateRootBlockChain() {
	var (
		addr1        = account.Address{Recipient: account.Recipient{1}, FullShardKey: 0}
		addr2        = account.Address{Recipient: account.Recipient{2}, FullShardKey: 0}
		addr3        = account.Address{Recipient: account.Recipient{3}, FullShardKey: 0}
		db           = ethdb.NewMemDatabase()
		qkcconfig    = config.NewQuarkChainConfig()
		genesis      = Genesis{qkcConfig: qkcconfig}
		genesisBlock = genesis.MustCommitRootBlock(db)
		engine       = new(consensus.FakeEngine)
	)

	chain := GenerateRootBlockChain(genesisBlock, engine, 5, func(i int, gen *RootBlockGen) {
		switch i {
		case 0:
			// In block 1, addr1 sends addr2 some ether.
			header := types.MinorBlockHeader{Number: 1, Coinbase: addr1, ParentHash: genesisBlock.Hash(), Time: genesisBlock.Time()}
			gen.headers = append(gen.headers, &header)
		case 1:
			// In block 2, addr1 sends some more ether to addr2.
			// addr2 passes it on to addr3.
			header1 := types.MinorBlockHeader{Number: 1, Coinbase: addr1, ParentHash: genesisBlock.Hash(), Time: genesisBlock.Time()}
			header2 := types.MinorBlockHeader{Number: 2, Coinbase: addr2, ParentHash: header1.Hash(), Time: genesisBlock.Time()}
			gen.headers = append(gen.headers, &header1)
			gen.headers = append(gen.headers, &header2)
		case 2:
			// Block 3 is empty but was mined by addr3.
			gen.SetCoinbase(addr3)
			gen.SetExtra([]byte("yeehaw"))
		}
	})

	// Import the chain. This runs all block validation rules.
	blockchain, err := NewRootBlockChain(db, nil, qkcconfig, engine, nil)
	if err != nil {
		fmt.Printf("new root block chain error %v\n", err)
		return
	}
	defer blockchain.Stop()

	blockchain.SetValidator(&FackRootBlockValidator{nil})
	if i, err := blockchain.InsertChain(ToBlocks(chain)); err != nil {
		fmt.Printf("insert error (block %d): %v\n", chain[i].NumberU64(), err)
		return
	}

	fmt.Printf("last block: #%d\n", blockchain.CurrentBlock().Number())
	// Output:
	// last block: #5
}

func ExampleGenerateMinorBlockChain() {
	var (
		id1, _ = account.CreatRandomIdentity()
		id2, _ = account.CreatRandomIdentity()
		id3, _ = account.CreatRandomIdentity()
		addr1  = account.CreatAddressFromIdentity(id1, 0)
		addr2  = account.CreatAddressFromIdentity(id2, 0)
		addr3  = account.CreatAddressFromIdentity(id3, 0)

		db                = ethdb.NewMemDatabase()
		fakeClusterConfig = config.NewClusterConfig()
	)
	prvKey1, err := crypto.HexToECDSA(hex.EncodeToString(id1.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}
	prvKey2, err := crypto.HexToECDSA(hex.EncodeToString(id2.GetKey().Bytes()))
	if err != nil {
		panic(err)
	}

	ids := fakeClusterConfig.Quarkchain.GetGenesisShardIds()
	for _, v := range ids {
		addr := addr1.AddressInShard(v)
		shardConfig := fakeClusterConfig.Quarkchain.GetShardConfigByFullShardID(v)
		shardConfig.Genesis.Alloc[addr] = big.NewInt(1000000)
	}
	fakeClusterConfig.Quarkchain.SkipMinorDifficultyCheck = true
	// Ensure that key1 has some funds in the genesis block.
	gspec := &Genesis{
		qkcConfig: fakeClusterConfig.Quarkchain,
	}

	rootBlock := gspec.CreateRootBlock()
	genesis := gspec.MustCommitMinorBlock(db, rootBlock, gspec.qkcConfig.Chains[0].ShardSize|0)

	chain, _ := GenerateMinorBlockChain(params.TestChainConfig, fakeClusterConfig.Quarkchain, genesis, new(consensus.FakeEngine), db, 5, func(config *config.QuarkChainConfig, i int, gen *MinorBlockGen) {
		switch i {
		case 0:
			// In block 1, addr1 sends addr2 some ether.
			tx, _ := types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr1.Recipient), account.BytesToIdentityRecipient(addr2.Recipient.Bytes()), big.NewInt(100000), params.TxGas, nil, 0, 0, 3, 0, nil), types.MakeSigner(0), prvKey1)
			gen.AddTx(config, TransEvmTxToTx(tx))
		case 1:
			// In block 2, addr1 sends some more ether to addr2.
			// addr2 passes it on to addr3.
			tx1, _ := types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr1.Recipient), account.BytesToIdentityRecipient(addr2.Recipient.Bytes()), big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil), types.MakeSigner(0), prvKey1)
			tx2, _ := types.SignTx(types.NewEvmTransaction(gen.TxNonce(addr2.Recipient), account.BytesToIdentityRecipient(addr3.Recipient.Bytes()), big.NewInt(1000), params.TxGas, nil, 0, 0, 3, 0, nil), types.MakeSigner(0), prvKey2)
			gen.AddTx(config, TransEvmTxToTx(tx1))
			gen.AddTx(config, TransEvmTxToTx(tx2))
		case 2:
			// Block 3 is empty but was mined by addr3.
			gen.SetCoinbase(account.NewAddress(account.BytesToIdentityRecipient(addr3.Recipient.Bytes()), 0))
			gen.SetExtra([]byte("yeehaw"))
		}
	})

	fakeFullShardID := fakeClusterConfig.Quarkchain.Chains[0].ShardSize | 0
	// Import the chain. This runs all block validation rules.
	chainConfig := params.TestChainConfig
	blockchain, _ := NewMinorBlockChain(db, nil, chainConfig, fakeClusterConfig, new(consensus.FakeEngine), vm.Config{}, nil, fakeFullShardID, nil)
	genesis, err = blockchain.InitGenesisState(rootBlock, genesis)
	if err != nil {
		panic(err)
	}
	defer blockchain.Stop()

	skip := make([]bool, len(chain))
	if i, _, err := blockchain.InsertChain(toMinorBlocks(chain), skip); err != nil {
		fmt.Printf("insert error (block %d): %v\n", chain[i].NumberU64(), err)
		return
	}

	state, _ := blockchain.State()
	fmt.Printf("last block: #%d\n", blockchain.CurrentBlock().Number())
	fmt.Println("balance of addr1:", state.GetBalance(addr1.Recipient))
	fmt.Println("balance of addr2:", state.GetBalance(addr2.Recipient))
	fmt.Println("balance of addr3:", state.GetBalance(addr3.Recipient))
	// Output:
	// last block: #5
	// balance of addr1: 899000
	// balance of addr2: 100000
	// balance of addr3: 2500000000000001000
}
