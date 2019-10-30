# Use Deploy Tool to Start Clusters

Here we provide a deploy tool based on pre-built Docker image. With this tool you can build and start a private QuarkChain 
network in one line command. 

NOTE with this tool at most one Slave service can be deployed per host, but there is no limitation for Shard number.

It is encouraged that you build your own deploy scripts or tools, especially if you prefer different service distribution 
among hosts.  You can also build your own Docker image, starting from [this Dockerfile](../Dockerfile), or if you are 
interested in build everything without Docker, start from [here](../../../README.md#development-setup). 

NOTE it is better to run deployer in the same LAN with the hosts you plan to deploy a cluster, because some file copy work 
will be done across network during the deploy process. 

## Run Docker Image

Usually you'll need a GoQuarkChain development environment to run the deploy tool, but the pre-built Docker image 
saved the effort for you. If you choose not to use Docker to run deployer, skip this step.

Run the following commands to pull and start a container with `deployer` in it:

```bash
# replace docker image name if a custom image is used
docker pull quarkchaindocker/goquarkchain
docker run -it quarkchaindocker/goquarkchain /bin/bash 
```
Then you can change configuration inside Docker container.

## Configure Clusters
With the configuration file `deployConfig.json`, you can configure multiple clusters that connected to each other. 

To change configuration for the cluster:
```bash
# inside container if use Docker to deploy
vi $GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer/deployConfig.json
```
Parameters explained:
- `Hosts` a list of hosts run same cluster
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

NOTE For each of the hosts, besides 38291, 38391, 38491, the port range [48000, 48000 + host number] should be opened too.

## Deploy and Run a Cluster

The following command will generate network configuration file used by the cluster, deploy the clusters to remote Docker 
containers, and start the services of each cluster:

```bash
# inside container if use Docker to deploy
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer
go run deploy_cluster.go
```
The deploying process will be printed on the console log. 

## Check Cluster Status

To check the status of the cluster, you need to enter the Docker container on the target hosts: 
```bash
docker exec -it bjqkc /bin/bash
```
If everything goes correctly, you will see from `$GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster/master.log` that 
cluster start successfully, and from shard logs such as `S0.log` in the same folder that 12,000 accounts loaded automatically for each shard.

Try the following command to see if mining works:
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
## Back to Loadtest

Now that you have running clusters, you can continue with loadtest from [here](../README.md#generate-transactions).