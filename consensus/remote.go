package consensus

import (
	"errors"
	"fmt"
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
	errNoMiningWork      = errors.New("no mining work available yet")
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
		works = make(map[common.Hash]types.IBlock)

		results      chan<- types.IBlock
		currentBlock types.IBlock = nil
		currentWork  MiningWork
	)

	makeWork := func(block types.IBlock) {
		hash := block.Hash()
		currentWork.HeaderHash = hash
		currentWork.Number = block.NumberU64()
		currentWork.Difficulty = block.IHeader().GetDifficulty()

		currentBlock = block
		works[hash] = block
	}

	submitWork := func(nonce uint64, mixDigest common.Hash, sealhash common.Hash) bool {
		if currentBlock == nil {
			log.Error("Pending work without block", "sealhash", sealhash)
			return false
		}
		block := works[sealhash]
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
		if err := c.spec.VerifySeal(nil, block.IHeader(), block.IHeader().GetDifficulty()); err != nil {
			log.Warn("Invalid proof-of-work submitted", "sealhash", sealhash, "elapsed", time.Since(start), "err", err)
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

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case work := <-c.workCh:
			results = work.results
			makeWork(work.block)

		case work := <-c.fetchWorkCh:
			if currentBlock == nil {
				work.errc <- errNoMiningWork
			} else {
				work.res <- currentWork
			}

		case result := <-c.submitWorkCh:
			if submitWork(result.nonce, result.mixDigest, result.hash) {
				result.errc <- nil
			} else {
				result.errc <- errInvalidSealResult
			}

		case <-ticker.C:
			if currentBlock != nil {
				now := uint64(time.Now().Unix())
				for hash, block := range works {
					if block.IHeader().GetTime()+5 < now {
						delete(works, hash)
					}
				}
			}

		case errc := <-c.exitCh:
			errc <- nil
			log.Trace(fmt.Sprintf("Qkchash remote %s from remote", c.Name()))
		}
	}
}
