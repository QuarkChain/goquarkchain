package types

import (
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
)

type IHeader interface {
	Hash() common.Hash
	SealHash() common.Hash
	NumberU64() uint64
	GetVersion() uint32
	GetParentHash() common.Hash
	GetCoinbase() account.Address
	GetTime() uint64
	GetCoinbaseAmount() *TokenBalances
	GetDifficulty() *big.Int
	GetTotalDifficulty() *big.Int
	GetNonce() uint64
	GetExtra() []byte
	SetCoinbase(account.Address)
	SetExtra([]byte)
	SetDifficulty(*big.Int)
	SetNonce(uint64)
	GetMixDigest() common.Hash
}

type IBlock interface {
	Hash() common.Hash
	NumberU64() uint64
	IHeader() IHeader
	WithMingResult(nonce uint64, mixDigest common.Hash, signature *[65]byte) IBlock
	Content() []IHashable
	GetTrackingData() []byte
	GetSize() common.StorageSize
	ParentHash() common.Hash
	Coinbase() account.Address
	Time() uint64
}

type IHashable interface {
	Hash() common.Hash
}
