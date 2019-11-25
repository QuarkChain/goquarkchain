# Use Deploy Tool to Start GoQuarkChain Clusters

Here we provide a deploy tool based on pre-built Docker image(`quarkchaindocker/goquarkchain`). With this tool you can deploy multiple clusters to build 
and start a private QuarkChain network in one line command. 

Here is a quick [demo video](https://www.youtube.com/watch?v=0_aME3vUILQ).

You can also build your own Docker image, starting from [this Dockerfile](../../../docker/Dockerfile), or if you are 
interested in build everything without Docker, start from [here](../../../README.md#development-setup). 

NOTE it is recommended to run deployer in the same LAN with the hosts you plan to deploy a cluster, because some file copy work 
will be done across network during the deploy process. 

## System Requirements

To use `deployer` to run Docker image, it is required for the hosts that:

   - Ubuntu 18.04, 
   - ssh server installed and running,
   - root account is enabled, 
   - Docker version >= 18.09.7, and
   - 38291, 38391, 38491, [48000, 48000 + host number] ports opened.

## Run Docker Image

Usually you'll need a GoQuarkChain development environment to run the deploy tool, but the pre-built Docker image 
saved the effort for you. If you choose not to use Docker to run deployer, skip this step.

Run the following commands to pull the Docker image and start a container:

```bash
# replace docker image name if a custom image is used
# specify a version tag if needed; use 'latest' for latest code 
sudo docker pull quarkchaindocker/goquarkchain:<version tag>
sudo docker run -it quarkchaindocker/goquarkchain:<version tag> /bin/bash 
```
Then you will be inside a Docker container with `deployer` in it.

## Update Code (Optional)
The code and cluster executable inside the container are ready to run. 
If you would like to make any code changes to all the clusters that will run later, you can do it here. 
Just remember to build cluster after your code updates:
```bash
#inside Docker container
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
go build
```

## Configure Clusters
With the configuration file `deployConfig.json`, you can configure multiple clusters that connected to each other. 

If you use Docker to deploy, you can use vi in the container(for other editors you need to install before use) :
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer
vi deployConfig.json
```
Parameters explained:
- `Hosts` defines a list of hosts where GoQuarkChain services run inside Docker containers
- `IP` host IP
- `Port` SSH port
- `User` login name; currently only `root` is supported
- `Password` password
- `IsMaster` bool value specify if the host runs a master service; make sure each cluster contains exact one master 
- `SlaveNumber` defines the number of slave services on the host; make sure the number of slaves in each cluster is a power of 2 (at least 1).
- `ClusterID` used to specify which cluster the service(s) on the host belongs to; so hosts with same ClusterID belongs 
to same cluster; ClusterID must be consecutive integers start from 0; if ClusterID is set to 0, the cluster will be 
started as a bootstrap node
- `CHAIN_SIZE` defines the number of chains in each cluster, where each chain has a number of shards; CHAIN_SIZE must be bigger or equal to the number of slaves.
- `SHARD_SIZE` defines the number of shards of each chain (must be a power of 2)
- `TargetRootBlockTime` refers to ROOT/CONSENSUS_CONFIG/TARGET_BLOCK_TIME in cluster config that defines the target block interval of root chain in seconds, since "POW_SIMULATE" is used for consensus
- `TargetMinorBlockTime` refers to CHAINS/CONSENSUS_CONFIG/TARGET_BLOCK_TIME in cluster config that defines the target block interval of each shard in seconds
- `GasLimit` refers to CHAINS/GENESIS/GAS_LIMIT in cluster config that defines the gas limit for a block; note that in-shard transactions uses 50% of the total gas limit in a block

[This sample config in the repo](./deployConfig-sample.json) illustrates how 3 clusters running 256 shards
(64 chains * 4 shards per chain) can be deployed to 17 hosts.

In this example, cluster 0, 1, 2 are deployed on 9, 4, 4 hosts respectively. 
Cluster 0 runs its master service alone in one of its 9 hosts, and 64 slave services on another 8 hosts with 8 slaves each.
Cluster 1 runs its master service with 8 slaves in one of its 4 hosts, and other 24 slave services on another 3 hosts with 8 slaves each.
Cluster 2 has the same structure as cluster 1.

So, cluster 0, 1, 2 have 64, 32, 32 slaves deployed respectively. Notice the slave number of each cluster is a power of 2. 

## Deploy and Run Clusters

The following command will parse `deployConfig.json`, generate cluster configuration file accordingly, deploy the clusters to remote Docker 
containers, and start the services of each cluster:

```bash
# suppose your working directory is "$GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer"
go run deploy_cluster.go
```
The deploying process will be printed on the console log. 

NOTE pulling docker image may take a while on the first run.

## Check Status of Clusters

If everything goes correctly, you will see from `deployer` console log that each cluster started successfully and peers connected to each other.

You can also monitor the status of a cluster with the [stats tool](../../../cmd/stats).

For detailed information, you need to enter the Docker container on the target hosts and check logs: 
```bash
docker exec -it bjqkc /bin/bash
```
You can find master.log and shard logs such as `S0.log` from `$GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster`.
 
## Back to Loadtest

Now that you have running clusters, you can continue with loadtest from [here](../README.md#start-mining).

## FAQ

### Console hangs when running the deployer?
It takes some time to pull the Docker image from Docker hub to the hosts for the first time. 
You may consider to do the pulling directly on the remote hosts beforehand where you can see the downloading process.

## I'd like to deploy the clusters myself, but can you help me with the cluster configurations?
Sure. Describe you clusters in `deployConfig.json`, and run the deployer with flag `--genconf`:

```bash
# suppose your working directory is "$GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer"
go run deploy_cluster.go --genconf
```
And you will get cluster configuration files named cluster_config_${ClusterID}.json, with ${ClusterID} replaced by real ClusterIDs in `deployConfig.json`.
Also, the `PRIV_KEY` is specified for cluster_config_0.json , and the corresponding `BOOT_NODES` is set to others.