#Use Deploy Tool to Start Clusters

Here we provide a deploy tool based on pre-built Docker image. With this tool you can deploy GoQuarkChain master/shard 
services to build and start a cluster in one line command. 
It is encouraged that you build your own deploy scripts or tools, especially if you prefer different service distribution among hosts,   
You can also build your own Docker image, starting from [this Dockerfile](./Dockerfile), or if you are interested in build everything without
Docker, starting from [here](../../README.md#development-setup). 

## Run Docker Image

You'll need to [setup development environment](../../README.md#development-setup) to run the deploy tool. So a convenient 
way would be pull a pre-built Docker image of GoQuarkChain and run the tool inside a container. And it is better to run it
in the same LAN with the hosts you plan to deploy a cluster, because some file copy work will be done across network 
during the deploy process. 
```bash
$ docker run -it quarkchaindocker/goquarkchain:<version tag> /bin/bash 
```
Once you get inside the Docker container, you can change the cluster configuration in it.

## Configure Clusters

You can build and deploy one cluster each time using this deploy tool. You need to modify 
`/qkc/go/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer/deployConfig.json` 
to configure the cluster to run in your environment. 

Parameters explained:
- `Hosts` a list of hosts run same cluster/node
- `IP` host IP
- `Port` SSH port
- `User` login name
- `Password` password
- `Service` which service(s) you want to run in the host, can be "master", "slave", or "master,slave"
- `BootNode` bootnode URL to discover and connect to other clusters, refer to [Running Multiple Clusters and Boot Node](#running-multiple-clusters-and-bootnode) for detail
- `ChainNumber` defines the number of chains in the cluster, each chain has a number of shards 
- `ShardNumber` defines the number of shards in the cluster (must be power of 2, and an integral number of ChainNumber)
- `TargetRootBlockTime` defines the target block interval in seconds of root chain
- `TargetMinorBlockTime` defines the target block interval on each shard
- `GasLimit` defines the gas limit for a block; note that in-shard transactions uses 50% of the total gas limit in a block

## Deploy and Run a Cluster
Inside the container
```bash
cd /qkc/go/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer
go run deploy_cluster.go
```

The deploying process will be printed on the console log. If cluster start successfully, you can start mining using the following command:
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```

## Running Multiple Clusters and Boot Node
With different deployConfig.json you can deploy multiple clusters of one network with this tool. 

Leave `BootNode` field empty when you deploy the first cluster/node in the network, and you'll find 
"enode://...:38291" in console log. Use this URL for `BootNode` value in the configuration file to build other clusters in the same network.