package serialize

import (
	"encoding/binary"
	"fmt"
)

type ByteBuffer struct {
	data     *[]byte
	position int
	size     int
}

func NewByteBuffer(bytes []byte) *ByteBuffer {
	bb := ByteBuffer{&bytes, 0, len(bytes)}
	return &bb
}

func (bb *ByteBuffer) GetOffset() int {
	return bb.position
}

func (bb *ByteBuffer) getBytes(size int) ([]byte, error) {
	if size > bb.size-bb.position {
		return nil, fmt.Errorf("deser: buffer is shorter than expected")
	}

	bytes := (*bb.data)[bb.position : bb.position+size]
	bb.position += size
	return bytes, nil
}

func (bb *ByteBuffer) GetUInt8() (uint8, error) {
	bytes, err := bb.getBytes(1)
	if err != nil {
		return 0, err
	}

	return uint8(bytes[0]), nil
}

func (bb *ByteBuffer) GetUInt16() (uint16, error) {
	bytes, err := bb.getBytes(2)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint16(bytes), nil
}

func (bb *ByteBuffer) GetUInt32() (uint32, error) {
	bytes, err := bb.getBytes(4)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(bytes), nil
}

func (bb *ByteBuffer) GetUInt64() (uint64, error) {
	bytes, err := bb.getBytes(8)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(bytes), nil
}

func (bb *ByteBuffer) getLen(byteSize int) (int, error) {
	if byteSize < 1 {
		return 0, fmt.Errorf("deser: bytesize in GetVarBytes should larger than 0")
	}
	// Reject byteSize > 4 to prevent int overflow on 64-bit systems. For byteSize=8,
	// the accumulator `size = (size << 8) | int(b[i])` can overflow into negative,
	// bypassing the `size > Remaining()` guard (negative > positive → false). No
	// current callers use byteSize > 4, but the absence of an upper bound makes this
	// a latent vulnerability.
	if byteSize > 4 {
		return 0, fmt.Errorf("deser: bytesize %d exceeds maximum safe value 4", byteSize)
	}

	b, err := bb.getBytes(byteSize)
	if err != nil {
		return 0, err
	}

	var size int = 0
	for i := 0; i < byteSize; i++ {
		size = (size << 8) | int(b[i])
	}

	// Guard against malicious length prefixes: a length can never exceed the
	// number of bytes left in the buffer (each list element / byte consumes at
	// least one byte). Reject here, before any caller allocates based on it, so
	// an attacker-controlled length field cannot trigger a huge allocation/OOM.
	remaining := bb.Remaining()
	if size > remaining {
		bytesStr := "bytes"
		if remaining == 1 {
			bytesStr = "byte"
		}
		return 0, fmt.Errorf("deser: length %d exceeds remaining buffer %d %s", size, remaining, bytesStr)
	}

	return size, nil
}

func (bb *ByteBuffer) GetVarBytes(byteSizeOfSliceLen int) ([]byte, error) {
	size, err := bb.getLen(byteSizeOfSliceLen)
	if err != nil {
		return nil, err
	}

	bs, err := bb.getBytes(size)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, size, size)
	copy(bytes, bs)
	return bytes, nil
}

func (bb *ByteBuffer) Remaining() int {
	return bb.size - bb.position
}
