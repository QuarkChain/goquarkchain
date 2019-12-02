# GoQuarkChain

[![CircleCI](https://circleci.com/gh/QuarkChain/goquarkchain/tree/master.svg?style=shield&circle-token=afd6d8dfa04abf5da21613deb2572c330e4a6d49)](https://circleci.com/gh/QuarkChain/goquarkchain/tree/master)

Go implementation of [QuarkChain](https://quarkchain.io).

QuarkChain is a sharded blockchain protocol that employs a two-layer architecture - one extensible sharding layer consisting of multiple shard chains processing transactions and one root chain layer securing the network and coordinating cross-shard transactions among shard chains.

## Features

- Cluster implementation allowing multiple processes / physical machines to work together as a single full node
- State sharding dividing global state onto independent processing and storage units allowing the network capacity to scale linearly by adding more shards
- Cross-shard transaction allowing native token transfers among shard chains
- Adding shards dynamically to the network
- Support of different mining algorithms on different shards
- P2P network allowing clusters to join and leave anytime with encrypted transport
- Fully compatible with Ethereum smart contract

## Design

Check out the [Wiki](https://github.com/QuarkChain/pyquarkchain/wiki) to understand the design of QuarkChain.

## Development Setup

The following instructions are based on clean Ubuntu 18.04. For other operating systems, please refer to [this FAQ](#q-is-centos-supported).
If you prefer to use Docker, you can [run a cluster inside Docker container](#run-a-cluster-inside-docker).

###  Setup Go Environment
Goquarkchain requires golang sdk >= 1.12. You can skip this step if your environment meets the condition.
```bash
# take go1.12.10 for example:
wget https://studygolang.com/dl/golang/go1.12.10.linux-amd64.tar.gz
sudo tar xvzf go1.12.10.linux-amd64.tar.gz -C /usr/local
```
Append the following environment variables to ~/.profile. NOTE goproxy and go.mod are used.
```bash
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOPATH/bin:$GOROOT/bin:$PATH
export GOPROXY=https://goproxy.io
export GO111MODULE=on
```
Apply the changes immediately
```bash
source ~/.profile
```
Check Go installation
```bash
go version #go version go1.12.10 linux/amd64
```
### Setup RocksDB Environment
Before install RocksDB, you'll need to install the following dependency packages.
```bash
sudo apt-get update
sudo apt-get install -y git build-essential make g++ swig libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev liblz4-dev libzstd-dev
```
Install RocksDB. Version v6.1.2 is recommended.
```bash
git clone -b v6.1.2 https://github.com/facebook/rocksdb.git
cd rocksdb
sudo make shared_lib
sudo make install-shared
```
Append the following environment variables to ~/.profile
```bash
export CGO_CFLAGS=-I/usr/local/include
export CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy"
export LD_LIBRARY_PATH=/usr/local/lib
```
Apply the changes immediately
```bash
source ~/.profile
```
### Setup GoQuarkChain

```bash
mkdir -p $GOPATH/src/github.com/QuarkChain && cd $_
git clone https://github.com/QuarkChain/goquarkchain.git
#build qkchash
cd goquarkchain/consensus/qkchash/native
sudo g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++17 && make
```
Run all the unit tests under `goquarkchain` to verify the environment is correctly set up:

```
cd $GOPATH/src/github.com/QuarkChain/goquarkchain
go test ./...
```

## Running Clusters

The following instructions will lead you to run clusters step by step.

If you need to join mainnet and mine quickly, we recommend following 
[this instruction](docker/README.md#quick-start---use-docker-image-to-start-a-cluster-and-mining) 
to start a cluster using docker.

If you need to deploy clusters onto many hosts, a better option would be [Use Deploy Tool to Start Clusters](/tests/loadtest/deployer/README.md#use-deploy-tool-to-start-goquarkchain-clusters), which 
is based on pre-built Docker image.

### Build Cluster

Build GoQuarkChain cluster executable:
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
go build
```

### Running a single cluster for local testing

To run a local cluster which does not connect to anyone else, start each slave in different terminals with its ID specified 
in SLAVE_LIST of the json config. 

The following example has 2 slaves with 1 shard for each:
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json --service S0
```
In anther terminal,
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json --service S1
```
Start master in a third terminal
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json
```

## Run a Cluster Inside Docker 

Using pre-built Docker image(quarkchaindocker/goquarkchain), you can run a cluster inside Docker container without setting up environment step by step.

Refer to [Docker docs](https://docs.docker.com/v17.09/engine/installation/) if Docker is not yet installed on your machine.

Run the following commands to pull and start a container:

```bash
# specify a version tag if needed; use 'latest' for latest code 
sudo docker pull quarkchaindocker/goquarkchain:<version tag>
sudo docker run -it quarkchaindocker/goquarkchain:<version tag>
```
Now you are inside Docker container and are ready to start cluster services with a sample cluster config:
```bash
root@<container ID>:/go/src/github.com/QuarkChain/goquarkchain/cmd/cluster#./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json  --service S0 >> s0.log 2>&1 &
root@<container ID>:/go/src/github.com/QuarkChain/goquarkchain/cmd/cluster#./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json  --service S1 >> s1.log 2>&1 &
root@<container ID>:/go/src/github.com/QuarkChain/goquarkchain/cmd/cluster#./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json  >> master.log 2>&1 &
```
Check logs to see if the cluster is running successfully.

Next you can try to start a simulate [mining](#mining).

NOTE if you need the services available outside of the Docker host, you can publish the related ports using `-p` flag when start Docker:

```bash
sudo docker run -it -p 38291:38291 -p 38391:38391 -p 38491:38491 -p 38291:38291/udp quarkchaindocker/goquarkchain
```
And config rpc listening to `0.0.0.0` when start `master` service:
```bash
./cluster --cluster_config $CLUSTER_CONFIG_FILE --json_rpc_host 0.0.0.0 --json_rpc_private_host 0.0.0.0
```

### Join QuarkChain Network

If you are joining a testnet or mainnet, make sure ports are open and accessible from outside world: this means if you are running on AWS, 
open the ports (default both UDP and TCP 38291) in security group; if you are running from a LAN (connecting to the internet through a router), 
you need to setup [port forwarding](https://github.com/QuarkChain/pyquarkchain/wiki/Private-Network-Setting%2C-Port-Forwarding)
 for UDP/TCP 38291. 

Before running, you'll need to set up the configuration of your network, which all nodes need to be aware of and agree upon. 
We provide [an example config JSON](mainnet/singularity/cluster_config_template.json) which you can use as value 
of `--cluster_config` flag to start cluster service and connect to mainnet:

```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json (--service $SLAVE_ID)
```

NOTE that many parameters in the config are part of the consensus, please be very cautious when changing them. For example, 
COINBASE_AMOUNT is one such parameter, changing it to another value effectively creates a fork in the network.

NOTE if you run into the issue "Too many open files", which may occur while your cluster sync to mainnet, 
try to increase the limitation for current terminal by executing the following command before start cluster:

```bash
ulimit -HSn 102400
```

### Running multiple clusters with P2P network on different machines

To run a private network, first start a bootstrap cluster, then start other clusters with bootnode URL to connect to it.

#### Start Bootstrap Cluster
First start each of the `slave` services as in [Running a single cluster for local testing](#running-a-single-cluster-for-local-testing):
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json --service $SLAVE_ID
```
Next, start the `master` service of bootstrap cluster, optionally providing a private key:
```bash
./cluster --cluster_config $CLUSTER_CONFIG_FILE --privkey $BOOTSTRAP_PRIV_KEY
```
You can copy the full boot node URL from the console output in format: `enode://$BOOTSTRAP_PUB_KEY@$BOOTSTRAP_IP:$BOOTSTRAP_DISCOVERY_PORT`. 

Here is an example:

`INFO [11-04|18:05:54.832] Started P2P networking  self=enode://011bd77918a523c2d983de2508270420faf6263403a7a7f6daf1212a810537e4d27787e8885d8c696c3445158a75cfe521cfccab9bc25ba5ac6f8aebf60106f1@127.0.0.1:38291`

NOTE if your clusters are cross-internet, you need to replace `$BOOTSTRAP_IP` part with PUBLIC ip address when used as `--bootnodes` flag for other clusters.

NOTE if private key is not provided, the boot node URL will change at each restart of the service.

NOTE the `PRIV_KEY` field of `P2P` section in cluster config file has same effect and can be overridden by `--privkey` flag.

#### Start Other Clusters
First start each of the `slave` services as in [Running a single cluster for local testing](#running-a-single-cluster-for-local-testing):
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json --service ${SLAVE_ID}
```
Next, start the `master` service, providing the boot node URL as `$BOOTSTRAP_ENODE`:
```bash
./cluster --cluster_config $CLUSTER_CONFIG_FILE --bootnodes $BOOTSTRAP_ENODE
```
NOTE the `BOOT_NODES` field of `P2P` section in cluster config file has same effect and can be overridden by `--bootnodes` flag.

## Mining

Run the following command to start mining, replacing 127.0.0.1 with the host IP where the master service is deployed if not execute locally:

```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[true],"id":0}' http://127.0.0.1:38491
```
If need to stop mining,
```bash
curl -X POST -H 'content-type: application/json' --data '{"jsonrpc":"2.0","method":"setMining","params":[false],"id":0}' http://127.0.0.1:38491
```

## Monitoring Clusters
Use the [stats tool](cmd/stats) in the repo to monitor the status of a cluster. It queries the given cluster through 
JSON RPC every 10 seconds and produces an entry. 

## JSON RPC
JSON RPCs are defined in [`rpc.proto`](cluster/rpc/rpc.proto). Note that there are two JSON RPC ports. By default they 
are 38491 for private RPCs and 38391 for public RPCs. Since you are running your own clusters you get access to both.

Public RPCs are documented in the [Developer Guide](https://developers.quarkchain.io/#json-rpc). You can use the client 
library [quarkchain-web3.js](https://github.com/QuarkChain/quarkchain-web3.js) to query account state, send transactions, 
deploy and call smart contracts. Here is [a simple example](https://gist.github.com/qcgg/1ab0352c5b2299270b5795648cca83d8) 
to deploy smart contract on QuarkChain using the client library.

## Loadtest
Run loadtest to your cluster and see how fast it processes large volume of transactions. Please refer to 
[Loadtest Instruction](tests/loadtest/README.md#loadtest-instruction) for detail.

## Issue
Please open issues on github to report bugs or make feature requests.

## Contribution
All the help from community is appreciated! If you are interested in working on features or fixing bugs, please open an issue first
to describe the task you are planning to do. For small fixes (a few lines of change) feel
free to open pull requests directly.

## FAQ
### Q: Is CentOS supported?

A: We will support as many platforms as we can in the future, but currently only Ubuntu is fully tested, so it is recommended that you use Docker.  
However for CentOS specifically, you can try the following steps:
 ```bash
 #install gcc:
         wget http://ftp.gnu.org/gnu/gcc/gcc-7.4.0/gcc-7.4.0.tar.gz
         tar -xvzf gcc-7.4.0.tar.gz
         cd gcc-7.4.0 && ./contrib/download_prerequisites
         mkdir -p build_gcc_4.8.1 && cd &_
         ../gcc-7.4.0/configure --enable-checking=release --enable-languages=c,c++ --disable-multilib && make && make install
 	
 #install rocksdb:
         sudo yum install -y git build-essential make g++ swig
         sudo yum install -y snappy snappy-devel zlib zlib-devel bzip2 bzip2-devel lz4-devel libasan
         git clone -b v6.1.2 https://github.com/facebook/rocksdb.git
         cd rocksdb
         sudo make shared_lib
         sudo make install-shared
 	
 #install goquarkchain:
         mkdir -p $GOPATH/src/github.com/QuarkChain && cd $_
         git clone https://github.com/QuarkChain/goquarkchain.git
         cd $GOPATH/src/github.com/QuarkChain/goquarkchain/consensus/qkchash/native/ && g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++11
         cd ../../../cmd/cluster && go build -v
``` 
### Q: Is macOS supported?

A: We will support as many platforms as we can in the future, but currently only Ubuntu is fully tested, so it is recommended that you use Docker.  
However for macOS specifically, you can try the following steps:

First,you should download xcode through the app store 
and then open  terminal  
 ```bash
 #install brew:
         /usr/bin/ruby -e “$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/master/install)”

 #install gcc rocksdb swig:
         brew install rocksdb gcc swig

 #install golang:
         brew install go@1.12
         vim ~/.bash_profile

 #add the following content to .bash_profile file:
         export GOPATH=/usr/local/Cellar/go/<go version>
         export GOBIN=$GOPATH/bin
         export PATH=$PATH:$GOBIN

 #save .bash_profile file:
         source ~/.bash_profile
         
 #install goquarkchain:
         mkdir -p $GOPATH/src/github.com/QuarkChain && cd $_
         git clone https://github.com/QuarkChain/goquarkchain.git
         cd $GOPATH/src/github.com/QuarkChain/goquarkchain/consensus/qkchash/native/ 
         sudo g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++17 && make
         cd ../../../cmd/cluster && go build -v
``` 

## Developer Community
Join our developer community on [Discourse](https://community.quarkchain.io/) and [Discord](http://discord.me/quarkchain).

## License
Unless explicitly mentioned in a folder or a file, all files are licensed under GNU Lesser General Public License defined in LICENSE file.
