package doublesha256

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"

	"github.com/QuarkChain/goquarkchain/consensus"
	"github.com/ethereum/go-ethereum/common"
	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

var (
	// two256 is a big integer representing 2^256
	two256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

// DoubleSHA256 is a consensus engine implementing PoW with double-sha256 algo.
type DoubleSHA256 struct {
	commonEngine *consensus.CommonEngine
	// For reusing existing functions
	ethash *ethash.Ethash
}

// Author returns coinbase address.
func (d *DoubleSHA256) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules.
func (d *DoubleSHA256) VerifyHeader(chain ethconsensus.ChainReader, header *types.Header, seal bool) error {
	return d.commonEngine.VerifyHeader(chain, header, seal, d)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (d *DoubleSHA256) VerifyHeaders(chain ethconsensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	return d.commonEngine.VerifyHeaders(chain, headers, seals, d)
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of a given engine.
func (d *DoubleSHA256) VerifyUncles(chain ethconsensus.ChainReader, block *types.Block) error {
	// For now QuarkChain won't verify uncles.
	return nil
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (d *DoubleSHA256) VerifySeal(chain ethconsensus.ChainReader, header *types.Header) error {
	if header.Difficulty.Sign() <= 0 {
		return consensus.ErrInvalidDifficulty
	}

	target := new(big.Int).Div(two256, header.Difficulty)
	_, result := hashAlgo(d.SealHash(header).Bytes(), header.Nonce.Uint64())
	if new(big.Int).SetBytes(result[:]).Cmp(target) > 0 {
		return consensus.ErrInvalidPoW
	}
	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (d *DoubleSHA256) Prepare(chain ethconsensus.ChainReader, header *types.Header) error {
	panic("not implemented")
}

// Finalize runs any post-transaction state modifications (e.g. block rewards)
// and assembles the final block.
func (d *DoubleSHA256) Finalize(chain ethconsensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	panic("not implemented")
}

// SealHash returns the hash of a block prior to it being sealed.
func (d *DoubleSHA256) SealHash(header *types.Header) common.Hash {
	return d.commonEngine.SealHash(header)
}

// Seal generates a new block for the given input block with the local miner's
// seal place on top.
func (d *DoubleSHA256) Seal(
	chain ethconsensus.ChainReader,
	block *types.Block,
	results chan<- *types.Block,
	stop <-chan struct{}) error {

	return d.commonEngine.Seal(chain, block, results, stop)
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (d *DoubleSHA256) CalcDifficulty(chain ethconsensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	panic("not implemented")
}

// APIs returns the RPC APIs this consensus engine provides.
func (d *DoubleSHA256) APIs(chain ethconsensus.ChainReader) []rpc.API {
	panic("not implemented")
}

// Hashrate returns the current mining hashrate of a PoW consensus engine.
func (d *DoubleSHA256) Hashrate() float64 {
	return d.commonEngine.Hashrate()
}

// Close terminates any background threads maintained by the consensus engine.
func (d *DoubleSHA256) Close() error {
	return nil
}

func hashAlgo(hash []byte, nonce uint64) (digest, result []byte) {
	nonceBytes := make([]byte, 8)
	// Note it's big endian here
	binary.BigEndian.PutUint64(nonceBytes, nonce)
	hashNonceBytes := append(hash, nonceBytes...)

	hashOnce := sha256.Sum256(hashNonceBytes)
	resultArray := sha256.Sum256(hashOnce[:])
	result = resultArray[:]
	return // digest default to nil
}

// New returns a DoubleSHA256 scheme.
func New() *DoubleSHA256 {
	spec := consensus.MiningSpec{
		Name:     "DoubleSHA256",
		HashAlgo: hashAlgo,
	}
	return &DoubleSHA256{
		commonEngine: consensus.NewCommonEngine(spec),
	}
}
