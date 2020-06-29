package qkchash

import (
	"encoding/binary"
	"sort"
	"sync"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	cacheEntryCnt   = 1024 * 64
	cacheAheadRound = 64 // 64*30000
)

var (
	EpochLength = uint64(30000) //blocks pre epoch
)

type cacheSeed struct {
	mu     sync.Mutex
	caches []*qkcCache
}

func NewcacheSeed() *cacheSeed {
	firstCache := generateCache(cacheEntryCnt, common.Hash{}.Bytes())
	caches := make([]*qkcCache, 0)
	caches = append(caches, firstCache)
	return &cacheSeed{
		caches: caches,
	}
}

// getCacheFromHeight returns immutable cache.
func (c *cacheSeed) getCacheFromHeight(block uint64) *qkcCache {
	epoch := int(block / EpochLength)
	c.mu.Lock()
	defer c.mu.Unlock()
	lenCaches := len(c.caches)
	if epoch < lenCaches {
		cache := c.caches[epoch]
		if shrunk(cache) {
			fillInCache(cache)
		}
		return cache
	}
	needAddCnt := epoch - lenCaches + cacheAheadRound
	seed := c.caches[len(c.caches)-1].seed
	for i := 0; i < needAddCnt; i++ {
		seed = crypto.Keccak256(seed)
		c.caches = append(c.caches, generateCache(cacheEntryCnt, seed))
	}
	if len(c.caches) > 2*cacheAheadRound {
		for i := 0; i < len(c.caches)-2*cacheAheadRound; i++ {
			// new a cache to make sure the old cache will be
			// changed after return.
			c.caches[i] = newCache(c.caches[i].seed, nil)
		}
	}
	return c.caches[epoch]
}

// generateCache generates cache for qkchash. Will also generate underlying cache
func generateCache(cnt int, seed []byte) *qkcCache {
	ls := generatels(cnt, seed)
	return newCache(seed, ls)
}

func generatels(cnt int, seed []byte) []uint64 {
	ls := []uint64{}
	set := make(map[uint64]struct{})
	for i := uint32(0); i < uint32(cnt/8); i++ {
		iBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(iBytes, i)
		bs := crypto.Keccak512(append(seed, iBytes...))
		// Read 8 bytes as uint64
		for j := 0; j < len(bs); j += 8 {
			ele := binary.LittleEndian.Uint64(bs[j:])
			if _, ok := set[ele]; !ok {
				ls = append(ls, ele)
				set[ele] = struct{}{}
			}
		}
	}
	sort.Slice(ls, func(i, j int) bool { return ls[i] < ls[j] })
	return ls
}

// QKCHash is a consensus engine implementing PoW with qkchash algo.
// See the interface definition:
// https://github.com/ethereum/go-ethereum/blob/9e9fc87e70accf2b81be8772ab2ab0c914e95666/consensus/consensus.go#L111
// Implements consensus.Pow
type QKCHash struct {
	*consensus.CommonEngine
	cache          *cacheSeed
	qkcHashXHeight uint64
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (q *QKCHash) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (q *QKCHash) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction,
	receipts []*types.Receipt) (types.IBlock, error) {
	panic("not implemented")
}

func (q *QKCHash) hashAlgo(cache *consensus.ShareCache) (err error) {
	c := q.cache.getCacheFromHeight(cache.Height)
	copy(cache.Seed, cache.Hash)
	binary.LittleEndian.PutUint64(cache.Seed[32:], cache.Nonce)
	cache.Digest, cache.Result, err = qkcHashX(cache.Seed, c, cache.Height >= q.qkcHashXHeight)
	return
}

func (q *QKCHash) RefreshWork(tip uint64) {
	q.CommonEngine.RefreshWork(tip)
}

// New returns a QKCHash scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool, pubKey []byte, qkcHashXHeight uint64) *QKCHash {
	q := &QKCHash{
		// TODO: cache may depend on block, so a LRU-stype cache could be helpful
		cache:          NewcacheSeed(),
		qkcHashXHeight: qkcHashXHeight,
	}
	spec := consensus.MiningSpec{
		Name:       config.PoWQkchash,
		HashAlgo:   q.hashAlgo,
		VerifySeal: q.verifySeal,
	}
	q.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey)
	return q
}
