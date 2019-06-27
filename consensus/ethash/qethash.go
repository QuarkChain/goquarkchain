package ethash

import (
	"bytes"
	"github.com/QuarkChain/goquarkchain/consensus"
	"math/big"
	"runtime"

	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
)

// QEthash is our wrapper over geth ethash implementation.
type QEthash struct {
	*Ethash
	*consensus.CommonEngine
	height    uint64
	qethCache *cache
	qethSize  uint64
}

func (q *QEthash) hashAlgo(cache *consensus.ShareCache) error {
	if q.height != cache.Height {
		q.height = cache.Height
		q.qethCache = q.cache(q.height)
		q.qethSize = datasetSize(q.height)
		if q.config.PowMode == ModeTest {
			q.qethSize = 32 * 1024
		}
	}

	cache.Digest, cache.Result = hashimotoLight(q.qethSize, q.qethCache.cache, cache.Hash, cache.Nonce)
	return nil
}

// verifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (q *QEthash) verifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	// Ensure that we have a valid difficulty for the block
	if header.GetDifficulty().Sign() <= 0 {
		return errInvalidDifficulty
	}
	// Recompute the digest and PoW values
	number := header.NumberU64()

	var (
		digest []byte
		result []byte
	)
	cache := q.cache(number)

	size := datasetSize(number)
	if q.config.PowMode == ModeTest {
		size = 32 * 1024
	}
	digest, result = hashimotoLight(size, cache.cache, header.SealHash().Bytes(), header.GetNonce())

	// Caches are unmapped in a finalizer. Ensure that the cache stays alive
	// until after the call to hashimotoLight so it's not unmapped while being used.
	runtime.KeepAlive(cache)
	// Verify the calculated values against the ones provided in the header
	if !bytes.Equal(header.GetMixDigest().Bytes(), digest) {
		return errInvalidMixDigest
	}
	diff := adjustedDiff
	if diff == nil || diff.Cmp(big.NewInt(0)) == 0 {
		diff = header.GetDifficulty()
	}
	target := new(big.Int).Div(two256, diff)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return errInvalidPoW
	}
	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (q *QEthash) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (q *QEthash) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction,
	receipts []*types.Receipt) (types.IBlock, error) {
	panic("not implemented")
}

// New returns a Ethash scheme.
func New(
	config Config,
	diffCalculator consensus.DifficultyCalculator,
	remote bool,
) *QEthash {
	ethash := newEthash(config)
	q := &QEthash{
		Ethash: ethash,
	}
	spec := consensus.MiningSpec{
		Name:       "Ethash",
		HashAlgo:   q.hashAlgo,
		VerifySeal: q.verifySeal,
	}
	q.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote)
	return q
}
