package serialize

import (
	"encoding/binary"
	"fmt"
)

type ByteBuffer struct {
	data     []byte
	position int
}

func NewByteBuffer(bytes []byte) *ByteBuffer {
	bb := ByteBuffer{bytes, 0}
	return &bb
}

func (bb ByteBuffer) GetOffset() int {
	return bb.position
}

func (bb *ByteBuffer) checkSpace(space int) error {
	if space > len(bb.data)-bb.position {
		return fmt.Errorf("deser: buffer is shorter than expected")
	}

	return nil
}

func (bb *ByteBuffer) GetBytes(size int) ([]byte, error) {
	err := bb.checkSpace(size)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, size, size)
	copy(bytes, bb.data[bb.position:bb.position+size])
	bb.position += size
	return bytes, nil
}

func (bb *ByteBuffer) GetUInt8() (uint8, error) {
	bytes, err := bb.GetBytes(1)
	if err != nil {
		return 0, err
	}

	return uint8(bytes[0]), nil
}

func (bb *ByteBuffer) GetUInt16() (uint16, error) {
	bytes, err := bb.GetBytes(2)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint16(bytes), nil
}

func (bb *ByteBuffer) GetUInt32() (uint32, error) {
	bytes, err := bb.GetBytes(4)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint32(bytes), nil
}

func (bb *ByteBuffer) GetUInt64() (uint64, error) {
	bytes, err := bb.GetBytes(8)
	if err != nil {
		return 0, err
	}

	return binary.BigEndian.Uint64(bytes), nil
}

func (bb *ByteBuffer) getLen(byteSize int) (int, error) {
	if byteSize < 1 {
		return 0, fmt.Errorf("deser: bytesize in GetVarBytes should larger than 0")
	}

	b, err := bb.GetBytes(byteSize)
	if err != nil {
		return 0, err
	}

	var size int = 0
	for i := 0; i < byteSize; i++ {
		size = (size << 8) | int(b[i])
	}

	return size, nil
}

func (bb *ByteBuffer) GetVarBytes(byteSizeOfSliceLen int) ([]byte, error) {
	size, err := bb.getLen(byteSizeOfSliceLen)
	if err != nil {
		return nil, err
	}

	return bb.GetBytes(size)
}

func (bb *ByteBuffer) Remaining() int {
	return len(bb.data) - bb.position
}
