package types

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/rlp"
)

var (
	prefixOfRlpUint32 = byte(0x84)
	lenOfRlpUint32    = 5
)

type Uint32 uint32

func (u *Uint32) GetValue() uint32 {
	return uint32(*u)
}

func (u *Uint32) EncodeRLP(w io.Writer) error {
	bytes := make([]byte, lenOfRlpUint32)
	bytes[0] = prefixOfRlpUint32
	binary.BigEndian.PutUint32(bytes[1:], uint32(*u))
	_, err := w.Write(bytes)
	return err
}

func (u *Uint32) DecodeRLP(s *rlp.Stream) error {
	data, err := s.Raw()
	if err != nil {
		return err
	}
	if len(data) != lenOfRlpUint32 {
		return fmt.Errorf("len is %v should %v", len(data), lenOfRlpUint32)
	}

	if data[0] != prefixOfRlpUint32 {
		return fmt.Errorf("preString is wrong, is %v should %v", data[0], lenOfRlpUint32)

	}

	*u = Uint32(binary.BigEndian.Uint32(data[1:]))
	return nil
}
