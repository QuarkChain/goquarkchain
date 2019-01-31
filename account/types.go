package account

import (
	"errors"
)

var (
	// ErrGenIdentityKey error info : err generate identity key
	ErrGenIdentityKey = errors.New("ErrGenIdentityKey")
)

<<<<<<< HEAD

// DefaultKeyStoreDirectory default keystore dir
const (
	DefaultKeyStoreDirectory = "./keystore/"
	kdfParamsPrf="prf"
	kdfParamsPrfValue="hmac-sha256"
	kdfParamsPrfDkLen="dklen"
	kdfParamsPrfDkLenValue=32
	kdfParamsC="c"
	kdfParamsCValue=262144
	kdfParamsSalt="salt"

	cryptoKDF="pbkdf2"
	cryptoCipher="aes-128-ctr"
	cryptoVersion=1

	jsonVersion=3

)

const (
	RecipientLength = 20
	KeyLength = 32
	FullShardKeyLength=4
=======
// DefaultKeyStoreDirectory default keystore dir
const (
	DefaultKeyStoreDirectory = "./keystore/"
	kdfParamsPrf             = "prf"
	kdfParamsPrfValue        = "hmac-sha256"
	kdfParamsPrfDkLen        = "dklen"
	kdfParamsPrfDkLenValue   = 32
	kdfParamsC               = "c"
	kdfParamsCValue          = 262144
	kdfParamsSalt            = "salt"

	cryptoKDF     = "pbkdf2"
	cryptoCipher  = "aes-128-ctr"
	cryptoVersion = 1

	jsonVersion = 3
)

const (
	RecipientLength    = 20
	KeyLength          = 32
	FullShardKeyLength = 4
>>>>>>> 08842181cc246dd74f95cd30a4ddc23474a35c80
)

// Recipient recipient type
type Recipient [RecipientLength]byte

// SetBytes set bytes to it's value
<<<<<<< HEAD
func (a *Recipient) SetBytes(b []byte)error {
	if len(b) > len(a) {
		b = b[len(b)-RecipientLength:]
	}
	if len(a)!=len(b){
		return errors.New("recipient SetBytes:length is wrong")
	}
	copy(a[:], b)
	return nil
=======
func (a *Recipient) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-RecipientLength:]
	}
	copy(a[RecipientLength-len(b):], b)
>>>>>>> 08842181cc246dd74f95cd30a4ddc23474a35c80
}

// Bytes return it's bytes
func (a Recipient) Bytes() []byte {
	return a[:]
}

// BytesToIdentityRecipient trans bytes to Recipient
<<<<<<< HEAD
func BytesToIdentityRecipient(b []byte) (Recipient ,error){
	var a Recipient
	err:=a.SetBytes(b)
	return a,err
=======
func BytesToIdentityRecipient(b []byte) Recipient {
	var a Recipient
	a.SetBytes(b)
	return a
>>>>>>> 08842181cc246dd74f95cd30a4ddc23474a35c80
}

// Key key type
type Key [KeyLength]byte

// SetBytes set bytes to it's value
<<<<<<< HEAD
func (a *Key) SetBytes(b []byte)error {
	if len(a)!=len(b){
		return errors.New("key setBytes length is wrong")
	}
	copy(a[:], b)
	return nil
=======
func (a *Key) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-KeyLength:]
	}
	copy(a[KeyLength-len(b):], b)
>>>>>>> 08842181cc246dd74f95cd30a4ddc23474a35c80
}

// Bytes return it's bytes
func (a Key) Bytes() []byte {
	return a[:]
}

// BytesToIdentityKey trans bytes to Key
<<<<<<< HEAD
func BytesToIdentityKey(b []byte) (Key,error) {
	var a Key
	err:=a.SetBytes(b)
	return a,err
=======
func BytesToIdentityKey(b []byte) Key {
	var a Key
	a.SetBytes(b)
	return a
>>>>>>> 08842181cc246dd74f95cd30a4ddc23474a35c80
}
