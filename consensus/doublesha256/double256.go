package doublesha256

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/big"
	"sync"

	"github.com/ethereum/go-ethereum/common"

	"github.com/QuarkChain/goquarkchain/account"
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

	closeOnce sync.Once
}

// Author returns coinbase address.
func (d *DoubleSHA256) Author(header types.IHeader) (account.Address, error) {
	return header.GetCoinbase(), nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (d *DoubleSHA256) Prepare(chain consensus.ChainReader, header types.IHeader) error {
	panic("not implemented")
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (d *DoubleSHA256) Seal(
	chain consensus.ChainReader,
	block types.IBlock,
	results chan<- types.IBlock,
	stop <-chan struct{}) error {
	if d.CommonEngine.IsRemoteMining() {
		d.CommonEngine.SetWork(block, results)
		return nil
	}
	return d.CommonEngine.Seal(block, results, stop)
}

func (d *DoubleSHA256) Finalize(chain consensus.ChainReader, header types.IHeader, state *state.StateDB, txs []*types.Transaction, receipts []*types.Receipt) (types.IBlock, error) {
	panic(errors.New("not finalize"))
}

func hashAlgo(hash []byte, nonce uint64) (consensus.MiningResult, error) {
	nonceBytes := make([]byte, 8)
	// Note it's big endian here
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	hashNonceBytes := append(hash, nonceBytes...)

	hashOnce := sha256.Sum256(hashNonceBytes)
	resultArray := sha256.Sum256(hashOnce[:])
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
	if adjustedDiff.Cmp(new(big.Int).SetUint64(0)) == 0 {
		adjustedDiff = header.GetDifficulty()
	}

	target := new(big.Int).Div(two256, adjustedDiff)
	miningRes, _ := hashAlgo(header.SealHash().Bytes(), header.GetNonce())
	if new(big.Int).SetBytes(miningRes.Result).Cmp(target) > 0 {
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
