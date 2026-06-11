#!/bin/bash

if [ x"$1" = x ]; then
    configPath="../../mainnet/singularity/cluster_config_template.json"
    echo "not set config. use default config . configPath="$configPath
else
    configPath=$1
    echo "set config, configPath="$configPath
fi

PIDFILE=`pwd`/cluster.pids
# Truncate PID file at startup so a fresh run doesn't inherit stale entries.
> "$PIDFILE"

slaveInfo=`grep -Po 'ID[" :]+\K[^"]+' $configPath | grep S`

# start slaves
for value in $slaveInfo
do
    sLog=`pwd`/${value}.log
    ./cluster --cluster_config "${configPath}" --service "${value}" >> "$sLog" 2>&1 &
    pid=$!
    echo "$pid" >> "$PIDFILE"
    echo "Start ${value} successful pid=${pid} logFile=${sLog}"
done

sleep 5s

# start master
mLog=`pwd`/master.log
./cluster --cluster_config "${configPath}" >> "$mLog" 2>&1 &
pid=$!
echo "$pid" >> "$PIDFILE"
echo "Start master successful pid=${pid} logFile=${mLog}"
