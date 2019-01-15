package account

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"github.com/ethereum/go-ethereum/crypto"
	"strings"
)

//Identity include recipient and key
type Identity struct {
	Recipient RecipientType
	Key       KeyType
}

//NewIdentity new identity include recipient and key
func NewIdentity(recipient RecipientType, key KeyType) Identity {
	return Identity{
		Recipient: recipient,
		Key:       key,
	}
}

//CreatRandomIdentity create a random identity
func CreatRandomIdentity() (Identity, error) {
	sk, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		return Identity{}, ErrGenIdentityKey
	}
	key := crypto.FromECDSA(sk)
	if len(key) != KeyLength {
		return Identity{}, fmt.Errorf("privateKey To Bytes falied: unexceptd %d ,excepted 32", len(key))
	}
	if len(crypto.FromECDSAPub(&sk.PublicKey)) != 2*KeyLength+1 {
		return Identity{}, fmt.Errorf("fromECDSAPub len is not match :unexcepted %d,excepted 65", len(crypto.FromECDSAPub(&sk.PublicKey)))
	}
	recipient := crypto.Keccak256(crypto.FromECDSAPub(&sk.PublicKey)[1:])
	return NewIdentity(BytesToIdentityRecipient(recipient[(len(recipient)-RecipientLength):]), BytesToIdentityKey(key)), nil
}

//CreatIdentityFromKey creat identity from key
func CreatIdentityFromKey(key KeyType) (Identity, error) {
	keys := key.Bytes()
	realKey := make([]byte, 8)
	realKey = append(realKey, keys...)
	sk, err := ecdsa.GenerateKey(crypto.S256(), strings.NewReader(string(realKey)))
	if err != nil {
		return Identity{}, ErrGenIdentityKey
	}
	if len(crypto.FromECDSAPub(&sk.PublicKey)) != 2*KeyLength+1 {
		return Identity{}, fmt.Errorf("fromECDSAPub len is not match :unexcepted %d,excepted %d", len(crypto.FromECDSAPub(&sk.PublicKey)), 2*KeyLength+1)
	}
	recipient := crypto.Keccak256(crypto.FromECDSAPub(&sk.PublicKey)[1:]) //"0x04"+64
	if len(recipient) != KeyLength {
		return Identity{}, fmt.Errorf("recipient len is not match:unexceptd %d,exceptd 65", len(recipient))
	}
	return NewIdentity(BytesToIdentityRecipient(recipient[len(recipient)-RecipientLength:]), BytesToIdentityKey(key.Bytes())), nil
}

//GetDefaultFullShardKey get identity's default fullShardKey
func (Self *Identity) GetDefaultFullShardKey() (ShardKeyType, error) {
	var fullShardKey ShardKeyType
	r := Self.Recipient
	real := []byte{0x00, 0x00}
	real = append(real, r[0:1]...)
	real = append(real, r[10:11]...)

	buffer := bytes.NewBuffer(real)
	err := binary.Read(buffer, binary.BigEndian, &fullShardKey)
	if err != nil {
		return fullShardKey, err
	}
	return fullShardKey, nil
}

//GetRecipient Get it's recipient
func (Self *Identity) GetRecipient() RecipientType {
	return Self.Recipient
}

//GetKey get it's key
func (Self *Identity) GetKey() KeyType {
	return Self.Key
}
