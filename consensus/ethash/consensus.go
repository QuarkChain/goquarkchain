// Modified from go-ethereum under GNU Lesser General Public License
package ethash

import (
	"bytes"
	"errors"
	"math/big"
	"runtime"
	"time"

	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"

	qkconsensus "github.com/QuarkChain/goquarkchain/consensus"
	qkctypes "github.com/QuarkChain/goquarkchain/core/types"
)

// Ethash proof-of-work protocol constants.
var (
	allowedFutureBlockTime = 15 * time.Second // Max time from current time allowed for blocks, before they're considered future blocks
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
)

// Some weird constants to avoid constant memory allocs for them.
var (
	big1       = big.NewInt(1)
	big2       = big.NewInt(2)
	big9       = big.NewInt(9)
	big10      = big.NewInt(10)
	bigMinus99 = big.NewInt(-99)
)

// verifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (ethash *Ethash) verifySeal(chain qkconsensus.ChainReader, header qkctypes.IHeader, adjustedDiff *big.Int) error {
	// If we're running a shared PoW, delegate verification to it
	if ethash.shared != nil {
		return ethash.shared.verifySeal(chain, header, adjustedDiff)
	}
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
	cache := ethash.cache(number)

	size := datasetSize(number)
	if ethash.config.PowMode == ModeTest {
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

// Prepare implements consensus.Engine, initializing the difficulty field of a
// header to conform to the ethash protocol. The changes are done inline.
func (ethash *Ethash) Prepare(chain consensus.ChainReader, header *types.Header) error {
	panic("not implemented")
}

// Some weird constants to avoid constant memory allocs for them.
var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)
