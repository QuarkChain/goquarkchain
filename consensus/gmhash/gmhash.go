//+build gm

package gmhash

import (
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/crypto"
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

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (g *GmSm3Hash) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (g *GmSm3Hash) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) (types.IBlock, error) {
	panic(errors.New("not finalize"))
}

func hashAlgo(cache *consensus.ShareCache) error {
	copy(cache.Seed, cache.Hash)
	// Note it's big endian here
	binary.BigEndian.PutUint64(cache.Seed[32:], cache.Nonce)

	hashOnce := crypto.SM3Hash(cache.Seed)
	result := crypto.SM3Hash(hashOnce[:])
	cache.Result = result[:]
	return nil
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
	minerRes := consensus.ShareCache{
		Hash:  header.SealHash().Bytes(),
		Seed:  make([]byte, 40),
		Nonce: header.GetNonce(),
	}
	_ = hashAlgo(&minerRes)
	if new(big.Int).SetBytes(minerRes.Result).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}

// New returns a GmSm3Hash scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool) *GmSm3Hash {
	spec := consensus.MiningSpec{
		Name:       config.PoWGmhash,
		HashAlgo:   hashAlgo,
		VerifySeal: verifySeal,
	}
	return &GmSm3Hash{
		CommonEngine: consensus.NewCommonEngine(spec, diffCalculator, remote),
	}
}
