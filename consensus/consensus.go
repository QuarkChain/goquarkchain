package consensus

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/ethereum/go-ethereum/params"
)

var (
	// maxUint256 is a big integer representing 2^256-1
	maxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
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

// MiningSpec contains a PoW algo's basic info and hash algo
type MiningSpec struct {
	Name     string
	HashAlgo func(hash []byte, nonce uint64) (digest []byte, result []byte)
}

// CommonEngine contains the common parts for consensus engines, where engine-specific
// logic is provided in func args as template pattern.
type CommonEngine struct {
	spec     MiningSpec
	hashrate metrics.Meter
	// For reusing existing functions
	ethash *ethash.Ethash
}

// SealHash returns the hash of a block prior to it being sealed.
func (c *CommonEngine) SealHash(header *types.Header) common.Hash {
	return c.ethash.SealHash(header)
}

// Hashrate returns the current mining hashrate of a PoW consensus engine.
func (c *CommonEngine) Hashrate() float64 {
	return c.hashrate.Rate1()
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *CommonEngine) VerifyHeader(
	chain ethconsensus.ChainReader,
	header *types.Header,
	seal bool,
	cengine ethconsensus.Engine,
) error {
	// Short-circuit if the header is known, or parent not
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return ethconsensus.ErrUnknownAncestor
	}

	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}

	if header.Time.Cmp(parent.Time) <= 0 {
		return errors.New("timestamp equals parent's")
	}

	expectedDiff := cengine.CalcDifficulty(chain, header.Time.Uint64(), parent)
	if expectedDiff.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expectedDiff)
	}

	// TODO: validate gas limit

	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}

	// TODO: verify gas limit is within allowed bounds

	if heightDiff := new(big.Int).Sub(header.Number, parent.Number); heightDiff.Cmp(big.NewInt(1)) != 0 {
		return ethconsensus.ErrInvalidNumber
	}

	if err := cengine.VerifySeal(chain, header); err != nil {
		return err
	}

	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (c *CommonEngine) VerifyHeaders(
	chain ethconsensus.ChainReader,
	headers []*types.Header,
	seals []bool,
	cengine ethconsensus.Engine,
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

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (c *CommonEngine) Seal(
	chain ethconsensus.ChainReader,
	block *types.Block,
	results chan<- *types.Block,
	stop <-chan struct{},
) error {

	abort := make(chan struct{})
	found := make(chan *types.Block)

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
			c.mine(block, id, nonce, abort, found)
		}(i, uint64(randGen.Int63()))
	}

	go func() {
		var result *types.Block
		select {
		case <-stop:
			close(abort)
		case result = <-found:
			select {
			case results <- result:
			default:
				log.Warn("Sealing result is not read by miner", "mode", "local", "sealhash", c.SealHash(block.Header()))
			}
			close(abort)
		}
		pend.Wait()
	}()
	return nil
}

func (c *CommonEngine) mine(
	block *types.Block,
	id int,
	startNonce uint64,
	abort chan struct{},
	found chan *types.Block,
) {

	var (
		header   = block.Header()
		hash     = c.SealHash(header).Bytes()
		target   = new(big.Int).Div(maxUint256, header.Difficulty)
		attempts = int64(0)
		nonce    = startNonce
	)
	logger := log.New("miner."+c.spec.Name, id)
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
			digest, result := c.spec.HashAlgo(hash, nonce)
			if new(big.Int).SetBytes(result).Cmp(target) <= 0 {
				// Nonce found
				header = types.CopyHeader(header)
				header.Nonce = types.EncodeNonce(nonce)
				header.MixDigest = common.BytesToHash(digest)

				select {
				case found <- block.WithSeal(header):
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
		ethash:   &ethash.Ethash{},
	}
}
