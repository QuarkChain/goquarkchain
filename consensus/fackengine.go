package consensus

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"math/big"
)

type FackEngine struct {
	NumberToFail uint64
	Err          error
}

// Author retrieves the Ethereum address of the account that minted the given
// block, which may be different from the header's coinbase if a consensus
// engine is based on signatures.
func (e *FackEngine) Author(header types.IHeader) (recipient account.Recipient, err error) {
	return header.GetCoinbase().Recipient, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of a
// given engine. Verifying the seal may be done optionally here, or explicitly
// via the VerifySeal method.
func (e *FackEngine) VerifyHeader(chain ChainReader, header types.IHeader, seal bool) error {
	if header.NumberU64() == e.NumberToFail {
		return e.Err
	}
	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (e *FackEngine) VerifyHeaders(chain ChainReader, headers []types.IHeader, seals []bool) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(headers))
	for i := 0; i < len(headers); i++ {
		if headers[i].NumberU64() == e.NumberToFail {
			results <- e.Err
		} else {
			results <- nil
		}
	}
	return abort, results
}

// VerifySeal checks whether the crypto seal on a header is valid according to
// the consensus rules of the given engine.
func (e *FackEngine) VerifySeal(chain ChainReader, header types.IHeader) error {
	if header.NumberU64() == e.NumberToFail {
		return e.Err
	}
	return nil
}

// Prepare initializes the consensus fields of a block header according to the
// rules of a particular engine. The changes are executed inline.
func (e *FackEngine) Prepare(chain ChainReader, header types.IHeader) error {
	if header.NumberU64() == e.NumberToFail {
		return e.Err
	}
	return nil
}

// Seal generates a new sealing request for the given input block and pushes
// the result into the given channel.
//
// Note, the method returns immediately and will send the result async. More
// than one result may also be returned depending on the consensus algorithm.
func (e *FackEngine) Seal(chain ChainReader, block types.IBlock, results chan<- types.IBlock, stop <-chan struct{}) error {
	if block.NumberU64() == e.NumberToFail {
		return e.Err
	}
	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns the difficulty
// that a new block should have.
func (e *FackEngine) CalcDifficulty(chain ChainReader, time uint64, parent types.IHeader) *big.Int {
	return parent.GetDifficulty()
}

// APIs returns the RPC APIs this consensus engine provides.
func (e *FackEngine) APIs(chain ChainReader) []rpc.API {
	return nil
}

// Close terminates any background threads maintained by the consensus engine.
func (e *FackEngine) Close() error {
	return e.Err
}
