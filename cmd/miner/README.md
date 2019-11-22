# External CPU Miner Commandline Tool

How to run (from source):

```bash
# Build qkchash-related interfaces after checking out the repo
$ pwd  #-> root directory at `goquarkchain`
$ cd consensus/qkchash/native && make && cd -
$ cd cmd/miner
$ go run main.go -config ../../mainnet/singularity/cluster_config_template.json -shards 1,393217 -host <ip>
# Or build the binary
$ go build -o goqkcminer
$ ./goqkcminer -config ../../mainnet/singularity/cluster_config_template.json -shards 1,393217 -host <ip>
```

Commandline options:

```text
$ goqkcminer -h
Usage of goqkcminer:
  -coinbase string
        coinbase for miner
  -config string
        cluster config file
  -gethloglvl string
        log level of geth (default "info")
  -host string
        remote host of a quarkchain cluster (default "localhost")
  -port int
        remote JSONRPC port of a quarkchain cluster (default 38391)
  -shards string
        comma-separated string indicating shards (default "R")
  -threads int
        Use how many threads to mine in a worker
  -timeout int
        timeout in seconds for RPC calls (default 10)
```

Misc:

1. `ethash` is not supported, due to:
    1. Need to adapt the consensus engine interface from go-ethereum to our own consensus module, because in go-ethereum CPU mining is tightly coupled with the block format while we modified a lot of it. For double-SHA256 and qkchash mining it's solved by only using the header hash, difficulty and block height parameters. Check [`FindNonce`](https://github.com/QuarkChain/goquarkchain/blob/e44e64f8b482b893c797d84e63fd70eb05f0c837/consensus/consensus.go#L72) method for more details
    2. Most people are running GPU for mining ethash right now, so supporting CPU mining doesn't really help
