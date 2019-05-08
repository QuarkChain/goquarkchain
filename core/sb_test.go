package core

import (
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/core/types"
	"testing"
)

func disPlayMinorHeader(header *types.MinorBlockHeader) {
	fmt.Println("header========")
	fmt.Println("Version", header.Version)
	fmt.Println("Branch", header.Branch.Value)
	fmt.Println("Number", header.Number)
	fmt.Println("Coinbase", header.Coinbase.ToHex())
	fmt.Println("CoinbaseAmount", header.CoinbaseAmount.Value)
	fmt.Println("ParentHash", header.ParentHash.Hex())
	fmt.Println("PrevRootBlockHash", header.PrevRootBlockHash.String())
	fmt.Println("GasLimit", header.GasLimit.Value.String())
	fmt.Println("MetaHash", header.MetaHash.String())
	fmt.Println("Time", header.Time)
	fmt.Println("Difficulty", header.Difficulty)
	fmt.Println("Nonce", header.Nonce)
	fmt.Println("Bloom", header.Bloom.Big().String())
	fmt.Println("Extra", hex.EncodeToString(header.Extra))
	fmt.Println("MixDigest", header.MixDigest.String())
}

func printMinor(block *types.MinorBlock) {
	fmt.Println("===========")
	fmt.Println("hash_merkle_root", block.Meta().TxHash.String())
	fmt.Println("hash_evm_state_root", block.Meta().Root.String())
	fmt.Println("hash_evm_receipt_root", block.Meta().ReceiptHash.String())
	fmt.Println("hash_meta", block.Header().MetaHash.String())
	fmt.Println("get_hash", block.Header().Hash().String())
	fmt.Println("block.header", block.Header())
	disPlayMinorHeader(block.Header())
}
func TestNewEVMContext(t *testing.T) {
	env := setUp(nil, nil, nil)
	shardState := createDefaultShardState(env, nil, nil, nil, nil)
	printMinor(shardState.CurrentBlock())
}
