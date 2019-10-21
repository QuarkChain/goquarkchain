package qkcapi

import (
	"bytes"
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	qrpc "github.com/QuarkChain/goquarkchain/cluster/rpc"
	qcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/internal/encoder"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
	"sort"
)

type CommonAPI struct {
	b Backend
}

func (c *CommonAPI) callOrEstimateGas(args *CallArgs, height *uint64, isCall bool) (hexutil.Bytes, error) {
	if args.To == nil {
		return nil, errors.New("missing to")
	}
	args.setDefaults()
	tx, err := args.toTx(c.b.GetClusterConfig().Quarkchain)
	if err != nil {
		return nil, err
	}
	if isCall {
		res, err := c.b.ExecuteTransaction(tx, args.From, height)
		if err != nil {
			return nil, err
		}
		return (hexutil.Bytes)(res), nil
	}
	data, err := c.b.EstimateGas(tx, args.From)
	if err != nil {
		return nil, err
	}
	return qcom.Uint32ToBytes(data), nil
}

func (c *CommonAPI) SendRawTransaction(encodedTx hexutil.Bytes) (hexutil.Bytes, error) {
	evmTx := new(types.EvmTransaction)
	if err := rlp.DecodeBytes(encodedTx, evmTx); err != nil {
		return nil, err
	}
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}

	if err := c.b.AddTransaction(tx); err != nil {
		return EmptyTxID, err
	}
	return encoder.IDEncoder(tx.Hash().Bytes(), tx.EvmTx.FromFullShardKey()), nil
}

func (c *CommonAPI) GetTransactionReceipt(txID hexutil.Bytes) (map[string]interface{}, error) {
	txHash, fullShardKey, err := encoder.IDDecoder(txID)
	if err != nil {
		return nil, err
	}

	fullShardId, err := clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardId}
	minorBlock, index, receipt, err := c.b.GetTransactionReceipt(txHash, branch)
	if err != nil {
		return nil, err
	}
	ret, err := encoder.ReceiptEncoder(minorBlock, int(index), receipt)
	if ret["transactionId"].(string) == "" {
		ret["transactionId"] = txID.String()
		ret["transactionHash"] = txHash.String()
	}
	return ret, err
}

func (c *CommonAPI) GetLogs(args *rpc.FilterQuery, fullShardKey *hexutil.Uint) ([]map[string]interface{}, error) {
	fullShardID, err := getFullShardId(fullShardKey)
	if err != nil {
		return nil, err
	}
	lastBlockHeight, err := c.b.GetLastMinorBlockByFullShardID(fullShardID)
	if err != nil {
		return nil, err
	}
	if args.FromBlock == nil || args.FromBlock.Int64() == rpc.LatestBlockNumber.Int64() {
		args.FromBlock = new(big.Int).SetUint64(lastBlockHeight)
	}

	if args.ToBlock == nil || args.ToBlock.Int64() == rpc.LatestBlockNumber.Int64() {
		args.ToBlock = new(big.Int).SetUint64(lastBlockHeight)
	}

	args.FullShardId = fullShardID

	log, err := c.b.GetLogs(args)
	return encoder.LogListEncoder(log, false), nil
}

// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	CommonAPI
	b Backend
}

// NewPublicBlockChainAPI creates a new QuarkChain blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{CommonAPI: CommonAPI{b: b}, b: b}
}

// Echoquantity :should use data without leading zero
func (p *PublicBlockChainAPI) Echoquantity(data hexutil.Big) *hexutil.Big {
	return &data

}

// EchoData echo data for test
func (p *PublicBlockChainAPI) EchoData(data hexutil.Big) *hexutil.Big {
	return &data
}

func (p *PublicBlockChainAPI) NetworkInfo() map[string]interface{} {

	type ChainIdToShardSize struct {
		chainID   uint32
		shardSize uint32
	}
	ChainIdToShardSizeList := make([]ChainIdToShardSize, 0)
	for _, v := range clusterCfg.Quarkchain.Chains {
		ChainIdToShardSizeList = append(ChainIdToShardSizeList, ChainIdToShardSize{chainID: v.ChainID, shardSize: v.ShardSize})
	}
	sort.Slice(ChainIdToShardSizeList, func(i, j int) bool { return ChainIdToShardSizeList[i].chainID < ChainIdToShardSizeList[j].chainID }) //Right???
	shardSize := make([]hexutil.Uint, 0)
	for _, v := range ChainIdToShardSizeList {
		shardSize = append(shardSize, hexutil.Uint(v.shardSize))
	}
	return map[string]interface{}{
		"networkId":        hexutil.Uint(clusterCfg.Quarkchain.NetworkID),
		"chainSize":        hexutil.Uint(clusterCfg.Quarkchain.ChainSize),
		"shardSizes":       shardSize,
		"syncing":          p.b.IsSyncing(),
		"mining":           p.b.IsMining(),
		"shardServerCount": p.b.GetSlavePoolLen(),
	}

}

func (p *PublicBlockChainAPI) getPrimaryAccountData(address account.Address, blockNr *rpc.BlockNumber) (data *qrpc.AccountBranchData, err error) {
	if blockNr == nil {
		data, err = p.b.GetPrimaryAccountData(&address, nil)
		return
	}

	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}

	return p.b.GetPrimaryAccountData(&address, blockNumber)
}

func (p *PublicBlockChainAPI) GetTransactionCount(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Uint64, error) {
	data, err := p.getPrimaryAccountData(address, blockNr)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(data.TransactionCount), nil
}

func (p *PublicBlockChainAPI) GetBalances(address account.Address, blockNr *rpc.BlockNumber) (map[string]interface{}, error) {
	data, err := p.getPrimaryAccountData(address, blockNr)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: data.Branch}
	balances := data.Balance
	fields := map[string]interface{}{
		"branch":      hexutil.Uint64(branch.Value),
		"fullShardId": hexutil.Uint64(branch.GetFullShardID()),
		"shardId":     hexutil.Uint64(branch.GetShardID()),
		"chainId":     hexutil.Uint64(branch.GetChainID()),
		"balances":    encoder.BalancesEncoder(balances),
	}
	return fields, nil
}

func (p *PublicBlockChainAPI) GetAccountData(address account.Address, blockNr *rpc.BlockNumber, includeShards *bool) (map[string]interface{}, error) {
	if includeShards != nil && blockNr == nil {
		return nil, errors.New("do not allow specify height if client wants info on all shards")
	}
	if includeShards == nil {
		t := false
		includeShards = &t
	}
	if !(*includeShards) {
		accountBranchData, err := p.getPrimaryAccountData(address, blockNr)
		if err != nil {
			return nil, err
		}
		branch := account.Branch{Value: accountBranchData.Branch}
		primary := map[string]interface{}{
			"fullShardId":        hexutil.Uint(branch.GetFullShardID()),
			"shardId":            hexutil.Uint(branch.GetShardID()),
			"chainId":            hexutil.Uint(branch.GetChainID()),
			"balances":           encoder.BalancesEncoder(accountBranchData.Balance),
			"transactionCount":   hexutil.Uint64(accountBranchData.TransactionCount),
			"isContract":         accountBranchData.IsContract,
			"minedBlocks":        hexutil.Uint64(accountBranchData.MinedBlocks),
			"poswMineableBlocks": hexutil.Uint64(accountBranchData.PoswMineableBlocks),
		}
		return map[string]interface{}{
			"primary": primary,
		}, nil
	}
	branchToAccountBranchData, err := p.b.GetAccountData(&address, nil)
	if err != nil {
		return nil, err
	}

	shards := make([]map[string]interface{}, 0)
	primary := make(map[string]interface{})
	for branch, accountBranchData := range branchToAccountBranchData {
		branch := account.Branch{Value: branch}
		shardData := map[string]interface{}{
			"fullShardId":      hexutil.Uint(branch.GetFullShardID()),
			"shardId":          hexutil.Uint(branch.GetShardID()),
			"chainId":          hexutil.Uint(branch.GetChainID()),
			"balances":         encoder.BalancesEncoder(accountBranchData.Balance),
			"transactionCount": hexutil.Uint(accountBranchData.TransactionCount),
			"isContract":       accountBranchData.IsContract,
		}
		shards = append(shards, shardData)
		fullShardIDByConfig, err := clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(address.FullShardKey)
		if err != nil {
			return nil, err
		}
		if branch.GetFullShardID() == fullShardIDByConfig {
			primary = shardData
		}
	}
	return map[string]interface{}{
		"primary": primary,
		"shards":  shards,
	}, nil

}

func (p *PublicBlockChainAPI) SendTransaction(args SendTxArgs) (hexutil.Bytes, error) {
	if err := args.setDefaults(clusterCfg.Quarkchain); err != nil {
		return nil, err
	}
	tx, err := args.toTransaction()
	if err != nil {
		return nil, err
	}
	if err := p.b.AddTransaction(tx); err != nil {
		return EmptyTxID, err
	}
	return encoder.IDEncoder(tx.Hash().Bytes(), tx.EvmTx.FromFullShardKey()), nil
}

func (p *PublicBlockChainAPI) GetRootBlockByHash(hash common.Hash, needExtraInfo *bool) (map[string]interface{}, error) {
	if needExtraInfo == nil {
		temp := true
		needExtraInfo = &temp
	}
	rootBlock, poswInfo, err := p.b.GetRootBlockByHash(hash, *needExtraInfo)
	if err != nil {
		return nil, err
	}
	return encoder.RootBlockEncoder(rootBlock, poswInfo)
}

func (p *PublicBlockChainAPI) GetRootBlockByHeight(heightInput *hexutil.Uint64, needExtraInfo *bool) (map[string]interface{}, error) {
	blockHeight, err := transHexutilUint64ToUint64(heightInput)
	if err != nil {
		return nil, err
	}
	if needExtraInfo == nil {
		temp := true
		needExtraInfo = &temp
	}
	rootBlock, poswInfo, err := p.b.GetRootBlockByNumber(blockHeight, *needExtraInfo)
	if err != nil {
		return nil, err
	}
	response, err := encoder.RootBlockEncoder(rootBlock, poswInfo)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func (p *PublicBlockChainAPI) GetMinorBlockById(blockID hexutil.Bytes, includeTxs *bool, needExtraInfo *bool) (map[string]interface{}, error) {
	if includeTxs == nil {
		temp := false
		includeTxs = &temp
	}

	if needExtraInfo == nil {
		temp := true
		needExtraInfo = &temp
	}
	blockHash, fullShardKey, err := encoder.IDDecoder(blockID)
	if err != nil {
		return nil, err
	}
	fullShardId, err := clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardId}
	minorBlock, extra, err := p.b.GetMinorBlockByHash(blockHash, branch, *needExtraInfo)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
	}
	return encoder.MinorBlockEncoder(minorBlock, *includeTxs, extra)

}

func (p *PublicBlockChainAPI) GetMinorBlockByHeight(fullShardKey hexutil.Uint, heightInput *hexutil.Uint64, includeTxs *bool, needExtraInfo *bool) (map[string]interface{}, error) {
	height, err := transHexutilUint64ToUint64(heightInput)
	if err != nil {
		return nil, err
	}
	if needExtraInfo == nil {
		temp := true
		needExtraInfo = &temp
	}
	if includeTxs == nil {
		temp := false
		includeTxs = &temp
	}

	fullShardId, err := getFullShardId(&fullShardKey)
	if err != nil {
		return nil, err
	}
	minorBlock, extraData, err := p.b.GetMinorBlockByHeight(height, account.Branch{Value: fullShardId}, *needExtraInfo)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
	}
	return encoder.MinorBlockEncoder(minorBlock, *includeTxs, extraData)
}

func (p *PublicBlockChainAPI) GetTransactionById(txID hexutil.Bytes) (map[string]interface{}, error) {
	txHash, fullShardKey, err := encoder.IDDecoder(txID)
	if err != nil {
		return nil, err
	}
	fullShardIDByConfig, err := clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(uint32(fullShardKey))
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardIDByConfig}
	minorBlock, index, err := p.b.GetTransactionByHash(txHash, branch)
	if err != nil {
		return nil, err
	}
	if len(minorBlock.Transactions()) <= int(index) {
		return nil, errors.New("index bigger than block's tx")
	}
	return encoder.TxEncoder(minorBlock, int(index))
}

func (p *PublicBlockChainAPI) Call(data CallArgs, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	if blockNr == nil {
		return p.CommonAPI.callOrEstimateGas(&data, nil, true)
	}
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	return p.CommonAPI.callOrEstimateGas(&data, blockNumber, true)

}

func (p *PublicBlockChainAPI) EstimateGas(data CallArgs) ([]byte, error) {
	return p.CommonAPI.callOrEstimateGas(&data, nil, false)
}

func (p *PublicBlockChainAPI) GetLogs(args *rpc.FilterQuery, fullShardKey hexutil.Uint) ([]map[string]interface{}, error) {
	return p.CommonAPI.GetLogs(args, &fullShardKey)
}

func (p *PublicBlockChainAPI) GetStorageAt(address account.Address, key common.Hash, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	hash, err := p.b.GetStorageAt(&address, key, blockNumber)
	return hash.Bytes(), err
}

func (p *PublicBlockChainAPI) GetCode(address account.Address, blockNr *rpc.BlockNumber) (hexutil.Bytes, error) {
	blockNumber, err := decodeBlockNumberToUint64(p.b, blockNr)
	if err != nil {
		return nil, err
	}
	return p.b.GetCode(&address, blockNumber)
}

func (p *PublicBlockChainAPI) GetTransactionsByAddress(address account.Address, start *hexutil.Bytes, limit *hexutil.Uint, transferTokenID *hexutil.Uint64) (map[string]interface{}, error) {
	limitValue := uint32(0)
	if limit != nil {
		limitValue = uint32(*limit)
	}
	if limitValue > 20 {
		limitValue = 20
	}
	startValue := make([]byte, 0)
	if start != nil {
		startValue = *start
	}

	transferTokenIDValue := new(uint64)
	if transferTokenID == nil {
		transferTokenIDValue = nil
	} else {
		t := uint64(*transferTokenID)
		transferTokenIDValue = &t
	}

	txs, next, err := p.b.GetTransactionsByAddress(&address, startValue, limitValue, transferTokenIDValue)
	if err != nil {
		return nil, err
	}

	return makeGetTransactionRes(txs, next)
}

func txDetailEncode(tx *qrpc.TransactionDetail) (map[string]interface{}, error) {
	toData := "0x"
	if tx.ToAddress != nil {
		toData = tx.ToAddress.ToHex()
	}
	transferTokenStr, err := qcom.TokenIdDecode(tx.TransferTokenID)
	if err != nil {
		return nil, err
	}
	gasTokenStr, err := qcom.TokenIdDecode(tx.GasTokenID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"txId":             encoder.IDEncoder(tx.TxHash.Bytes(), tx.FromAddress.FullShardKey),
		"fromAddress":      tx.FromAddress,
		"toAddress":        toData,
		"value":            (*hexutil.Big)(tx.Value.Value),
		"transferTokenId":  hexutil.Uint64(tx.TransferTokenID),
		"transferTokenStr": transferTokenStr,
		"gasTokenId":       hexutil.Uint64(tx.GasTokenID),
		"gasTokenStr":      gasTokenStr,
		"blockHeight":      hexutil.Uint(tx.BlockHeight),
		"timestamp":        hexutil.Uint(tx.Timestamp),
		"success":          tx.Success,
		"isFromRootChain":  tx.IsFromRootChain,
	}, nil
}

func (p *PublicBlockChainAPI) GetAllTransaction(fullShardKey hexutil.Uint, start *hexutil.Bytes, limit *hexutil.Uint) (map[string]interface{}, error) {
	var (
		err        error
		startValue = make([]byte, 0)
		limitValue = uint32(10)
	)
	if start != nil {
		startValue, err = start.MarshalText()
		if err != nil {
			return nil, err
		}
	}

	if limit != nil {
		limitValue = uint32(*limit)
	}

	if limitValue > 20 {
		limitValue = 20
	}

	fullShardID, err := getFullShardId(&fullShardKey)
	if err != nil {
		return nil, err
	}
	branch := account.Branch{Value: fullShardID}
	txs, next, err := p.b.GetAllTx(branch, startValue, limitValue)
	if err != nil {
		return nil, err
	}
	return makeGetTransactionRes(txs, next)

}

func makeGetTransactionRes(txs []*qrpc.TransactionDetail, next []byte) (map[string]interface{}, error) {
	txsFields := make([]map[string]interface{}, 0)
	for _, tx := range txs {
		txField, err := txDetailEncode(tx)
		if err != nil {
			return nil, err
		}
		txsFields = append(txsFields, txField)
	}
	return map[string]interface{}{
		"txList": txsFields,
		"next":   hexutil.Bytes(next),
	}, nil
}

func (p *PublicBlockChainAPI) GasPrice(fullShardKey hexutil.Uint, tokenID *string) (hexutil.Uint64, error) {
	fullShardId, err := getFullShardId(&fullShardKey)
	if err != nil {
		return hexutil.Uint64(0), err
	}
	tokenIDValue := DefaultTokenID
	if tokenID != nil {
		tokenIDValue = *tokenID
	}
	data, err := p.b.GasPrice(account.Branch{Value: fullShardId}, qcom.TokenIDEncode(tokenIDValue))
	return hexutil.Uint64(data), err
}

func (p *PublicBlockChainAPI) SubmitWork(fullShardKey *hexutil.Uint, headHash common.Hash, nonce hexutil.Uint64, mixHash common.Hash, signature *hexutil.Bytes) (bool, error) {
	var fullShardId *uint32
	if fullShardKey != nil {
		id, err := getFullShardId(fullShardKey)
		if err != nil {
			return false, err
		}
		fullShardId = &id
	}

	if signature != nil && len(*signature) != 65 {
		return false, errors.New("invalid signature, len should be 65")
	}

	var sig *[65]byte = nil
	if signature != nil {
		sig = new([65]byte)
		copy(sig[:], *signature)
	}

	submit, err := p.b.SubmitWork(fullShardId, headHash, uint64(nonce), mixHash, sig)
	if err != nil {
		log.Error("Submit remote minered block", "err", err)
		return false, err
	}
	return submit, nil
}

func (p *PublicBlockChainAPI) GetWork(fullShardKey *hexutil.Uint, coinbaseAddress *common.Address) ([]common.Hash, error) {
	var fullShardId *uint32
	if fullShardKey != nil {
		id, err := getFullShardId(fullShardKey)
		if err != nil {
			return nil, err
		}
		fullShardId = &id
	}

	work, err := p.b.GetWork(fullShardId, coinbaseAddress)
	if err != nil {
		return nil, err
	}
	height := new(big.Int).SetUint64(work.Number)
	var val = make([]common.Hash, 0, 3)
	val = append(val, work.HeaderHash)
	val = append(val, common.BytesToHash(height.Bytes()))
	val = append(val, common.BytesToHash(work.Difficulty.Bytes()))
	if work.OptionalDivider > 1 {
		val = append(val, common.BytesToHash(qcom.Uint64ToBytes(work.OptionalDivider)))
	}
	return val, nil
}

func (p *PublicBlockChainAPI) GetRootHashConfirmingMinorBlockById(mBlockID hexutil.Bytes) hexutil.Bytes {
	return p.b.GetRootHashConfirmingMinorBlock(mBlockID).Bytes() //key mHash , value rHash
}

func (p *PublicBlockChainAPI) GetTransactionConfirmedByNumberRootBlocks(txID hexutil.Bytes) (hexutil.Uint, error) {
	txHash, fullShardKey, err := encoder.IDDecoder(txID)
	if err != nil {
		return hexutil.Uint(0), err
	}
	fullShardID, err := clusterCfg.Quarkchain.GetFullShardIdByFullShardKey(fullShardKey)
	if err != nil {
		return hexutil.Uint(0), err
	}

	mBlock, _, err := p.b.GetTransactionByHash(txHash, account.Branch{Value: fullShardID})
	if err != nil {
		return hexutil.Uint(0), err
	}

	if mBlock == nil {
		return hexutil.Uint(0), errors.New("GetTxByHash mBlock is nil")
	}

	confirmingHash := p.b.GetRootHashConfirmingMinorBlock(encoder.IDEncoder(mBlock.Hash().Bytes(), mBlock.Header().Branch.Value))
	if bytes.Equal(confirmingHash.Bytes(), common.Hash{}.Bytes()) {
		return hexutil.Uint(0), nil
	}

	confirmingBlock, _, err := p.b.GetRootBlockByHash(confirmingHash, false)
	if err != nil {
		return hexutil.Uint(0), err
	}
	if confirmingBlock == nil {
		return hexutil.Uint(0), errors.New("confirmingBlock is nil")
	}
	confirmingHeight := confirmingBlock.NumberU64()
	canonicalBlock, _, err := p.b.GetRootBlockByNumber(&confirmingHeight, false)
	if err != nil {
		return hexutil.Uint(0), err
	}
	if canonicalBlock == nil {
		return hexutil.Uint(0), errors.New("canonicalBlock is nil")
	}
	if !bytes.Equal(canonicalBlock.Hash().Bytes(), confirmingHash.Bytes()) {
		return hexutil.Uint(0), errors.New("canonicalBlock's hash !=confirmingHash's hash")
	}
	tip := p.b.CurrentBlock()
	return hexutil.Uint(tip.NumberU64() - confirmingHeight + 1), nil

}

func (p *PublicBlockChainAPI) NetVersion() hexutil.Uint {
	return hexutil.Uint(clusterCfg.Quarkchain.NetworkID)
}

type PrivateBlockChainAPI struct {
	b Backend
}

func NewPrivateBlockChainAPI(b Backend) *PrivateBlockChainAPI {
	return &PrivateBlockChainAPI{b}
}

func (p *PrivateBlockChainAPI) GetPeers() map[string]interface{} {
	fields := make(map[string]interface{})

	list := make([]map[string]interface{}, 0)
	peerList := p.b.GetPeerInfolist()
	for _, v := range peerList {
		list = append(list, map[string]interface{}{
			"id":   hexutil.Bytes(v.ID),
			"ip":   hexutil.Uint(v.IP),
			"port": hexutil.Uint(v.Port),
		})
	}
	fields["peers"] = list
	return fields
}

func (p *PrivateBlockChainAPI) GetSyncStats() {
	//need to discuss
	panic("not implemented")
}

func (p *PrivateBlockChainAPI) GetStats() (map[string]interface{}, error) {
	return p.b.GetStats()
}

func (p *PrivateBlockChainAPI) GetBlockCount() (map[string]interface{}, error) {
	data, err := p.b.GetBlockCount()
	return map[string]interface{}{
		"rootHeight": p.b.CurrentBlock().Number(),
		"shardRC":    data,
	}, err
}

//TODO txGenerate implement
func (p *PrivateBlockChainAPI) CreateTransactions(args CreateTxArgs) error {
	config := clusterCfg.Quarkchain
	if err := args.setDefaults(config); err != nil {
		return err
	}
	tx := args.toTx(config)
	return p.b.CreateTransactions(uint32(*args.NumTxPreShard), uint32(*args.XShardPrecent), tx)
}

func (p *PrivateBlockChainAPI) SetTargetBlockTime(rootBlockTime *uint32, minorBlockTime *uint32) error {
	return p.b.SetTargetBlockTime(rootBlockTime, minorBlockTime)
}

func (p *PrivateBlockChainAPI) SetMining(flag bool) {
	p.b.SetMining(flag)
}

//TODO ?? necessary?
func (p *PrivateBlockChainAPI) GetJrpcCalls() { panic("not implemented") }

func (p *PrivateBlockChainAPI) GetKadRoutingTableSize() (hexutil.Uint, error) {
	urls, err := p.b.GetKadRoutingTable()
	if err != nil {
		return hexutil.Uint(0), err
	}
	return hexutil.Uint(len(urls)), nil
}

func (p *PrivateBlockChainAPI) GetKadRoutingTable() ([]string, error) {
	return p.b.GetKadRoutingTable()
}

type EthBlockChainAPI struct {
	CommonAPI
	b Backend
}

func NewEthAPI(b Backend) *EthBlockChainAPI {
	return &EthBlockChainAPI{b: b, CommonAPI: CommonAPI{b}}
}

func (e *EthBlockChainAPI) GasPrice(fullShardKey *hexutil.Uint) (hexutil.Uint64, error) {
	fullShardId, err := getFullShardId(fullShardKey)
	if err != nil {
		return hexutil.Uint64(0), err
	}
	data, err := e.b.GasPrice(account.Branch{Value: fullShardId}, qcom.TokenIDEncode(DefaultTokenID))
	return hexutil.Uint64(data), nil
}

func (e *EthBlockChainAPI) GetBlockByNumber(heightInput *hexutil.Uint64) (map[string]interface{}, error) {
	height, err := transHexutilUint64ToUint64(heightInput)
	if err != nil {
		return nil, err
	}
	minorBlock, extraData, err := e.b.GetMinorBlockByHeight(height, account.Branch{Value: 0}, false)
	if err != nil {
		return nil, err
	}
	if minorBlock == nil {
		return nil, errors.New("minor block is nil")
	}
	return encoder.MinorBlockEncoder(minorBlock, true, extraData)
}

func (e *EthBlockChainAPI) GetBalance(address common.Address, fullShardKey *hexutil.Uint) (*hexutil.Big, error) {
	fullShardId, err := getFullShardId(fullShardKey)
	if err != nil {
		return nil, err
	}

	addr := account.NewAddress(address, fullShardId)
	data, err := e.b.GetPrimaryAccountData(&addr, nil)
	if err != nil {
		return nil, err
	}
	balance := data.Balance.GetTokenBalance(qcom.TokenIDEncode(DefaultTokenID))
	return (*hexutil.Big)(balance), nil
}

func (e *EthBlockChainAPI) GetTransactionCount(address common.Address, fullShardKey *hexutil.Uint) (hexutil.Uint64, error) {
	fullShardId, err := getFullShardId(fullShardKey)
	if err != nil {
		return hexutil.Uint64(0), err
	}
	addr := account.NewAddress(address, fullShardId)
	data, err := e.b.GetPrimaryAccountData(&addr, nil)
	if err != nil {
		return 0, err
	}
	return hexutil.Uint64(data.TransactionCount), nil
}

func (e *EthBlockChainAPI) GetCode(address common.Address, fullShardKey *hexutil.Uint) (hexutil.Bytes, error) {
	fullShardId, err := getFullShardId(fullShardKey)
	if err != nil {
		return nil, err
	}
	addr := account.NewAddress(address, fullShardId)
	return e.b.GetCode(&addr, nil)
}

func (e *EthBlockChainAPI) Call(data EthCallArgs, fullShardKey *hexutil.Uint) (hexutil.Bytes, error) {
	args, err := convertEthCallData(&data, fullShardKey)
	if err != nil {
		return nil, err
	}
	return e.CommonAPI.callOrEstimateGas(args, nil, true)
}

func (e *EthBlockChainAPI) EstimateGas(data EthCallArgs, fullShardKey *hexutil.Uint) ([]byte, error) {
	args, err := convertEthCallData(&data, fullShardKey)
	if err != nil {
		return nil, err
	}
	return e.CommonAPI.callOrEstimateGas(args, nil, false)
}

func (e *EthBlockChainAPI) GetStorageAt(address common.Address, key common.Hash, fullShardKey *hexutil.Uint) (hexutil.Bytes, error) {
	fullShardId, err := getFullShardId(fullShardKey)
	if err != nil {
		return nil, err
	}
	addr := account.NewAddress(address, fullShardId)
	hash, err := e.b.GetStorageAt(&addr, key, nil)
	return hash.Bytes(), err
}
