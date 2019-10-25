package simulate

import (
	"crypto/rand"
	"errors"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"math"
	"math/big"
	"time"
)

type PowSimulate struct {
	*consensus.CommonEngine
}

func (p *PowSimulate) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (p *PowSimulate) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) (types.IBlock, error) {
	panic(errors.New("not finalize"))
}

func (p *PowSimulate) RefreshWork(tip uint64) {
	p.CommonEngine.RefreshWork(tip)
}

func (p *PowSimulate) hashAlgo(cache *consensus.ShareCache) error {
	timeAfterCreateTime := uint64(time.Now().Unix()) - cache.BlockTime
	needSleepTime := uint64(0)
	intervalBlockTime := p.CommonEngine.GetIntervalBlockTime()
	if intervalBlockTime > 2*timeAfterCreateTime {
		needSleepTime = intervalBlockTime - 2*timeAfterCreateTime
	}
	time.Sleep(time.Duration(needSleepTime) * time.Second)
	cache.Result = make([]byte, 0)
	digest, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return err
	}
	cache.Digest = common.BigToHash(digest).Bytes()
	return nil
}

func verifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	return nil
}

func New(diffCalculator consensus.DifficultyCalculator, remote bool, pubKey []byte, intervalBlockTime uint64) *PowSimulate {
	simualte := &PowSimulate{}
	spec := consensus.MiningSpec{
		Name:       config.PoWSimulate,
		HashAlgo:   simualte.hashAlgo,
		VerifySeal: verifySeal,
		BlockTime:  intervalBlockTime,
	}
	simualte.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey)
	return simualte
}
