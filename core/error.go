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

	errNoGenesis         = errors.New("Genesis not found in chain")
	ErrBlockIsNil        = errors.New("block is nil")
	ErrInvalidMinorBlock = errors.New("minor block is invalid")
	ErrHeightDisMatch    = errors.New("block's height mis match")
	ErrPreBlockNotFound  = errors.New("pre block not found")
	ErrBranch            = errors.New("branch not match")
	ErrTime              = errors.New("time is not match")
	ErrMetaHash          = errors.New("meta hash is not match")
	ErrExtraLimit        = errors.New("extra data's len exceeds limit")
	ErrTrackLimit        = errors.New("track data's len exceeds limit")
	ErrRootHash          = errors.New("root hash is not match")
	ErrFullShardKey      = errors.New("full shard key is not match")
	ErrDifficulty        = errors.New("diff is not match")
	ErrGasUsed           = errors.New("gas used is not match")
	ErrCoinBaseAmount    = errors.New("coinBaseAmount is err")
	ErrXShardList        = errors.New("xShardReceivedGasUsed is not match")
)
