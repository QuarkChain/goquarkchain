// Modified from go-ethereum under GNU Lesser General Public License

package types

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

var (
	ErrInvalidNetworkId = errors.New("invalid network id for signer")
)

// sigCache is used to cache the derived sender and contains
// the signer used to derive it.
type sigCache struct {
	signer Signer
	from   account.Recipient
}

// MakeSigner returns a Signer based on the given chain config and block number.
func MakeSigner(networkId uint32) Signer {
	return NewEIP155Signer(networkId)
}

// SignTx signs the transaction using the given signer and private key
func SignTx(tx *EvmTransaction, s Signer, prv *ecdsa.PrivateKey) (*EvmTransaction, error) {
	h := s.Hash(tx)
	sig, err := crypto.Sign(h[:], prv)
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(s, sig)
}

// Sender returns the address derived from the signature (V, R, S) using secp256k1
// elliptic curve and an error if it failed deriving or upon an incorrect
// signature.
//
// Sender may cache the address, allowing it to be used regardless of
// signing method. The cache is invalidated if the cached signer does
// not match the signer used in the current call.
func Sender(signer Signer, tx *EvmTransaction) (account.Recipient, error) {
	if sc := tx.from.Load(); sc != nil {
		sigCache := sc.(sigCache)
		// If the signer used to derive from in a previous
		// call is not the same as used current, invalidate
		// the cache.
		if sigCache.signer.Equal(signer) {
			return sigCache.from, nil
		}
	}

	addr, err := signer.Sender(tx)
	if err != nil {
		return account.Recipient{}, err
	}
	tx.from.Store(sigCache{signer: signer, from: addr})
	return addr, nil
}

// Signer encapsulates transaction signature handling. Note that this interface is not a
// stable API and may change at any time to accommodate new protocol rules.
type Signer interface {
	// Sender returns the sender address of the transaction.
	Sender(tx *EvmTransaction) (account.Recipient, error)
	// SignatureValues returns the raw R, S, V values corresponding to the
	// given signature.
	SignatureValues(tx *EvmTransaction, sig []byte) (r, s, v *big.Int, err error)
	// Hash returns the hash to be signed.
	Hash(tx *EvmTransaction) common.Hash
	// Equal returns true if the given signer is the same as the receiver.
	Equal(Signer) bool
}

// EIP155Transaction implements Signer using the EIP155 rules.
type EIP155Signer struct {
	networkId uint32
}

func NewEIP155Signer(networkId uint32) EIP155Signer {
	return EIP155Signer{
		networkId: networkId,
	}
}

func (s EIP155Signer) Equal(s2 Signer) bool {
	eip155, ok := s2.(EIP155Signer)
	return ok && eip155.networkId == s.networkId
}

func (s EIP155Signer) Sender(tx *EvmTransaction) (account.Recipient, error) {
	if tx.NetworkId() != s.networkId {
		return account.Recipient{}, ErrInvalidNetworkId
	}

	if tx.data.Version == 0 {
		return recoverPlain(tx.getUnsignedHash(), tx.data.R, tx.data.S, tx.data.V, true)
	} else if tx.data.Version == 1 {
		//todo
		return account.Recipient{}, fmt.Errorf("Version %d is not implemented yet", tx.data.Version)
	} else {
		return account.Recipient{}, fmt.Errorf("Version %d is not suppot", tx.data.Version)
	}
}

// SignatureValues returns signature values. This signature
// needs to be in the [R || S || V] format where V is 0 or 1.
func (s EIP155Signer) SignatureValues(tx *EvmTransaction, sig []byte) (R, S, V *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	R = new(big.Int).SetBytes(sig[:32])
	S = new(big.Int).SetBytes(sig[32:64])
	V = new(big.Int).SetBytes([]byte{sig[64] + 27})

	return R, S, V, nil
}

// Hash returns the hash to be signed by the sender.
// It does not uniquely identify the transaction.
func (s EIP155Signer) Hash(tx *EvmTransaction) common.Hash {
	return tx.getUnsignedHash()
}

func recoverPlain(sighash common.Hash, R, S, Vb *big.Int, homestead bool) (account.Recipient, error) {
	if Vb.BitLen() > 8 {
		return account.Recipient{}, ErrInvalidSig
	}
	// QuarkChain use NetworkId to store the chain Id instead of added to V,
	// so do not need to remove chain Id from VB
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, homestead) {
		return account.Recipient{}, ErrInvalidSig
	}
	// encode the signature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the signature
	pub, err := crypto.Ecrecover(sighash[:], sig)
	if err != nil {
		return account.Recipient{}, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return account.Recipient{}, errors.New("invalid public key")
	}
	var addr account.Recipient
	copy(addr[:], crypto.Keccak256(pub[1:])[12:])
	return addr, nil
}
