#!/bin/bash
configPath="../../mainnet/singularity/cluster_config_template.json"
logPath="./"
while getopts ":c:l:h:" opt
do
    case $opt in
        c)
        configPath=$OPTARG
        ;;
        l)
        logPath=$OPTARG"/"
        ;;
        ?)
        echo "unexecpted param $OPTARG"
        exit 0
    esac
done

echo "configPath=$configPath"
echo "logPath=$logPath"

slaveInfo=`grep -Po 'ID[" :]+\K[^"]+' $configPath | grep S`

# start slave
for value in $slaveInfo
do
   sLog=$logPath${value}".log"
	 cmd="./cluster --cluster_config "${configPath}" --service "${value}">> "$sLog" 2>&1 &"
	 eval $cmd
	 pid=$!
	 echo "Start "${value}"     successfull pid="$pid" logPath=$sLog"
done

sleep 5s

# start master
mLog=$logPath"master.log"
cmd="./cluster --cluster_config "${configPath}" >>$mLog 2>&1 &"
eval $cmd
pid=$!
echo "Start master successfull pid="$pid" logPath=$mLog"
