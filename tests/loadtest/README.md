# Loadtest Instruction

## Environment Setup

In order to run loadtest, you need to run your own GoQuarkChain clusters.

A convenient option is to [Use Deploy Tool to Start Clusters](./deployer/README.md#use-deploy-tool-to-start-goquarkchain-clusters).

## Start Mining

To start mining, run the following command:
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
To stop mining,
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[false],"id":0}' http://127.0.0.1:38491
```
## Generate Transactions

Trigger loadtest through `createTransactions` which requests the cluster to generate transactions on each shard.

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc": "2.0","method": "createTransactions","params": [{ "numTxPerShard": 10000,"xShardPercent": 0}],"id": 1}' http://127.0.0.1:38491
```
NOTE if xShardPercent > 0, make sure to mine at least one root block before send transactions, because the network should 
have at least one root block been mined before cross shard transaction can be handled, according to the default config.

## Monitoring

Now you can monitor the TPS using the [stats tool](../../cmd/stats).
