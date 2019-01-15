package account

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

//Address include recipient and fullShardKey
type Address struct {
	Recipient    RecipientType
	FullShardKey ShardKeyType
}

//NewAddress new address with recipient and fullShardKey
func NewAddress(recipient RecipientType, fullShardKey ShardKeyType) Address {
	return Address{
		Recipient:    recipient,
		FullShardKey: fullShardKey,
	}
}

//ToHex return bytes included recipient and fullShardKey
func (Self *Address) ToHex() []byte {
	address := Self.Recipient.Bytes()
	shardKey := Uint32ToBytes(uint32(Self.FullShardKey))
	address = append(address, shardKey...)
	return address
}

//GetFullShardID get fullShardID depend shardSize
func (Self *Address) GetFullShardID(shardSize ShardKeyType) (uint32, error) {
	if IsP2(shardSize) == false {
		return 0, fmt.Errorf("shardSize is not right shardSize:%d", shardSize)
	}
	chainID := Self.FullShardKey >> 16
	shardID := Self.FullShardKey & (shardSize - 1)
	return uint32(chainID<<16 | shardSize | shardID), nil
}

//AddressInShard return address depend new fullShardKey
func (Self *Address) AddressInShard(fullShardKey ShardKeyType) Address {
	return NewAddress(Self.Recipient, fullShardKey)
}

//AddressInBranch return address depend new branch
func (Self *Address) AddressInBranch(branch Branch) Address {
	shardKey := Self.FullShardKey & ((1 << 16) - 1)
	newShardKey := (shardKey & ^(branch.GetShardSize() - 1)) + branch.GetShardID()
	newFullShardKey := branch.GetChainID()<<16 | newShardKey
	return NewAddress(Self.Recipient, newFullShardKey)
}

//CreatAddressFromIdentity creat address from identity
func CreatAddressFromIdentity(identity Identity, fullShardKey ShardKeyType) Address {
	return NewAddress(identity.Recipient, fullShardKey)

}

//CreatRandomAccountWithFullShardKey creat random account with fullShardKey
func CreatRandomAccountWithFullShardKey(fullShardKey ShardKeyType) (Address, error) {
	identity, err := CreatRandomIdentity()
	if err != nil {
		return Address{}, err
	}
	return CreatAddressFromIdentity(identity, fullShardKey), nil
}

//CreatRandonAccountWitouthFullShardKey creat random account without fullShardKey
func CreatRandonAccountWitouthFullShardKey() (Address, error) {
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

//CreatEmptyAddress creat empty address from fullShardKey
func CreatEmptyAddress(fullShardKey ShardKeyType) Address {
	bytes := make([]byte, 20)
	return NewAddress(BytesToIdentityRecipient(bytes), fullShardKey)
}

//CreatAddressFromBS creat address from bytes
func CreatAddressFromBS(bs []byte) (Address, error) {
	if len(bs) != 24 {
		return Address{}, errors.New("bs length is not 24")
	}
	buffer := bytes.NewBuffer(bs[RecipientLength:])
	var x uint32
	err := binary.Read(buffer, binary.BigEndian, &x)
	if err != nil {
		return Address{}, err
	}
	return NewAddress(BytesToIdentityRecipient(bs[0:RecipientLength]), ShardKeyType(x)), nil
}

//IsEmpty check address is empty
func (Self *Address) IsEmpty() bool {
	zero := make([]byte, RecipientLength)
	return bytes.Equal(zero, Self.Recipient.Bytes())

}
