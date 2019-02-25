package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/consensus/qkchash"
	"github.com/ethereum/go-ethereum/common"
	ethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rpc"
)

const (
	fetchWorkInterval = 5 * time.Second
)

var (
	shardToPoW = map[uint32]consensus.PoW{
		// TODO: ethash not supported yet, need wrapping
		4: doublesha256.New(), 5: doublesha256.New(),
		6: qkchash.New(true), 7: qkchash.New(true),
	}

	jrpcCli *rpc.Client

	// Flags
	shardList  = flag.String("shards", "", "comma-separated string indicating shards")
	host       = flag.String("host", "localhost", "remote host of a quarkchain cluster")
	port       = flag.Int("port", 38391, "remote JSONRPC port of a quarkchain cluster")
	rpcTimeout = flag.Int("timeout", 10, "timeout in seconds for RPC calls")
	gethlogLvl = flag.String("gethloglvl", "info", "log level of geth")
)

// Wrap mining result, because the global receiver need to differentiate between workers
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
			log.Print("WARN: Failed to fetch work: ", err)
			return
		}

		if lastFetchedWork != nil {
			if lastFetchedWork.Number > work.Number {
				w.log("WARN", "skip work with lower height, height: %d", work.Number)
				return
			}
			if lastFetchedWork.HeaderHash == work.HeaderHash {
				w.log("INFO", "skip same work, height: %d", work.Number)
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
			if currWork != nil && work.Number >= currWork.Number {
				abortWorkCh <- struct{}{}
			}

			// Start finding the nonce
			if err := w.pow.FindNonce(work, resultsCh, abortWorkCh); err != nil {
				panic(err) // TODO: Send back err in an error channel
			}
			currWork = &work
			w.log("INFO", "started new work, height: %d", work.Number)

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

func getJRPCCli() *rpc.Client {
	if jrpcCli == nil {
		var err error
		url := fmt.Sprintf("http://%s:%d", *host, *port)
		jrpcCli, err = rpc.Dial(url)
		if err != nil {
			log.Fatal("ERROR: failed to get JRPC client: ", err)
		}
	}
	return jrpcCli
}

func shardRepr(optShardID *uint32) string {
	if optShardID == nil {
		return "R"
	}
	return strconv.FormatUint(uint64(*optShardID), 10)
}

func fetchWorkRPC(shardID *uint32) (work consensus.MiningWork, err error) {
	cli := getJRPCCli()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*rpcTimeout)*time.Second)
	defer cancel()

	var shardIDArg interface{}
	if shardID != nil {
		shardIDArg = "0x" + strconv.FormatUint(uint64(*shardID), 16)
	}
	ret := make([]string, 3)
	err = cli.CallContext(
		ctx,
		&ret,
		"getWork",
		shardIDArg,
	)
	if err != nil {
		return work, err
	}

	headerHash := common.HexToHash(ret[0])
	height := new(big.Int).SetBytes(common.FromHex(ret[1])).Uint64()
	diff := new(big.Int).SetBytes(common.FromHex(ret[2]))
	return consensus.MiningWork{
		HeaderHash: headerHash,
		Number:     height,
		Difficulty: diff,
	}, nil
}

func submitWorkRPC(shardID *uint32, work consensus.MiningWork, res consensus.MiningResult) error {
	cli := getJRPCCli()
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*rpcTimeout)*time.Second)
	defer cancel()

	var shardIDArg interface{}
	if shardID != nil {
		shardIDArg = "0x" + strconv.FormatUint(uint64(*shardID), 16)
	}
	var success bool
	err := cli.CallContext(
		ctx,
		&success,
		"submitWork",
		shardIDArg,
		work.HeaderHash.Hex(),
		"0x"+strconv.FormatUint(res.Nonce, 16),
		res.Digest.Hex(),
	)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("submit work failed")
	}
	return nil
}

func main() {
	flag.Parse()

	lvl, err := ethlog.LvlFromString(*gethlogLvl)
	if err != nil {
		log.Fatal("ERROR: invalid geth log level: ", err)
	}
	ethlog.Root().SetHandler(ethlog.LvlFilterHandler(lvl, ethlog.StdoutHandler))

	if *shardList == "" {
		log.Fatal("ERROR: empty shard list")
	}
	shards := []uint32{}
	for _, shardStr := range strings.Split(*shardList, ",") {
		s, err := strconv.Atoi(shardStr)
		if err != nil {
			log.Fatal("ERROR: invalid shard ID")
		}
		shards = append(shards, uint32(s))
	}

	var workers []worker
	var infoSummary []string
	// Init global channels and workers
	submitWorkCh := make(chan result, len(shards))
	abortCh := make(chan struct{})
	// TODO: support specifying root chain when necessary
	for _, shardID := range shards {
		shardID := shardID
		pow, ok := shardToPoW[shardID]
		if !ok {
			log.Fatal("ERROR: unsupported shard / mining algorithm")
		}
		w := worker{
			shardID:      &shardID,
			pow:          pow,
			fetchWorkCh:  make(chan consensus.MiningWork),
			submitWorkCh: submitWorkCh,
			abortCh:      abortCh,
		}
		workers = append(workers, w)
		infoSummary = append(infoSummary, fmt.Sprintf("[%s] %s", shardRepr(w.shardID), pow.Name()))
	}

	// Information summary
	fmt.Printf("QuarkChain Mining\n\tShards:\t%s\n", strings.Join(infoSummary, ", "))
	fmt.Printf("\tHost:\t%s\n\tPort:\t%d\n", *host, *port)
	fmt.Printf("\tGeth Log Level:\t%s\n\tRPC Timeout:\t%d sec\n\n", *gethlogLvl, *rpcTimeout)

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
			if err := submitWorkRPC(w.shardID, mWork, mRes); err != nil {
				w.log("WARN", "failed to submit work: %v\n", err)
			} else {
				w.log("INFO", "submitted work, height: %d\n", mWork.Number)
			}
		}
	}
}
