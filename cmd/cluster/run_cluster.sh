#!/bin/bash

configPath=$1

slaveInfo=`grep -Po 'ID[" :]+\K[^"]+' $configPath | grep S`

# start slave
for value in $slaveInfo
do
	 cmd="./cluster --cluster_config "${configPath}" --service "${value}">> "${value}".log 2>&1 &"
	 echo $cmd
	 eval $cmd
done

# start master
cmd="./cluster --cluster_config "${configPath}" --json_rpc_host 0.0.0.0 --json_rpc_private_host 0.0.0.0   >>master.log 2>&1 &"
echo $cmd
eval $cmd
