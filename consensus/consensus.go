package consensus

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"strings"
	"sync"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus/posw"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
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
	Digest    []byte
	Result    []byte
	Height    uint64
	Hash      []byte
	Nonce     uint64
	Seed      []byte
	BlockTime uint64
}

// MiningWork represents the params of mining work.
type MiningWork struct {
	HeaderHash      common.Hash
	Number          uint64
	Difficulty      *big.Int
	OptionalDivider uint64
	BlockTime       uint64
}

// MiningResult represents the found digest and result bytes.
type MiningResult struct {
	Digest    common.Hash
	Result    []byte
	Nonce     uint64
	Signature *[]byte
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
	pubKey   []byte

	threads int
	lock    sync.Mutex

	closeOnce         sync.Once
	exitCh            chan chan error
	currentWorks      *currentWorks
	sealVerifiedCache *lru.Cache
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
func (c *CommonEngine) CalcDifficulty(chain ChainReader, time uint64, parent types.IBlock) (*big.Int, error) {
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
	if header.GetVersion() != 0 {
		return errors.New("incorrect block's version")
	}

	/*	if chain.GetHeader(header.Hash()) != nil {
			fmt.Println(header.Hash())
			//return nil
		}
	*/
	parent := chain.GetBlock(header.GetParentHash())
	if parent == nil {
		return ErrUnknownAncestor
	}
	if parent.NumberU64() != number-1 {
		return errors.New("incorrect block height")
	}
	if uint32(len(header.GetExtra())) > chain.Config().BlockExtraDataSizeLimit {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.GetExtra()), chain.Config().BlockExtraDataSizeLimit)
	}

	if header.GetTime() <= parent.Time() {
		return fmt.Errorf("incorrect create time tip time %d, new block time %d",
			header.GetTime(), parent.Time())
	}

	if !chain.SkipDifficultyCheck() {
		expectedDiff, err := c.diffCalc.CalculateDifficulty(parent, header.GetTime())
		if err != nil {
			return err
		}
		if expectedDiff.Cmp(header.GetDifficulty()) != 0 {
			errMsg := fmt.Sprintf("invalid difficulty: have %v, want %v", header.GetDifficulty(), expectedDiff)
			logger.Error(errMsg)
			return errors.New(errMsg)
		}
	}

	diff, divider, err := chain.GetAdjustedDifficulty(header)
	if err != nil {
		return err
	}
	adjustedDiff := new(big.Int).Div(diff, new(big.Int).SetUint64(divider))
	return c.VerifySeal(chain, header, adjustedDiff)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (c *CommonEngine) VerifySeal(chain ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	if v, ok := c.sealVerifiedCache.Get(header.Hash()); ok {
		if v.(*big.Int).Cmp(adjustedDiff) == 0 {
			return nil
		}
	}
	err := c.spec.VerifySeal(chain, header, adjustedDiff)
	if err == nil {
		c.sealVerifiedCache.Add(header.Hash(), adjustedDiff)
	}
	return err
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
	diff *big.Int,
	optionalDivider uint64,
	results chan<- types.IBlock,
	stop <-chan struct{}) error {

	if diff == nil {
		diff = block.IHeader().GetDifficulty()
	}
	if c.isRemote {
		c.SetWork(block, diff, optionalDivider, results)
		return nil
	}
	adjustedDiff := new(big.Int).Div(diff, new(big.Int).SetUint64(optionalDivider))
	return c.localSeal(block, adjustedDiff, results, stop)
}

// localSeal generates a new block for the given input block with the local miner's
// seal place on top.
func (c *CommonEngine) localSeal(
	block types.IBlock,
	diff *big.Int,
	results chan<- types.IBlock,
	stop <-chan struct{},
) error {

	found := make(chan MiningResult)
	header := block.IHeader()
	if diff == nil {
		diff = header.GetDifficulty()
	}
	work := MiningWork{HeaderHash: header.SealHash(), Number: header.NumberU64(), Difficulty: diff, BlockTime: block.Time()}
	if err := c.FindNonce(work, found, stop); err != nil {
		return err
	}
	// Convert found header to block
	go func() {
		select {
		case result := <-found:
			select {
			case results <- block.WithMingResult(result.Nonce, result.Digest, nil):
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
	if new(big.Int).Cmp(work.Difficulty) == 0 {
		work.Difficulty = new(big.Int).SetUint64(1)
	}
	var (
		target   = new(big.Int).Div(two256, work.Difficulty)
		minerRes = ShareCache{
			Height:    work.Number,
			Hash:      work.HeaderHash.Bytes(),
			Seed:      make([]byte, 40),
			Nonce:     startNonce,
			BlockTime: work.BlockTime,
		}
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

func (c *CommonEngine) GetWork(addr account.Address) (*MiningWork, error) {
	if !c.isRemote {
		return nil, ErrNotRemote
	}
	var (
		workCh = make(chan MiningWork, 1)
		errc   = make(chan error, 1)
	)
	select {
	case c.fetchWorkCh <- &sealWork{errc: errc, res: workCh, addr: addr}:
	case <-c.exitCh:
		return nil, errors.New(fmt.Sprintf("%s hash stopped", c.Name()))
	}

	select {
	case work := <-workCh:
		return &MiningWork{
			HeaderHash:      work.HeaderHash,
			Number:          work.Number,
			Difficulty:      work.Difficulty,
			OptionalDivider: work.OptionalDivider,
		}, nil
	case err := <-errc:
		return nil, err
	}
}

func (c *CommonEngine) RefreshWork(tip uint64) {
	if c.isRemote {
		c.currentWorks.refresh(tip)
	}
}

func (c *CommonEngine) SubmitWork(nonce uint64, hash, digest common.Hash, signature *[65]byte) bool {
	if !c.isRemote {
		return false
	}
	var errc = make(chan error, 1)

	select {
	case c.submitWorkCh <- &mineResult{
		nonce:     nonce,
		mixDigest: digest,
		hash:      hash,
		signature: signature,
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

func (c *CommonEngine) SetWork(block types.IBlock, diff *big.Int, optionalDivider uint64, results chan<- types.IBlock) {
	c.workCh <- &sealTask{block, diff, optionalDivider, results}
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
func NewCommonEngine(spec MiningSpec, diffCalc DifficultyCalculator, remote bool, pubKey []byte) *CommonEngine {
	cache, _ := lru.New(512)
	c := &CommonEngine{
		spec:              spec,
		diffCalc:          diffCalc,
		pubKey:            pubKey,
		sealVerifiedCache: cache,
	}
	if remote {
		c.isRemote = true
		c.workCh = make(chan *sealTask)
		c.fetchWorkCh = make(chan *sealWork)
		c.submitWorkCh = make(chan *mineResult)
		c.exitCh = make(chan chan error)
		c.currentWorks = newCurrentWorks()
		go c.remote()
	}

	return c
}

func CreatePoSWCalculator(cr ChainReader, poswConfig *config.POSWConfig) PoSWCalculator {
	return posw.NewPoSW(cr, poswConfig)
}
