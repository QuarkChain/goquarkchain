package root

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/QuarkChain/goquarkchain/core/types"
)

// PrimaryServer wraps the state of the root chain and manages network calls between slave servers
// and the outside world.
type PrimaryServer interface {
	Tip() *types.RootBlock
	RootBlockExists(rootBlockHash common.Hash) bool
	MinorBlockValidated(minorBlockHash common.Hash) bool
	AddBlock(*types.RootBlock) error
}
