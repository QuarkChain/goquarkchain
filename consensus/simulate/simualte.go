package simulate

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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

func hashAlgo(cache *consensus.ShareCache) error {
	for true {
		time.Sleep(100 * time.Millisecond)
		if uint64(time.Now().Unix()) > cache.BlockTime {
			break
		}
	}
	cache.Result = make([]byte, 0)
	digest, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return err
	}
	cache.Digest = common.BigToHash(digest).Bytes()
	fmt.Println("?????", hex.EncodeToString(cache.Digest))
	return nil
}

func verifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	return nil
}

// New returns a DoubleSHA256 scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool, pubKey []byte, blockTime uint64) *PowSimulate {
	spec := consensus.MiningSpec{
		Name:       config.PoWSimulate,
		HashAlgo:   hashAlgo,
		VerifySeal: verifySeal,
		BlockTime:  blockTime,
	}
	return &PowSimulate{
		CommonEngine: consensus.NewCommonEngine(spec, diffCalculator, remote, pubKey),
	}
}
