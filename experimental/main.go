package main

import (
	"fmt"
	"io/ioutil"

	"github.com/QuarkChain/goquarkchain/go-ethereum/rlp"
)

func main() {
	buf := make([]byte, 50)
	_, r, _ := rlp.EncodeToReader("foo")
	ioutil.ReadAll(r)
	r.Read(buf)
	fmt.Println(r)
}
