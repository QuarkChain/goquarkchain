#!/bin/bash
# script to sync data into s3:

set -ex

DATA_DIR=$GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster/qkc-data/
DATE=`date +%Y-%m-%d.%H:%M:%S`
BACKUP_DIR=/home/ubuntu/backup
OUTPUT_FILE=$BACKUP_DIR/$DATE.tar.gz
LATEST_FILE=$BACKUP_DIR/LATEST
# 3 day's backup
RETENTION=6
cd $DATA_DIR
cd ..
./stop.sh
mkdir -p $BACKUP_DIR
cd $DATA_DIR
tar cvfz $OUTPUT_FILE ./mainnet/
cd ..
./run.sh



cd $BACKUP_DIR
sz=$(ls | wc -l)
# includes `LATEST`
retention=$((RETENTION + 1))
if [ "$sz" -gt "$retention" ]; then
	ls -t | tail -$((sz - retention)) | xargs -I {} rm {}
fi
cd -

echo $DATE > $LATEST_FILE

# this  need aws s3 'Access key ID' and 'Private access key',and that key must have permission to s3
aws s3 sync $BACKUP_DIR s3://qkcmainnet/data --acl public-read
