package core

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// ConstMinorBlockRewardCalculator blockReward struct
type ConstMinorBlockRewardCalculator struct {
}

// GetBlockReward getBlockReward
func (c *ConstMinorBlockRewardCalculator) GetBlockReward() *big.Int {
	data := new(big.Int).SetInt64(100)
	return new(big.Int).Mul(data, new(big.Int).SetInt64(1000000000000000000))
}

type gasPriceSuggestionOracle struct {
	LastPrice   uint64
	LastHead    common.Hash
	CheckBlocks uint64
	Percentile  uint64
}

// Uint64List sort uint64 slice
type Uint64List []uint64

func (u Uint64List) Len() int {
	return len(u)
}
func (u Uint64List) Less(i, j int) bool {
	return u[i] < u[j]
}
func (u Uint64List) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}

type ShardStatus struct {
	Branch             account.Branch
	Height             uint64
	Difficulty         *big.Int
	CoinBaseAddress    account.Address
	TimeStamp          uint64
	TxCount60s         uint32
	PendingTxCount     uint32
	TotalTxCount       uint32
	BlockCount60s      uint32
	StaleBlockCount60s uint32
	LastBlockTime      uint32
}
