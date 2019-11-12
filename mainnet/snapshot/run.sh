kill  `ps -ef | grep cluster_config | awk '{print $2}'`
#git pull
#go build -o /tmp/QKC/cluster && chmod +x /tmp/QKC/cluster
rm *.log
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S0 >> s0.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S1 >> s1.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S2 >> s2.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S3 >> s3.log 2>&1 &
./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --json_rpc_host 0.0.0.0 --json_rpc_private_host 0.0.0.0 --verbosity 3 >>master.log 2>&1 &


