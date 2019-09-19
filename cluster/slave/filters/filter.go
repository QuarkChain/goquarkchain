// Modified from go-ethereum under GNU Lesser General Public License
package filters

import (
	"context"
	"github.com/QuarkChain/goquarkchain/core"
	"math/big"

	qrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
)

type ShardBackend interface {
	GetHeaderByNumber(height qrpc.BlockNumber) (*types.MinorBlockHeader, error)
	GetHeaderByHash(blockHash common.Hash) (*types.MinorBlockHeader, error)
	GetReceiptsByHash(hash common.Hash) (types.Receipts, error)
	GetLogs(hash common.Hash) ([][]*types.Log, error)

	SubscribeChainHeadEvent(ch chan<- core.MinorChainHeadEvent) event.Subscription
	SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription
	SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription
	SubscribeChainEvent(ch chan<- core.MinorChainEvent) event.Subscription
	SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription
	SubscribeSyncEvent(ch chan<- *qsync.SyncingResult) event.Subscription
}

// Filter can be used to retrieve and filter logs.
type Filter struct {
	backend ShardBackend

	addresses []common.Address
	topics    [][]common.Hash

	block      common.Hash // Block hash if filtering a single block
	begin, end int64       // Range interval if filtering multiple blocks
}

// NewRangeFilter creates a new filter which uses a bloom filter on blocks to
// figure out whether a particular block is interesting or not.

func NewRangeFilter(backend ShardBackend, begin, end int64, addresses []common.Address, topics [][]common.Hash) *Filter {
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

// NewBlockFilter creates a new filter which directly inspects the contents of
// a block to figure out whether it is interesting or not.
func NewBlockFilter(backend ShardBackend, block common.Hash, addresses []common.Address, topics [][]common.Hash) *Filter {
	// Create a generic filter and convert it into a block filter
	filter := newFilter(backend, addresses, topics)
	filter.block = block
	return filter
}

// newFilter creates a generic filter that can either filter based on a block hash,
// or based on range queries. The search criteria needs to be explicitly set.
func newFilter(backend ShardBackend, addresses []common.Address, topics [][]common.Hash) *Filter {
	return &Filter{
		backend:   backend,
		addresses: addresses,
		topics:    topics,
	}
}

// Logs searches the blockchain for matching log entries, returning all from the
// first block that contains matches, updating the start of the filter accordingly.
func (f *Filter) Logs(ctx context.Context) ([]*types.Log, error) {
	// If we're doing singleton block filtering, execute and return
	if f.block != (common.Hash{}) {
		header, err := f.backend.GetHeaderByHash(f.block)
		if err != nil {
			return nil, err
		}
		return f.blockLogs(ctx, header)
	}
	// Figure out the limits of the filter range
	header, err := f.backend.GetHeaderByNumber(qrpc.LatestBlockNumber)
	if err != nil {
		return nil, err
	}
	head := header.Number

	if f.begin == -1 {
		f.begin = int64(head)
	}
	end := uint64(f.end)
	if f.end == -1 {
		end = head
	}
	// Gather all indexed logs, and finish with non indexed ones
	var (
		logs []*types.Log
	)

	rest, err := f.unindexedLogs(ctx, end)
	logs = append(logs, rest...)
	return logs, err
}

// indexedLogs returns the logs matching the filter criteria based on raw block
// iteration and bloom matching.
func (f *Filter) unindexedLogs(ctx context.Context, end uint64) ([]*types.Log, error) {
	var logs []*types.Log

	for ; f.begin <= int64(end); f.begin++ {
		header, err := f.backend.GetHeaderByNumber(qrpc.BlockNumber(f.begin))
		if header == nil || err != nil {
			return logs, err
		}
		found, err := f.blockLogs(ctx, header)
		if err != nil {
			return logs, err
		}
		logs = append(logs, found...)
	}
	return logs, nil
}

// blockLogs returns the logs matching the filter criteria within a single block.
func (f *Filter) blockLogs(ctx context.Context, header *types.MinorBlockHeader) (logs []*types.Log, err error) {
	if bloomFilter(header.Bloom, f.addresses, f.topics) {
		found, err := f.checkMatches(ctx, header)
		if err != nil {
			return logs, err
		}
		logs = append(logs, found...)
	}
	return logs, nil
}

// checkMatches checks if the receipts belonging to the given header contain any log events that
// match the filter criteria. This function is called when the bloom filter signals a potential match.
func (f *Filter) checkMatches(ctx context.Context, header *types.MinorBlockHeader) (logs []*types.Log, err error) {
	// Get the logs of the block
	logsList, err := f.backend.GetLogs(header.Hash())
	if err != nil {
		return nil, err
	}

	var unfiltered []*types.Log
	for _, logs := range logsList {
		unfiltered = append(unfiltered, logs...)
	}
	logs = filterLogs(unfiltered, nil, nil, f.addresses, f.topics)
	if len(logs) > 0 {
		// We have matching logs, check if we need to resolve full logs via the light client
		if logs[0].TxHash == (common.Hash{}) {
			receipts, err := f.backend.GetReceiptsByHash(header.Hash())
			if err != nil {
				return nil, err
			}
			unfiltered = unfiltered[:0]
			for _, receipt := range receipts {
				unfiltered = append(unfiltered, receipt.Logs...)
			}
			logs = filterLogs(unfiltered, nil, nil, f.addresses, f.topics)
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
func filterLogs(logs []*types.Log, fromBlock, toBlock *big.Int, addresses []common.Address, topics [][]common.Hash) []*types.Log {
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
