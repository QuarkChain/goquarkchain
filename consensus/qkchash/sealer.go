package qkchash

import (
	"bytes"
	crand "crypto/rand"
	"encoding/binary"
	"math"
	"math/big"
	"math/rand"
	"runtime"
	"sync"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/ethereum/go-ethereum/common"
	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

var (
	// maxUint256 is a big integer representing 2^256-1
	maxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

// SealHash returns the hash of a block prior to it being sealed.
func (q *QKCHash) SealHash(header *types.Header) common.Hash {
	return q.ethash.SealHash(header)
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (q *QKCHash) Seal(
	chain ethconsensus.ChainReader,
	block *types.Block,
	results chan<- *types.Block,
	stop <-chan struct{}) error {

	abort := make(chan struct{})
	found := make(chan *types.Block)

	// Random number generator.
	seed, err := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return err
	}
	randGen := rand.New(rand.NewSource(seed.Int64()))

	// TODO: support turning this off (remote mining)
	threads := runtime.NumCPU()
	pend := sync.WaitGroup{}
	for i := 0; i < threads; i++ {
		pend.Add(1)
		go func(id int, nonce uint64) {
			defer pend.Done()
			q.mine(block, id, nonce, abort, found)
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
				log.Warn("Sealing result is not read by miner", "mode", "local", "sealhash", q.SealHash(block.Header()))
			}
			close(abort)
		}
		pend.Wait()
	}()
	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (q *QKCHash) VerifySeal(chain ethconsensus.ChainReader, header *types.Header) error {
	if header.Difficulty.Sign() <= 0 {
		return consensus.ErrInvalidDifficulty
	}

	// TODO: cache should be block number specific in the future
	qkcCache := generateQKCCache(cacheEntryCnt, cacheSeed)

	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, header.Nonce.Uint64())
	digest, result := qkcHash(q.SealHash(header).Bytes(), nonceBytes, qkcCache)

	if !bytes.Equal(header.MixDigest[:], digest) {
		return consensus.ErrInvalidMixDigest
	}
	target := new(big.Int).Div(maxUint256, header.Difficulty)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}

func (q *QKCHash) mine(block *types.Block, id int, startNonce uint64, abort chan struct{}, found chan *types.Block) {
	var (
		header = block.Header()
		hash   = q.SealHash(header).Bytes()
		target = new(big.Int).Div(maxUint256, header.Difficulty)
		//TODO: number   = header.Number.Uint64()
		attempts = int64(0)
		nonce    = startNonce
		// TODO: cache should be block number specific in the future
		qkcCache = generateQKCCache(cacheEntryCnt, cacheSeed)
	)
	logger := log.New("miner.qkc", id)
	logger.Trace("Started qkchash search for new nonces", "startNonce", startNonce)
search:
	for {
		select {
		case <-abort:
			logger.Trace("QKCHash nonce search aborted", "attempts", nonce-startNonce)
			q.hashrate.Mark(attempts)
			break search
		default:
			attempts++
			if (attempts % (1 << 15)) == 0 {
				q.hashrate.Mark(attempts)
				attempts = 0
			}
			nonceBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(nonceBytes, nonce)
			digest, result := qkcHash(hash, nonceBytes, qkcCache)
			if new(big.Int).SetBytes(result).Cmp(target) <= 0 {
				// Nonce found
				header = types.CopyHeader(header)
				header.Nonce = types.EncodeNonce(nonce)
				header.MixDigest = common.BytesToHash(digest)

				select {
				case found <- block.WithSeal(header):
					logger.Trace("QKCHash nonce found and reported", "attempts", nonce-startNonce, "nonce", nonce)
				case <-abort:
					logger.Trace("QKCHash nonce found but discarded", "attempts", nonce-startNonce, "nonce", nonce)
				}
				break search
			}
		}
		nonce++
	}
}
