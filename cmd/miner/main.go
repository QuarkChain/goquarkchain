package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus/ethash"
	"io/ioutil"
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
	jrpcCli *rpc.Client

	// Flags
	clusterConfig = flag.String("config", "", "cluster config file")
	shardList     = flag.String("shards", "R", "comma-separated string indicating shards")
	host          = flag.String("host", "localhost", "remote host of a quarkchain cluster")
	port          = flag.Int("port", 38391, "remote JSONRPC port of a quarkchain cluster")
	preThreads    = flag.Int("threads", 0, "Use how many threads to mine in a worker")
	rpcTimeout    = flag.Int("timeout", 500, "timeout in seconds for RPC calls")
	gethlogLvl    = flag.String("gethloglvl", "info", "log level of geth")
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
				w.log("INFO", "skip same work, height:\t %d", work.Number)
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
			w.log("INFO", "started new work, height:\t %d", work.Number)

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
		"qkc_getWork",
		shardIDArg,
	)
	if err != nil {
		return work, err
	}

	headerHash := common.HexToHash(ret[0])
	if headerHash == (common.Hash{}) {
		return work, errors.New("Empty work can't be used ")
	}
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
		"qkc_submitWork",
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

func loadConfig(file string, cfg *config.ClusterConfig) error {
	var (
		content []byte
		err     error
	)
	if content, err = ioutil.ReadFile(file); err != nil {
		return errors.New(file + ", " + err.Error())
	}
	return json.Unmarshal(content, cfg)
}

func createMiner(consensusType string) (consensus.PoW, bool) {
	switch consensusType {
	case config.PoWEthash:
		return ethash.New(ethash.Config{CachesInMem: 3, CachesOnDisk: 10, CacheDir: "", PowMode: ethash.ModeNormal}, nil, false), true
	case config.PoWQkchash:
		return qkchash.New(true, nil, false), true
	case config.PoWDoubleSha256:
		return doublesha256.New(nil, false), true
	default:
		ethlog.Error("Failed to create consensus engine consensus type is not known", "consensus type", consensusType)
		return nil, false
	}
}

func main() {
	flag.Parse()

	lvl, err := ethlog.LvlFromString(*gethlogLvl)
	if err != nil {
		log.Fatal("ERROR: invalid geth log level: ", err)
	}
	ethlog.Root().SetHandler(ethlog.LvlFilterHandler(lvl, ethlog.StdoutHandler))

	var (
		cfg         config.ClusterConfig
		shardCfgs   = make(map[uint32]*config.ShardConfig)
		workers     []worker
		infoSummary []string
		// Init global channels and workers
		submitWorkCh = make(chan result, len(shardCfgs))
		abortCh      = make(chan struct{})
	)

	err = loadConfig(*clusterConfig, &cfg)
	if err != nil {
		log.Fatal("ERROR: invalid config path: ", err)
	}

	// Root chain miner, default
	if *shardList == "R" {
		pow, ok := createMiner(cfg.Quarkchain.Root.ConsensusType)
		if !ok {
			log.Fatal("ERROR: unsupported root / mining algorithm")
		}
		pow.SetThreads(*preThreads)
		w := worker{
			pow:          pow,
			fetchWorkCh:  make(chan consensus.MiningWork),
			submitWorkCh: submitWorkCh,
			abortCh:      abortCh,
		}
		workers = append(workers, w)
		infoSummary = append(infoSummary, fmt.Sprintf("[%s] %s", shardRepr(w.shardID), pow.Name()))
		*shardList = ""
	} else if *shardList != "" {
		for _, shardStr := range strings.Split(*shardList, ",") {
			s, err := strconv.Atoi(shardStr)
			if err != nil {
				log.Fatal("ERROR: invalid shard ID")
			}
			fullShardId := cfg.Quarkchain.GetFullShardIdByFullShardKey(uint32(s))
			shardCfg := cfg.Quarkchain.GetShardConfigByFullShardID(fullShardId)
			shardCfgs[uint32(s)] = shardCfg
		}
	}

	for shardID, shardCfg := range shardCfgs {
		pow, ok := createMiner(shardCfg.ConsensusType)
		if !ok {
			log.Fatal("ERROR: unsupported shard / mining algorithm")
		}
		pow.SetThreads(*preThreads)
		w := worker{
			shardID:      &shardID,
			pow:          pow,
			fetchWorkCh:  make(chan consensus.MiningWork),
			submitWorkCh: submitWorkCh,
			abortCh:      abortCh,
		}
		workers = append(workers, w)
		infoSummary = append(infoSummary, fmt.Sprintf("[%s] %s", shardRepr(w.shardID), pow.Name()))
		ethlog.Info("create shard worker", "shard id", shardID, "consensus type", shardCfg.ConsensusType)
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
				w.log("INFO", "submitted work, height:\t %d\n", mWork.Number)
			}
		}
	}
}
