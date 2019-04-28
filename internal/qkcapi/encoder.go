package qkcapi

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	From     ethCommon.Address  `json:"from"`
	To       *ethCommon.Address `json:"to"`
	Gas      *hexutil.Uint64    `json:"gas"`
	GasPrice *hexutil.Big       `json:"gasPrice"`
	Value    *hexutil.Big       `json:"value"`
	Nonce    *hexutil.Uint64    `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data             *hexutil.Bytes  `json:"data"`
	FromFullShardKey *hexutil.Uint64 `json:"fromFullShardId"`
	ToFullShardKey   *hexutil.Uint64 `json:"toFullShardId"`
}

var (
	DEFAULT_STARTGAS = uint64(100 * 1000)
	DEFAULT_GASPRICE = new(big.Int).Mul(new(big.Int).SetUint64(10), new(big.Int).SetUint64(1000000000))
)

// setDefaults is a helper function that fills in default values for unspecified tx fields.
func (args *SendTxArgs) setDefaults() {
	// ingore nonce
	// ingore to
	if args.Gas == nil {
		args.Gas = new(hexutil.Uint64)
		*(*uint64)(args.Gas) = DEFAULT_STARTGAS
	}
	if args.GasPrice == nil {
		args.GasPrice = (*hexutil.Big)(DEFAULT_GASPRICE)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
}

func (args *SendTxArgs) toTransaction(networkID uint32) (*types.Transaction, error) {
	if args.Nonce == nil {
		return nil, errors.New("nonce is missing")
	}
	if args.FromFullShardKey == nil {
		return nil, errors.New("fromFullShardKey is missing")
	}
	if args.ToFullShardKey == nil {
		args.ToFullShardKey = args.FromFullShardKey
	}
	evmTx := types.NewEvmTransaction(uint64(*args.Nonce), account.BytesToIdentityRecipient(args.To.Bytes()), (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), uint32(*args.FromFullShardKey), uint32(*args.ToFullShardKey), networkID, 0, *args.Data)
	return &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}, nil
}

func IDEncoder(hashByte []byte, fullShardKey uint32) hexutil.Bytes {
	hashByte = append(hashByte, common.Uint32ToBytes(fullShardKey)...)
	return hexutil.Bytes(hashByte)
}
func DataEncoder(bytes []byte) string {
	return "0x" + hex.EncodeToString(bytes)
}

func FullShardKeyEncoder(fullShardKey uint32) string {
	return DataEncoder(common.Uint32ToBytes(fullShardKey))
}
func rootBlockEncoder(rootBlock *types.RootBlock) (map[string]interface{}, error) {
	serData, err := serialize.SerializeToBytes(rootBlock)
	if err != nil {
		return nil, err
	}
	header := rootBlock.Header()

	tempID, _ := account.CreatRandomIdentity()
	add1 := account.NewAddress(tempID.Recipient, 3)

	fmt.Println("hhex", hex.EncodeToString(add1.ToHex()))
	//minerData, err := serialize.SerializeToBytes(header.Coinbase)
	minerData, err := serialize.SerializeToBytes(add1)
	if err != nil {
		return nil, err
	}

	fields := map[string]interface{}{
		"id":             header.Hash(),
		"height":         hexutil.Uint64(header.Number),
		"hash":           header.Hash(),
		"hashPrevBlock":  header.ParentHash,
		"idPrevBlock":    header.ParentHash,
		"nonce":          hexutil.Uint64(header.Nonce),
		"hashMerkleRoot": header.MinorHeaderHash,
		"miner":          DataEncoder(minerData),
		"coinbase":       (*hexutil.Big)(header.CoinbaseAmount.Value),
		"difficulty":     (*hexutil.Big)(header.Difficulty),
		"timestamp":      hexutil.Uint64(header.Time),
		"size":           hexutil.Uint64(len(serData)),
	}

	minorHeaders := make([]map[string]interface{}, 0)
	for _, header := range rootBlock.MinorBlockHeaders() {
		minerData, err := serialize.SerializeToBytes(header.Coinbase)
		if err != nil {
			return nil, err
		}
		h := map[string]interface{}{
			"id":                 IDEncoder(header.Hash().Bytes(), header.Branch.GetFullShardID()),
			"height":             hexutil.Uint64(header.Number),
			"hash":               header.Hash(),
			"fullShardId":        hexutil.Uint64(header.Branch.GetFullShardID()),
			"chainId":            hexutil.Uint64(header.Branch.GetChainID()),
			"shardId":            hexutil.Uint64(header.Branch.GetShardID()),
			"hashPrevMinorBlock": header.ParentHash,
			"idPrevMinorBlock":   IDEncoder(header.ParentHash.Bytes(), header.Branch.GetFullShardID()),
			"hashPrevRootBlock":  header.PrevRootBlockHash,
			"nonce":              hexutil.Uint64(header.Nonce),
			"difficulty":         (*hexutil.Big)(header.Difficulty),
			"miner":              DataEncoder(minerData),
			"coinbase":           (*hexutil.Big)(header.CoinbaseAmount.Value),
			"timestamp":          hexutil.Uint64(header.Time),
		}
		minorHeaders = append(minorHeaders, h)
	}
	fields["minorBlockHeaders"] = minorHeaders
	return fields, nil
}

func minorBlockEncoder(block *types.MinorBlock, includeTransaction bool) (map[string]interface{}, error) {
	serData, err := serialize.SerializeToBytes(block)
	if err != nil {
		return nil, err
	}
	header := block.Header()
	meta := block.Meta()
	minerData, err := serialize.SerializeToBytes(header.Coinbase)
	if err != nil {
		return nil, err
	}
	field := map[string]interface{}{
		"id":                 IDEncoder(header.Hash().Bytes(), header.Branch.GetFullShardID()),
		"height":             hexutil.Uint64(header.Number),
		"hash":               header.Hash(),
		"fullShardId":        hexutil.Uint64(header.Branch.GetFullShardID()),
		"chainId":            hexutil.Uint64(header.Branch.GetChainID()),
		"shardId":            hexutil.Uint64(header.Branch.GetShardID()),
		"hashPrevMinorBlock": header.ParentHash,
		"idPrevMinorBlock":   IDEncoder(header.ParentHash.Bytes(), header.Branch.GetFullShardID()),
		"hashPrevRootBlock":  header.PrevRootBlockHash,
		"nonce":              hexutil.Uint64(header.Nonce),
		"hashMerkleRoot":     meta.TxHash,
		"hashEvmStateRoot":   meta.Root,
		"miner":              DataEncoder(minerData),
		"coinbase":           (*hexutil.Big)(header.CoinbaseAmount.Value),
		"difficulty":         (*hexutil.Big)(header.Difficulty),
		"extraData":          hexutil.Bytes(header.Extra),
		"gasLimit":           (*hexutil.Big)(header.GasLimit.Value),
		"gasUsed":            (*hexutil.Big)(meta.GasUsed.Value),
		"timestamp":          hexutil.Uint64(header.Time),
		"size":               hexutil.Uint64(len(serData)),
	}
	return field, nil
}

func txEncoder(block *types.MinorBlock, i int) (map[string]interface{}, error) {
	header := block.Header()
	tx := block.Transactions()[i].EvmTx
	v, r, s := tx.RawSignatureValues()
	sender, err := types.Sender(types.MakeSigner(tx.NetworkId()), tx)
	if err != nil {
		return nil, err
	}
	branch := block.Header().Branch
	field := map[string]interface{}{
		"id":               IDEncoder(tx.Hash().Bytes(), tx.FromFullShardId()),
		"hash":             tx.Hash(),
		"nonce":            hexutil.Uint64(tx.Nonce()),
		"timestamp":        hexutil.Uint64(header.Time),
		"fullShardId":      hexutil.Uint64(header.Branch.GetFullShardID()),
		"chainId":          hexutil.Uint64(header.Branch.GetChainID()),
		"shardId":          hexutil.Uint64(header.Branch.GetShardID()),
		"blockId":          IDEncoder(header.Hash().Bytes(), branch.GetFullShardID()),
		"blockHeight":      hexutil.Uint64(header.Number),
		"transactionIndex": hexutil.Uint64(i),
		"from":             DataEncoder(sender.Bytes()),
		"to":               DataEncoder(tx.To().Bytes()),
		"fromFullShardKey": hexutil.Uint64(tx.FromFullShardId()), //TODO full_shard_key
		"toFullShardKey":   hexutil.Uint64(tx.ToFullShardId()),   //TODO full_shard_key
		"value":            (*hexutil.Big)(tx.Value()),
		"gasPrice":         (*hexutil.Big)(tx.GasPrice()),
		"gas":              hexutil.Uint64(tx.Gas()),
		"data":             hexutil.Bytes(tx.Data()),
		"networkId":        hexutil.Uint64(tx.NetworkId()),
		//TODO TokenID
		"transferTokenId": hexutil.Uint64(1),
		"gasTokenId":      hexutil.Uint64(1),
		//	"transferTokenStr gasTokenStr":
		"r": (*hexutil.Big)(r),
		"s": (*hexutil.Big)(s),
		"v": (*hexutil.Big)(v),
	}
	return field, nil
}

func logListEncoder(logList []*types.Log) []map[string]interface{} {
	field := make([]map[string]interface{}, 0)
	for _, log := range logList {
		l := map[string]interface{}{
			"logIndex":         hexutil.Uint64(log.Index),
			"transactionIndex": hexutil.Uint64(log.TxIndex),
			"transactionHash":  log.TxHash,
			"blockHash":        log.BlockHash,
			"blockNumber":      hexutil.Uint64(log.BlockNumber),
			"blockHeight":      hexutil.Uint64(log.BlockNumber),
			"address":          log.Recipient.ToAddress(),
			"recipient":        log.Recipient.ToAddress(),
			"data":             hexutil.Bytes(log.Data),
		}
		topics := make([]ethCommon.Hash, 0)
		for _, v := range log.Topics {
			topics = append(topics, v)
		}
		l["topocs"] = topics
		field = append(field, l)
	}
	return field
}

func receiptEncoder(block *types.MinorBlock, i int, receipt *types.Receipt) map[string]interface{} {
	tx := block.Transactions()[i].EvmTx
	header := block.Header()

	field := map[string]interface{}{
		"transactionId":     IDEncoder(tx.Hash().Bytes(), tx.FromFullShardId()), //TODO fullShardKey
		"transactionHash":   tx.Hash(),
		"transactionIndex":  hexutil.Uint64(i),
		"blockId":           IDEncoder(header.Hash().Bytes(), header.Branch.GetFullShardID()),
		"blockHash":         header.Hash(),
		"blockHeight":       hexutil.Uint64(header.Number),
		"blockNumber":       hexutil.Uint64(header.Number),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"gasUsed":           hexutil.Uint64(receipt.GetPrevGasUsed()),
		"status":            hexutil.Uint64(receipt.Status),
		"logs":              logListEncoder(receipt.Logs),
	}
	if receipt.ContractAddress.Big().Uint64() == 0 {
		field["contractAddress"] = make([]struct{}, 0)
	} else {
		field["contractAddress"] = DataEncoder(receipt.ContractAddress.ToAddress().Bytes())
	}
	return field
}

func balancesEncoder(balances []rpc.TokenBalancePair) []map[string]interface{} {
	fields := make([]map[string]interface{}, 0)
	for _, v := range balances {
		field := map[string]interface{}{
			// TODO tokenID tokenSte
			"balance": (*hexutil.Big)(v.Balance.Value),
		}
		fields = append(fields, field)
	}
	return fields
}
