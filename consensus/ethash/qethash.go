package ethash

import (
	"bytes"
	"math/big"
	"runtime"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
)

// QEthash is our wrapper over geth ethash implementation.
type QEthash struct {
	*Ethash
	*consensus.CommonEngine
}

func (q *QEthash) hashAlgo(shareCache *consensus.ShareCache) (err error) {
	epoch := shareCache.Height / epochLength
	cache := q.cache(epoch)
	size := datasetSize(epoch)
	if q.config.PowMode == ModeTest {
		size = 32 * 1024
	}
	shareCache.Digest, shareCache.Result = hashimotoLight(size, cache.cache, shareCache.Hash, shareCache.Nonce)
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
	epoch := header.NumberU64() / epochLength

	var (
		digest []byte
		result []byte
	)
	cache := q.cache(epoch)

	size := datasetSize(epoch)
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
	if diff == nil {
		diff = header.GetDifficulty()
	}
	if diff.Cmp(big.NewInt(0)) == 0 {
		diff = big.NewInt(1)
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

func (q *QEthash) RefreshWork(tip uint64) {
	q.CommonEngine.RefreshWork(tip)
}

// New returns a Ethash scheme.
func New(
	config Config,
	diffCalculator consensus.DifficultyCalculator,
	remote bool,
	pubKey []byte,
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
	q.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey)
	return q
}
