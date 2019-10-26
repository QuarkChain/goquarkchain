go build -o /tmp/QKC/cluster
kill -9 `ps -ef | grep cluster_config | awk '{print $2}'`
#rm -rf ./qkc-data
rm *.log
chmod +x /tmp/QKC/cluster && /tmp/QKC/cluster --cluster_config /tmp/QKC/cluster_config_template.json --service S0 >> s0.log 2>&1 &
chmod +x /tmp/QKC/cluster && /tmp/QKC/cluster --cluster_config /tmp/QKC/cluster_config_template.json --service S1 >> s1.log 2>&1 &
chmod +x /tmp/QKC/cluster && /tmp/QKC/cluster --cluster_config /tmp/QKC/cluster_config_template.json --json_rpc_host 0.0.0.0 --json_rpc_private_host 0.0.0.0 >>master.log 2>&1 &

