// Copyright 2016 The go-ethereum Authors
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
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"math/big"

	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/core/vm"
	"github.com/ethereum/go-ethereum/common"
)

// ChainContext supports retrieving Headers and consensus parameters from the
// current blockchain to be used during transaction processing.
type ChainContext interface {
	// Engine retrieves the chain's consensus engine.
	Engine() consensus.Engine
	Config() *config.QuarkChainConfig
	// GetHeader returns the hash corresponding to their hash.
	GetHeader(common.Hash) types.IHeader
}

func NewEVMContext(msg types.Message, mheader types.IHeader, chain ChainContext) vm.Context {
	header := mheader.(*types.MinorBlockHeader)
	return vm.Context{
		CanTransfer:     CanTransfer,
		Transfer:        Transfer,
		GetHash:         GetHashFn(header, chain),
		Origin:          msg.From(),
		Coinbase:        header.GetCoinbase().Recipient,
		BlockNumber:     new(big.Int).SetUint64(header.NumberU64()),
		Time:            new(big.Int).SetUint64(header.GetTime()),
		Difficulty:      new(big.Int).Set(header.GetDifficulty()),
		GasLimit:        header.GasLimit.Value.Uint64(),
		GasPrice:        new(big.Int).Set(msg.GasPrice()),
		ToFullShardKey:  msg.ToFullShardKey(),
		GasTokenID:      msg.GasTokenID(),
		TransferTokenID: msg.TransferTokenID(),
	}
}

// GetHashFn returns a GetHashFunc which retrieves header hashes by number
func GetHashFn(ref types.IHeader, chain ChainContext) func(n uint64) common.Hash {
	var cache map[uint64]common.Hash

	return func(n uint64) common.Hash {
		// If there's no hash cache yet, make one
		if cache == nil {
			cache = map[uint64]common.Hash{
				ref.NumberU64() - 1: ref.GetParentHash(),
			}
		}
		// Try to fulfill the request from the cache
		if hash, ok := cache[n]; ok {
			return hash
		}
		// Not cached, iterate the blocks and cache the hashes
		for header := chain.GetHeader(ref.GetParentHash()); header != nil; header = chain.GetHeader(header.GetParentHash()) {
			cache[header.NumberU64()-1] = header.GetParentHash()
			if n == header.NumberU64()-1 {
				return header.GetParentHash()
			}
		}
		return common.Hash{}
	}
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr common.Address, amount *big.Int, tokenID *big.Int) bool {
	return db.GetBalance(addr, tokenID).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *big.Int, tokenID *big.Int) {
	db.SubBalance(sender, amount, tokenID)
	db.AddBalance(recipient, amount, tokenID)
}
