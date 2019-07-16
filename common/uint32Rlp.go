package common

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/serialize"
	"github.com/ethereum/go-ethereum/rlp"
	"io"
)

var (
	preStrForRlpUint32 = []byte{0x84}
	NewRlpForUint32Len = 5
)

type NewRlpForUint32 struct {
	Value uint32
}

func (d *NewRlpForUint32) EncodeRLP(w io.Writer) error {
	bytes := make([]byte, 0, NewRlpForUint32Len)
	bytes = append(bytes, preStrForRlpUint32...)
	valueBytes, err := serialize.SerializeToBytes(d.Value)
	if err != nil {
		return err
	}
	bytes = append(bytes, valueBytes...)
	_, err = w.Write(bytes)
	return err
}

func (d *NewRlpForUint32) DecodeRLP(s *rlp.Stream) error {
	bytes, err := s.Raw()
	if err != nil {
		return err
	}
	if len(bytes) != NewRlpForUint32Len {
		return fmt.Errorf("len is %v should %v", len(bytes), NewRlpForUint32Len)
	}

	var value uint32
	if err := serialize.DeserializeFromBytes(bytes[1:], &value); err != nil {
		return err
	}
	d.Value = value
	return nil
}
