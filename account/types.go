package account

import (
	"errors"
)

var (
	//ErrGenIdentityKey error info : err generate identity key
	ErrGenIdentityKey = errors.New("ErrGenIdentityKey")
)

//ShardKeyType shardKey type uint32
type ShardKeyType uint32

//DefaultKeyStoreDirectory default keystore dir
const DefaultKeyStoreDirectory = "./keystore/"

const (
	//RecipientLength recipient length
	RecipientLength = 20
	//KeyLength key length
	KeyLength = 32
)

//RecipientType recipient type
type RecipientType [RecipientLength]byte

//SetBytes set bytes to it's value
func (a *RecipientType) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-RecipientLength:]
	}
	copy(a[RecipientLength-len(b):], b)
}

//Bytes return it's bytes
func (a RecipientType) Bytes() []byte {
	return a[:]
}

//BytesToIdentityRecipient trans bytes to RecipientType
func BytesToIdentityRecipient(b []byte) RecipientType {
	var a RecipientType
	a.SetBytes(b)
	return a
}

//KeyType key type
type KeyType [KeyLength]byte

//SetBytes set bytes to it's value
func (a *KeyType) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-KeyLength:]
	}
	copy(a[KeyLength-len(b):], b)
}

//Bytes return it's bytes
func (a KeyType) Bytes() []byte {
	return a[:]
}

//BytesToIdentityKey trans bytes to KeyType
func BytesToIdentityKey(b []byte) KeyType {
	var a KeyType
	a.SetBytes(b)
	return a
}
