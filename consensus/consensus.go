package consensus

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"runtime"
	"strings"
	"sync"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
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

	// ErrNotRemote is returned if GetWork be called in local mining
	// is not remote
	ErrNotRemote = errors.New("is not remote mining")
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	ErrInvalidDifficulty = errors.New("non-positive difficulty")
	ErrInvalidMixDigest  = errors.New("invalid mix digest")
	ErrInvalidPoW        = errors.New("invalid proof-of-work")
	ErrPreTime           = errors.New("parent time is smaller than time for CalculateDifficulty")
)

type ShareCache struct {
	Digest []byte
	Result []byte
	Height uint64
	Hash   []byte
	Nonce  uint64
	Seed   []byte
}

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
	Name       string
	HashAlgo   func(result *ShareCache) error
	VerifySeal func(chain ChainReader, header types.IHeader, adjustedDiff *big.Int) error
}

// CommonEngine contains the common parts for consensus engines, where engine-specific
// logic is provided in func args as template pattern.
type CommonEngine struct {
	spec MiningSpec

	// Remote sealer related fields
	isRemote     bool
	workCh       chan *sealTask
	fetchWorkCh  chan *sealWork
	submitWorkCh chan *mineResult

	diffCalc DifficultyCalculator

	threads int
	lock    sync.Mutex

	closeOnce sync.Once
	exitCh    chan chan error
}

// Name returns the consensus engine's name.
func (c *CommonEngine) Name() string {
	return c.spec.Name
}

// Author returns coinbase address.
func (c *CommonEngine) Author(header types.IHeader) (account.Address, error) {
	return header.GetCoinbase(), nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (c *CommonEngine) CalcDifficulty(chain ChainReader, time uint64, parent types.IHeader) (*big.Int, error) {
	return c.diffCalc.CalculateDifficulty(parent, time)
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *CommonEngine) VerifyHeader(
	chain ChainReader,
	header types.IHeader,
	seal bool,
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
		return fmt.Errorf("incorrect create time tip time %d, new block time %d",
			header.GetTime(), parent.GetTime())
	}

	adjustedDiff := new(big.Int).SetUint64(0)
	if !chain.Config().SkipRootDifficultyCheck {
		expectedDiff, err := c.diffCalc.CalculateDifficulty(parent, header.GetTime())
		if err != nil {
			return err
		}
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
	if new(big.Int).Add(header.GetDifficulty(), parent.GetTotalDifficulty()).Cmp(header.GetTotalDifficulty()) != 0 {
		return fmt.Errorf("error total diff header.diff:%v parent.total:%v,header.total:%v", header.GetDifficulty(), parent.GetTotalDifficulty(), header.GetTotalDifficulty())
	}
	return c.spec.VerifySeal(chain, header, adjustedDiff)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (c *CommonEngine) VerifySeal(chain ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	return c.spec.VerifySeal(chain, header, adjustedDiff)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (c *CommonEngine) VerifyHeaders(
	chain ChainReader,
	headers []types.IHeader,
	seals []bool,
) (chan<- struct{}, <-chan error) {

	// TODO: verify concurrently, and support aborting
	abort := make(chan struct{})
	errorsOut := make(chan error, len(headers))
	go func() {
		for _, h := range headers {
			err := c.VerifyHeader(chain, h, true /*seal flag not used*/)
			errorsOut <- err
		}
	}()
	return abort, errorsOut
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
	c.lock.Lock()
	threads := c.threads
	c.lock.Unlock()
	if threads == 0 {
		threads = runtime.NumCPU()
	}
	if threads < 0 {
		threads = 0 // Allows disabling local mining without extra logic around local/remote
	}

	pend := sync.WaitGroup{}
	for i := 0; i < threads; i++ {
		pend.Add(1)
		go func(id int, nonce uint64) {
			defer pend.Done()
			c.mine(work, id, nonce, abort, found)
		}(i, uint64(randGen.Uint64()))
	}

	go func() {
		select {
		case <-stop:
			close(abort)
			return
		case result := <-found:
			select {
			case results <- result:
			}
			close(abort)
		}
		pend.Wait()
	}()
	return nil
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (c *CommonEngine) Seal(
	chain ChainReader,
	block types.IBlock,
	results chan<- types.IBlock,
	stop <-chan struct{}) error {
	if c.isRemote {
		c.SetWork(block, results)
		return nil
	}
	return c.localSeal(block, results, stop)
}

// localSeal generates a new block for the given input block with the local miner's
// seal place on top.
func (c *CommonEngine) localSeal(
	block types.IBlock,
	results chan<- types.IBlock,
	stop <-chan struct{},
) error {

	found := make(chan MiningResult)
	header := block.IHeader()
	work := MiningWork{HeaderHash: header.SealHash(), Number: header.NumberU64(), Difficulty: header.GetDifficulty()}
	if err := c.FindNonce(work, found, stop); err != nil {
		return err
	}
	// Convert found header to block
	go func() {
		select {
		case result := <-found:
			select {
			case results <- block.WithMingResult(result.Nonce, result.Digest):
			default:
				log.Warn("Sealing result is not read by miner", "mode", "local", "sealhash", work.HeaderHash)
			}
		}
	}()
	return nil
}

func (c *CommonEngine) mine(
	work MiningWork,
	id int,
	startNonce uint64,
	abort <-chan struct{},
	found chan MiningResult,
) {

	var (
		target   = new(big.Int).Div(two256, work.Difficulty)
		minerRes = ShareCache{
			Height: work.Number,
			Hash:   work.HeaderHash.Bytes(),
			Seed:   make([]byte, 40),
			Nonce:  startNonce}
	)
	logger := log.New("miner", "spec", strings.ToLower(c.spec.Name), "id", id)
	logger.Trace("Started search for new nonces", "minerName", c.spec.Name, "startNonce", startNonce)
search:
	for {
		select {
		case <-abort:
			logger.Trace("Nonce search aborted", "minerName", c.spec.Name, "attempts", minerRes.Nonce-startNonce)
			break search
		default:
			err := c.spec.HashAlgo(&minerRes)
			if err != nil {
				logger.Warn("Failed to run hash algo", "miner", c.spec.Name, "error", err)
				continue // Continue the for loop. Nonce not incremented
			}
			if new(big.Int).SetBytes(minerRes.Result).Cmp(target) <= 0 {
				// Nonce found
				select {
				case found <- MiningResult{
					Nonce:  minerRes.Nonce,
					Result: minerRes.Result,
					Digest: common.BytesToHash(minerRes.Digest),
				}:
					logger.Trace("Nonce found and reported", "minerName", c.spec.Name, "attempts", minerRes.Nonce-startNonce, "nonce", minerRes.Nonce)
				case <-abort:
					logger.Trace("Nonce nonce found but discarded", "minerName", c.spec.Name, "attempts", minerRes.Nonce-startNonce, "nonce", minerRes.Nonce)
				}
				break search
			}
		}
		minerRes.Nonce++
	}
}

func (c *CommonEngine) GetWork() (*MiningWork, error) {
	if !c.isRemote {
		return nil, ErrNotRemote
	}
	var (
		workCh = make(chan MiningWork, 1)
		errc   = make(chan error, 1)
	)
	select {
	case c.fetchWorkCh <- &sealWork{errc: errc, res: workCh}:
	case <-c.exitCh:
		return nil, errors.New(fmt.Sprintf("%s hash stoped", c.Name()))
	}

	select {
	case work := <-workCh:

		return &MiningWork{
			work.HeaderHash,
			work.Number,
			work.Difficulty,
		}, nil
	case err := <-errc:
		return nil, err
	}
}

func (c *CommonEngine) SubmitWork(nonce uint64, hash, digest common.Hash) bool {
	if !c.isRemote {
		return false
	}
	var errc = make(chan error, 1)

	select {
	case c.submitWorkCh <- &mineResult{
		nonce:     nonce,
		mixDigest: digest,
		hash:      hash,
		errc:      errc,
	}:
	case <-c.exitCh:
		return false
	}
	err := <-errc
	return err == nil
}

func (c *CommonEngine) SetThreads(threads int) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.threads = threads
}

func (c *CommonEngine) SetWork(block types.IBlock, results chan<- types.IBlock) {
	c.workCh <- &sealTask{block: block, results: results}
}

func (c *CommonEngine) Close() error {
	var err error
	if !c.isRemote {
		return err
	}
	c.closeOnce.Do(func() {
		if c.exitCh == nil {
			return
		}
		errc := make(chan error)
		c.exitCh <- errc
		err = <-errc
		close(c.exitCh)
	})
	return err
}

// NewCommonEngine returns the common engine mixin.
func NewCommonEngine(spec MiningSpec, diffCalc DifficultyCalculator, remote bool) *CommonEngine {
	c := &CommonEngine{
		spec:     spec,
		diffCalc: diffCalc,
	}
	if remote {
		c.isRemote = true
		c.workCh = make(chan *sealTask)
		c.fetchWorkCh = make(chan *sealWork)
		c.submitWorkCh = make(chan *mineResult)
		c.exitCh = make(chan chan error)
		go c.remote()
	}

	return c
}
