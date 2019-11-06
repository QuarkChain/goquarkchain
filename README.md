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

The following instructions are based on Ubuntu 18.04.

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

Another option would be [Use Deploy Tool to Start Clusters](/tests/loadtest/deployer/README.md#use-deploy-tool-to-start-clusters), which 
is based on pre-built Docker image.

### Build Cluster

Build GoQuarkChain cluster executable:
```bash
#build cluster
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
go build
```
### Join QuarkChain Network

If you are joining a testnet or mainnet, make sure ports are open and accessible from outside world: this means if you are running on AWS, 
open the ports (default both UDP and TCP 38291) in security group; if you are running from a LAN (connecting to the internet through a router), 
you need to setup [port forwarding](https://github.com/QuarkChain/pyquarkchain/wiki/Private-Network-Setting%2C-Port-Forwarding)
 for UDP/TCP 38291. 

Before running, you'll need to set up the configuration of your network, which all nodes need to be aware of and agree upon. 
We provide an example config JSON in the repo (mainnet/singularity/cluster_config_template.json) which you can use to connect to mainnet. 

Note that many parameters in the config are part of the consensus, please be very cautious when changing them. For example, 
COINBASE_AMOUNT is one such parameter, changing it to another value effectively creates a fork in the network.

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
Start master in another terminal
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json
```
### Running multiple clusters with P2P network on different machines

To run a private network, first start a bootstrap cluster, then start other clusters with bootnode URL to connect to it.

#### Start Bootstrap Cluster
First start each of the `slave` services as in [last section](#running-a-single-cluster-for-local-testing):
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json --service ${SLAVE_ID}
```
Next, start the `master` service of bootstrap cluster, optionally providing a private key:
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluser
./cluster --cluster_config $CLUSTER_CONFIG_FILE --privkey=$BOOTSTRAP_PRIV_KEY
```
You can read the full boot node URL from the console output in format: `enode://$BOOTSTRAP_PUB_KEY@$BOOTSTRAP_IP:$BOOTSTRAP_DISCOVERY_PORT`. 

Here is an example:

`INFO [11-04|18:05:54.832] Started P2P networking  self=enode://011bd77918a523c2d983de2508270420faf6263403a7a7f6daf1212a810537e4d27787e8885d8c696c3445158a75cfe521cfccab9bc25ba5ac6f8aebf60106f1@127.0.0.1:38291`

NOTE if private key is not provided, the boot node URL will change at each restart of the service.

NOTE using `PRIV_KEY` field of `P2P` section in cluster config file will have the same effect as `--privkey` flag.

#### Start Other Clusters
First start each of the `slave` services as in [last section](#running-a-single-cluster-for-local-testing):
```bash
cd $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../tests/testnet/egconfig/cluster_config_template.json --service ${SLAVE_ID}
```
Next, start the `master` service, providing the boot node URL as `$BOOTSTRAP_ENODE`:
```bash
./cluster --cluster_config $CLUSTER_CONFIG_FILE --bootnodes=$BOOTSTRAP_ENODE
```
NOTE using `BOOT_NODES` field of `P2P` section in cluster config file will have the same effect as `--bootnodes` flag.

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

## Developer Community
Join our developer community on [Discord](http://discord.me/quarkchain).

## License
Unless explicitly mentioned in a folder or a file, all files are licensed under GNU Lesser General Public License defined in LICENSE file.
