// Modified from go-ethereum under GNU Lesser General Public License
package slave

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/slave/filters"
	qsync "github.com/QuarkChain/goquarkchain/cluster/sync"
	"github.com/QuarkChain/goquarkchain/core"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/encoder"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	deadline = 5 * time.Minute // consider a filter inactive if it has not been polled for within deadline
)

type NetApi struct {
	version string
}

func NewNetApi(version string) *NetApi {
	return &NetApi{version: version}
}

func (e *NetApi) Version() string {
	return e.version
}

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
	backend     filters.SlaveFilter
	quit        chan struct{}
	events      *filters.EventSystem
	filtersMu   sync.Mutex
	shardId     uint32 // as default shardId
	shardFilter filters.ShardFilter
	hashMap     map[common.Hash]common.Hash
}

// NewPublicFilterAPI returns a new PublicFilterAPI instance.
func NewPublicFilterAPI(backend filters.SlaveFilter, shardId uint32) *PublicFilterAPI {
	api := &PublicFilterAPI{
		shardId: shardId,
		backend: backend,
		events:  filters.NewEventSystem(backend),
		hashMap: make(map[common.Hash]common.Hash),
	}

	return api
}

func (api *PublicFilterAPI) getShardFilter() filters.ShardFilter {
	if api.shardFilter == nil {
		shardFilter, err := api.backend.GetShardFilter(api.shardId)
		if err != nil {
			panic(fmt.Sprintf("Shard %d is not support for filter API", api.shardId))
		}
		api.shardFilter = shardFilter
	}
	return api.shardFilter
}

// NewPendingTransactions creates a subscription that is triggered each time a transaction
// enters the transaction pool and was signed from one of the transactions this nodes manages.
func (api *PublicFilterAPI) NewPendingTransactions(ctx context.Context, fullShardId *hexutil.Uint) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	id := api.shardId
	if fullShardId != nil {
		id = uint32(*fullShardId)
	}
	rpcSub := notifier.CreateSubscription()

	go func() {
		txlist := make(chan []*types.Transaction, filters.TxsChanSize)
		pendingTxSub := api.events.SubscribePendingTxs(txlist, id)

		for {
			select {
			case txs := <-txlist:
				for _, tx := range txs {
					mBlock, idx := api.shardFilter.GetTransactionByHash(tx.Hash())
					if mBlock == nil {
						log.Error("failed to call getTransactionByHash when subscription pending transactions", "err", "emtpy block")
						continue
					}
					data, err := encoder.TxEncoder(mBlock, int(idx))
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
func (api *PublicFilterAPI) NewHeads(ctx context.Context, fullShardId *hexutil.Uint) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()
	id := api.shardId
	if fullShardId != nil {
		id = uint32(*fullShardId)
	}

	go func() {
		blocks := make(chan *types.MinorBlock, filters.ChainEvChanSize)
		blocksSub := api.events.SubscribeNewHeads(blocks, id)

		for {
			select {
			case b := <-blocks:
				hd, err := encoder.MinorBlockHeaderEncoderForEthClient(b.Header(), b.Meta())
				if err != nil {
					log.Error("encode MinorBlockHeader error", "err", err)
				} else {
					hd["miner"] = b.Header().Coinbase.Recipient
					notifier.Notify(rpcSub.ID, hd)
				}

			case <-rpcSub.Err():
				blocksSub.Unsubscribe()
				return
			case <-notifier.Closed():
				blocksSub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// Logs creates a subscription that fires for all new log that match the given filter criteria.
func (api *PublicFilterAPI) Logs(ctx context.Context, crit rpc.FilterQuery, fullShardId *hexutil.Uint) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub      = notifier.CreateSubscription()
		matchedLogs = make(chan core.LoglistEvent, filters.LogsChanSize)
	)
	if fullShardId != nil {
		crit.FullShardId = uint32(*fullShardId)
	} else {
		crit.FullShardId = api.shardId
	}

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

// Syncing provides information when this node starts synchronising with the Ethereum network and when it's finished.
func (api *PublicFilterAPI) Syncing(ctx context.Context, fullShardId *hexutil.Uint) (*rpc.Subscription, error) {
	notifier, supported := rpc.NotifierFromContext(ctx)
	if !supported {
		return &rpc.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	var (
		rpcSub = notifier.CreateSubscription()
	)
	id := api.shardId
	if fullShardId != nil {
		id = uint32(*fullShardId)
	}

	go func() {
		statuses := make(chan *qsync.SyncingResult, filters.SyncSize)
		sub := api.events.SubscribeSyncing(statuses, id)
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

func (api *PublicFilterAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	ethtx := new(ethTypes.Transaction)
	if err := rlp.DecodeBytes(encodedTx, ethtx); err != nil {
		return common.Hash{}, err
	}
	log.Info("SendRawTransaction: get ethtx", "hash", common.Bytes2Hex(ethtx.Hash().Bytes()))
	evmTx := new(types.EvmTransaction)
	shardId := api.shardId - (api.shardId & 65535)
	if ethtx.To() != nil {
		evmTx = types.NewEvmTransaction(ethtx.Nonce(), *ethtx.To(), ethtx.Value(), ethtx.Gas(), ethtx.GasPrice(), shardId,
			shardId, api.getShardFilter().GetEthChainID(), 2, ethtx.Data(), 35760, 35760)
	} else {
		evmTx = types.NewEvmContractCreation(ethtx.Nonce(), ethtx.Value(), ethtx.Gas(), ethtx.GasPrice(), shardId,
			shardId, api.getShardFilter().GetEthChainID(), 2, ethtx.Data(), 35760, 35760)
	}
	evmTx.SetVRS(ethtx.RawSignatureValues())
	tx := &types.Transaction{
		TxType: types.EvmTx,
		EvmTx:  evmTx,
	}
	log.Info("SendRawTransaction: get evmtx", "hash", common.Bytes2Hex(evmTx.Hash().Bytes()))
	log.Info("SendRawTransaction: get tx", "hash", common.Bytes2Hex(tx.Hash().Bytes()))
	err := api.getShardFilter().AddTransactionAndBroadcast(tx)
	if err != nil {
		return common.Hash{}, err
	}
	api.hashMap[ethtx.Hash()] = tx.Hash()

	return tx.Hash(), nil
}

func (api *PublicFilterAPI) ChainId(ctx context.Context) (*hexutil.Big, error) {
	chainID := api.getShardFilter().GetEthChainID()
	return (*hexutil.Big)(new(big.Int).SetUint64(uint64(chainID))), nil
}

func (api *PublicFilterAPI) GetHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (map[string]interface{}, error) {
	height := number.Uint64()
	block, err := api.getShardFilter().GetMinorBlock(common.Hash{}, &height)
	if err != nil {
		return nil, err
	}
	return encoder.MinorBlockHeaderEncoderForEthClient(block.Header(), block.Meta())
}

// GetBlockByNumber returns the requested canonical block.
// * When blockNr is -1 the chain head is returned.
// * When blockNr is -2 the pending chain head is returned.
// * When fullTx is true all transactions in the block are returned, otherwise
//   only the transaction hash is returned.
func (api *PublicFilterAPI) GetBlockByNumber(ctx context.Context, number rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	height := number.Uint64()
	block, err := api.getShardFilter().GetMinorBlock(common.Hash{}, &height)
	if err != nil {
		return nil, err
	}
	return encoder.MinorBlockEncoderForEthClient(block, true, fullTx)
}

// GetBlockByNumber returns the requested canonical block.
// * When blockNr is -1 the chain head is returned.
// * When blockNr is -2 the pending chain head is returned.
// * When fullTx is true all transactions in the block are returned, otherwise
//   only the transaction hash is returned.
func (api *PublicFilterAPI) GetBlockByHash(ctx context.Context, hash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block, err := api.getShardFilter().GetMinorBlock(hash, nil)
	if err != nil {
		return nil, err
	}
	return encoder.MinorBlockEncoderForEthClient(block, true, fullTx)
}

func (api *PublicFilterAPI) BlockNumber(ctx context.Context) hexutil.Uint64 {
	header, _ := api.getShardFilter().GetHeaderByNumber(rpc.LatestBlockNumber) // latest header should always be available
	return hexutil.Uint64(header.NumberU64())
}

// GetTransactionByHash returns the transaction for the given hash
func (api *PublicFilterAPI) GetTransactionByHash(ctx context.Context, ethhash common.Hash) (*encoder.RPCTransaction, error) {
	hash, ok := api.hashMap[ethhash]
	if !ok {
		hash = ethhash
	}
	// Try to return an already finalized transaction
	log.Info("GetTransactionByHash:", "ethhash", common.Bytes2Hex(ethhash.Bytes()), "hash", common.Bytes2Hex(hash.Bytes()))
	block, index := api.getShardFilter().GetTransactionByHash(hash)
	if block == nil {
		return nil, nil
	}
	if len(block.GetTransactions()) <= int(index) {
		return nil, fmt.Errorf("GetTransactionByHash error %s", hash)
	}
	tx := block.GetTransactions()[index]
	log.Info("GetTransactionByHash: get block and tx", "index", index)
	if block.NumberU64() == 0 {
		return encoder.NewRPCTransaction(tx, common.Hash{}, 0, 0), nil
	}
	// Transaction unknown, return as such
	return encoder.NewRPCTransaction(tx, block.Hash(), block.Number(), uint64(index)), nil
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (api *PublicFilterAPI) GetTransactionReceipt(ctx context.Context, ethhash common.Hash) (map[string]interface{}, error) {
	hash, ok := api.hashMap[ethhash]
	if !ok {
		hash = ethhash
	}
	block, index := api.getShardFilter().GetTransactionByHash(hash)
	if block == nil {
		return nil, nil
	}
	if len(block.GetTransactions()) <= int(index) {
		return nil, nil
	}
	tx := block.GetTransactions()[index]
	receipts, err := api.getShardFilter().GetReceiptsByHash(block.Hash())
	if err != nil {
		return nil, err
	}
	if len(receipts) <= int(index) {
		return nil, nil
	}
	receipt := receipts[index]

	signer := types.NewEIP155Signer(tx.EvmTx.NetworkId())

	from, _ := types.Sender(signer, tx.EvmTx)

	fields := map[string]interface{}{
		"blockHash":         block.Hash(),
		"blockNumber":       hexutil.Uint64(block.NumberU64()),
		"transactionHash":   hash,
		"transactionIndex":  hexutil.Uint64(index),
		"from":              from,
		"to":                tx.EvmTx.To(),
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
	}

	// Assign receipt status or post state.
	if len(receipt.PostState) > 0 {
		fields["root"] = hexutil.Bytes(receipt.PostState)
	} else {
		fields["status"] = hexutil.Uint(receipt.Status)
	}
	if receipt.Logs == nil {
		fields["logs"] = [][]*types.Log{}
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress
	}
	return fields, nil
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (api *PublicFilterAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNrOrHash rpc.BlockNumberOrHash) (*hexutil.Uint64, error) {
	nonce, err := api.getShardFilter().GetTransactionCount(address, blockNrOrHash)
	log.Info("GetTransactionCount: ", "nonce", nonce)
	return (*hexutil.Uint64)(nonce), err
}

// MetaCallArgs represents the arguments for a call.
type MetaCallArgs struct {
	From     *account.Recipient `json:"from"`
	To       *account.Recipient `json:"to"`
	Gas      hexutil.Big        `json:"gas"`
	GasPrice hexutil.Big        `json:"gasPrice"`
	Value    hexutil.Big        `json:"value"`
	Data     hexutil.Bytes      `json:"data"`
}

func toTransaction(a *MetaCallArgs, shardId uint32, networkID uint32) *types.Transaction {
	defaultToken := uint64(35760)
	evmTx := new(types.EvmTransaction)
	if a.To == nil {
		evmTx = types.NewEvmContractCreation(0, a.Value.ToInt(), a.Gas.ToInt().Uint64(), a.GasPrice.ToInt(),
			shardId, shardId, networkID, 0, a.Data, defaultToken, defaultToken)
	} else {
		evmTx = types.NewEvmTransaction(0, *a.To, a.Value.ToInt(), a.Gas.ToInt().Uint64(), a.GasPrice.ToInt(),
			shardId, shardId, networkID, 0, a.Data, defaultToken, defaultToken)
	}
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	return tx
}

func (api *PublicFilterAPI) Call(mdata MetaCallArgs, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	tx := toTransaction(&mdata, api.shardId, api.getShardFilter().GetNetworkId())
	var (
		result []byte
		err    error
	)
	if blockNr == nil || blockNr.Int64() < 0 {
		result, err = api.shardFilter.ExecuteTx(tx, &account.Address{*mdata.From, api.shardId}, nil)
	} else {
		number := blockNr.Uint64()
		result, err = api.shardFilter.ExecuteTx(tx, &account.Address{*mdata.From, api.shardId}, &number)
	}

	if err != nil {
		return nil, err
	}

	return hexutil.Bytes(result), nil
}

func (api *PublicFilterAPI) EstimateGas(mdata MetaCallArgs) (hexutil.Uint, error) {
	tx := toTransaction(&mdata, api.shardId, api.getShardFilter().GetEthChainID())
	result, err := api.shardFilter.EstimateGas(tx, &account.Address{*mdata.From, api.shardId})
	return hexutil.Uint(result), err
}
