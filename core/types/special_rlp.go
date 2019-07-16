package types

import (
	"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
)

var (
	preStrForRlpUint32 = byte(0x84)
	NewRlpForUint32Len = 5
)

type Uint32 uint32

func (u *Uint32) getValue() uint32 {
	return uint32(*u)
}

func (u *Uint32) EncodeRLP(w io.Writer) error {
	bytes := make([]byte, NewRlpForUint32Len)
	bytes[0] = preStrForRlpUint32
	binary.BigEndian.PutUint32(bytes[1:], uint32(*u))
	_, err := w.Write(bytes)
	return err
}

func (u *Uint32) DecodeRLP(s *rlp.Stream) error {
	data, err := s.Raw()
	if err != nil {
		return err
	}
	if len(data) != NewRlpForUint32Len {
		return fmt.Errorf("len is %v should %v", len(data), NewRlpForUint32Len)
	}

	if data[0] != preStrForRlpUint32 {
		return fmt.Errorf("preString is wrong, is %v should %v", data[0], preStrForRlpUint32)

	}

	*u = Uint32(binary.BigEndian.Uint32(data[1:]))
	return nil
}
