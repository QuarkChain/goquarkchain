package common

import (
	"encoding/hex"
	"fmt"
	"github.com/ethereum/go-ethereum/rlp"
	"testing"
)

func TestBigIntMulBigRat(t *testing.T) {
	ans := &NewRlpForUint32{3}

	bytes, err := rlp.EncodeToBytes(ans)
	fmt.Println("ans", len(bytes), hex.EncodeToString(bytes), err)

	newAns := new(NewRlpForUint32)
	err = rlp.DecodeBytes(bytes, newAns)
	fmt.Println("newAns", newAns, err)

}
