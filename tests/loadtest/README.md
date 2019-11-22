# Loadtest Instruction

## Environment Setup

In order to run loadtest, you need to [run your own GoQuarkChain clusters](../../README.md#running-multiple-clusters-with-p2p-network-on-different-machines).

A convenient option is to [Use Deploy Tool to Start Clusters](./deployer/README.md#use-deploy-tool-to-start-goquarkchain-clusters).

## Cluster Configuration

Key parameters in cluster config json file for loadtest:

- ROOT/CONSENSUS_CONFIG/TARGET_BLOCK_TIME: root block interval
- CHAINS/CONSENSUS_CONFIG/TARGET_BLOCK_TIME: minor block interval
- CHAINS/GENESIS/GAS_LIMIT: minor block gas limit
- CHAINS/SHARD_SIZE: number of shards per chain
- GENESIS_DIR: location of account data; should be "../../tests/loadtest/accounts" if you start cluster under goquarkchain/cmd/cluster

## Start Mining

Your clusters need to keep mining while loadtest is ongoing. 

Run the following command to start mining, replacing 127.0.0.1 with the host IP where the master service is deployed if not execute locally:

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
If need to stop mining,
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[false],"id":0}' http://127.0.0.1:38491
```
## Generate Transactions

Trigger loadtest through `createTransactions` which requests the cluster to generate transactions on each shard. 
Remember to replace 127.0.0.1 with the host IP where the master service is deployed if not execute locally:

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc": "2.0","method": "createTransactions","params": [{ "numTxPerShard": 10000,"xShardPercent": 0}],"id": 1}' http://127.0.0.1:38491
```
NOTE if xShardPercent > 0, make sure to mine at least one root block before send transactions, because the network should 
have at least one root block been mined before cross shard transaction can be handled, according to the default config.

## Monitoring

Now you can monitor the TPS using the [stats tool](../../cmd/stats).
