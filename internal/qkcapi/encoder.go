package qkcapi

import (
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func IDEncoder(hashByte []byte, fullShardKey uint32) string {
	hashByte = append(hashByte, common.Uint32ToBytes(fullShardKey)...)
	return "0x" + hex.EncodeToString(hashByte)
}
func AddressEncoder(bytes []byte) string {
	return "0x" + hex.EncodeToString(bytes)
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
		"miner":          AddressEncoder(minerData),
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
			"miner":              AddressEncoder(minerData),
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
		"miner":              AddressEncoder(minerData),
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
		"from":             AddressEncoder(sender.Bytes()),
		"to":               AddressEncoder(tx.To().Bytes()),
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

func logListEncoder(logList []*types.Log) {

}
