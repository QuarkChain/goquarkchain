package account

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

// Address include recipient and fullShardKey
type Address struct {
	Recipient    Recipient
	FullShardKey uint32
}

// NewAddress new address with recipient and fullShardKey
func NewAddress(recipient Recipient, fullShardKey uint32) Address {
	return Address{
		Recipient:    recipient,
		FullShardKey: fullShardKey,
	}
}

// ToHex return bytes included recipient and fullShardKey
func (Self Address) ToHex() string {
	address := Self.ToBytes()
	return hexutil.Encode(address)

}

func (Self Address) ToBytes() []byte {
	address := Self.Recipient.Bytes()
	shardKey := Uint32ToBytes(Self.FullShardKey)
	address = append(address, shardKey...)
	return address
}

// GetFullShardID get fullShardID depend shardSize
func (Self *Address) GetFullShardID(shardSize uint32) (uint32, error) {
	if common.IsP2(shardSize) == false {
		return 0, fmt.Errorf("shardSize is not right shardSize:%d", shardSize)
	}

	chainID := Self.FullShardKey >> 16
	shardID := Self.FullShardKey & (shardSize - 1)
	return uint32(chainID<<16 | shardSize | shardID), nil
}

func (self *Address) GetChainID() uint32 {
	return self.FullShardKey >> 16
}

// AddressInShard return address depend new fullShardKey
func (Self *Address) AddressInShard(fullShardKey uint32) Address {
	return NewAddress(Self.Recipient, fullShardKey)
}

// AddressInBranch return address depend new branch
func (Self *Address) AddressInBranch(branch Branch) Address {
	shardKey := Self.FullShardKey & ((1 << 16) - 1)
	newShardKey := (shardKey & ^(branch.GetShardSize() - 1)) + branch.GetShardID()
	newFullShardKey := branch.GetChainID()<<16 | newShardKey
	return NewAddress(Self.Recipient, newFullShardKey)
}

// CreatAddressFromIdentity creat address from identity
func CreatAddressFromIdentity(identity Identity, fullShardKey uint32) Address {
	return NewAddress(identity.recipient, fullShardKey)
}

// CreatRandomAccountWithFullShardKey creat random account with fullShardKey
func CreatRandomAccountWithFullShardKey(fullShardKey uint32) (Address, error) {
	identity, err := CreatRandomIdentity()
	if err != nil {
		return Address{}, err
	}
	return CreatAddressFromIdentity(identity, fullShardKey), nil
}

// CreatRandomAccountWithoutFullShardKey creat random account without fullShardKey
func CreatRandomAccountWithoutFullShardKey() (Address, error) {
	identity, err := CreatRandomIdentity()
	if err != nil {
		return Address{}, err
	}

	defaultFullShardKey, err := identity.GetDefaultFullShardKey()
	if err != nil {
		return Address{}, err
	}
	return CreatAddressFromIdentity(identity, defaultFullShardKey), nil
}

// CreatEmptyAddress creat empty address from fullShardKey
func CreatEmptyAddress(fullShardKey uint32) Address {
	zeroBytes := make([]byte, RecipientLength)
	recipient := BytesToIdentityRecipient(zeroBytes)
	return NewAddress(recipient, fullShardKey)
}

// CreatAddressFromBytes creat address from bytes
func CreatAddressFromBytes(bs []byte) (Address, error) {
	if len(bs) != RecipientLength+FullShardKeyLength {
		return Address{}, fmt.Errorf("bs length excepted %d,unexcepted %d", RecipientLength+FullShardKeyLength, len(bs))
	}

	buffer := bytes.NewBuffer(bs[RecipientLength:])
	var x uint32
	err := binary.Read(buffer, binary.BigEndian, &x)
	if err != nil {
		return Address{}, err
	}
	recipient := BytesToIdentityRecipient(bs[0:RecipientLength])
	return NewAddress(recipient, x), nil
}

// IsEmpty check address is empty
func (Self *Address) IsEmpty() bool {
	zero := make([]byte, RecipientLength)
	return bytes.Equal(zero, Self.Recipient.Bytes())
}
