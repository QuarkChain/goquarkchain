package common

import (
	"errors"
	"fmt"
	"math/big"
)

var (
	TOKENBASE  = uint64(36)
	TOKENIDMAX = 4873763662273663091 // ZZZZZZZZZZZZ
	TOKENMAX   = "ZZZZZZZZZZZZ"
)

func TokenIDEncode(str string) *big.Int {
	if len(str) >= 13 {
		panic(errors.New("name too long"))
	}
	// TODO check name can only contain 0-9, A-Z

	id := TokenCharEncode(str[len(str)-1])
	base := TOKENBASE

	len := len(str)
	for index := len - 2; index >= 0; index-- {
		id += base * (TokenCharEncode(str[index] + 1))
		base *= TOKENBASE
	}
	return new(big.Int).SetUint64(uint64(id))
}

func TokenCharEncode(char byte) uint64 {
	if char >= byte('A') && char <= byte('Z') {
		return 10 + uint64(char-byte('a'))
	}
	if char >= byte('0') && char <= byte('9') {
		return uint64(char - byte('0'))
	}
	panic(fmt.Errorf("unknown character %v", char))
}
