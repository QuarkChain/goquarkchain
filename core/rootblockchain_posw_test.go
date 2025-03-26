package core

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus/doublesha256"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/stretchr/testify/assert"
)

var (
	EpochInterval = uint64(8)
)

type fackCalculator struct {
}

func (c *fackCalculator) CalculateDifficulty(parent types.IBlock, time uint64) (*big.Int, error) {
	return parent.Difficulty(), nil
}

func appendNewRootBlock(blockchain *RootBlockChain, acc1 account.Address, t *testing.T) (*types.RootBlock, *big.Int) {
	newBlock, err := blockchain.CreateBlockToMine(make([]*types.MinorBlockHeader, 0), &acc1, nil)
	assert.NoError(t, err, fmt.Sprintf("failed to CreateBlockToMine: %v", err))

	data := make(map[uint64]*big.Int)
	data[testGenesisTokenID] = new(big.Int).Mul(big.NewInt(200), config.QuarkashToJiaozi)
	tokenBalance := types.NewTokenBalancesWithMap(data)

	stakePreBlock := blockchain.GetTotalStakePerBlock(newBlock.NumberU64(), newBlock.Time())
	adjustedDiff, err := blockchain.GetPoSW().PoSWDiffAdjust(newBlock.Header(), tokenBalance.GetTokenBalance(testGenesisTokenID), stakePreBlock)
	assert.NoError(t, err, fmt.Sprintf("failed to adjust posw diff: %v", err))

	return newBlock, adjustedDiff
}

func TestPoSWForRootChain(t *testing.T) {
	var (
		addr0        = account.Address{Recipient: account.Recipient{1}, FullShardKey: 0}
		db           = ethdb.NewMemDatabase()
		qkcConfig    = config.NewQuarkChainConfig()
		genesis      = NewGenesis(qkcConfig)
		genesisBlock = genesis.MustCommitRootBlock(db)
		diffCalc     = &fackCalculator{}
		engine       = doublesha256.New(diffCalc, false, []byte{})
	)

	qkcConfig.Root.PoSWConfig.Enabled = true
	qkcConfig.Root.PoSWConfig.EnableTimestamp = 100
	qkcConfig.Root.PoSWConfig.WindowSize = 4
	qkcConfig.Root.EpochInterval = EpochInterval
	qkcConfig.Root.PoSWConfig.TotalStakePerBlock = new(big.Int).Mul(big.NewInt(100), config.QuarkashToJiaozi)
	qkcConfig.EnableRootPoswStakingDecayTimestamp = 100

	// Import the chain. This runs all block validation rules.
	blockchain, err := NewRootBlockChain(db, qkcConfig, engine)
	if err != nil {
		fmt.Printf("new root block chain error %v\n", err)
		return
	}
	defer blockchain.Stop()

	blockchain.SetValidator(&fakeRootBlockValidator{nil})
	//
	expectedAdjested := []uint64{1000, 1000, 1000000, 1000000, 1000000, 1000000, 1000000, 1000, 1000, 1000, 1000, 1000}
	block := genesisBlock
	var adjustedDiff *big.Int
	for i := 0; i < 12; i++ {
		block, adjustedDiff = appendNewRootBlock(blockchain, addr0, t)
		assert.Equal(t, expectedAdjested[i], adjustedDiff.Uint64(), fmt.Sprintf("adjusted difficulty not equal for %d, expected %d, adjested %d", i, expectedAdjested[i], adjustedDiff.Uint64()))
		if i, err := blockchain.InsertChain([]types.IBlock{block}); err != nil {
			fmt.Printf("insert error (block %d): %s\n", i+1, err.Error())
			return
		}
	}
}
