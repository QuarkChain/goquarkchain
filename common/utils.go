package common

import (
	"bytes"
	"encoding/gob"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"math/big"
	"math/bits"
	"reflect"
)

var (
	EmptyHash = ethCommon.Hash{}
)

/*
	0b101, 0b11 -> True
	0b101, 0b10 -> False
*/
func MasksHaveOverlap(m1, m2 uint32) bool {
	i1 := IntLeftMostBit(m1)
	i2 := IntLeftMostBit(m2)
	if i1 > i2 {
		i1 = i2
	}
	bitMask := uint32((1 << (i1 - 1)) - 1)
	return (m1 & bitMask) == (m2 & bitMask)
}

// IsP2 is check num is 2^x
func IsP2(shardSize uint32) bool {
	return (shardSize & (shardSize - 1)) == 0
}

// IntLeftMostBit left most bit
func IntLeftMostBit(v uint32) uint32 {
	return uint32(32 - bits.LeadingZeros32(v))
}

func DeepCopy(dst, src interface{}) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(src); err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(buf.Bytes())).Decode(dst)
}

func IsNil(data interface{}) bool {
	return data == nil || reflect.ValueOf(data).IsNil()
}

// ConstMinorBlockRewardCalculator blockReward struct
type ConstMinorBlockRewardCalculator struct {
}

// GetBlockReward getBlockReward
func (c *ConstMinorBlockRewardCalculator) GetBlockReward() *big.Int {
	data := new(big.Int).SetInt64(100)
	return new(big.Int).Mul(data, new(big.Int).SetInt64(1000000000000000000))
}

func BigIntMulBigRat(bigInt *big.Int, bigRat *big.Rat) *big.Int {
	ans := new(big.Int).Mul(bigInt, bigRat.Num())
	ans.Div(ans, bigRat.Denom())
	return ans
}
