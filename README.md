# GoQuarkChain

[![CircleCI](https://circleci.com/gh/QuarkChain/goquarkchain/tree/master.svg?style=shield&circle-token=afd6d8dfa04abf5da21613deb2572c330e4a6d49)](https://circleci.com/gh/QuarkChain/goquarkchain/tree/master)

Go implementation of [quarkchain](https://quarkchain.io).

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
### Environment configuration for Go
Install golang
```bash
#requires golang sdk >= 1.12
wget https://studygolang.com/dl/golang/go1.13.1.linux-amd64.tar.gz
tar xzvf go1.13.1.linux-amd64.tar.gz -C /usr/lib/
```
In bashrc, add environment variables for golang
```bash
#GOROOT
export GOROOT=/usr/lib/go
#GOPATH  go project path
export GOPATH=/home/gocode
#GOPATH bin
export PATH=$PATH:$GOPATH/bin
#GOROOT bin
export PATH=$PATH:$GOROOT/bin
```
Refesh bash
```bash
source ~/.bashrc
```

Check 
```bash
go version
go version go1.12.8 linux/amd64
```
### Install rocksdb
```bash
wget https://github.com/facebook/rocksdb/archive/v6.1.2.tar.gz
tar xzvf v6.1.2.tar.gz -C /usr/lib/
cd  /usr/lib
mkdir rocksdb
mv rocksdb-6.1.2/* rocksdb
cd rocksdb
PORTABLE=1 make shared_lib
INSTALL_PATH=/usr/local make install-shared
```
In bashrc, add environment variables for rocksdb
```bash
export CPLUS_INCLUDE_PATH=${CPLUS_INCLUDE_PATH}:/usr/lib/rocksdb/include
export LD_LIBRARY_PATH=${LD_LIBRARY_PATH}:/usr/lib/rocksdb
export LIBRARY_PATH=${LIBRARY_PATH}:/usr/lib/rocksdb
```
Refesh bash 
```bash
source ~/.bashrc
```
### Install goquarkchain
Setup goquarkchain 
```bash
git clone https://github.com/QuarkChain/goquarkchain.git
#build qkchash
cd /goquarkchain/consensus/qkchash/native
g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++17
make
```
Run all the unit tests under `goquarkchain`

```
go test ./...
```
## Joining Testnet

Please check [Testnet2-Schedule](https://github.com/QuarkChain/pyquarkchain/wiki/Testnet2-Schedule) for updates and schedule.

### Running a cluster to join QuarkChain testnet
If you are on a private network (e.g. running from a laptop which connects to the Internet through a router), you need to first setup [port forwarding](https://github.com/QuarkChain/pyquarkchain/wiki/Private-Network-Setting%2C-Port-Forwarding) for UDP/TCP 38291.

Then fill in your own coinbase address and [bootstrap a cluster](https://github.com/QuarkChain/pyquarkchain/wiki/Run-a-Private-Cluster-on-the-QuarkChain-Testnet-2.0) on QuarkChain Testnet 2.0.

We provide the [demo implementation of CPU mining software](https://github.com/QuarkChain/goquarkchain/tree/master/cmd/miner). Please refer to [QuarkChain mining](https://github.com/QuarkChain/pyquarkchain/wiki/Introduction-of-Mining-Algorithms) for more details.

### <a name="single_cluster"></a>Running a single cluster for local testing
Start running a local cluster which does not connect to anyone else. The default cluster has 8 shards and 4 slaves.

```bash
#build goquarkchain executable
cd cmd/cluser
go build .
#start each shard in different terminals with its ID specified in SLAVE_LIST of the json config:
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S0
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S1
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S2
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S3
#start master in another terminal
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

## <a name="monitor"></a>Monitoring Clusters
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
Run loadtest to your cluster and see how fast it processes large volume of transactions. [12,000 loadtest accounts](https://github.com/QuarkChain/goquarkchain/blob/master/tests/testdata/genesis_data/loadtest.json) are [loaded into genesis alloc config](https://github.com/QuarkChain/goquarkchain/blob/98343d5c4500883f6d31e757502e23f1aed5acd5/cluster/config/config.go#L285) for each shard.
1. Follow the [instruction](#single_cluster) to start a local cluster

2. Trigger loadtest through `createTransactions ` JSON RPC which requests the cluster to generate transactions on each shard. `numTxPerShard` <= 12000, `xShardPercent` <= 100

   ```bash
   curl -X POST --data '{"jsonrpc":"2.0","method":"createTransactions","params":{"numTxPerShard":10000, "xShardPercent":10},"id":0}' http://localhost:38491
   ```
3. At your virtual environment, [monitor](#monitor) the TPS using the stats tool.
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
