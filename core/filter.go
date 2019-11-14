// Copyright 2014 The go-ethereum Authors
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
	"errors"
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

type Backend interface {
	GetBlockByNumber(number uint64) types.IBlock
	GetReceiptsByHash(hash common.Hash) types.Receipts
	GetLogs(hash common.Hash) [][]*types.Log
	CurrentBlock() *types.MinorBlock
}

// Filter can be used to retrieve and filter logs.
type Filter struct {
	backend Backend

	addresses []common.Address
	topics    [][]common.Hash

	block      common.Hash // Block hash if filtering a single block
	begin, end uint64      // Range interval if filtering multiple blocks
}

// NewRangeFilter creates a new filter which uses a bloom filter on blocks to
// figure out whether a particular block is interesting or not.
func NewRangeFilter(backend Backend, begin, end uint64, addresses []common.Address, topics [][]common.Hash) *Filter {
	// Flatten the address and topic filter clauses into a single bloombits filter
	// system. Since the bloombits are not positional, nil topics are permitted,
	// which get flattened into a nil byte slice.
	var filters [][][]byte
	if len(addresses) > 0 {
		filter := make([][]byte, len(addresses))
		for i, address := range addresses {
			filter[i] = address.Bytes()
		}
		filters = append(filters, filter)
	}
	for _, topicList := range topics {
		filter := make([][]byte, len(topicList))
		for i, topic := range topicList {
			filter[i] = topic.Bytes()
		}
		filters = append(filters, filter)
	}

	// Create a generic filter and convert it into a range filter
	filter := newFilter(backend, addresses, topics)

	filter.begin = begin
	filter.end = end

	return filter
}

// newFilter creates a generic filter that can either filter based on a block hash,
// or based on range queries. The search criteria needs to be explicitly set.
func newFilter(backend Backend, addresses []common.Address, topics [][]common.Hash) *Filter {
	return &Filter{
		backend:   backend,
		addresses: addresses,
		topics:    topics,
	}
}

// Logs searches the blockchain for matching log entries, returning all from the
// first block that contains matches, updating the start of the filter accordingly.
func (f *Filter) Logs() ([]*types.Log, error) {
	// Gather all indexed logs, and finish with non indexed ones
	var (
		logs []*types.Log
		err  error
	)

	rest, err := f.unindexedLogs(f.end)
	logs = append(logs, rest...)
	return logs, err
}

// indexedLogs returns the logs matching the filter criteria based on raw block
// iteration and bloom matching.
func (f *Filter) unindexedLogs(end uint64) ([]*types.Log, error) {
	var logs []*types.Log

	for ; f.begin <= end; f.begin++ {
		block, ok := f.backend.GetBlockByNumber(f.begin).(*types.MinorBlock)
		if !ok {
			return nil, errors.New("no such block")
		}
		header := block.Header()
		fmt.Println("block", block.NumberU64(), len(block.Transactions()))
		found, err := f.blockLogs(header)
		if err != nil {
			return logs, err
		}
		logs = append(logs, found...)
	}
	return logs, nil
}

// blockLogs returns the logs matching the filter criteria within a single block.
func (f *Filter) blockLogs(header *types.MinorBlockHeader) (logs []*types.Log, err error) {
	fmt.Println("blockLosgs", header.Number)
	if bloomFilter(header.Bloom, f.addresses, f.topics) {
		fmt.Println("fillllll", header.Number)
		found, err := f.checkMatches(header)
		if err != nil {
			fmt.Println("sb", err)
			return logs, err
		}
		fmt.Println("addddd", len(found))
		logs = append(logs, found...)
	}
	return logs, nil
}

// checkMatches checks if the receipts belonging to the given header contain any log events that
// match the filter criteria. This function is called when the bloom filter signals a potential match.
func (f *Filter) checkMatches(header *types.MinorBlockHeader) (logs []*types.Log, err error) {
	// Get the logs of the block
	logsList := f.backend.GetLogs(header.Hash())
	var unfiltered []*types.Log
	for _, logs := range logsList {
		unfiltered = append(unfiltered, logs...)
	}
	logs = FilterLogs(unfiltered, nil, nil, f.addresses, f.topics)
	if len(logs) > 0 {
		// We have matching logs, check if we need to resolve full logs via the light client
		if logs[0].TxHash == (common.Hash{}) {
			receipts := f.backend.GetReceiptsByHash(header.Hash())
			unfiltered = unfiltered[:0]
			for _, receipt := range receipts {
				unfiltered = append(unfiltered, receipt.Logs...)
			}
			logs = FilterLogs(unfiltered, nil, nil, f.addresses, f.topics)
		}
		return logs, nil
	}
	return nil, nil
}

func includes(addresses []common.Address, a common.Address) bool {
	for _, addr := range addresses {
		if addr == a {
			return true
		}
	}

	return false
}

// filterLogs creates a slice of logs matching the given criteria.
func FilterLogs(logs []*types.Log, fromBlock, toBlock *big.Int, addresses []common.Address, topics [][]common.Hash) []*types.Log {
	var ret []*types.Log
Logs:
	for _, log := range logs {
		if fromBlock != nil && fromBlock.Int64() >= 0 && fromBlock.Uint64() > log.BlockNumber {
			continue
		}
		if toBlock != nil && toBlock.Int64() >= 0 && toBlock.Uint64() < log.BlockNumber {
			continue
		}

		if len(addresses) > 0 && !includes(addresses, log.Recipient) {
			continue
		}
		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(log.Topics) {
			continue Logs
		}
		for i, sub := range topics {
			match := len(sub) == 0 // empty rule set == wildcard
			for _, topic := range sub {
				if log.Topics[i] == topic {
					match = true
					break
				}
			}
			if !match {
				continue Logs
			}
		}
		ret = append(ret, log)
	}
	return ret
}

func bloomFilter(bloom types.Bloom, addresses []common.Address, topics [][]common.Hash) bool {
	if len(addresses) > 0 {
		var included bool
		for _, addr := range addresses {
			if types.BloomLookup(bloom, addr) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	for _, sub := range topics {
		included := len(sub) == 0 // empty rule set == wildcard
		for _, topic := range sub {
			if types.BloomLookup(bloom, topic) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	return true
}
