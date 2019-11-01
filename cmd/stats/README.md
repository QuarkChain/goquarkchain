# Statistical Tool for Loadtest

Suppose your current working directory is `goquarkchain/cmd/stats`.

## Monitor TPS

```bash 
go run stats.go --ip 127.0.0.1
============================
QuarkChain Cluster Stats
============================
CPU:                8
Memory:             16 GB
IP:                 127.0.0.1
Chains:             8
Network Id:         252
Peers:              
============================
Timestamp               Syncing TPS     Pend.TX Conf.TX CPU     ROOT    CHAIN/SHARD-HEIGHT
2019-10-22 16:25:40     false   0.00    0       0       18.75   48      0/0-34 1/0-7 2/0-37 3/0-41 4/0-37 5/0-34 6/0-34 7/0-43
2019-10-22 16:25:50     false   0.00    0       0       0.95    48      0/0-34 1/0-7 2/0-37 3/0-41 4/0-37 5/0-34 6/0-34 7/0-43
2019-10-22 16:26:00     false   0.00    0       0       0.83    48      0/0-34 1/0-7 2/0-37 3/0-41 4/0-37 5/0-34 6/0-34 7/0-43
2019-10-22 16:26:10     false   0.00    0       0       1.20    48      0/0-34 1/0-7 2/0-37 3/0-41 4/0-37 5/0-34 6/0-34 7/0-43
2019-10-22 16:26:20     false   0.00    0       0       0.66    48      0/0-34 1/0-7 2/0-37 3/0-41 4/0-37 5/0-34 6/0-34 7/0-43
2019-10-22 16:26:30     false   0.00    0       0       0.78    48      0/0-34 1/0-7 2/0-37 3/0-41 4/0-37 5/0-34 6/0-34 7/0-43
```
## Query Account Balance

```bash
# will query balance if --a used
go run stats.go --a 0x5c01452896371fa085a890ec2557116cf0476a7900010000 
```

## Flags

```bash

--ip 127.0.0.1  #cluster IP; defaults to localhost

--i 10 #query interval in seconds; defaults to 10

--a 0x5c01452896371fa085a890ec2557116cf0476a7900010000 #quarkchain address of 48 bytes long

--t QI #query account balance for a specific token; default to QKC

```