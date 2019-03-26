package common

import "github.com/QuarkChain/goquarkchain/account"

/*
	0b101, 0b11 -> True
	0b101, 0b10 -> False
*/
func MasksHaveOverlap(m1, m2 uint32) bool {
	i1 := account.IntLeftMostBit(m1)
	i2 := account.IntLeftMostBit(m2)
	if i1 > i2 {
		i1 = i2
	}
	bitMask := uint32((1 << (i1 - 1)) - 1)
	return (m1 & bitMask) == (m2 & bitMask)
}

func IsP2(v uint64) bool {
	return (v & (v - 1)) == 0
}
