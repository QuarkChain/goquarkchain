# Loadtest Instruction

## Environment Setup

To run loadtest, you need to get your own GoQuarkChain clusters up and running. 

If you are interested in building everything from scratch, please refer to the [instruction](../../README.md#development-setup) to set up development environment for each for your hosts, then [run clusters](../../README.md#running-clusters) on them.

### Manage Clusters using Docker

As another option, [a handy tool](../../tools/README.md) has been provided for you to deploy clusters to remote hosts based on Docker image, which is more automatic and convenience.

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

You can [monitor](../../README.md#monitoring-clusters) the TPS using the [stats tool](../../cmd/stats).
