// Modified from go-ethereum under GNU Lesser General Public License
package slave

import (
	"context"
	"sync"
	"time"

	"github.com/QuarkChain/goquarkchain/core"

	"github.com/QuarkChain/goquarkchain/cluster/slave/filters"
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/encoder"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
)

var (
	deadline = 5 * time.Minute // consider a filter inactive if it has not been polled for within deadline
)

// filter is a helper struct that holds meta information over the filter type
// and associated subscription in the event system.
type filter struct {
	typ      filters.Type
	deadline *time.Timer // filter is inactiv when deadline triggers
	hashes   []common.Hash
	crit     rpc.FilterQuery
	logs     []*types.Log
	s        *filters.Subscription // associated subscription in event system
}

// PublicFilterAPI offers support to create and manage filters. This will allow external clients to retrieve various
// information related to the Ethereum protocol such als blocks, transactions and logs.
type PublicFilterAPI struct {
	backend   filters.SlaveFilter
	quit      chan struct{}
	events    *filters.EventSystem
	filtersMu sync.Mutex
}

// NewPublicFilterAPI returns a new PublicFilterAPI instance.
func NewPublicFilterAPI(backend filters.SlaveFilter) *PublicFilterAPI {
	api := &PublicFilterAPI{
		backend: backend,
		events:  filters.NewEventSystem(backend),
	}

	return api
}

// NewPendingTransactions creates a subscription that is triggered each time a transaction
// enters the transaction pool and was signed from one of the transactions this nodes manages.
func (api *PublicFilterAPI) NewPendingTransactions(ctx context.Context, fullShardId hexutil.Uint) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	id := uint32(fullShardId)
	rpcSub := notifier.CreateSubscription()

	go func() {
		txlist := make(chan []*types.Transaction, filters.TxsChanSize)
		pendingTxSub := api.events.SubscribePendingTxs(txlist, id)

		for {
			select {
			case txs := <-txlist:
				for _, tx := range txs {
					mBlock, idx, err := api.backend.GetTransactionByHash(tx.Hash(), id)
					if err != nil {
						log.Error("failed to call getTransactionByHash when subscription pending transactions", "err", err)
						continue
					}
					data, err := encoder.TxEncoder(mBlock, int(idx), api.backend.GetClusterConfig())
					if err != nil {
						log.Error("failed to encode tx when subscription pending transactions", "err", err)
						continue
					}
					notifier.Notify(rpcSub.ID, data)
				}
			case <-rpcSub.Err():
				pendingTxSub.Unsubscribe()
				return
			case <-notifier.Closed():
				pendingTxSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// NewHeads send a notification each time a new (header) block is appended to the chain.
func (api *PublicFilterAPI) NewHeads(ctx context.Context, fullShardId hexutil.Uint) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		headers := make(chan *types.MinorBlockHeader, filters.ChainEvChanSize)
		headersSub := api.events.SubscribeNewHeads(headers, uint32(fullShardId))

		for {
			select {
			case h := <-headers:
				hd, err := encoder.MinorBlockHeaderEncoder(h)
				if err != nil {
					log.Error("encode MinorBlockHeader error", "err", err)
				} else {
					notifier.Notify(rpcSub.ID, hd)
				}

			case <-rpcSub.Err():
				headersSub.Unsubscribe()
				return
			case <-notifier.Closed():
				headersSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// Logs creates a subscription that fires for all new log that match the given filter criteria.
func (api *PublicFilterAPI) Logs(ctx context.Context, fullShardId hexutil.Uint, crit rpc.FilterQuery) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub      = notifier.CreateSubscription()
		matchedLogs = make(chan core.LoglistEvent, filters.LogsChanSize)
	)
	crit.FullShardId = uint32(fullShardId)

	logsSub, err := api.events.SubscribeLogs(crit, matchedLogs)
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case logs := <-matchedLogs:
				for _, loglist := range logs.Logs {
					for _, log := range loglist {
						notifier.Notify(rpcSub.ID, encoder.LogEncoder(log, logs.IsRemoved))
					}
				}
			case <-rpcSub.Err(): // client send an unsubscribe request
				logsSub.Unsubscribe()
				return
			case <-notifier.Closed(): // connection dropped
				logsSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

func (api *PublicFilterAPI) Syncing(ctx context.Context, fullShardId hexutil.Uint) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub = notifier.CreateSubscription()
	)

	go func() {
		statuses := make(chan *qsync.SyncingResult, filters.SyncSize)
		sub := api.events.SubscribeSyncing(statuses, uint32(fullShardId))
		for {
			select {
			case status := <-statuses:
				notifier.Notify(rpcSub.ID, status)
			case <-rpcSub.Err():
				sub.Unsubscribe()
				return
			case <-notifier.Closed():
				sub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}
