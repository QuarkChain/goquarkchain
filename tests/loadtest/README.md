# Loadtest Instruction

## Development Setup

First of all, follow the [instruction](../../README.md#development-setup) to set up development environment for goquarkchain.
 
## Running a Cluster

Start a local cluster follow the [instruction](../../README.md#running-a-single-cluster-for-local-testing).
```bash
cd cmd/cluser
./cluster --num_shards 1024 --num_slaves=128
```
Some interesting command line flags regarding loadtest:

- `--num_shards` (default 8) defines the number of shards in the cluster (must be power of 2)
- `--num_slaves` (default 4) defines the number of slave servers in the cluster. Each slave server can serve one or more shards. Since each slave server is an independent process, you may want to make this equal to `--num_shards` to utilize as many CPU cores as possible.
- `--root_block_interval_sec` (default 10) defines the target block interval of root chain
- `--minor_block_interval_sec` (default 3) defines the target block interval on each shard
- `--mine` enables mining as soon as the cluster starts. Mining can also be toggled at runtime through `setMining` JSON RPC.
- `--clean` clears any existing data to start a fresh cluster from genesis

## Start Mining
Before loadtest, try to mine a few blocks to make sure the clusters work correctly.

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
To stop mining,
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[false],"id":0}' http://127.0.0.1:38491
```

## Prepare Account Data
Put account data into default location to load.
```bash
cd cmd
mkdir genesis_data
cp ../../tests/testdata/genesis_data/loadtest.json genesis_data
```
12,000 accounts will be loaded into genesis alloc config for each shard.

## Generate Transactions

Request the cluster through `createTransactions ` JSON RPC to generate transactions on each shard. NOTE the parameters are encoded in Hex.

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc": "2.0","method": "createTransactions","params": [{ "numTxPerShard": "0x186e0","xShardPercent": "0x0"}],"id": 1}' http://127.0.0.1:38491
```
   
## Monitoring

At your virtual environment, [monitor](../../README.md#monitoring-clusters) the TPS using the stats tool.
