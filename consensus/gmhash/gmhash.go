//+build gm

package gmhash

import (
	"encoding/binary"
	"github.com/ethereum/go-ethereum/common"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/crypto"
	"github.com/QuarkChain/goquarkchain/core/types"
	"math/big"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

// GmSm3Hash is a consensus engine implementing PoW with Gm Sm3 algo.
// See the interface definition:
// Implements consensus.Pow
type GmSm3Hash struct {
	*consensus.CommonEngine
}

func hashAlgo(height uint64, hash []byte, nonce uint64) (consensus.MiningResult, error) {
	seed := make([]byte, 40)
	copy(seed, hash)
	binary.LittleEndian.PutUint64(seed[32:], nonce)

	hashOnce := crypto.SM3Hash(seed)
	resultArray := crypto.SM3Hash(hashOnce[:])
	return consensus.MiningResult{
		Digest: common.Hash{},
		Result: resultArray[:],
		Nonce:  nonce,
	}, nil
}

func verifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
	if header.GetDifficulty().Sign() <= 0 {
		return consensus.ErrInvalidDifficulty
	}
	diff := adjustedDiff
	if diff == nil || diff.Cmp(big.NewInt(0)) == 0 {
		diff = header.GetDifficulty()
	}

	target := new(big.Int).Div(two256, diff)
	miningRes, _ := hashAlgo(0 /* not used */, header.SealHash().Bytes(), header.GetNonce())
	if new(big.Int).SetBytes(miningRes.Result).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}

// New returns a GmSm3Hash scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool) *GmSm3Hash {
	spec := consensus.MiningSpec{
		Name:       "GmSm3Hash",
		HashAlgo:   hashAlgo,
		VerifySeal: verifySeal,
	}
	return &GmSm3Hash{
		CommonEngine: consensus.NewCommonEngine(spec, diffCalculator, remote),
	}
}
