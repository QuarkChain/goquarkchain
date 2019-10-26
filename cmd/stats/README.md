# Statistical Tool for Loadtest


## Monitor TPS

```bash 
go run stats.go 
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