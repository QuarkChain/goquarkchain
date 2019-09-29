package encoder

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

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
			"coinbase":           BalancesEncoder(header.CoinbaseAmount),
			"timestamp":          hexutil.Uint64(header.Time),
		}
		minorHeaders = append(minorHeaders, h)
	}
	fields["minorBlockHeaders"] = minorHeaders
	return fields, nil
}

func MinorBlockEncoder(block *types.MinorBlock, includeTransaction bool, extraInfo *rpc.PoSWInfo) (map[string]interface{}, error) {
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
		"coinbase":           (BalancesEncoder)(header.CoinbaseAmount),
		"difficulty":         (*hexutil.Big)(header.Difficulty),
		"extraData":          hexutil.Bytes(header.Extra),
		"gasLimit":           (*hexutil.Big)(header.GasLimit.Value),
		"gasUsed":            (*hexutil.Big)(meta.GasUsed.Value),
		"timestamp":          hexutil.Uint64(header.Time),
		"size":               hexutil.Uint64(len(serData)),
	}

	if includeTransaction {
		txForDisplay := make([]map[string]interface{}, 0)
		for txIndex, _ := range block.Transactions() {
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
			txHashForDisplay = append(txHashForDisplay, IDEncoder(tx.Hash().Bytes(), block.Header().Branch.Value))
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
	branch := block.Header().Branch
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
		"fromFullShardKey": hexutil.Uint64(evmtx.FromFullShardKey()),
		"toFullShardKey":   hexutil.Uint64(evmtx.ToFullShardKey()),
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

func LogListEncoder(logList []*types.Log) []map[string]interface{} {
	fields := make([]map[string]interface{}, 0)
	for _, log := range logList {
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
		}
		topics := make([]ethCommon.Hash, 0)
		for _, v := range log.Topics {
			topics = append(topics, v)
		}
		field["topics"] = topics
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
		"gasUsed":           hexutil.Uint64(receipt.GasUsed),
		"status":            hexutil.Uint64(receipt.Status),
		"logs":              LogListEncoder(receipt.Logs),
		"timestamp":         hexutil.Uint64(block.Header().Time),
	}
	if receipt.ContractAddress.Big().Uint64() == 0 {
		field["contractAddress"] = make([]struct{}, 0)
	} else {
		addr := account.Address{
			Recipient:    receipt.ContractAddress,
			FullShardKey: receipt.ContractFullShardKey,
		}
		field["contractAddress"] = DataEncoder(addr.ToBytes())
	}
	return field, nil
}
