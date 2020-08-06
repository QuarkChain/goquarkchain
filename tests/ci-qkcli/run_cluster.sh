#!/bin/bash

if [ x"$1" = x ]; then
	if [ -f "../../mainnet/singularity/cluster_config_template.json" ]; then
		configPath="../../mainnet/singularity/cluster_config_template.json"
	elif [ -f "./cluster_config_template.json" ]; then
		configPath="./cluster_config_template.json"
	else
		echo "config not set"
		exit
	fi
else
	configPath=$1
fi
echo "configPath="$configPath
slaveInfo=$(grep -Po 'ID[" :]+\K[^"]+' $configPath | grep S)

# start slave
for value in $slaveInfo; do
	sLog=$(pwd)/${value}.log
	cmd="./cluster --cluster_config "${configPath}" --service "${value}">> "$sLog" 2>&1 &"
	eval $cmd
	pid=$!
	echo "Start "${value}"     successfull pid="$pid" logFile=$sLog"
done

sleep 5s

# start master
mLog=$(pwd)/master.log
cmd="./cluster --cluster_config "${configPath}" >>$mLog 2>&1 &"
eval $cmd
pid=$!
echo "Start master successfull pid="$pid" logFile=$mLog"
