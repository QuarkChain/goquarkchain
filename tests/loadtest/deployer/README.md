# Use Deploy Tool to Start GoQuarkChain Clusters

Here we provide a deploy tool based on pre-built Docker image(`quarkchaindocker/goquarkchain`). With this tool you can deploy multiple clusters to build 
and start a private QuarkChain network in one line command. 

You can also build your own Docker image, starting from [this Dockerfile](../Dockerfile), or if you are 
interested in build everything without Docker, start from [here](../../../README.md#development-setup). 

NOTE it is recommended to run deployer in the same LAN with the hosts you plan to deploy a cluster, because some file copy work 
will be done across network during the deploy process. 

## System Requirements

To use `deployer` to run Docker image, it is required for the hosts that:

   - Ubuntu 18.04, 
   - root account is enabled, 
   - Docker version >= 18.09.7, and
   - 38291, 38391, 38491, [48000, 48000 + host number] ports opened.

## Run Docker Image

Usually you'll need a GoQuarkChain development environment to run the deploy tool, but the pre-built Docker image 
saved the effort for you. If you choose not to use Docker to run deployer, skip this step.

Run the following commands to pull and start a container with `deployer` in it:

```bash
# replace docker image name if a custom image is used
sudo docker pull quarkchaindocker/goquarkchain
sudo docker run -it quarkchaindocker/goquarkchain /bin/bash 
```
Then you can change configuration inside Docker container.

## Configure Clusters
With the configuration file `deployConfig.json`, you can configure multiple clusters that connected to each other. 

If you use Docker to deploy, you can use vi in the container:
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
- `Service` defines type of service(s) you want to run in the host, can be "master", "slave", or "master,slave"; make sure 
each cluster contains exact one master and at least one slave service; make sure the number of slaves in each cluster is a power of 2.
- `ClusterID` used to specify which cluster the service(s) on the host belongs to; so hosts with same ClusterID belongs 
to same cluster; ClusterID must be consecutive integers start from 0; if ClusterID is set to 0, the cluster will be 
started as a bootstrap node
- `CHAIN_SIZE` defines the number of chains in each cluster, where each chain has a number of shards; CHAIN_SIZE must be bigger or equal to the number of slaves.
- `SHARD_SIZE` defines the number of shards of each chain (must be a power of 2)
- `TargetRootBlockTime` defines the target block interval of root chain in seconds, since "POW_SIMULATE" is used for consensus
- `TargetMinorBlockTime` defines the target block interval of each shard
- `GasLimit` defines the gas limit for a block; note that in-shard transactions uses 50% of the total gas limit in a block

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
 
Try the following command to see if mining works:
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
## Back to Loadtest

Now that you have running clusters, you can continue with loadtest from [here](../README.md#generate-transactions).