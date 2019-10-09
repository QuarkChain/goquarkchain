# goquarkchain for ubuntu
Environment configuration for go,require golang sdk >= 1.12
```bash
wget https://studygolang.com/dl/golang/go1.13.1.linux-amd64.tar.gz
tar xzvf go1.13.1.linux-amd64.tar.gz -C /usr/lib/
```
in bashrc,add environment variables for golang
```bash
vim ~/.bashrc

#GOROOT
export GOROOT=/usr/lib/go

#GOPATH go项目路径
export GOPATH=/root/gocode

#GOPATH bin
export PATH=$PATH:$GOPATH/bin

#GOPATH root bin
export PATH=$PATH:$GOROOT/bin
source ~/.bashrc
```
check 
```bash
go version
#show
go version go1.13.1 linux/amd64 ##this is ok
```
Next, to install rocksdb for goquarkchain run
```bash
wget https://github.com/facebook/rocksdb/archive/v6.1.2.tar.gz
tar xzvf v6.1.2.tar.gz -C /usr/lib/
cd  /usr/lib
mkdir rocksdb
mv rocksdb-6.1.2/* rocksdb
cd rocksdb
PORTABLE=1 make shared_lib
INSTALL_PATH=/usr/local make install-shared
```
add environment variables for rocksdb
```bash
vim ~/.bashrc
```
add
```bash
export CPLUS_INCLUDE_PATH=${CPLUS_INCLUDE_PATH}:/usr/lib/rocksdb/include
export LD_LIBRARY_PATH=${LD_LIBRARY_PATH}:/usr/lib/rocksdb
export LIBRARY_PATH=${LIBRARY_PATH}:/usr/lib/rocksdb
```
refesh bash 
```bash
source ~/.bashrc
```
Download and build goquarkchain
```bash
git clone https://github.com/QuarkChain/goquarkchain.git
cd /goquarkchain/consensus/qkchash/native
g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++17
make
cd /goquarkchain/cmd/cluser
go build .
```
run goquarchain for S1 
```bash
cd /goquarkchain/cmd/cluser
 ./cluster --cluster_config ../../mainnet/singularity/cluster_config_template.json --service S1 
```

