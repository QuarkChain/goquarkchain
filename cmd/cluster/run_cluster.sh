#!/bin/bash

if [ x"$1" = x ]; then
    configPath="../../mainnet/singularity/cluster_config_template.json"
    echo "not set config. use default config . configPath="$configPath
else
    configPath=$1
    echo "set config, configPath="$configPath
fi

slaveInfo=`grep -Po 'ID[" :]+\K[^"]+' $configPath | grep S`

# start slave
for value in $slaveInfo
do
   sLog=`pwd`/${value}.log
	 cmd="./cluster --cluster_config "${configPath}" --service "${value}">> "$sLog" 2>&1 &"
	 eval $cmd
	 pid=$!
	 echo "Start "${value}"     successfull pid="$pid" logFile=$sLog"
done

sleep 5s

# start master
mLog=`pwd`/master.log
cmd="./cluster --cluster_config "${configPath}" >>$mLog 2>&1 &"
eval $cmd
pid=$!
echo "Start master successfull pid="$pid" logFile=$mLog"
