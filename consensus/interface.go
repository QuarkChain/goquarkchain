package consensus

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"math/big"
)

// ChainReader defines a small collection of methods needed to access the local
// blockchain during header verification.
type ChainReader interface {
	// Config retrieves the blockchain's chain configuration.
	Config() *config.QuarkChainConfig

	// CurrentHeader retrieves the current header from the local chain.
	CurrentHeader() types.IHeader

	// GetHeader retrieves a block header from the database by hash and number.
	GetHeader(hash common.Hash) types.IHeader

	// GetHeaderByNumber retrieves a block header from the database by number.
	GetHeaderByNumber(number uint64) types.IHeader

	// GetBlock retrieves a block from the database by hash and number.
	GetBlock(hash common.Hash) types.IBlock
}

// Engine is an algorithm agnostic consensus engine.
type Engine interface {
	// Author retrieves the address of the account that minted the given
	// block, which may be different from the header's coinbase if a consensus
	// engine is based on signatures.
	Author(header types.IHeader) (account.Address, error)

	// VerifyHeader checks whether a header conforms to the consensus rules of a
	// given engine. Verifying the seal may be done optionally here, or explicitly
	// via the VerifySeal method.
	VerifyHeader(chain ChainReader, header types.IHeader, seal bool) error

	// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
	// concurrently. The method returns a quit channel to abort the operations and
	// a results channel to retrieve the async verifications (the order is that of
	// the input slice).
	VerifyHeaders(chain ChainReader, headers []types.IHeader, seals []bool) (chan<- struct{}, <-chan error)

	// VerifySeal checks whether the crypto seal on a header is valid according to
	// the consensus rules of the given engine.
	VerifySeal(chain ChainReader, header types.IHeader, adjustedDiff *big.Int) error

	// Prepare initializes the consensus fields of a block header according to the
	// rules of a particular engine. The changes are executed inline.
	Prepare(chain ChainReader, header types.IHeader) error

	// Seal generates a new sealing request for the given input block and pushes
	// the result into the given channel.
	//
	// Note, the method returns immediately and will send the result async. More
	// than one result may also be returned depending on the consensus algorithm.
	Seal(chain ChainReader, block types.IBlock, results chan<- types.IBlock, stop <-chan struct{}) error

	// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
	// that a new block should have.
	CalcDifficulty(chain ChainReader, time uint64, parent types.IHeader) *big.Int

	// Close terminates any background threads maintained by the consensus engine.
	Close() error
}

// PoW is the quarkchain version of PoW consensus engine, with a conveninent method for
// remote miners.
type PoW interface {
	Engine
	// Hashrate returns the current mining hashrate of a PoW consensus engine.
	Hashrate() float64
	FindNonce(work MiningWork, results chan<- MiningResult, stop <-chan struct{}) error
	Name() string
}
