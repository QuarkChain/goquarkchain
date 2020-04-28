// Modified from go-ethereum under GNU Lesser General Public License

package core

import "errors"

var (
	// ErrKnownBlock is returned when a block to import is already known locally.
	ErrKnownBlock = errors.New("block already known")

	// ErrGasLimitReached is returned by the gas pool if the amount of gas required
	// by a transaction is higher than what's left in the block.
	ErrGasLimitReached = errors.New("gas limit reached")

	// ErrBlacklistedHash is returned if a block to import is on the blacklist.
	ErrBlacklistedHash = errors.New("blacklisted hash")

	// ErrNonceTooHigh is returned if the nonce of a transaction is higher than the
	// next one expected based on the local chain.
	ErrNonceTooHigh = errors.New("nonce too high")

	//ErrPrevBlockMissing is returned if a block's previous block is not exist in DB
	ErrPrevBlockMissing = errors.New("previous hash block mismatch")

	// ErrUnknownAncestor is returned when validating a block requires an ancestor
	// that is unknown.
	ErrUnknownAncestor = errors.New("unknown ancestor")

	// ErrPrunedAncestor is returned when validating a block requires an ancestor
	// that is known, but the state of which is not available.
	ErrPrunedAncestor = errors.New("pruned ancestor")

	// ErrFutureBlock is returned when a block's timestamp is in the future according
	// to the current node.
	ErrFutureBlock = errors.New("block in the future")

	// ErrInvalidNumber is returned if a block's number doesn't equal it's parent's
	// plus one.
	ErrInvalidNumber = errors.New("invalid block number")

	errNoGenesis                 = errors.New("genesis not found in chain")
	ErrMinorBlockIsNil           = errors.New("minor block is nil")
	ErrRootBlockIsNil            = errors.New("root block is nil")
	ErrInvalidMinorBlock         = errors.New("minor block is invalid")
	ErrHeightMismatch            = errors.New("block height not match")
	ErrPreBlockNotFound          = errors.New("parent block not found")
	ErrBranch                    = errors.New("branch not match")
	ErrTime                      = errors.New("time not match")
	ErrMetaHash                  = errors.New("meta hash not match")
	ErrExtraLimit                = errors.New("extra data exceeds limit")
	ErrTrackLimit                = errors.New("track data exceeds limit")
	ErrTxHash                    = errors.New("tx hash not match")
	ErrMinerFullShardKey         = errors.New("coinbase full shard key not match")
	ErrDifficulty                = errors.New("diff not match")
	ErrGasUsed                   = errors.New("gas used not match")
	ErrCoinbaseAmount            = errors.New("wrong coinbase amount")
	ErrXShardList                = errors.New("xShardReceivedGasUsed not match")
	ErrNetWorkID                 = errors.New("network id not match")
	ErrNotNeighbor               = errors.New("is not a neighbor")
	ErrNotSameRootChain          = errors.New("is not same root chain")
	ErrPoswOnRootChainIsNotFound = errors.New("PoSW-on-root-chain contract is not found")
	ErrContractNotFound          = errors.New("contract not found")
)
