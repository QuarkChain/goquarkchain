package qkchash

import (
	"bytes"
	"errors"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"time"
)

const (
	// staleThreshold is the maximum depth of the acceptable stale but valid qkchash solution.
	staleThreshold = 7
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))

	errNoMiningWork      = errors.New("no mining work available yet")
	errInvalidSealResult = errors.New("invalid or stale proof-of-work solution")
)

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (q *QKCHash) Seal(
	chain consensus.ChainReader,
	block types.IBlock,
	results chan<- types.IBlock,
	stop <-chan struct{}) error {

	return q.commonEngine.Seal(block, results, stop)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (q *QKCHash) VerifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	if header.GetDifficulty().Sign() <= 0 {
		return consensus.ErrInvalidDifficulty
	}
	if adjustedDiff.Cmp(new(big.Int).SetUint64(0)) == 0 {
		adjustedDiff = header.GetDifficulty()
	}

	miningRes, err := q.hashAlgo(header.SealHash().Bytes(), header.GetNonce())
	if err != nil {
		return err
	}
	if !bytes.Equal(header.GetMixDigest().Bytes(), miningRes.Digest.Bytes()) {
		return consensus.ErrInvalidMixDigest
	}
	target := new(big.Int).Div(two256, adjustedDiff)
	if new(big.Int).SetBytes(miningRes.Result).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}

func (q *QKCHash) remote() {
	var (
		works = make(map[common.Hash]types.IBlock)
		rates = make(map[common.Hash]hashrate)

		results      chan<- types.IBlock
		currentBlock types.IBlock
		currentWork  [4]string
	)

	makeWork := func(block types.IBlock) {
		hash := block.Hash()
		currentWork[0] = hash.Hex()
		currentWork[1] = common.BytesToHash(ethash.SeedHash(block.NumberU64())).Hex()
		currentWork[2] = common.BytesToHash(new(big.Int).Div(two256, block.IHeader().GetDifficulty()).Bytes()).Hex()
		currentWork[3] = hexutil.EncodeUint64(block.NumberU64())

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
		case work := <-q.workCh:
			// Update current work with new received block.
			results = work.results
			makeWork(work.block)

		case work := <-q.fetchWorkCh:
			// Return current mining work to remote miner.
			if currentBlock == nil {
				work.errc <- errNoMiningWork
			} else {
				work.res <- currentWork
			}

		case result := <-q.submitWorkCh:
			if submitWork(result.nonce, result.mixDigest, result.hash) {
				result.errc <- nil
			} else {
				result.errc <- errInvalidSealResult
			}

		case result := <-q.submitRateCh:
			rates[result.id] = hashrate{rate: result.rate, ping: time.Now()}
			close(result.done)

		case <-ticker.C:
			for id, rate := range rates {
				if time.Since(rate.ping) > 10*time.Second {
					delete(rates, id)
				}
			}
			// Clear stale pending blocks
			if currentBlock != nil {
				for hash, block := range works {
					if block.NumberU64()+staleThreshold <= currentBlock.NumberU64() {
						delete(works, hash)
					}
				}
			}

		case errc := <-q.exitCh:
			errc <- nil
			log.Trace("Qkcash remote sealer is exiting")
			return
		}
	}
}
