package sm3

import (
	"encoding/binary"
	"hash"
	"math/bits"
)

const (
	BlockSize  = 16
	ResultSize = 32
	BufferSize = 64
)

// Initial value
var iv = [8]uint32{0x7380166F, 0x4914B2B9, 0x172442D7, 0xDA8A0600, 0xA96F30BC, 0x163138AA, 0xE38DEE4D, 0xB0FB0E4E}

// const array value
// T[j] = 79cc4519 0≤j≤15 ;T[j] = 7a879d8a 16≤j≤63
// t[i] = bits.RotateLeft32(T[i], i)
var t = [BufferSize]uint32{
	0x79CC4519, 0xF3988A32, 0xE7311465, 0xCE6228CB, 0x9CC45197, 0x3988A32F, 0x7311465E, 0xE6228CBC,
	0xCC451979, 0x988A32F3, 0x311465E7, 0x6228CBCE, 0xC451979C, 0x88A32F39, 0x11465E73, 0x228CBCE6,
	0x9D8A7A87, 0x3B14F50F, 0x7629EA1E, 0xEC53D43C, 0xD8A7A879, 0xB14F50F3, 0x629EA1E7, 0xC53D43CE,
	0x8A7A879D, 0x14F50F3B, 0x29EA1E76, 0x53D43CEC, 0xA7A879D8, 0x4F50F3B1, 0x9EA1E762, 0x3D43CEC5,
	0x7A879D8A, 0xF50F3B14, 0xEA1E7629, 0xD43CEC53, 0xA879D8A7, 0x50F3B14F, 0xA1E7629E, 0x43CEC53D,
	0x879D8A7A, 0x0F3B14F5, 0x1E7629EA, 0x3CEC53D4, 0x79D8A7A8, 0xF3B14F50, 0xE7629EA1, 0xCEC53D43,
	0x9D8A7A87, 0x3B14F50F, 0x7629EA1E, 0xEC53D43C, 0xD8A7A879, 0xB14F50F3, 0x629EA1E7, 0xC53D43CE,
	0x8A7A879D, 0x14F50F3B, 0x29EA1E76, 0x53D43CEC, 0xA7A879D8, 0x4F50F3B1, 0x9EA1E762, 0x3D43CEC5}

func ff0(x uint32, y uint32, z uint32) uint32 {
	return x ^ y ^ z
}

func ff1(x uint32, y uint32, z uint32) uint32 {
	return (x & y) | (x & z) | (y & z)
}

func gg0(x uint32, y uint32, z uint32) uint32 {
	return x ^ y ^ z
}

func gg1(x uint32, y uint32, z uint32) uint32 {
	return (x & y) | ((^x) & z)
}

func p0(x uint32) uint32 {
	r9 := bits.RotateLeft32(x, 9)
	r17 := bits.RotateLeft32(x, 17)
	return x ^ r9 ^ r17
}

func p1(x uint32) uint32 {
	r15 := bits.RotateLeft32(x, 15)
	r23 := bits.RotateLeft32(x, 23)
	return x ^ r15 ^ r23
}

type sm3Hash struct {
	value     [8]uint32
	buffer    [BufferSize]byte
	bufOffset int
	byteCount uint64
}

func NewSM3Hash() hash.Hash {
	sm3 := new(sm3Hash)
	sm3.value = iv
	return sm3
}

// Sum appends the current hash to b and returns the resulting slice.
// It does not change the underlying hash state.
func (sm3 *sm3Hash) Sum(b []byte) []byte {
	d1 := sm3
	h := d1.checkSum()
	return append(b, h[:]...)
}

// Size returns the number of bytes Sum will return.
func (sm3 *sm3Hash) Size() int {
	return ResultSize
}

// BlockSize returns the hash's underlying block size.
// The Write method must be able to accept any amount
// of data, but it may operate more efficiently if all writes
// are a multiple of the block size.
func (sm3 *sm3Hash) BlockSize() int {
	return BlockSize
}

// Reset resets the Hash to its initial state.
func (sm3 *sm3Hash) Reset() {
	for i := 0; i < BufferSize; i++ {
		sm3.buffer[i] = 0
	}
	sm3.bufOffset = 0
	sm3.byteCount = 0

	sm3.value = iv
}

// Write (via the embedded io.Writer interface) adds more data to the running hash.
// It never returns an error.
func (sm3 *sm3Hash) Write(p []byte) (int, error) {
	_ = p[0]
	size := len(p)
	index := 0

	// if buffer + p is larger than or equal to 64 bytes,
	// process block and set the offset to 0,
	// till the size is smaller than 64 bytes
	for sm3.bufOffset+size >= BufferSize {
		for j, len := 0, BufferSize-sm3.bufOffset; j < len; j++ {
			sm3.buffer[sm3.bufOffset] = p[index]
			sm3.bufOffset++
			index++
		}
		sm3.processBlock()
		size = size - BufferSize
	}

	// save the remaining data to buffer
	for j := 0; j < size; j++ {
		sm3.buffer[sm3.bufOffset] = p[index]
		sm3.bufOffset++
		index++
	}

	sm3.byteCount += uint64(index)
	return index, nil
}

func (sm3 *sm3Hash) finish() {
	// calculate the bit length (byteCount * 8)
	bitLength := sm3.byteCount << 3
	// write 128 (10000000) which mean end
	sm3.Write([]byte{128})

	// fill in 0 till the last 8 bytes
	if sm3.bufOffset > 56 {
		for i := sm3.bufOffset; i < BufferSize; i++ {
			sm3.buffer[i] = 0
		}
		sm3.processBlock()
	}

	for i := sm3.bufOffset; i < 56; i++ {
		sm3.buffer[i] = 0
	}

	// fill in bit length to the last 8 bytes
	var bytes [8]byte
	binary.BigEndian.PutUint64(bytes[:], bitLength)
	copy(sm3.buffer[56:], bytes[:])

	// process the last block
	sm3.processBlock()
}

func (sm3 *sm3Hash) checkSum() [ResultSize]byte {
	sm3.finish()
	vlen := len(sm3.value)
	var out [ResultSize]byte
	for i := 0; i < vlen; i++ {
		binary.BigEndian.PutUint32(out[i*4:(i+1)*4], sm3.value[i])
	}
	return out
}

// new Value = processBlock(old Value, Buffer)
func (sm3 *sm3Hash) processBlock() {
	var W [68]uint32
	// Initial the first 16 words using buffer
	for j := 0; j < BlockSize; j++ {
		W[j] = binary.BigEndian.Uint32(sm3.buffer[j*4 : (j+1)*4])
	}
	// extend the first 16 words to fill in the rest of the array
	// Wj ← P1(Wj−16 ⊕Wj−9 ⊕bits.RotateLeft32(W[j-3], 15))⊕bits.RotateLeft32(W[j-13], 7)⊕Wj−6
	for j := BlockSize; j < 68; j++ {
		r15 := bits.RotateLeft32(W[j-3], 15)
		r7 := bits.RotateLeft32(W[j-13], 7)
		W[j] = p1(W[j-16]^W[j-9]^r15) ^ r7 ^ W[j-6]
	}

	A := sm3.value[0]
	B := sm3.value[1]
	C := sm3.value[2]
	D := sm3.value[3]
	E := sm3.value[4]
	F := sm3.value[5]
	G := sm3.value[6]
	H := sm3.value[7]

	// calculate ABCDEFGH
	for j := 0; j < BlockSize; j++ {
		a12 := bits.RotateLeft32(A, 12)
		SS1 := bits.RotateLeft32(a12+E+t[j], 7)
		SS2 := SS1 ^ a12
		W1j := W[j] ^ W[j+4]
		TT1 := ff0(A, B, C) + D + SS2 + W1j
		TT2 := gg0(E, F, G) + H + SS1 + W[j]
		D = C
		C = bits.RotateLeft32(B, 9)
		B = A
		A = TT1
		H = G
		G = bits.RotateLeft32(F, 19)
		F = E
		E = p0(TT2)
	}

	for j := BlockSize; j < 64; j++ {
		a12 := bits.RotateLeft32(A, 12)
		SS1 := bits.RotateLeft32(a12+E+t[j], 7)
		SS2 := SS1 ^ a12
		W1j := W[j] ^ W[j+4]
		TT1 := ff1(A, B, C) + D + SS2 + W1j
		TT2 := gg1(E, F, G) + H + SS1 + W[j]
		D = C
		C = bits.RotateLeft32(B, 9)
		B = A
		A = TT1
		H = G
		G = bits.RotateLeft32(F, 19)
		F = E
		E = p0(TT2)
	}

	sm3.value[0] ^= A
	sm3.value[1] ^= B
	sm3.value[2] ^= C
	sm3.value[3] ^= D
	sm3.value[4] ^= E
	sm3.value[5] ^= F
	sm3.value[6] ^= G
	sm3.value[7] ^= H

	sm3.bufOffset = 0
}

func Sum(data []byte) [ResultSize]byte {
	var d sm3Hash
	d.Reset()
	d.Write(data)
	return d.checkSum()
}
