package serialize

import (
	"fmt"
)

type ByteBuffer struct {
	data     []byte
	position int
}

func (bb *ByteBuffer) checkSpace(space int) error {
	if space > len(bb.data)-bb.position {
		return fmt.Errorf("deser: buffer is shorter than expected")
	}

	return nil
}

func (bb *ByteBuffer) getBytes(size int) ([]byte, error) {
	err := bb.checkSpace(size)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, size, size)
	copy(bytes, bb.data[bb.position:bb.position+size])
	bb.position += size
	return bytes, nil
}

func (bb *ByteBuffer) getUInt8() (uint8, error) {
	bytes, err := bb.getBytes(1)
	if err != nil {
		return 0, err
	}

	return uint8(bytes[0]), nil
}

func (bb *ByteBuffer) getUInt16() (uint16, error) {
	bytes, err := bb.getBytes(2)
	if err != nil {
		return 0, err
	}
	var val uint16 = 0
	for _, b := range bytes {
		val = val<<8 | uint16(b)
	}

	return val, nil
}

func (bb *ByteBuffer) getUInt32() (uint32, error) {
	bytes, err := bb.getBytes(4)
	if err != nil {
		return 0, err
	}
	var val uint32 = 0
	for _, b := range bytes {
		val = val<<8 | uint32(b)
	}

	return val, nil
}

func (bb *ByteBuffer) getUInt64() (uint64, error) {
	bytes, err := bb.getBytes(8)
	if err != nil {
		return 0, err
	}

	var val uint64 = 0
	for _, b := range bytes {
		val = val<<8 | uint64(b)
	}

	return val, nil
}

func (bb *ByteBuffer) getLen(byteSize int) (int, error) {
	if byteSize < 1 {
		return 0, fmt.Errorf("deser: bytesize in getVarBytes should larger than 0")
	}

	b, err := bb.getBytes(byteSize)
	if err != nil {
		return 0, err
	}

	var size int = 0
	for i := 0; i < byteSize; i++ {
		size = (size << 8) | int(b[i])
	}

	return size, nil
}

func (bb *ByteBuffer) getVarBytes(byteSize int) ([]byte, error) {
	size, err := bb.getLen(byteSize)
	if err != nil {
		return nil, err
	}

	return bb.getBytes(size)
}

func (bb *ByteBuffer) remaining() int {
	return len(bb.data) - bb.position
}
