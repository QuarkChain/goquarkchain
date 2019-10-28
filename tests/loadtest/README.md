# Loadtest Instruction

## Environment Setup

In order to run loadtest, you need to get your own GoQuarkChain clusters up and running.

If you are interested in building everything from scratch, please refer to the [instruction](../../README.md#development-setup) to set up development environment for each for your hosts, then [run clusters](../../README.md#running-clusters) on them.

To save your time and effort, [an automatic tool](./deployer) has been provided for you to deploy clusters to remote hosts based on Docker image for your convenience. Please refer to [deployer guide](./deployer/README.md) for detail.

## Loadtest Configuration

The cluster configuration file will be created automatically by the deploy tool. If you run your cluster by hand, you 
need to configure the cluster manually. A cluster [config template file](cluster_config.json) has been provided for loadtest.

Parameters related to loadtest:
- `CHAIN_SIZE` defines the number of chains in the cluster, each chain has a number of shards 
- `TRANSACTION_QUEUE_SIZE_LIMIT_PER_SHARD` defines the maximum number of pending transactions the transaction pool can hold
- `SHARD_SIZE` defines the number of shards per chain
- `TARGET_BLOCK_TIME` defines the target block interval in seconds for root chain and each shard
- `GAS_LIMIT` defines the gas limit for a block; note that in-shard transactions uses 50% of the total gas limit in a block
- `SLAVE_LIST` defines the slaves so that the services can be located.

## Start Mining

Before loadtest, try to mine a few blocks to make sure the clusters work correctly.

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
To stop mining,
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[false],"id":0}' http://127.0.0.1:38491
```

## Generate Transactions

Request the cluster through `createTransactions` JSON RPC to generate transactions on each shard. NOTE the parameters are encoded in Hex.

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc": "2.0","method": "createTransactions","params": [{ "numTxPerShard": "0x186e0","xShardPercent": "0x0"}],"id": 1}' http://127.0.0.1:38491
```
   
## Monitoring

Now you can [monitor](../../README.md#monitoring-clusters) the TPS using the [stats tool](../../cmd/stats).
