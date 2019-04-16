package doublesha256

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/QuarkChain/goquarkchain/core/state"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

// DoubleSHA256 is a consensus engine implementing PoW with double-sha256 algo.
// See the interface definition:
// Implements consensus.Pow
type DoubleSHA256 struct {
	commonEngine   *consensus.CommonEngine
	diffCalculator consensus.DifficultyCalculator
}

// Author returns coinbase address.
func (d *DoubleSHA256) Author(header types.IHeader) (account.Address, error) {
	return header.GetCoinbase(), nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (d *DoubleSHA256) VerifyHeader(chain consensus.ChainReader, header types.IHeader, seal bool) error {
	return d.commonEngine.VerifyHeader(chain, header, seal, d)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (d *DoubleSHA256) VerifyHeaders(chain consensus.ChainReader, headers []types.IHeader, seals []bool) (chan<- struct{}, <-chan error) {
	return d.commonEngine.VerifyHeaders(chain, headers, seals, d)
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (d *DoubleSHA256) VerifySeal(chain consensus.ChainReader, header types.IHeader, adjustedDiff *big.Int) error {
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

	return d.commonEngine.Seal(block, results, stop)
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (d *DoubleSHA256) CalcDifficulty(chain consensus.ChainReader, time uint64, parent types.IHeader) *big.Int {
	if d.diffCalculator == nil {
		panic("diffCalculator is not existed")
	}

	return d.diffCalculator.CalculateDifficulty(parent, time)
}

// Hashrate returns the current mining hashrate of a PoW consensus engine.
func (d *DoubleSHA256) Hashrate() float64 {
	return d.commonEngine.Hashrate()
}

// Close terminates any background threads maintained by the consensus engine.
func (d *DoubleSHA256) Close() error {
	return nil
}

// FindNonce finds the desired nonce and mixhash for a given block header.
func (d *DoubleSHA256) FindNonce(
	work consensus.MiningWork,
	results chan<- consensus.MiningResult,
	stop <-chan struct{},
) error {
	return d.commonEngine.FindNonce(work, results, stop)
}

// Name returns the consensus engine's name.
func (d *DoubleSHA256) Name() string {
	return d.commonEngine.Name()
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

// New returns a DoubleSHA256 scheme.
func New(diffCalculator consensus.DifficultyCalculator) *DoubleSHA256 {
	spec := consensus.MiningSpec{
		Name:     "DoubleSHA256",
		HashAlgo: hashAlgo,
	}
	return &DoubleSHA256{
		commonEngine:   consensus.NewCommonEngine(spec),
		diffCalculator: diffCalculator,
	}
}
