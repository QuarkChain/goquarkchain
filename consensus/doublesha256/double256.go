package doublesha256

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

// DoubleSHA256 is a consensus engine implementing PoW with double-sha256 algo.
// See the interface definition:
// Implements consensus.Pow
type DoubleSHA256 struct {
	*consensus.CommonEngine
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (d *DoubleSHA256) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

func (d *DoubleSHA256) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) (types.IBlock, error) {
	panic(errors.New("not finalize"))
}

func hashAlgo(height uint64, hash []byte, nonce uint64) ([]byte, []byte, error) {
	nonceBytes := make([]byte, 8)
	// Note it's big endian here
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	hashNonceBytes := append(hash, nonceBytes...)

	hashOnce := sha256.Sum256(hashNonceBytes)
	resultArray := sha256.Sum256(hashOnce[:])
	return []byte{}, resultArray[:], nil
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
	_, result, _ := hashAlgo(0 /* not used */, header.SealHash().Bytes(), header.GetNonce())
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}

// New returns a DoubleSHA256 scheme.
func New(diffCalculator consensus.DifficultyCalculator, remote bool) *DoubleSHA256 {
	spec := consensus.MiningSpec{
		Name:       "DoubleSHA256",
		HashAlgo:   hashAlgo,
		VerifySeal: verifySeal,
	}
	return &DoubleSHA256{
		CommonEngine: consensus.NewCommonEngine(spec, diffCalculator, remote),
	}
}
