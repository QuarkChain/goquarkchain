# Loadtest Instruction

## Environment Setup

In order to run loadtest, you need to get your own GoQuarkChain clusters up and running.

If you are interested in building everything from scratch, please refer to the [instruction](../../README.md#development-setup) 
to set up development environment for each of your hosts, then [run clusters](../../README.md#running-clusters) on them.

To save your time and effort, an automatic deploy tool has been provided for you to deploy clusters to remote hosts 
based on Docker image for your convenience. Please refer to [Use Deploy Tool to Start Clusters](./deployer/README.md#use-deploy-tool-to-start-clusters) for detail.

## Loadtest Configuration

If you choose to use the deploy tool, the cluster configuration file will be created automatically based on [`deployConfig.json`](deployer/deployConfig.json). 
If you run your cluster from scratch, you need to configure the cluster manually. 
A cluster [config template file](cluster_config.json) has been provided for loadtest specifically.

Some parameters you may consider make changes:
- `CHAIN_SIZE` defines the number of chains in the cluster, each chain has a number of shards 
- `SHARD_SIZE` defines the number of shards per chain
- `TARGET_BLOCK_TIME` defines the target block interval in seconds for root chain and each shard
- `GAS_LIMIT` defines the gas limit for a block; note that in-shard transactions uses 50% of the total gas limit in a block
- `SLAVE_LIST` defines the slaves so that the services can be located

## Generate Transactions

Request the cluster through `createTransactions` JSON RPC to generate transactions on each shard.

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc": "2.0","method": "createTransactions","params": [{ "numTxPerShard": 10000,"xShardPercent": 0}],"id": 1}' http://127.0.0.1:38491
```
NOTE if xShardPercent > 0, make sure to mine at least one root block before send transactions, because the network should 
have at least one root block been mined before cross shard transaction can be handled, according to the default config.

You can start mining once `createTransactions` returns. It may take a few minutes if you create a considerable amount of transactions like 100,000. 

## Start Mining

To start mining, run the following command:
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
To stop mining,
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[false],"id":0}' http://127.0.0.1:38491
```
  
## Monitoring

Now you can [monitor](../../README.md#monitoring-clusters) the TPS using the [stats tool](../../cmd/stats).
