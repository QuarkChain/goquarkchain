package simulate

import (
	"crypto/rand"
	"errors"
	"math"
	"math/big"
	mrand "math/rand"
	"time"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

type PowSimulate struct {
	*consensus.CommonEngine
	blockInterval uint64
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
	s1 := mrand.NewSource(time.Now().UnixNano())
	diff := mrand.New(s1).Intn(1000)
	diffDuration := time.Duration(diff) * time.Millisecond
	timeAfterCreateTime := uint64(time.Now().Add(diffDuration).Unix()) - cache.BlockTime
	if p.blockInterval > 2*timeAfterCreateTime {
		sleepTime := time.Duration(p.blockInterval-2*timeAfterCreateTime)*time.Second + diffDuration - 500*time.Millisecond
		time.Sleep(sleepTime)
	}

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

func New(diffCalculator consensus.DifficultyCalculator, remote bool, pubKey []byte, blockInterval uint64) *PowSimulate {
	simulate := &PowSimulate{blockInterval: blockInterval}
	spec := consensus.MiningSpec{
		Name:       config.PoWSimulate,
		HashAlgo:   simulate.hashAlgo,
		VerifySeal: verifySeal,
	}

	simulate.CommonEngine = consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey)
	return simulate
}
