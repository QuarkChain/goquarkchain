package consensus

import (
	"errors"
	"fmt"
	"github.com/hashicorp/golang-lru"
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
	block   types.IBlock
	results chan<- types.IBlock
}

type mineResult struct {
	nonce     uint64
	mixDigest common.Hash
	hash      common.Hash

	errc chan error
}

type sealWork struct {
	errc chan error
	res  chan MiningWork
}

func (c *CommonEngine) remote() {
	var (
		results      chan<- types.IBlock
		currentBlock types.IBlock = nil
		currentWork  MiningWork
	)
	works, err := lru.New(staleThreshold)
	if err != nil {
		log.Error("Failed to create unmined block cache", "err", err)
		return
	}

	makeWork := func(block types.IBlock) {
		hash := block.IHeader().SealHash()
		if works.Contains(hash) {
			return
		}
		currentWork.HeaderHash = hash
		currentWork.Number = block.NumberU64()
		currentWork.Difficulty = block.IHeader().GetDifficulty()

		currentBlock = block
		works.Add(hash, block)
	}

	submitWork := func(nonce uint64, mixDigest common.Hash, sealhash common.Hash) bool {
		if currentBlock == nil {
			log.Error("Pending work without block", "sealhash", sealhash)
			return false
		}
		var block types.IBlock
		value, ok := works.Get(sealhash)
		if ok {
			block = value.(types.IBlock)
		}
		if block == nil {
			log.Warn("Work submitted but none pending", "sealhash", sealhash, "curnumber", currentBlock.NumberU64())
			return false
		}

		if results == nil {
			log.Warn("Qkcash result channel is empty, submitted mining result is rejected")
			return false
		}

		solution := block.WithMingResult(nonce, mixDigest)
		start := time.Now()
		if err := c.spec.VerifySeal(nil, solution.IHeader(), solution.IHeader().GetDifficulty()); err != nil {
			log.Warn("Invalid proof-of-work submitted", "sealhash", sealhash.Hex(), "elapsed", time.Since(start), "err", err)
			return false
		}
		if solution.NumberU64()+staleThreshold > currentBlock.NumberU64() {
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
		case work := <-c.workCh:
			results = work.results
			makeWork(work.block)

		case work := <-c.fetchWorkCh:
			if currentBlock == nil {
				work.errc <- ErrNoMiningWork
			} else {
				work.res <- currentWork
			}

		case result := <-c.submitWorkCh:
			if submitWork(result.nonce, result.mixDigest, result.hash) {
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
