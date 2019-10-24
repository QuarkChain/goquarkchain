#Quick Start - Use Docker Image to Start Clusters

Here we provide detailed instructions for running clusters and starting the mining process using a pre-built docker image.

## Configure Clusters

You can build and deploy one cluster or node each time using this deploy tool. You need to modify deployConfig.json for the cluster to run in your environment. 

- `Hosts` a list of hosts run same cluster/node
- `IP` host IP
- `Port` SSH port
- `User` login name
- `Password` password
- `Service` which service(s) you want to run in the host, can be "master", "slave", or "master,slave"
- `BootNode` bootnode URL to discover and connect to other clusters. Leave blank for the first node in the network, you'll find "enode://...:38291" in console log which need to be set for other nodes.
- `ChainNumber` defines the number of chains in the cluster, each chain has a number of shards 
- `ShardNumber` defines the number of shards in the cluster (must be power of 2, and an integral number of ChainNumber)
- `TargetRootBlockTime` defines the target block interval in seconds of root chain
- `TargetMinorBlockTime` defines the target block interval on each shard
- `GasLimit` defines the gas limit for a block
