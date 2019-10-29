./cluster --cluster_config ../../tests/loadtest/cluster_config.json --service S0 &
S0=$!
./cluster --cluster_config ../../tests/loadtest/cluster_config.json --service S1 &
S1=$!
./cluster --cluster_config ../../tests/loadtest/cluster_config.json --service S2 &
S2=$!
./cluster --cluster_config ../../tests/loadtest/cluster_config.json --service S3 &
S3=$!
./cluster --cluster_config ../../tests/loadtest/cluster_config.json
M=$!
wait $S0 $S1 $S2 $S3 $M
