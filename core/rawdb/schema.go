// Modified from go-ethereum under GNU Lesser General Public License
package rawdb

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/metrics"
)

// The fields below define the low level database schema prefixing.
var (
	// databaseVerisionKey tracks the current database version.
	databaseVerisionKey = []byte("DatabaseVersion")

	// headHeaderKey tracks the latest know header's hash.
	headHeaderKey = []byte("LastHeader")

	// headBlockKey tracks the latest know full block's hash.
	headBlockKey = []byte("LastBlock")

	// headFastBlockKey tracks the latest known incomplete block's hash during fast sync.
	headFastBlockKey = []byte("LastFast")
	rbCommittingKey  = []byte("rbCommitting")

	// fastTrieProgressKey tracks the number of trie entries imported during fast sync.
	fastTrieProgressKey = []byte("TrieSync")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	headerPrefix                   = []byte("h")    // headerPrefix + hash -> header
	latestMHeaderPrefix            = []byte("lmh")  //latestMHeaderPrefix + hash -> latest minor header list
	headerTDSuffix                 = []byte("t")    // headerPrefix + hash + headerTDSuffix -> td
	rootHashPrefix                 = []byte("rn")   // rootHashPrefix + num (uint64 big endian) -> root hash
	minorHashPrefix                = []byte("mn")   // minorHashPrefix + num (uint64 big endian) -> minorhash
	headerNumberPrefix             = []byte("H")    // headerNumberPrefix + hash -> num (uint64 big endian)
	validatedMinorHeaderHashPrefix = []byte("vmhh") //validatedMinorHeaderHashPrefix + hash -> empty, use to record whether this header has been validated

	blockPrefix         = []byte("b") // blockPrefix + hash -> block rootBlockBody
	blockReceiptsPrefix = []byte("r") // blockReceiptsPrefix + num (uint64 big endian) + hash -> block receipts

	lookupPrefix    = []byte("l") // lookupPrefix + hash -> transaction/receipt lookup metadata
	bloomBitsPrefix = []byte("B") // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash -> bloom bits

	preimagePrefix = []byte("secure-key-")      // preimagePrefix + hash -> preimage
	configPrefix   = []byte("ethereum-config-") // config prefix for the db

	// Chain index prefixes (use `i` + single byte to avoid mixing data types).
	BloomBitsIndexPrefix = []byte("iB") // BloomBitsIndexPrefix is the data table of a chain indexer to track its progress

	preimageCounter    = metrics.NewRegisteredCounter("db/preimage/total", nil)
	preimageHitCounter = metrics.NewRegisteredCounter("db/preimage/hits", nil)

	totalTxKey         = []byte("txCount")
	xConfirmedShardKey = []byte("xr")
	xShardLists        = []byte("xShard")
	rLastM             = []byte("rLastM")
	rBlock             = []byte("rBlock")
	genesis            = []byte("genesis")
	countMinor         = []byte("cntM")
)

type ChainType byte

const (
	ChainTypeRoot  = ChainType(0)
	ChainTypeMinor = ChainType(1)
)

// LookupEntry is a positional metadata to help looking up the data content of
// a transaction or receipt given only its hash.
type LookupEntry struct {
	BlockHash common.Hash
	Index     uint32
}

// encodeBlockNumber encodes a block number as big endian uint64
func encodeBlockNumber(number uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, number)
	return enc
}

func encodeUint32(number uint32) []byte {
	enc := make([]byte, 4)
	binary.BigEndian.PutUint32(enc, number)
	return enc
}

// headerKey = headerPrefix + hash
func headerKey(hash common.Hash) []byte {
	return append(headerPrefix, hash.Bytes()...)
}

// latestMHeaderKey = latestMHeaderPrefix + hash
func latestMHeaderKey(hash common.Hash) []byte {
	return append(latestMHeaderPrefix, hash.Bytes()...)
}

// headerTDKey = headerPrefix + hash + headerTDSuffix
func headerTDKey(hash common.Hash) []byte {
	return append(headerKey(hash), headerTDSuffix...)
}

// headerHashKey = headerPrefix + num (uint64 big endian) + headerHashSuffix
func headerHashKey(chainType ChainType, number uint64) []byte {
	if chainType == 0 {
		return append(rootHashPrefix, encodeBlockNumber(number)...)
	} else {
		return append(minorHashPrefix, encodeBlockNumber(number)...)
	}
}

// headerNumberKey = headerNumberPrefix + hash
func headerNumberKey(hash common.Hash) []byte {
	return append(headerNumberPrefix, hash.Bytes()...)
}

// blockBodyKey = blockBodyPrefix + num (uint64 big endian) + hash
func blockKey(hash common.Hash) []byte {
	return append(blockPrefix, hash.Bytes()...)
}

// blockReceiptsKey = blockReceiptsPrefix + num (uint64 big endian) + hash
func blockReceiptsKey(hash common.Hash) []byte {
	return append(blockReceiptsPrefix, hash.Bytes()...)
}

// lookupKey = txLookupPrefix + hash
func lookupKey(hash common.Hash) []byte {
	return append(lookupPrefix, hash.Bytes()...)
}

// validatedMinorHeaderHashKey = validatedMinorHeaderHashPrefix + hash
func validatedMinorHeaderHashKey(hash common.Hash) []byte {
	return append(validatedMinorHeaderHashPrefix, hash.Bytes()...)
}

// bloomBitsKey = bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) + hash
func bloomBitsKey(bit uint, section uint64, hash common.Hash) []byte {
	key := append(append(bloomBitsPrefix, make([]byte, 10)...), hash.Bytes()...)

	binary.BigEndian.PutUint16(key[1:], uint16(bit))
	binary.BigEndian.PutUint64(key[3:], section)

	return key
}

// preimageKey = preimagePrefix + hash
func preimageKey(hash common.Hash) []byte {
	return append(preimagePrefix, hash.Bytes()...)
}

// configKey = configPrefix + hash
func configKey(hash common.Hash) []byte {
	return append(configPrefix, hash.Bytes()...)
}

func totalTxCountKey(hash common.Hash) []byte {
	return append(totalTxKey, hash.Bytes()...)
}

func makeConfirmedXShardKey(hash common.Hash) []byte {
	return append(xConfirmedShardKey, hash.Bytes()...)
}
func makeXShardTxList(hash common.Hash) []byte {
	return append(xShardLists, hash.Bytes()...)
}
func makeGenesisKey(hash common.Hash) []byte {
	return append(genesis, hash.Bytes()...)
}

func makeRLastMHash(hash common.Hash) []byte {
	return append(rLastM, hash.Bytes()...)
}

func makeRootBlockForShard(hash common.Hash) []byte {
	return append(rBlock, hash.Bytes()...)
}
func makeMinorCount(fullShardID uint32, height uint32) []byte {
	data := append(countMinor, encodeUint32(fullShardID)...)
	return append(data, encodeUint32(height)...)
}
