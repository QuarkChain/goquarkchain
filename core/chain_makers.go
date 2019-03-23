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
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

// BlockGen creates blocks for testing.
// See GenerateRootBlockChain for a detailed explanation.
type RootBlockGen struct {
	i       int
	parent  *types.RootBlock
	chain   []*types.RootBlock
	header  *types.RootBlockHeader
	headers types.MinorBlockHeaders
	engine  consensus.Engine
}

// SetCoinbase sets the coinbase of the generated block.
// It can be called at most once.
func (b *RootBlockGen) SetCoinbase(addr account.Address) {
	b.header.SetCoinbase(addr)
}

// SetExtra sets the extra data field of the generated block.
func (b *RootBlockGen) SetExtra(data []byte) {
	b.header.SetExtra(data)
}

// SetNonce sets the nonce field of the generated block.
func (b *RootBlockGen) SetNonce(nonce uint64) {
	b.header.SetNonce(nonce)
}

// Number returns the block number of the block being generated.
func (b *RootBlockGen) Number() uint64 {
	return b.header.NumberU64()
}

// PrevBlock returns a previously generated block by number. It panics if
// num is greater or equal to the number of the block being generated.
// For index -1, PrevBlock returns the parent block given to GenerateRootBlockChain.
func (b *RootBlockGen) PrevBlock(index int) *types.RootBlock {
	if index >= b.i {
		panic(fmt.Errorf("block index %d out of range (%d,%d)", index, -1, b.i))
	}
	if index == -1 {
		return b.parent
	}
	return b.chain[index]
}

// OffsetTime modifies the time instance of a block, implicitly changing its
// associated difficulty. It's useful to test scenarios where forking is not
// tied to chain length directly.
func (b *RootBlockGen) SetDifficulty(value uint64) {
	b.header.Difficulty = new(big.Int).SetUint64(value)
}

// GenerateRootBlockChain creates a chain of n blocks. The first block's
// parent will be the provided parent. db is used to store
// intermediate states and should contain the parent's state trie.
//
// The generator function is called with a new block generator for
// every block. Any transactions and uncles added to the generator
// become part of the block. If gen is nil, the blocks will be empty
// and their coinbase will be the zero address.
//
// Blocks created by GenerateRootBlockChain do not contain valid proof of work
// values. Inserting them into BlockChain requires use of FakePow or
// a similar non-validating proof of work implementation.
func GenerateRootBlockChain(parent *types.RootBlock, engine consensus.Engine, n int, gen func(int, *RootBlockGen)) []*types.RootBlock {
	blocks := make([]*types.RootBlock, n)
	genblock := func(i int, parent *types.RootBlock) *types.RootBlock {
		b := &RootBlockGen{i: i, chain: blocks, parent: parent, engine: engine}
		b.header = makeRootBlockHeader(parent, engine.CalcDifficulty(nil, parent.Time(), parent.Header()))

		// Execute any user modifications to the block
		if gen != nil {
			gen(i, b)
		}

		return types.NewRootBlock(b.header, b.headers, nil)
	}
	for i := 0; i < n; i++ {
		block := genblock(i, parent)
		blocks[i] = block
		parent = block
	}
	return blocks
}

func makeRootBlockHeader(parent *types.RootBlock, difficulty *big.Int) *types.RootBlockHeader {
	var time uint64 = parent.Time() + 40

	return &types.RootBlockHeader{
		ParentHash: parent.Hash(),
		Coinbase:   parent.Coinbase(),
		Difficulty: difficulty,
		Number:     parent.Number() + 1,
		Time:       time,
	}
}

// makeHeaderChain creates a deterministic chain of headers rooted at parent.
func makeRootBlockHeaderChain(parent *types.RootBlockHeader, n int, engine consensus.Engine, seed int) []*types.RootBlockHeader {
	blocks := makeRootBlockChain(types.NewRootBlockWithHeader(parent), n, engine, seed)
	headers := make([]*types.RootBlockHeader, len(blocks))
	for i, block := range blocks {
		headers[i] = block.Header()
	}

	return headers
}

// makeBlockChain creates a deterministic chain of blocks rooted at parent.
func makeRootBlockChain(parent *types.RootBlock, n int, engine consensus.Engine, seed int) []*types.RootBlock {
	blocks := GenerateRootBlockChain(parent, engine, n, func(i int, b *RootBlockGen) {
		b.SetCoinbase(account.Address{Recipient: account.Recipient{0: byte(seed), 19: byte(i)}, FullShardKey: 0})
	})
	return blocks
}
