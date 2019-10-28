# Use Deploy Tool to Start Clusters

Here we provide a deploy tool based on pre-built Docker image. With this tool you can deploy GoQuarkChain master/shard 
services to build and start a cluster in one line command. 

It is encouraged that you build your own deploy scripts or tools, especially if you prefer different service distribution 
among hosts.  You can also build your own Docker image, starting from [this Dockerfile](../Dockerfile), or if you are 
interested in build everything without Docker, starting from [here](../../../README.md#development-setup). 

## Run Docker Image

Usually you'll need a GoQuarkChain development environment to run the deploy tool, but the pre-built Docker image 
saved the effort for you. All you need to do is to run the following command:

```bash
# replace docker image name if a custom image is used
$ docker run  -itd quarkchaindocker/goquarkchain:<version tag> /bin/bash 
```
NOTE it is better to run it in the same LAN with the hosts you plan to deploy a cluster, because some file copy work 
will be done across network during the deploy process. 

Once you get inside the Docker container, you can change the cluster configuration in it.

## Configure Clusters

You can build and deploy one cluster each time using this deploy tool. You need to modify 
`$GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer/deployConfig.json` 
to configure the cluster to run in your environment. 

Parameters explained:
- `Hosts` a list of hosts run same cluster/node
- `IP` host IP
- `Port` SSH port
- `User` login name
- `Password` password
- `Service` which service(s) you want to run in the host, can be "master", "slave", or "master,slave"
- `BootNode` bootnode URL to discover and connect to other clusters, refer to [here](#running-multiple-clusters-and-boot-node) 
for detail
- `ChainNumber` defines the number of chains in the cluster, each chain has a number of shards 
- `ShardNumber` defines the number of shards in the cluster (must be power of 2, and an integral multiple of ChainNumber)
- `TargetRootBlockTime` defines the target block interval in seconds of root chain
- `TargetMinorBlockTime` defines the target block interval on each shard
- `GasLimit` defines the gas limit for a block; note that in-shard transactions uses 50% of the total gas limit in a block

## Deploy and Run a Cluster
Inside the container, execute:
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer
go run deploy_cluster.go
```

The deploying process will be printed on the console log. 
## Check Cluster Status

To check the status of the cluster, you need to enter the Docker container on the target hosts and check the service log:
```bash
# enter the container
docker exec  -it bjqkc /bin/bash
$ cat /tmp/QKC/S0.log
```
If everything goes correclty, you will see from the log that cluster start successfully, and 12,000 accounts loaded 
automatically for each shard.

Try the following command to see if mining works:
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
## Running Multiple Clusters and Boot Node
With different deployConfig.json you can deploy multiple clusters in same network with this tool. 

Leave `BootNode` field empty when you deploy the first cluster/node in the network, and you'll find 
"enode://...:38291" in console log. Use this URL for `BootNode` value in the configuration file to build other clusters 
in the same network.
