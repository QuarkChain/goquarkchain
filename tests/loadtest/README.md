# Loadtest Instruction

## Environment Setup

To run loadtest, you need to get your own GoQuarkChain clusters up and running.

### Automatic Tool
 
[A handy tool](../../tools/README.md) has been provided for you to deploy clusters to remote hosts based on Docker image for your convenience.

If you are interested in building everything from scratch, please refer to the [instruction](../../README.md#development-setup) to set up development environment for each for your hosts, then [run clusters](../../README.md#running-clusters) on them.


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

Request the cluster through `createTransactions ` JSON RPC to generate transactions on each shard. NOTE the parameters are encoded in Hex.

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc": "2.0","method": "createTransactions","params": [{ "numTxPerShard": "0x186e0","xShardPercent": "0x0"}],"id": 1}' http://127.0.0.1:38491
```
   
## Monitoring

Now you can [monitor](../../README.md#monitoring-clusters) the TPS using the [stats tool](../../cmd/stats).
