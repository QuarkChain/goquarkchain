package core

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// DiffCalc calc diff interface
type DiffCalc interface {
	CalculateDiff()
	CalculateDiffWithParent(types.IHeader, uint64) (uint64, error)
}

// EthDifficultyCalculator Using metropolis or homestead algorithm (check_uncle=True or False)
type EthDifficultyCalculator struct {
	cutoff      uint64
	diffFactor  uint64
	minimumDiff int64
}

//CalculateDiff calculate only for demo
func (e *EthDifficultyCalculator) CalculateDiff() {
	panic("Not Implemented")
}

//CalculateDiffWithParent calc diff with preHeader
func (e *EthDifficultyCalculator) CalculateDiffWithParent(preHeader types.IHeader, createTime uint64) (uint64, error) {
	if preHeader.GetTime() >= createTime {
		return 0, errors.New("time is not match")
	}
	t := (createTime - preHeader.GetTime()) / e.cutoff
	var sign int64
	if int64(1-t) > -99 {
		sign = int64(1 - t)
	} else {
		sign = -99
	}
	offset := preHeader.GetDifficulty().Uint64() / e.diffFactor
	if int64(preHeader.GetDifficulty().Uint64())+int64(offset)*int64(sign) < e.minimumDiff {
		return uint64(e.minimumDiff), nil
	}
	return uint64(preHeader.GetDifficulty().Uint64() + uint64(int64(offset)*int64(sign))), nil

}

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
