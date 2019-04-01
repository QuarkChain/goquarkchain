package consensus

import (
	crand "crypto/rand"
	"errors"
	"fmt"

	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"

	"math"
	"math/big"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
	// ErrUnknownAncestor is returned when validating a block requires an ancestor
	// that is unknown.
	ErrUnknownAncestor = errors.New("unknown ancestor")

	// ErrPrunedAncestor is returned when validating a block requires an ancestor
	// that is known, but the state of which is not available.
	ErrPrunedAncestor = errors.New("pruned ancestor")

	// ErrFutureBlock is returned when a block's timestamp is in the future according
	// to the current node.
	ErrFutureBlock = errors.New("block in the future")

	// ErrInvalidNumber is returned if a block's number doesn't equal it's parent's
	// plus one.
	ErrInvalidNumber = errors.New("invalid block number")
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	ErrInvalidDifficulty = errors.New("non-positive difficulty")
	ErrInvalidMixDigest  = errors.New("invalid mix digest")
	ErrInvalidPoW        = errors.New("invalid proof-of-work")
)

// MiningWork represents the params of mining work.
type MiningWork struct {
	HeaderHash common.Hash
	Number     uint64
	Difficulty *big.Int
}

// MiningResult represents the found digest and result bytes.
type MiningResult struct {
	Digest common.Hash
	Result []byte
	Nonce  uint64
}

// MiningSpec contains a PoW algo's basic info and hash algo
type MiningSpec struct {
	Name     string
	HashAlgo func(hash []byte, nonce uint64) (MiningResult, error)
}

// CommonEngine contains the common parts for consensus engines, where engine-specific
// logic is provided in func args as template pattern.
type CommonEngine struct {
	spec     MiningSpec
	hashrate metrics.Meter
}

// Name returns the consensus engine's name.
func (c *CommonEngine) Name() string {
	return c.spec.Name
}

// Hashrate returns the current mining hashrate of a PoW consensus engine.
func (c *CommonEngine) Hashrate() float64 {
	return c.hashrate.Rate1()
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *CommonEngine) VerifyHeader(
	chain ChainReader,
	header types.IHeader,
	seal bool,
	cengine Engine,
) error {
	// Short-circuit if the header is known, or parent not
	number := header.NumberU64()
	logger := log.New("engine")

	if chain.GetHeader(header.Hash()) != nil {
		return nil
	}
	parent := chain.GetHeader(header.GetParentHash())
	if parent == nil {
		return ErrUnknownAncestor
	}
	if parent.NumberU64() != number-1 {
		return errors.New("incorrect block height")
	}
	if uint32(len(header.GetExtra())) > chain.Config().BlockExtraDataSizeLimit {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.GetExtra()), chain.Config().BlockExtraDataSizeLimit)
	}

	if header.GetTime() < parent.GetTime() {
		return errors.New("timestamp equals parent's")
	}

	adjustedDiff := new(big.Int).SetUint64(0)
	if !chain.Config().SkipRootDifficultyCheck {
		expectedDiff := cengine.CalcDifficulty(chain, header.GetTime(), parent)
		if expectedDiff.Cmp(header.GetDifficulty()) != 0 {
			errMsg := fmt.Sprintf("invalid difficulty: have %v, want %v", header.GetDifficulty(), expectedDiff)
			logger.Error(errMsg)
			return errors.New(errMsg)
		}
		if reflect.TypeOf(header) == reflect.TypeOf(new(types.RootBlockHeader)) {
			rootHeader := header.(*types.RootBlockHeader)
			if crypto.VerifySignature(common.Hex2Bytes(chain.Config().GuardianPublicKey), rootHeader.Hash().Bytes(), rootHeader.Signature[:]) {
				adjustedDiff = expectedDiff.Div(expectedDiff, new(big.Int).SetUint64(1000))
			}
		}
	}

	return cengine.VerifySeal(chain, header, adjustedDiff)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (c *CommonEngine) VerifyHeaders(
	chain ChainReader,
	headers []types.IHeader,
	seals []bool,
	cengine Engine,
) (chan<- struct{}, <-chan error) {

	// TODO: verify concurrently, and support aborting
	errorsOut := make(chan error, len(headers))
	go func() {
		for _, h := range headers {
			err := c.VerifyHeader(chain, h, true /*seal flag not used*/, cengine)
			errorsOut <- err
		}
	}()
	return nil, errorsOut
}

// FindNonce finds the desired nonce and mixhash for a given block header.
func (c *CommonEngine) FindNonce(
	work MiningWork,
	results chan<- MiningResult,
	stop <-chan struct{},
) error {

	abort := make(chan struct{})
	found := make(chan MiningResult)

	// Random number generator.
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return err
	}
	randGen := rand.New(rand.NewSource(seed.Int64()))

	// TODO: support turning this off (for remote mining)
	threads := runtime.NumCPU()
	pend := sync.WaitGroup{}
	for i := 0; i < threads; i++ {
		pend.Add(1)
		go func(id int, nonce uint64) {
			defer pend.Done()
			c.mine(work, id, nonce, abort, found)
		}(i, uint64(randGen.Uint64()))
	}

	go func() {
		var result MiningResult
		select {
		case <-stop:
			close(abort)
		case result = <-found:
			results <- result
			close(abort)
		}
		pend.Wait()
	}()
	return nil
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (c *CommonEngine) Seal(
	block types.IBlock,
	results chan<- types.IBlock,
	stop <-chan struct{},
) error {

	abort := make(chan struct{})
	found := make(chan MiningResult)
	header := block.IHeader()
	work := MiningWork{HeaderHash: header.SealHash(), Number: header.NumberU64(), Difficulty: header.GetDifficulty()}
	if err := c.FindNonce(work, found, abort); err != nil {
		return err
	}
	// Convert found header to block
	go func() {
		var result MiningResult
		select {
		case <-stop:
			close(abort)
		case result = <-found:
			select {
			case results <- block.WithMingResult(result.Nonce, result.Digest):
			default:
				log.Warn("Sealing result is not read by miner", "mode", "local", "sealhash", work.HeaderHash)
			}
			close(abort)
		}
	}()
	return nil
}

func (c *CommonEngine) mine(
	work MiningWork,
	id int,
	startNonce uint64,
	abort chan struct{},
	found chan MiningResult,
) {

	var (
		hash     = work.HeaderHash.Bytes()
		target   = new(big.Int).Div(two256, work.Difficulty)
		attempts = int64(0)
		nonce    = startNonce
	)
	logger := log.New("miner", "spec", strings.ToLower(c.spec.Name), "id", id)
	logger.Trace("Started search for new nonces", "minerName", c.spec.Name, "startNonce", startNonce)
search:
	for {
		select {
		case <-abort:
			logger.Trace("Nonce search aborted", "minerName", c.spec.Name, "attempts", nonce-startNonce)
			c.hashrate.Mark(attempts)
			break search
		default:
			attempts++
			if (attempts % (1 << 15)) == 0 {
				c.hashrate.Mark(attempts)
				attempts = 0
			}
			miningRes, err := c.spec.HashAlgo(hash, nonce)
			if err != nil {
				logger.Warn("Failed to run hash algo", "miner", c.spec.Name, "error", err)
				continue // Continue the for loop. Nonce not incremented
			}
			if new(big.Int).SetBytes(miningRes.Result).Cmp(target) <= 0 {
				// Nonce found
				select {
				case found <- miningRes:
					logger.Trace("Nonce found and reported", "minerName", c.spec.Name, "attempts", nonce-startNonce, "nonce", nonce)
				case <-abort:
					logger.Trace("Nonce nonce found but discarded", "minerName", c.spec.Name, "attempts", nonce-startNonce, "nonce", nonce)
				}
				break search
			}
		}
		nonce++
	}
}

// NewCommonEngine returns the common engine mixin.
func NewCommonEngine(spec MiningSpec) *CommonEngine {
	return &CommonEngine{
		spec:     spec,
		hashrate: metrics.NewMeter(),
	}
}
