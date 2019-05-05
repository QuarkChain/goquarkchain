package common

import (
	"bytes"
	"encoding/gob"
	"math/bits"
	"net"
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

func GetIPV4Addr() (string, error) {

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ones, bits := ipnet.Mask.Size()
			if bits == 32 && ones == 24 {
				return ipnet.IP.To4().String(), nil
			}
		}
	}
	return "127.0.0.1", nil
}
