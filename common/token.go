package common

import (
	"errors"
	"fmt"
	"math/big"
)

var (
	TOKENBASE  = uint64(36)
	TOKENIDMAX = uint64(4873763662273663091) // ZZZZZZZZZZZZ
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

func TokenIdDecode(id *big.Int) (string, error) {
	if id == nil {
		return "", errors.New("id is nil")
	}
	if !(new(big.Int).Cmp(new(big.Int)) >= 0 && new(big.Int).Cmp(new(big.Int).SetUint64(TOKENIDMAX)) <= 0) {
		return "", errors.New("it too big or negative")
	}
	name := make([]byte, 0)
	t, err := TokenCharDecode(id.Mod(id, new(big.Int).SetUint64(TOKENBASE)))
	if err != nil {
		return "", err
	}
	name = append(name, t)
	id = id.Div(id, new(big.Int).SetUint64(TOKENBASE))
	for id.Cmp(new(big.Int)) >= 0 {
		t, err := TokenCharDecode(id.Mod(id, new(big.Int).SetUint64(TOKENBASE)))
		if err != nil {
			return "", err
		}
		name = append(name, t)
		id = id.Div(id, new(big.Int).SetUint64(TOKENBASE))
	}
	return string(name), nil
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

func TokenCharDecode(id *big.Int) (byte, error) {
	if !(id.Cmp(new(big.Int).SetUint64(TOKENBASE)) < 0 && id.Cmp(new(big.Int)) >= 0) {
		return byte(0), errors.New("invalid char")
	}
	if id.Cmp(new(big.Int).SetUint64(10)) < 0 {
		return byte('0' + id.Uint64()), nil
	}
	return byte('A' + id.Uint64() - 10), nil
}
