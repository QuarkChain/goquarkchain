FROM circleci/golang:1.12

MAINTAINER ninjaahhh, junjiah@protonmail.com

RUN sudo apt-get update

# set up basic system packages
RUN sudo apt-get install -y git build-essential wget pkg-config libgmp-dev libssl-dev ca-certificates libxml2-dev libxslt1-dev swig

# install rocksdb
RUN git clone https://github.com/facebook/rocksdb.git /tmp/rocksdb
WORKDIR /tmp/rocksdb
RUN sudo apt-get install -y libsnappy-dev zlib1g-dev libbz2-dev libgflags-dev liblz4-dev libzstd-dev
RUN sudo make shared_lib
RUN sudo make install
RUN sudo ln -fs /tmp/rocksdb/librocksdb.so* /usr/lib/

WORKDIR /go

ENV CGO_CFLAGS  "-I/usr/local/include"
ENV CGO_LDFLAGS "-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy"
