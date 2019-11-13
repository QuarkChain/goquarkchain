kill -9  `ps -ef | grep cluster_config | awk '{print $2}'`
git pull
rm cluster
go build
