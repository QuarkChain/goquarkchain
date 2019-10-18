# Loadtest Instruction

## Development Setup
First of all, follow the [instruction](../../README.md#development-setup) to set up development environment for goquarkchain.
 
## Running a Cluster
Before loadtest, try to start a local cluster successfully follow the [instruction](../../README.md#running-a-single-cluster-for-local-testing).

## Loadtest

1. Trigger loadtest through `createTransactions ` JSON RPC which requests the cluster to generate transactions on each shard. `numTxPerShard` <= 12000, `xShardPercent` <= 100

   ```bash
   curl -X POST --data '{"jsonrpc":"2.0","method":"createTransactions","params":{"numTxPerShard":10000, "xShardPercent":10},"id":0}' http://localhost:38491
   ```
2. At your virtual environment, [monitor](../../README.md#monitoring-clusters) the TPS using the stats tool.

## Code Pointers
**Loadtest Accounts**

 [12,000 loadtest accounts](../testdata/genesis_data/loadtest.json) are [loaded into genesis alloc config](../../cluster/config/config.go#L285) for each shard.

**JSON RPC**

JSON RPCs are defined in [`rpc.proto`](../../cluster/rpc/rpc.proto). Note that there are two JSON RPC ports. By default they are 38491 for private RPCs and 38391 for public RPCs. Since you are running your own clusters you get access to both.

**Command Line Flags**

Command line flags are defined in [`flags.go`](../../cmd/utils/flags.go#L77). Some interesting ones regarding loadtest:

- `--num_shards` (default 8) defines the number of shards in the cluster (must be power of 2)
- `--num_slaves` (default 4) defines the number of slave servers in the cluster. Each slave server can serve one or more shards. Since each slave server is an independent process, you may want to make this equal to `--num_shards` to utilize as many CPU cores as possible.
- `--root_block_interval_sec` (default 10) defines the target block interval of root chain
- `--minor_block_interval_sec` (default 3) defines the target block interval on each shard
- `--mine` enables mining as soon as the cluster starts. Mining can also be toggled at runtime through `setMining` JSON RPC.
- `--clean` clears any existing data to start a fresh cluster from genesis
