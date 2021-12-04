package encoder

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	ethCommon "github.com/ethereum/go-ethereum/common"
)

// A BlockNonce is a 64-bit hash which proves (combined with the
// mix-hash) that a sufficient amount of computation has been carried
// out on a block.
type BlockNonce [8]byte

// EncodeNonce converts the given integer to a block nonce.
func EncodeNonce(i uint64) BlockNonce {
	var n BlockNonce
	binary.BigEndian.PutUint64(n[:], i)
	return n
}

// Uint64 returns the integer value of a block nonce.
func (n BlockNonce) Uint64() uint64 {
	return binary.BigEndian.Uint64(n[:])
}

// MarshalText encodes n as a hex string with 0x prefix.
func (n BlockNonce) MarshalText() ([]byte, error) {
	return hexutil.Bytes(n[:]).MarshalText()
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *BlockNonce) UnmarshalText(input []byte) error {
	return hexutil.UnmarshalFixedText("BlockNonce", input, n[:])
}

func IDEncoder(hashByte []byte, fullShardKey uint32) hexutil.Bytes {
	hashByte = append(hashByte, common.Uint32ToBytes(fullShardKey)...)
	return hexutil.Bytes(hashByte)
}

func IDDecoder(bytes []byte) (ethCommon.Hash, uint32, error) {
	if len(bytes) != 36 {
		return ethCommon.Hash{}, 0, errors.New("len should 36")
	}
	return ethCommon.BytesToHash(bytes[:32]), common.BytesToUint32(bytes[32:]), nil

}

func DataEncoder(bytes []byte) hexutil.Bytes {
	return hexutil.Bytes(bytes)
}

func FullShardKeyEncode(fullShardKey uint32) hexutil.Bytes {
	return hexutil.Bytes(common.Uint32ToBytes(fullShardKey))
}

func BalancesEncoder(balances *types.TokenBalances) []map[string]interface{} {
	balanceList := make([]map[string]interface{}, 0)
	bMap := balances.GetBalanceMap()
	for k, v := range bMap {
		tokenStr, err := common.TokenIdDecode(k)
		if err != nil {
			panic(err) //TODO ??
		}
		balanceList = append(balanceList, map[string]interface{}{
			"tokenId":  (hexutil.Uint64)(k),
			"tokenStr": tokenStr,
			"balance":  (*hexutil.Big)(v),
		})
	}
	return balanceList
}

func RootBlockEncoder(rootBlock *types.RootBlock, extraInfo *rpc.PoSWInfo) (map[string]interface{}, error) {
	serData, err := serialize.SerializeToBytes(rootBlock)
	if err != nil {
		return nil, err
	}
	header := rootBlock.Header()

	minerData, err := serialize.SerializeToBytes(header.Coinbase)
	if err != nil {
		return nil, err
	}

	fields := map[string]interface{}{
		"id":                header.Hash(),
		"height":            hexutil.Uint64(header.Number),
		"number":            EncodeNonce(uint64(header.Number)),
		"hash":              header.Hash(),
		"sealHash":          header.SealHash(),
		"hashPrevBlock":     header.ParentHash,
		"idPrevBlock":       header.ParentHash,
		"nonce":             hexutil.Uint64(header.Nonce),
		"hashMerkleRoot":    header.MinorHeaderHash,
		"miner":             DataEncoder(minerData),
		"coinbase":          BalancesEncoder(header.CoinbaseAmount),
		"difficulty":        (*hexutil.Big)(header.Difficulty),
		"timestamp":         hexutil.Uint64(header.Time),
		"size":              hexutil.Uint64(len(serData)),
		"minorBlockHeaders": make([]types.MinorBlockHeader, 0),
		"signature":         DataEncoder(header.Signature[:]),
	}
	if extraInfo != nil && !extraInfo.IsNil() {
		fields["effectiveDifficulty"] = (*hexutil.Big)(extraInfo.EffectiveDifficulty)
		fields["poswMineableBlocks"] = (hexutil.Uint64)(extraInfo.PoswMineableBlocks)
		fields["poswMinedBlocks"] = (hexutil.Uint64)(extraInfo.PoswMinedBlocks)
		fields["stakingApplied"] = extraInfo.EffectiveDifficulty.Cmp(header.Difficulty) < 0
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
			"number":             hexutil.Uint64(header.Number),
			"hash":               header.Hash(),
			"fullShardId":        hexutil.Uint64(header.Branch.GetFullShardID()),
			"chainId":            hexutil.Uint64(header.Branch.GetChainID()),
			"shardId":            hexutil.Uint64(header.Branch.GetShardID()),
			"hashPrevMinorBlock": header.ParentHash,
			"idPrevMinorBlock":   IDEncoder(header.ParentHash.Bytes(), header.Branch.GetFullShardID()),
			"hashPrevRootBlock":  header.PrevRootBlockHash,
			"nonce":              EncodeNonce(header.Nonce),
			"difficulty":         (*hexutil.Big)(header.Difficulty),
			"miner":              DataEncoder(minerData),
			"coinbase":           BalancesEncoder(header.CoinbaseAmount),
			"timestamp":          hexutil.Uint64(header.Time),
			"extraData":          hexutil.Bytes(header.Extra),
			"gasLimit":           hexutil.Big(*header.GasLimit.Value),
			"sha3Uncles":         types.EmptyHash,
		}
		minorHeaders = append(minorHeaders, h)
	}
	fields["minorBlockHeaders"] = minorHeaders
	return fields, nil
}

func MinorBlockHeaderEncoder(header *types.MinorBlockHeader, meta *types.MinorBlockMeta) (map[string]interface{}, error) {
	minerData, err := serialize.SerializeToBytes(header.Coinbase)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":                 IDEncoder(header.Hash().Bytes(), header.Branch.GetFullShardID()),
		"height":             hexutil.Uint64(header.Number),
		"number":             hexutil.Uint64(header.Number),
		"hash":               header.Hash(),
		"fullShardId":        hexutil.Uint64(header.Branch.GetFullShardID()),
		"chainId":            hexutil.Uint64(header.Branch.GetChainID()),
		"shardId":            hexutil.Uint64(header.Branch.GetShardID()),
		"hashPrevMinorBlock": header.ParentHash,
		"parentHash":         header.ParentHash,
		"idPrevMinorBlock":   IDEncoder(header.ParentHash.Bytes(), header.Branch.GetFullShardID()),
		"hashPrevRootBlock":  header.PrevRootBlockHash,
		"nonce":              EncodeNonce(header.Nonce),
		"mixHash":            header.MixDigest,
		"miner":              DataEncoder(minerData),
		"coinbase":           (BalancesEncoder)(header.CoinbaseAmount),
		"difficulty":         (*hexutil.Big)(header.Difficulty),
		"extraData":          hexutil.Bytes(header.Extra),
		"gasLimit":           (*hexutil.Big)(header.GasLimit.Value),
		"timestamp":          hexutil.Uint64(header.Time),
		"logsBloom":          header.Bloom,
		"sha3Uncles":         types.EmptyUncleHash,
		"hashMerkleRoot":     meta.TxHash,
		"transactionsRoot":   meta.TxHash,
		"hashEvmStateRoot":   meta.Root,
		"stateRoot":          meta.Root,
		"receiptsRoot":       meta.ReceiptHash,
		"gasUsed":            (*hexutil.Big)(meta.GasUsed.Value),
	}, nil
}

func MinorBlockEncoder(block *types.MinorBlock, includeTransaction bool, extraInfo *rpc.PoSWInfo) (map[string]interface{}, error) {
	serData, err := serialize.SerializeToBytes(block)
	if err != nil {
		return nil, err
	}
	header := block.Header()
	meta := block.Meta()
	field, err := MinorBlockHeaderEncoder(header, meta)
	if err != nil {
		return nil, err
	}

	field["size"] = hexutil.Uint64(len(serData))

	if includeTransaction {
		txForDisplay := make([]map[string]interface{}, 0)
		for txIndex := range block.Transactions() {
			temp, err := TxEncoder(block, txIndex)
			if err != nil {
				return nil, err
			}
			txForDisplay = append(txForDisplay, temp)
		}
		field["transactions"] = txForDisplay
	} else {
		txHashForDisplay := make([]hexutil.Bytes, 0)
		for _, tx := range block.Transactions() {
			txHashForDisplay = append(txHashForDisplay, IDEncoder(tx.Hash().Bytes(), block.Branch().Value))
		}
		field["transactions"] = txHashForDisplay
	}
	if extraInfo != nil && !extraInfo.IsNil() {
		field["effectiveDifficulty"] = (*hexutil.Big)(extraInfo.EffectiveDifficulty)
		field["poswMineableBlocks"] = (hexutil.Uint64)(extraInfo.PoswMineableBlocks)
		field["poswMinedBlocks"] = (hexutil.Uint64)(extraInfo.PoswMinedBlocks)
		field["stakingApplied"] = extraInfo.EffectiveDifficulty.Cmp(header.Difficulty) < 0
	}
	return field, nil
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        *ethCommon.Hash    `json:"blockHash"`
	BlockNumber      *hexutil.Big       `json:"blockNumber"`
	From             ethCommon.Address  `json:"from"`
	Gas              hexutil.Uint64     `json:"gas"`
	GasPrice         *hexutil.Big       `json:"gasPrice"`
	Hash             ethCommon.Hash     `json:"hash"`
	Input            hexutil.Bytes      `json:"input"`
	Nonce            hexutil.Uint64     `json:"nonce"`
	To               *ethCommon.Address `json:"to"`
	TransactionIndex *hexutil.Uint64    `json:"transactionIndex"`
	Value            *hexutil.Big       `json:"value"`
	V                *hexutil.Big       `json:"v"`
	R                *hexutil.Big       `json:"r"`
	S                *hexutil.Big       `json:"s"`
}

// NewRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func NewRPCTransaction(tx *types.Transaction, blockHash ethCommon.Hash, blockNumber uint64, index uint64) *RPCTransaction {
	var signer types.Signer = types.MakeSigner(tx.EvmTx.NetworkId())
	from, _ := types.Sender(signer, tx.EvmTx)
	v, r, s := tx.EvmTx.RawSignatureValues()

	result := &RPCTransaction{
		From:     from,
		Gas:      hexutil.Uint64(tx.EvmTx.Gas()),
		GasPrice: (*hexutil.Big)(tx.EvmTx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.EvmTx.Data()),
		Nonce:    hexutil.Uint64(tx.EvmTx.Nonce()),
		To:       tx.EvmTx.To(),
		Value:    (*hexutil.Big)(tx.EvmTx.Value()),
		V:        (*hexutil.Big)(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
	}
	if blockHash != (ethCommon.Hash{}) {
		result.BlockHash = &blockHash
		result.BlockNumber = (*hexutil.Big)(new(big.Int).SetUint64(blockNumber))
		result.TransactionIndex = (*hexutil.Uint64)(&index)
	}
	return result
}

func MinorBlockHeaderEncoderForEthClient(header *types.MinorBlockHeader, meta *types.MinorBlockMeta) (map[string]interface{}, error) {
	return map[string]interface{}{
		"number":           hexutil.Uint64(header.Number),
		"hash":             header.Hash(),
		"parentHash":       header.ParentHash,
		"nonce":            EncodeNonce(header.Nonce),
		"mixHash":          header.MixDigest,
		"miner":            header.Coinbase.Recipient,
		"coinbase":         (BalancesEncoder)(header.CoinbaseAmount),
		"difficulty":       (*hexutil.Big)(header.Difficulty),
		"extraData":        hexutil.Bytes(header.Extra),
		"gasLimit":         (*hexutil.Big)(header.GasLimit.Value),
		"timestamp":        hexutil.Uint64(header.Time),
		"logsBloom":        header.Bloom,
		"sha3Uncles":       types.EmptyUncleHash,
		"uncles":           types.EmptyUncleHash,
		"transactionsRoot": meta.TxHash,
		"stateRoot":        meta.Root,
		"receiptsRoot":     meta.ReceiptHash,
		"gasUsed":          (*hexutil.Big)(meta.GasUsed.Value),
	}, nil
}

func MinorBlockEncoderForEthClient(block *types.MinorBlock, includeTransaction bool, fullTx bool) (map[string]interface{}, error) {
	serData, err := serialize.SerializeToBytes(block)
	if err != nil {
		return nil, err
	}
	header := block.Header()
	meta := block.Meta()
	fields, err := MinorBlockHeaderEncoderForEthClient(header, meta)
	if err != nil {
		return nil, err
	}

	fields["size"] = hexutil.Uint64(len(serData))

	if includeTransaction {
		formatTx := func(tx *types.Transaction, idx uint64) (interface{}, error) {
			return tx.Hash(), nil
		}
		if fullTx {
			formatTx = func(tx *types.Transaction, idx uint64) (interface{}, error) {
				return NewRPCTransaction(tx, block.Hash(), block.Number(), idx), nil
			}
		}
		txs := block.Transactions()
		transactions := make([]interface{}, len(txs))
		var err error
		for i, tx := range txs {
			if transactions[i], err = formatTx(tx, uint64(i)); err != nil {
				return nil, err
			}
		}
		fields["transactions"] = transactions
	}
	fields["uncles"] = ""
	return fields, nil
}

func TxEncoder(block *types.MinorBlock, i int) (map[string]interface{}, error) {
	header := block.Header()
	tx := block.Transactions()[i]
	evmtx := tx.EvmTx
	v, r, s := evmtx.RawSignatureValues()
	sender, err := types.Sender(types.MakeSigner(evmtx.NetworkId()), evmtx)
	if err != nil {
		return nil, err
	}
	var toBytes []byte
	if evmtx.To() != nil {
		toBytes = evmtx.To().Bytes()
	}
	branch := block.Branch()
	transferTokenStr, err := common.TokenIdDecode(evmtx.TransferTokenID())
	if err != nil {
		return nil, err
	}
	gasTokenStr, err := common.TokenIdDecode(evmtx.GasTokenID())
	if err != nil {
		return nil, err
	}
	field := map[string]interface{}{
		"id":               IDEncoder(tx.Hash().Bytes(), evmtx.FromFullShardKey()),
		"hash":             tx.Hash(),
		"nonce":            hexutil.Uint64(evmtx.Nonce()),
		"timestamp":        hexutil.Uint64(header.Time),
		"fullShardId":      hexutil.Uint64(header.Branch.GetFullShardID()),
		"chainId":          hexutil.Uint64(header.Branch.GetChainID()),
		"shardId":          hexutil.Uint64(header.Branch.GetShardID()),
		"blockId":          IDEncoder(header.Hash().Bytes(), branch.GetFullShardID()),
		"blockHeight":      hexutil.Uint64(header.Number),
		"transactionIndex": hexutil.Uint64(i),
		"from":             DataEncoder(sender.Bytes()),
		"to":               DataEncoder(toBytes),
		"fromFullShardKey": FullShardKeyEncode(evmtx.FromFullShardKey()),
		"toFullShardKey":   FullShardKeyEncode(evmtx.ToFullShardKey()),
		"value":            (*hexutil.Big)(evmtx.Value()),
		"gasPrice":         (*hexutil.Big)(evmtx.GasPrice()),
		"gas":              hexutil.Uint64(evmtx.Gas()),
		"data":             hexutil.Bytes(evmtx.Data()),
		"networkId":        hexutil.Uint64(evmtx.NetworkId()),
		"transferTokenId":  hexutil.Uint64(evmtx.TransferTokenID()),
		"gasTokenId":       hexutil.Uint64(evmtx.GasTokenID()),
		"transferTokenStr": transferTokenStr,
		"gasTokenStr":      gasTokenStr,
		"r":                (*hexutil.Big)(r),
		"s":                (*hexutil.Big)(s),
		"v":                (*hexutil.Big)(v),
	}
	return field, nil
}

func LogEncoder(log *types.Log, isRemoved bool) map[string]interface{} {
	field := map[string]interface{}{
		"logIndex":         hexutil.Uint64(log.Index),
		"transactionIndex": hexutil.Uint64(log.TxIndex),
		"transactionHash":  log.TxHash,
		"blockHash":        log.BlockHash,
		"blockNumber":      hexutil.Uint64(log.BlockNumber),
		"blockHeight":      hexutil.Uint64(log.BlockNumber),
		"address":          log.Recipient,
		"recipient":        log.Recipient,
		"data":             hexutil.Bytes(log.Data),
		"removed":          isRemoved,
	}
	topics := make([]ethCommon.Hash, len(log.Topics))
	for i, v := range log.Topics {
		topics[i] = v
	}
	field["topics"] = topics
	return field
}

func LogListEncoder(logList []*types.Log, isRemoved bool) []map[string]interface{} {
	fields := make([]map[string]interface{}, 0)
	for _, log := range logList {
		field := LogEncoder(log, isRemoved)
		fields = append(fields, field)
	}
	return fields
}

func ReceiptEncoder(block *types.MinorBlock, i int, receipt *types.Receipt) (map[string]interface{}, error) {
	if block == nil {
		return nil, errors.New("block is nil")
	}
	txID := ""
	txHash := ""
	if len(block.Transactions()) > i {
		tx := block.Transactions()[i]
		evmTx := tx.EvmTx
		txID = IDEncoder(tx.Hash().Bytes(), evmTx.FromFullShardKey()).String()
		txHash = tx.Hash().String()
	}
	if receipt == nil {
		return nil, errors.New("receipt is nil")
	}
	header := block.Header()

	field := map[string]interface{}{
		"transactionId":     txID,
		"transactionHash":   txHash,
		"transactionIndex":  hexutil.Uint64(i),
		"blockId":           IDEncoder(header.Hash().Bytes(), header.Branch.GetFullShardID()),
		"blockHash":         header.Hash(),
		"blockHeight":       hexutil.Uint64(header.Number),
		"blockNumber":       hexutil.Uint64(header.Number),
		"cumulativeGasUsed": hexutil.Uint64(receipt.CumulativeGasUsed),
		"gasUsed":           hexutil.Uint64(receipt.GasUsed - receipt.GetPrevGasUsed()),
		"status":            hexutil.Uint64(receipt.Status),
		"logs":              LogListEncoder(receipt.Logs, false),
		"timestamp":         hexutil.Uint64(block.Time()),
	}
	if receipt.ContractAddress != (ethCommon.Address{}) {
		field["contractAddress"] = nil
	} else {
		addr := account.Address{
			Recipient:    receipt.ContractAddress,
			FullShardKey: receipt.ContractFullShardKey,
		}
		field["contractAddress"] = DataEncoder(addr.ToBytes())
	}
	return field, nil
}
