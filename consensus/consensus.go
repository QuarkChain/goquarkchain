package consensus

import (
	"errors"
	"fmt"
	"math/big"

	ethconsensus "github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

// VerifyHeader checks whether a header conforms to the consensus rules.
func VerifyHeader(chain ethconsensus.ChainReader, header, parent *types.Header, cengine ethconsensus.Engine) error {
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
