package consensus

import (
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/hashicorp/golang-lru"
	"math/big"
	"sync"
	"time"

	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

const (
	// staleThreshold is the maximum depth of the acceptable stale but valid qkchash solution.
	staleThreshold = 7
)

var (
	ErrNoMiningWork      = errors.New("no mining work available yet")
	errInvalidSealResult = errors.New("invalid or stale proof-of-work solution")
)

type sealTask struct {
	block           types.IBlock
	diff            *big.Int
	optionalDivider uint64
	results         chan<- types.IBlock
}

type mineResult struct {
	nonce     uint64
	mixDigest common.Hash
	hash      common.Hash
	signature *[65]byte
	errc      chan error
}

type sealWork struct {
	errc chan error
	res  chan MiningWork
	addr account.Address
}

func (c *CommonEngine) remote() {
	var (
		results       chan<- types.IBlock
		currentHeight uint64
	)
	works, _ := lru.New(128)

	makeWork := func(block types.IBlock, adjustedDiff *big.Int, optionalDivider uint64) {
		hash := block.IHeader().SealHash()
		if works.Contains(hash) {
			return
		}

		diff := block.IHeader().GetDifficulty()
		if adjustedDiff != nil {
			diff = adjustedDiff
		}

		c.currentWorks.setCurrentWork(block, diff, optionalDivider)

		works.Add(hash, block)
		currentHeight = block.NumberU64()
	}

	submitWork := func(nonce uint64, mixDigest common.Hash, sealhash common.Hash, signature *[65]byte) bool {
		if c.currentWorks.len() == 0 {
			log.Error("Pending work without block", "sealhash", sealhash)
			return false
		}
		var block types.IBlock
		value, ok := works.Get(sealhash)
		if ok {
			block = value.(types.IBlock)
		}
		if block == nil {
			log.Warn("Work submitted but none pending", "sealhash", sealhash)
			return false
		}

		work, err := c.currentWorks.getWorkBySealHash(sealhash)
		if err != nil {
			log.Info("already be delete", "height", block.NumberU64())
			return false
		}

		if results == nil {
			log.Warn("Qkc cash result channel is empty, submitted mining result is rejected")
			return false
		}

		solution := block.WithMingResult(nonce, mixDigest, signature)
		adjustedDiff := work.Difficulty
		// if tx has been sign by miner and difficulty has not been adjusted before
		// we can adjust difficulty here if the signature pub key is
		if signature != nil && adjustedDiff.Cmp(solution.IHeader().GetDifficulty()) == 0 {
			if crypto.VerifySignature(c.pubKey, solution.IHeader().SealHash().Bytes(), signature[:64]) {
				adjustedDiff = new(big.Int).Div(solution.IHeader().GetDifficulty(), new(big.Int).SetUint64(1000))
			} else {
				adjustedDiff = new(big.Int).Div(solution.IHeader().GetDifficulty(), new(big.Int).SetUint64(work.OptionalDivider))
			}
		}

		start := time.Now()
		if err := c.spec.VerifySeal(nil, solution.IHeader(), adjustedDiff); err != nil {
			log.Warn("Invalid proof-of-work submitted", "sealhash", sealhash.Hex(), "elapsed", time.Since(start), "err", err)
			return false
		}
		if solution.NumberU64()+staleThreshold > currentHeight {
			select {
			case results <- solution:
				log.Debug("Work submitted is acceptable", "number", solution.NumberU64(), "sealhash", sealhash, "hash", solution.Hash())
				return true
			default:
				log.Warn("Sealing result is not read by miner", "mode", "remote", "sealhash", sealhash)
				return false
			}
		}
		// The submitted block is too old to accept, drop it.
		log.Warn("Work submitted is too old", "number", solution.NumberU64(), "sealhash", sealhash, "hash", solution.Hash())
		return false
	}

	for {
		select {
		case task := <-c.workCh:
			results = task.results
			makeWork(task.block, task.diff, task.optionalDivider)

		case work := <-c.fetchWorkCh:
			currWork, err := c.currentWorks.getWorkByAddr(work.addr)
			if err != nil {
				work.errc <- err
			} else {
				work.res <- *currWork
			}

		case result := <-c.submitWorkCh:
			if submitWork(result.nonce, result.mixDigest, result.hash, result.signature) {
				result.errc <- nil
			} else {
				result.errc <- errInvalidSealResult
			}

		case errc := <-c.exitCh:
			errc <- nil
			log.Trace(fmt.Sprintf("Qkchash remote %s from remote", c.Name()))
		}
	}
}

type currentWorks struct {
	works map[account.Address]*MiningWork
	mu    sync.RWMutex
}

func newCurrentWorks() *currentWorks {
	return &currentWorks{
		works: make(map[account.Address]*MiningWork),
	}
}

func (c *currentWorks) setCurrentWork(block types.IBlock, diff *big.Int, optionalDivider uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	height := block.NumberU64()

	miningWork := new(MiningWork)
	miningWork.HeaderHash = block.IHeader().SealHash()
	miningWork.Number = height
	miningWork.Difficulty = diff
	miningWork.OptionalDivider = optionalDivider

	c.works[block.Coinbase()] = miningWork
}

func (c *currentWorks) len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.works)
}

func (c *currentWorks) getWorkByAddr(addr account.Address) (*MiningWork, error) {
	work := c.works[addr]
	if work == nil {
		return nil, ErrNoMiningWork
	}
	return work, nil
}

func (c *currentWorks) getDifficultByAddr(addr account.Address) *big.Int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	work := c.works[addr]
	if work == nil {
		panic("bug fix getDifficultByAddr func")
	}
	return work.Difficulty
}
func (c *currentWorks) refresh(tip uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.works = make(map[account.Address]*MiningWork)
}

func (c *currentWorks) getWorkBySealHash(hash common.Hash) (*MiningWork, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, v := range c.works {
		if v.HeaderHash == hash {
			return v, nil
		}
	}
	return nil, ErrNoMiningWork
}
