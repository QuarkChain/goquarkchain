#!/bin/bash

set -u

source function.sh

type jq >/dev/null 2>&1 || { echo >&2 "Please install jq"; exit 1; }
type docker >/dev/null 2>&1 || { echo >&2 "Please install docker"; exit 1; }

IMAGE="sunchunfeng/goqkc"

testcase=$1
echo "running testcase: $testcase"
echo "======"
echo

# pull latest full node image. use pyquarkchain for now
docker pull $IMAGE
# keep the string after the first pattern from left to right
# (e.g. "1-auction-bid", pattern = "-", result = "auction-bid")
# then trim the file extension
container=$(echo "qkc-${testcase#*-}" | cut -f 1 -d '.')


docker run --name $container -itd -p 38391 $IMAGE
docker cp ../../goquarkchain/tests/ci-qkcli/cluster_config.json $container:/go/src/github.com/QuarkChain/goquarkchain/cmd/cluster/
docker cp ../../goquarkchain/tests/ci-qkcli/start_go_devent.sh $container:/go/src/github.com/QuarkChain/goquarkchain/cmd/cluster/
echo "111"
docker exec -i $container /bin/bash -c \
  'chmod +x ./start_go_devent.sh && ./start_go_devent.sh'
echo "222"

docker exec -i $container /bin/bash -c \
  'cat master.log'

#docker run --name $container -d -p 38391 $IMAGE bash -c \
#  'git pull origin master && \
#   mv testnet/ci-qkcli/constants.py.bac quarkchain/constants.py && \
#   ./run_cluster.sh --cluster_config `pwd`/testnet/ci-qkcli/cluster_config.json'
port=$(docker port $container | awk -F':' '{print $2}')

# check docker started
tmp_dir=$(home $port)
empty_addr=0000000000000000000000000000000000000000
output=""
echo "home directory: $tmp_dir, port: $port"
while [ "$output" == "" ]; do
  echo "waiting for docker to start at test case $testcase"
  sleep 5
  output=$(qkcli query balance $empty_addr 0 --homeDir $tmp_dir 2>/dev/null)
  echo "scf-----"$output
  echo $tmp_dir
done

bash "${testcase}.sh" $tmp_dir
ret_val=$?

# clean up
rm -rf $tmp_dir
docker kill $container > /dev/null
docker rm $container > /dev/null

exit $ret_val
