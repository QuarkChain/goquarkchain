// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package core

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
)

func ExampleGenerateChain() {
	var (
		addr1        = account.Address{account.Recipient{1}, 0}
		addr2        = account.Address{account.Recipient{2}, 0}
		addr3        = account.Address{account.Recipient{3}, 0}
		db           = ethdb.NewMemDatabase()
		genesis      = Genesis{config.NewQuarkChainConfig()}
		genesisBlock = genesis.MustCommitRootBlock(db)
		qkcconfig    = config.NewQuarkChainConfig()
		engine       = new(consensus.FakeEngine)
		/*engine       = qkchash.New(true,
		&consensus.EthDifficultyCalculator{
			qkcconfig.Root.DifficultyAdjustmentCutoffTime,
			qkcconfig.Root.DifficultyAdjustmentFactor,
			new(big.Int).SetUint64(qkcconfig.Root.Genesis.Difficulty)})*/
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
