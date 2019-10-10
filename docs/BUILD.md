# QuarkChain for golang

[![CircleCI](https://circleci.com/gh/QuarkChain/pyquarkchain/tree/master.svg?style=shield&circle-token=c17a071129e4ab6c0911154c955efc236b1a5015)](https://circleci.com/gh/QuarkChain/pyquarkchain/tree/master)

QuarkChain is a sharded blockchain protocol that employs a two-layer architecture - one extensible sharding layer consisting of multiple shard chains processing transactions and one root chain layer securing the network and coordinating cross-shard transactions among shard chains. The capacity of the network scales linearly as the number of shard chains increase while the root chain is always providing strong security guarantee regardless of the number of shards. QuarkChain testnet consistently hit [10,000+ TPS](https://youtu.be/dUldrq3zKwE?t=8m28s) with 256 shards run by 50 clusters consisting of 6450 servers with each loadtest submitting 3,000,000 transactions to the network.

## Features

- Cluster implementation allowing multiple processes / physical machines to work together as a single full node
- State sharding dividing global state onto independent processing and storage units allowing the network capacity to scale linearly by adding more shards
- Cross-shard transaction allowing native token transfers among shard chains
- Adding shards dynamically to the network
- Support of different mining algorithms on different shards
- P2P network allowing clusters to join and leave anytime with encrypted transport
- Fully compatible with Ethereum smart contract

## Design

![QuarkChain Cluster](https://docs.google.com/drawings/d/e/2PACX-1vRkF6Wd-I-1j-601IFWPwd9u8A5oqa_c2JVBad1SDY48ATY1aRaJvhObiX8p9Jh1ra5G-HIqhhYl0NM/pub?w=960&h=576)

Check out the [Wiki](https://github.com/QuarkChain/pyquarkchain/wiki) to understand the design of QuarkChain.
## Development Setup
### Environment configuration for go,require golang sdk >= 1.12
Download golang
```bash
wget https://studygolang.com/dl/golang/go1.13.1.linux-amd64.tar.gz
tar xzvf go1.13.1.linux-amd64.tar.gz -C /usr/lib/
```
In bashrc,add environment variables for golang
```bash
vim ~/.bashrc
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
#show
go version go1.13.1 linux/amd64 ##this is ok
```
### Next, to install rocksdb for goquarkchain run
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
Add environment variables for rocksdb
```bash
vim ~/.bashrc
```
Add
```bash
export CPLUS_INCLUDE_PATH=${CPLUS_INCLUDE_PATH}:/usr/lib/rocksdb/include
export LD_LIBRARY_PATH=${LD_LIBRARY_PATH}:/usr/lib/rocksdb
export LIBRARY_PATH=${LIBRARY_PATH}:/usr/lib/rocksdb
```
Refesh bash 
```bash
source ~/.bashrc
```
### Run goquarkchain
Download and build goquarkchain
```bash
git clone https://github.com/QuarkChain/goquarkchain.git
cd /goquarkchain/consensus/qkchash/native
g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++17
make
cd /goquarkchain/cmd/cluser
go build .
```
Run goquarchain for S1 
```bash
cd /goquarkchain/cmd/cluser
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S1 
```

