FROM ubuntu:24.04
LABEL maintainer="goquarkchain"

#prerequisite
RUN apt-get update && apt-get install -y \
    vim-tiny \
    curl \
    git \
    build-essential \
    make \
    g++ \
    swig \
    libbz2-dev \
    libgflags-dev \
    liblz4-dev \
    libsnappy-dev \
    libzstd-dev \
    zlib1g-dev \
 && rm -rf /var/lib/apt/lists/*

# install rocksdb
WORKDIR /code
RUN git clone -b v8.11.4 --single-branch https://github.com/facebook/rocksdb.git
WORKDIR /code/rocksdb
RUN make shared_lib
RUN make install-shared
ENV CGO_CFLAGS=-I/usr/local/include
ENV CGO_LDFLAGS="-L/usr/local/lib -lrocksdb -lstdc++ -lm -lz -lbz2 -lsnappy"
ENV LD_LIBRARY_PATH=/usr/local/lib

# install golang
WORKDIR /code
RUN curl -sSL https://storage.googleapis.com/golang/go1.14.12.linux-amd64.tar.gz \
		| tar -v -C /usr/local -xz
ENV GOPATH $HOME/go
ENV PATH $PATH:/usr/local/go/bin
ENV GOPROXY https://goproxy.io

# install goquarkchain
WORKDIR $GOPATH/src/github.com/QuarkChain
RUN git clone https://github.com/QuarkChain/goquarkchain.git
WORKDIR $GOPATH/src/github.com/QuarkChain/goquarkchain/consensus/qkchash/native
RUN g++ -shared -o libqkchash.so -fPIC qkchash.cpp -O3 -std=gnu++17 && make
WORKDIR $GOPATH/src/github.com/QuarkChain/goquarkchain/tests/loadtest/deployer
RUN go build
WORKDIR $GOPATH/src/github.com/QuarkChain/goquarkchain/cmd/cluster
RUN go build -tags nocgo