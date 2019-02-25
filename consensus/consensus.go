package consensus

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"strings"
	"sync"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/metrics"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
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

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header and/or uncle verification.
type ChainReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *config.QuarkChainConfig

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() types.IHeader

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash, number uint64) types.IHeader

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) types.IHeader

	// GetHeaderByHash retrieves a block header from the database by its hash.
	GetHeaderByHash(hash common.Hash) types.IHeader

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash common.Hash, number uint64) types.IBlock
}

// Engine is an algorithm agnostic consensus engine.
type Engine interface {
	// Author retrieves the Ethereum address of the account that minted the given
	// block, which may be different from the header's coinbase if a consensus
	// engine is based on signatures.
	Author(header types.IHeader) (recipient account.Recipient, err error)

	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain ChainReader, header types.IHeader, seal bool) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(chain ChainReader, headers []types.IHeader, seals []bool) (chan<- struct{}, <-chan error)

	// VerifySeal checks whether the crypto seal on a header is valid according to
	// the consensus rules of the given engine.
	VerifySeal(chain ChainReader, header types.IHeader) error

	// Prepare initializes the consensus fields of a block header according to the
	// rules of a particular engine. The changes are executed inline.
	Prepare(chain ChainReader, header *types.IHeader) error

	// Finalize runs any post-transaction state modifications (e.g. block rewards)
	// and assembles the final block.
	// Note: The block header and state database might be updated to reflect any
	// consensus rules that happen at finalization (e.g. block rewards).
	/*	Finalize(chain ChainReader, header *types.IHeader, state *state.StateDB, txs []*types.Transaction,
		receipts []*types.Receipt) (*types.IBlock, error)*/ //TODO

	// Seal generates a new sealing request for the given input block and pushes
	// the result into the given channel.
	//
	// Note, the method returns immediately and will send the result async. More
	// than one result may also be returned depending on the consensus algorithm.
	Seal(chain ChainReader, block types.IBlock, results chan<- types.IBlock, stop <-chan struct{}) error

	// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
	// that a new block should have.
	CalcDifficulty(chain ChainReader, time uint64, parent types.IHeader) *big.Int

	// APIs returns the RPC APIs this consensus engine provides.
	APIs(chain ChainReader) []rpc.API

	// Close terminates any background threads maintained by the consensus engine.
	Close() error
}

// CommonEngine contains the common parts for consensus engines, where engine-specific
// logic is provided in func args as template pattern.
type CommonEngine struct {
	spec     MiningSpec
	hashrate metrics.Meter
}

// PoW is the quarkchain version of PoW consensus engine, with a conveninent method for
// remote miners.
type PoW interface {
	Engine
	// Hashrate returns the current mining hashrate of a PoW consensus engine.
	Hashrate() float64
	FindNonce(work MiningWork, results chan<- MiningResult, stop <-chan struct{}) error
	Name() string
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
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.GetParentHash(), number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}

	if uint64(len(header.GetExtra())) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.GetExtra()), params.MaximumExtraDataSize)
	}

	if header.GetTime() < parent.GetTime() {
		return errors.New("timestamp equals parent's")
	}

	expectedDiff := cengine.CalcDifficulty(chain, header.GetTime(), parent)
	if expectedDiff.Cmp(header.GetDifficulty()) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.GetDifficulty(), expectedDiff)
	}

	// TODO: validate gas limit

	// TODO: verify gas limit is within allowed bounds

	if header.NumberU64()-parent.NumberU64() != 1 {
		return consensus.ErrInvalidNumber
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
	logger := log.New("miner."+strings.ToLower(c.spec.Name), id)
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
