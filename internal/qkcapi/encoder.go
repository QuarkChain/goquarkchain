package qkcapi

import (
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

//func minorBlockEncoder(block *types.MinorBlock, includeTransaction *bool) {
//	header := block.Header()
//	meta := block.Meta()
//	fields := map[string]interface{}{}
//}

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
		"height":         (*hexutil.Big)(new(big.Int).SetUint64(header.NumberU64())),
		"hash":           header.Hash(),
		"hashPrevBlock":  header.ParentHash,
		"nonce":          (*hexutil.Big)(new(big.Int).SetUint64(header.Nonce)),
		"hashMerkleRoot": header.MinorHeaderHash,
		"miner":          AddressEncoder(minerData),
		"coinbase":       (*hexutil.Big)(header.CoinbaseAmount.Value),
		"difficulty":     (*hexutil.Big)(header.Difficulty),
		"timestamp":      (*hexutil.Big)(new(big.Int).SetUint64(header.Time)),
		"size":           (*hexutil.Big)(new(big.Int).SetUint64(uint64(len(serData)))),
	}
	return fields, nil
}
