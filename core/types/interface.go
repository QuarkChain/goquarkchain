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
	GetParentHash() common.Hash
	GetCoinbase() account.Address
	GetTime() uint64
	GetCoinbaseAmount() *TokenBalanceMap
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
	WithMingResult(nonce uint64, mixDigest common.Hash) IBlock
	Content() []IHashable
	GetTrackingData() []byte
	GetSize() common.StorageSize
}

type IHashable interface {
	Hash() common.Hash
}
