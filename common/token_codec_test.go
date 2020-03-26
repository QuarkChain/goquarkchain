package common

import (
	"fmt"
	"testing"

	"github.com/QuarkChain/goquarkchain/common/hexutil"
)

func TestTokenCharEncode(t *testing.T) {
	EncodedValues := make(map[string]uint64)
	EncodedValues["0"] = 0
	EncodedValues["Z"] = 35
	EncodedValues["00"] = 36
	EncodedValues["0Z"] = 71
	EncodedValues["1Z"] = 107
	EncodedValues["20"] = 108
	EncodedValues["ZZ"] = 1331
	EncodedValues["QKC"] = 35760
	EncodedValues[TOKENMAX] = TOKENIDMAX

	for key, value := range EncodedValues {
		if value != TokenIDEncode(key) {
			t.Fatalf("key:%v should: %v is %v", key, value, TokenIDEncode(key))
		}
	}
}

func TestRandomToken(t *testing.T) {
	data, err := hexutil.DecodeUint64("0x40001")
	fmt.Println("data", data, err, 0<<16+1)
	fmt.Println("data", data, err, 1<<16+1)
	fmt.Println("data", data, err, 2<<16+1)
	fmt.Println("data", data, err, 3<<16+1)
	fmt.Println("data", data, err, 4<<16+1)
	fmt.Println("data", data, err, 5<<16+1)
	fmt.Println("data", data, err, 6<<16+1)
	fmt.Println("data", data, err, 7<<16+1)

}
