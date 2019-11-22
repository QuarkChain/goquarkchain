# Quick Start - Use Docker Image to Start a Cluster and Mining

Here we provide detailed instructions for running a cluster and starting the mining process using a pre-built docker image. 
If you are interested in building everything from scratch, please refer to this [Dockerfile](Dockerfile) for more details.

```bash
# NOTE the version should be in sync with the release version, e.g. quarkchaindocker/goquarkchain:latest
$ sudo docker pull quarkchaindocker/goquarkchain:<version tag> 

# recommend using some window management tool to start
# different programs, for example `screen` or `tmux`.
$ sudo docker run -it -p 38291:38291 -p 38391:38391 -p 38491:38491 -p 38291:38291/udp quarkchaindocker/goquarkchain:<version tag>
# if you already have synced data available, can mount it during running docker (note the -v flag)
$ sudo docker run -v /path/to/data:/go/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/mainnet -it -p 38291:38291 -p 38391:38391 -p 38491:38491 -p 38291:38291/udp quarkchaindocker/goquarkchain:<version tag> 

# (optional) if you want to get the data quickly without validating the blocks,
# you can download a snapshot of the database by running the following command.
# every 24 hours a snapshot of the database will be taken and uploaded by the QuarkChain team.
# once you start your cluster using the downloaded database your cluster only need to sync
# the blocks mined in the past 24 hours or less.
curl https://qkcmainnet-go.s3.amazonaws.com/data/`curl https://qkcmainnet-go.s3.amazonaws.com/data/LATEST`.tar.gz --output data.tar.gz
# if you are in China, use the following instead
# curl https://s3.cn-north-1.amazonaws.com.cn/qkcmainnet-go-cn/data/`curl https://s3.cn-north-1.amazonaws.com.cn/qkcmainnet-go-cn/data/LATEST`.tar.gz --output data.tar.gz
# then should unzip to the right path

# INSIDE the container
# IMPORTANT: always update coinbase address for mining
# modify the config file /go/src/github.com/QuarkChain/goquarkchain/mainnet/singularity/cluster_config_template.json
# make sure the config file has been updated your specified coinbase address
# start cluster:
cd /go/src/github.com/QuarkChain/goquarkchain/cmd/cluster
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S0 >> s0.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S1 >> s1.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S2 >> s2.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S3 >> s3.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --json_rpc_host 0.0.0.0 --json_rpc_private_host 0.0.0.0 >>master.log 2>&1 &

# to start mining in another screen (outside of container)
# note this is only a sample mining program. feel free to change `goqkcminer`
$ sudo docker ps  # find container ID
# you can specify the full shard keys which you want to mine, the command should be:
$ sudo docker exec -it <container ID> go run /go/src/github.com/QuarkChain/goquarkchain/cmd/miner/main.go -config ../../mainnet/singularity/cluster_config_template.json -shards <full shard key>
```

* To monitor the current state of the network (e.g. chain height, account balance) refer to [stats tool](../cmd/stats).

# Configure the Network

First, you'll need to set up the configuration of your network, which all nodes need to be aware of and agree upon. 
We provide an example config JSON in the repo (`mainnet/singularity/cluster_config_template.json`). 
**Note that many parameters in the config are part the consensus, please be very cautious when changing them.** 
For example, `COINBASE_AMOUNT` is one of such parameters, changing it to another value effectively creates a fork in the network.

To set up mining, **you need to open the config and specify your own coinbase address in each shard 
(under `QUARKCHAIN.CHAINS[].COINBASE_ADDRESS`) or the root chain (under `QUARKCHAIN.ROOT.COINBASE_ADDRESS`)**. 

Note:
Please remove the first two characters "0x" from the coinbase address.

Following is a snippet of the config for a single shard.

```bash
...
{
                "CONSENSUS_TYPE": "POW_ETHASH",
                "CONSENSUS_CONFIG": {
                    "TARGET_BLOCK_TIME": 10,
                    "REMOTE_MINE": true
                },
                "GENESIS": {
                    "ROOT_HEIGHT": 0,
                    "VERSION": 0,
                    "HEIGHT": 0,
                    "HASH_PREV_MINOR_BLOCK": "0000000000000000000000000000000000000000000000000000000000000000",
                    "HASH_MERKLE_ROOT": "0000000000000000000000000000000000000000000000000000000000000000",
                    "EXTRA_DATA": "497420776173207468652062657374206f662074696d65732c206974207761732074686520776f727374206f662074696d65732c202e2e2e202d20436861726c6573204469636b656e73",
                    "TIMESTAMP": 1519147489,
                    "DIFFICULTY": 3000,
                    "GAS_LIMIT": 12000000,
                    "NONCE": 0,
                    "ALLOC": {}
                },
                "COINBASE_ADDRESS":  "D50F23E410711C1391F5Ef88fC11245e564c76840000EF5e", 
                "COINBASE_AMOUNT": 5,
                "GAS_LIMIT_EMA_DENOMINATOR": 1024,
                "GAS_LIMIT_ADJUSTMENT_FACTOR": 1024,
                "GAS_LIMIT_MINIMUM": 5000,
                "GAS_LIMIT_MAXIMUM": 9223372036854775807,
                "GAS_LIMIT_USAGE_ADJUSTMENT_NUMERATOR": 3,
                "GAS_LIMIT_USAGE_ADJUSTMENT_DENOMINATOR": 2
            },
...
```

Note the coinbase address is your Quarkchain wallet address (20-byte recipient + 4-byte full shard key). 
If you do not have one, please create one from our online [testnet](http://devnet.quarkchain.io/wallet) / 
[mainnet](https://mainnet.quarkchain.io/wallet) wallet.

## Mainnet mining configuration

For specifying shards in goqkcminer (--shards)

|Chain |Shard |Hash Algo|Parameter for goqkcminer|Block Interval| QKC Per Block |
| ---      | ---     |---  | --- | --- | --- |
| 0     | 0 | Ethash|1|10s|3.25|
| 1       | 0  |Ethash             | 65537 |10s|3.25|
| 2       | 0   |Ethash     | 131073 |10s|3.25|
| 3       | 0     |Ethash        | 196609 |10s|3.25|
|4|0|Ethash|262145|10s|3.25|
|5|0|Ethash|327681|10s|3.25|
|6|0|Qkchash|393217|10s|3.25|
|7|0|Qkchash|458753|10s|3.25|

[Instructions for Ethash GPU mining](https://github.com/jyouyj/ethminer)

|Chain |Shard |Hash Algo |Parameter for Ethminer shard ID|
| ---      | ---     |---  | --- |
| 0  | 0      | Ethash               | 1 |
| 1  |  0      | Ethash        | 10001 |
| 2  |  0       | Ethash              | 20001 |
| 3  |  0       | Ethash              | 30001 |
| 4  |  0       | Ethash              | 40001 |
| 5  |  0       | Ethash              | 50001 |


