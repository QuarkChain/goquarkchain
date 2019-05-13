package qkcapi

import (
	"errors"
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
	panic(-1)
}

func (args *SendTxArgs) toTransaction(networkID uint32) (*types.Transaction, error) {
	panic(-1)
}

func IDEncoder(hashByte []byte, fullShardKey uint32) hexutil.Bytes {
	hashByte = append(hashByte, common.Uint32ToBytes(fullShardKey)...)
	return hexutil.Bytes(hashByte)
}

func IDDecoder(bytes []byte) (ethCommon.Hash, uint32, error) {
	dataBytes, err := DataDecoder(bytes)
	if err != nil {
		return ethCommon.Hash{}, 0, err
	}
	if len(dataBytes) != 36 {
		return ethCommon.Hash{}, 0, errors.New("len should 36")
	}
	return ethCommon.BytesToHash(dataBytes[:32]), common.BytesToUint32(dataBytes[32:]), nil

}
func DataEncoder(bytes []byte) hexutil.Bytes {
	return hexutil.Bytes(bytes)
}
func DataDecoder(bytes []byte) (hexutil.Bytes, error) {
	if len(bytes) >= 2 && bytes[0] == '0' && bytes[1] == 'x' {
		return hexutil.Bytes(bytes[2:]), nil
	}
	return nil, errors.New("should have 0x")
}
func FullShardKeyEncoder(fullShardKey uint32) hexutil.Bytes {
	panic(-1)
}
func rootBlockEncoder(rootBlock *types.RootBlock) (map[string]interface{}, error) {
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
	panic(-1)
}

func logListEncoder(logList []*types.Log) []map[string]interface{} {
	panic(-1)
}

func receiptEncoder(block *types.MinorBlock, i int, receipt *types.Receipt) map[string]interface{} {
	panic(-1)
}
