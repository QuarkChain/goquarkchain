package consensus

import (
	"errors"
	"fmt"
	"math/big"

	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// CommonEngine contains the common parts for consensus engines, where engine-specific
// logic is provided in func args as template pattern.
type CommonEngine struct{}

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	ErrInvalidDifficulty = errors.New("non-positive difficulty")
	ErrInvalidMixDigest  = errors.New("invalid mix digest")
	ErrInvalidPoW        = errors.New("invalid proof-of-work")
)

// VerifyHeader checks whether a header conforms to the consensus rules.
func (c *CommonEngine) VerifyHeader(
	chain ethconsensus.ChainReader,
	header *types.Header,
	seal bool,
	cengine ethconsensus.Engine,
) error {
	// Short-circuit if the header is known, or parent not
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return ethconsensus.ErrUnknownAncestor
	}

	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}

	if header.Time.Cmp(parent.Time) <= 0 {
		return errors.New("timestamp equals parent's")
	}

	expectedDiff := cengine.CalcDifficulty(chain, header.Time.Uint64(), parent)
	if expectedDiff.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expectedDiff)
	}

	// TODO: validate gas limit

	if header.GasUsed > header.GasLimit {
		return fmt.Errorf("invalid gasUsed: have %d, gasLimit %d", header.GasUsed, header.GasLimit)
	}

	// TODO: verify gas limit is within allowed bounds

	if heightDiff := new(big.Int).Sub(header.Number, parent.Number); heightDiff.Cmp(big.NewInt(1)) != 0 {
		return ethconsensus.ErrInvalidNumber
	}

	if err := cengine.VerifySeal(chain, header); err != nil {
		return err
	}

	return nil
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications (the order is that of
// the input slice).
func (c *CommonEngine) VerifyHeaders(
	chain ethconsensus.ChainReader,
	headers []*types.Header,
	seals []bool,
	cengine ethconsensus.Engine,
) (chan<- struct{}, <-chan error) {

	// TODO: verify concurrently, and support aborting
	errorsOut := make(chan error, len(headers))
	go func() {
		for _, h := range headers {
			err := c.VerifyHeader(chain, h, true /*seal flag not used*/, cengine)
			errorsOut <- err
		}
	}()
	return nil, errorsOut
}
