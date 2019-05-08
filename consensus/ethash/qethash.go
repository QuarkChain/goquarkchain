package ethash

import (
	"bytes"
	"math/big"
	"runtime"

	"github.com/ethereum/go-ethereum/common"

	qkconsensus "github.com/QuarkChain/goquarkchain/consensus"
	qkctypes "github.com/QuarkChain/goquarkchain/core/types"
)

// QEthash is our wrapper over geth ethash implementation.
type QEthash struct {
	*Ethash
	*qkconsensus.CommonEngine
}

func (q *QEthash) hashAlgo(height uint64, hash []byte, nonce uint64) (ret qkconsensus.MiningResult, err error) {
	cache := q.cache(height)
	size := datasetSize(height)
	if q.config.PowMode == ModeTest {
		size = 32 * 1024
	}
	digest, result := hashimotoLight(size, cache.cache, hash, nonce)
	return qkconsensus.MiningResult{
		Digest: common.BytesToHash(digest),
		Result: result,
		Nonce:  nonce,
	}, nil
}

// verifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (q *QEthash) verifySeal(chain qkconsensus.ChainReader, header qkctypes.IHeader, adjustedDiff *big.Int) error {
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
	target := new(big.Int).Div(two256, header.GetDifficulty())
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return errInvalidPoW
	}
	return nil
}

// New returns a Ethash scheme.
func New(
	config Config,
	diffCalculator qkconsensus.DifficultyCalculator,
	remote bool,
) *QEthash {
	ethash := newEthash(config )
	q := &QEthash{
		Ethash: ethash,
	}
	spec := qkconsensus.MiningSpec{
		Name:       "Ethash",
		HashAlgo:   q.hashAlgo,
		VerifySeal: q.verifySeal,
	}
	q.CommonEngine = qkconsensus.NewCommonEngine(spec, diffCalculator, remote)
	return q
}
