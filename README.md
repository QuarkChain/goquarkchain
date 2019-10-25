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
###  Setup Go Environment
Goquarkchain requires golang sdk >= 1.12. You can skip this step if your environment meets the condition.
```bash
# take go1.12.10 for example:
wget https://studygolang.com/dl/golang/go1.12.10.linux-amd64.tar.gz
sudo tar xzf go1.12.10.linux-amd64.tar.gz -C /usr/local
```
Create a folder as $GOPATH, for example ~/go. This is where your Go code goes. Skip this step if you've already done so.
```bash
mkdir ~/go
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
sudo apt-get install -y build-essential make g++ swig 
sudo apt-get install -y libgflags-dev libsnappy-dev zlib1g-dev libbz2-dev liblz4-dev libzstd-dev
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
Install GoQuarkChain 
```bash
cd $GOPATH
sudo mkdir -p src/github.com/QuarkChain
cd src/github.com/QuarkChain
sudo git clone https://github.com/QuarkChain/goquarkchain.git
#build qkchash
cd goquarkchain/consensus/qkchash/native
sudo g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++17
sudo make
```
Run all the unit tests under `goquarkchain`

```
cd $GOPATH/src/github.com/QuarkChain/goquarkchain
go test ./...
```
## Running Clusters
If you are on a private network (e.g. running from a laptop which connects to the Internet through a router), you need to first setup [port forwarding](https://github.com/QuarkChain/pyquarkchain/wiki/Private-Network-Setting%2C-Port-Forwarding) for UDP/TCP 38291.

Before start a local cluster, you need to set the ulimit values in /etc/security/limits.conf. Append the following lines to the file:
```bash
${USER} soft nofile ${NUMBER}
${USER} hard nofile ${NUMBER}
```
${USER} should be replaced with your user name, or * for all users; and ${NUMBER} should be replaced with the total shard number plus one. 

### Running a single cluster for local testing

Start running a local cluster which does not connect to anyone else. Build goquarkchain executable:
```bash
#
cd cmd/cluser
go build .
```
Start each slave in different terminals with its ID specified in SLAVE_LIST of the json config. Take the default configuration cluster as example, which has 4 slaves with 2 shards for each:
```bash
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S0
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S1
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S2
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S3
```
Start master in another terminal
```bash
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json
```

### Running multiple clusters with P2P network on different machines
NOTE this is effectively a private network. If you would like to join our testnet or mainnet, look back a few sections for instructions.

Just follow the same command to run single cluster and provide `--bootnodes` flag to discover and connect to other clusters. Make sure ports are open and accessible from outside world: this means if you are running on AWS, open the ports (default both UDP and TCP 38291) in security group; if you are running from a LAN (connecting to the internet through a router), you need to setup port forwarding for UDP/TCP 38291. We have a convenience UPNP module as well, but you will need to check if it has successfully set port forwarding.

(Optional) Not needed if you are joining a testnet or mainnet. If you are starting your own network, first start the bootstrap cluster:
```bash
cd cmd/cluser
./cluster --cluster_config $CLUSTER_CONFIG_FILE --p2p --privkey=$BOOTSTRAP_PRIV_KEY
```
You can read the full bootnode URL from the console output. Then start other clusters and provide the bootnode URL.
```bash
./cluster --cluster_config $CLUSTER_CONFIG_FILE --p2p --bootnodes=$BOOTSTRAP_ENODE
```

## Monitoring Clusters
Use the [`stats`](https://github.com/QuarkChain/pyquarkchain/blob/master/quarkchain/tools/#stats) tool in the repo to monitor the status of a cluster. It queries the given cluster through JSON RPC every 10 seconds and produces an entry. You may need to [setup python environment](https://github.com/QuarkChain/pyquarkchain#development-setup) to run the tool.
```bash
$ pyquarkchain/quarkchain/tools/stats --ip=localhost
----------------------------------------------------------------------------------------------------
                                      QuarkChain Cluster Stats
----------------------------------------------------------------------------------------------------
CPU:                8
Memory:             16 GB
IP:                 localhost
Shards:             8
Servers:            4
Shard Interval:     60
Root Interval:      10
Syncing:            False
Mining:             False
Peers:              127.0.0.1:38293, 127.0.0.1:38292
----------------------------------------------------------------------------------------------------
Timestamp                     TPS   Pending tx  Confirmed tx       BPS      SBPS      ROOT       CPU
----------------------------------------------------------------------------------------------------
2018-09-21 16:35:07          0.00            0             0      0.00      0.00        84     12.50
2018-09-21 16:35:17          0.00            0          9000      0.02      0.00        84      7.80
2018-09-21 16:35:27          0.00            0         18000      0.07      0.00        84      6.90
2018-09-21 16:35:37          0.00            0         18000      0.07      0.00        84      4.49
2018-09-21 16:35:47          0.00            0         18000      0.10      0.00        84      6.10
```
## JSON RPC
JSON RPCs are defined in [`rpc.proto`](https://github.com/QuarkChain/goquarkchain/blob/master/cluster/rpc/rpc.proto). Note that there are two JSON RPC ports. By default they are 38491 for private RPCs and 38391 for public RPCs. Since you are running your own clusters you get access to both.

Public RPCs are documented in the [Developer Guide](https://developers.quarkchain.io/#json-rpc). You can use the client library [quarkchain-web3.js](https://github.com/QuarkChain/quarkchain-web3.js) to query account state, send transactions, deploy and call smart contracts. Here is [a simple example](https://gist.github.com/qcgg/1ab0352c5b2299270b5795648cca83d8) to deploy smart contract on QuarkChain using the client library.
## Loadtest
Run loadtest to your cluster and see how fast it processes large volume of transactions. Please refer to [Loadtest Guide](tests/loadtest/README.md) for instructions.

## Issue
Please open issues on github to report bugs or make feature requests.

## Contribution
All the help from community is appreciated! If you are interested in working on features or fixing bugs, please open an issue first
to describe the task you are planning to do. For small fixes (a few lines of change) feel
free to open pull requests directly.

## Developer Community
Join our developer community on [Discord](https://discord.gg/Jbp35ZC).

## License
Unless explicitly mentioned in a folder or a file, all files are licensed under GNU Lesser General Public License defined in LICENSE file.
