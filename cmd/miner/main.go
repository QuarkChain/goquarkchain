package main

import (
	"fmt"
	"log"
	"math/big"
	"math/rand"
	"strconv"
	"time"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/ethereum/go-ethereum/common"
	ethlog "github.com/ethereum/go-ethereum/log"
)

const (
	fetchWorkInterval = 5 * time.Second
)

var (
	shardToPoW = map[uint32]consensus.PoW{
		// TODO: ethash not supported yet, need wrapping
		4: doublesha256.New(), 5: doublesha256.New(),
		6: qkchash.New(), 7: qkchash.New(),
	}

	// Global var, configured through cmd flags
	host     = "localhost"
	jrpcPort = 38391
	timeout  = 10 * time.Second
)

// Wrap mining result, because the global receiver need to differentiate between
// workers
type result struct {
	worker *worker
	res    consensus.MiningResult
	work   consensus.MiningWork
}

type worker struct {
	shardID      *uint32 // nil means root chain
	pow          consensus.PoW
	fetchWorkCh  chan consensus.MiningWork
	submitWorkCh chan<- result
	abortCh      chan struct{}
}

func (w worker) fetch() {
	ticker := time.NewTicker(fetchWorkInterval)
	defer ticker.Stop()
	// One-time ticker
	coldStarter := make(chan struct{}, 1)
	coldStarter <- struct{}{}
	// Last fetched work
	var lastFetchedWork *consensus.MiningWork

	handleWork := func() {
		work, err := fetchWorkRPC(w.shardID)
		if err != nil {
			log.Print("WARN: Failed to fetch work", err)
			return
		}

		if lastFetchedWork != nil {
			if lastFetchedWork.Number.Cmp(work.Number) == 1 {
				w.log("WARN", "skip work with lower height, height: %s", work.Number.String())
				return
			}
			if lastFetchedWork.HeaderHash == work.HeaderHash {
				w.log("INFO", "skip same work, height: %s", work.Number.String())
				return
			}
		}

		lastFetchedWork = &work
		w.fetchWorkCh <- work
	}

	for {
		select {
		case <-w.abortCh:
			return
		case <-coldStarter:
			handleWork()
		case <-ticker.C:
			handleWork()
		}
	}
}

func (w worker) work() {
	// Another abort channel to stop mining
	abortWorkCh := make(chan struct{})
	resultsCh := make(chan consensus.MiningResult)
	// Current work
	var currWork *consensus.MiningWork

	for {
		select {
		case <-w.abortCh:
			close(abortWorkCh)
			return
		case work := <-w.fetchWorkCh:
			// If new work has equal or higher height, abort previous work
			if currWork != nil && work.Number.Cmp(currWork.Number) >= 0 {
				abortWorkCh <- struct{}{}
			}

			// Start finding the nonce
			if err := w.pow.FindNonce(work, resultsCh, abortWorkCh); err != nil {
				panic(err) // TODO: Send back err in an error channel
			}
			currWork = &work
			w.log("INFO", "started new work, height: %s", work.Number.String())

		case res := <-resultsCh:
			w.submitWorkCh <- result{&w, res, *currWork}
			currWork = nil
		}
	}
}

func (w worker) log(msgLevel, msgTemplate string, args ...interface{}) {
	log.Printf(
		fmt.Sprintf("%s [%s]: %s", msgLevel, shardRepr(w.shardID), msgTemplate),
		args...,
	)
}

func shardRepr(optShardID *uint32) string {
	if optShardID == nil {
		return "R"
	}
	return strconv.FormatUint(uint64(*optShardID), 10)
}

func fetchWorkRPC(shardID *uint32) (consensus.MiningWork, error) {
	// TODO: mock
	hex := fmt.Sprintf("0x51aa9d598cc3c628757b818cf9ba5cae7623a8f9630c85cc7d437d790b16175%d", rand.Intn(10))
	height := 123123123 + rand.Intn(2)
	return consensus.MiningWork{
		HeaderHash: common.HexToHash(hex),
		Number:     big.NewInt(int64(height)),
		Difficulty: big.NewInt(123123123),
	}, nil
}

func submitWorkRPC(shardID *uint32, res consensus.MiningResult) error {
	// TODO: implement
	return nil
}

func main() {
	// TODO: configure ethlog level
	ethlog.Root().SetHandler(ethlog.LvlFilterHandler(
		ethlog.Lvl(ethlog.LvlInfo),
		ethlog.StdoutHandler,
	))

	var workers []worker
	// TODO: Mock
	shards := []uint32{5}

	// Init global channels and workers
	submitWorkCh := make(chan result, len(shards))
	abortCh := make(chan struct{})
	for _, shardID := range shards {
		shardID := shardID
		w := worker{
			shardID:      &shardID,
			pow:          shardToPoW[shardID],
			fetchWorkCh:  make(chan consensus.MiningWork),
			submitWorkCh: submitWorkCh,
			abortCh:      abortCh,
		}
		workers = append(workers, w)
	}

	// Start fetching and mining
	for _, w := range workers {
		go w.fetch()
		go w.work()
	}

	// Main event loop for coordinating workers
	for {
		select {
		case res := <-submitWorkCh:
			w, mRes, mWork := res.worker, res.res, res.work
			if err := submitWorkRPC(w.shardID, mRes); err != nil {
				w.log("WARN", "Failed to submit work: %v\n", err)
			} else {
				w.log("INFO", "submitted work, height: %s\n", mWork.Number.String())
			}
		}
	}
}
