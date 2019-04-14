package sync

import (
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/core"

	"github.com/QuarkChain/goquarkchain/core/types"
)

const (
	// Number of root block headers to download from peers.
	headerDownloadSize = 500
	// Number root blocks to download from peers.
	blockDownloadSize = 100
)

var (
	// TODO: should use config
	maxSyncStaleness = big.NewInt(60)
)

// Task represents a synchronization task for the synchronizer.
type Task interface {
	Run(blockchain) error
	Peer() peer
	Priority() uint
}

// All of the sync tasks to are to catch up with the root chain from peers.
type rootChainTask struct {
	rootBlock *types.RootBlock
	peer
	header *types.RootBlockHeader
}

// Run will execute the synchronization task.
func (r *rootChainTask) Run(bc *core.RootBlockChain) error {
	if bc.HasBlock(r.header.Hash()) {
		return nil
	}

	logger := log.New("synctask", r.header.NumberU64())
	headerTip := bc.CurrentHeader()
	tipHeight := new(big.Int).SetUint64(headerTip.NumberU64())

	// Prepare for downloading.
	chain := []*types.RootBlockHeader{r.header} // Descending.
	lastHeader := r.header
	for !bc.HasBlock(lastHeader.ParentHash) {
		height, hash := new(big.Int).SetUint64(lastHeader.NumberU64()), lastHeader.Hash()
		hDiff := new(big.Int).Sub(tipHeight, height)
		if hDiff.Cmp(maxSyncStaleness) > 0 {
			logger.Warn("Abort synching due to forking at super old block", "currentHeight", tipHeight, "oldHeight", height)
			return nil
		}

		logger.Info("Downloading block header list", "height", height, "hash", hash)
		// Order should be descending. Download size is min(500, h-tip) if h > tip.
		downloadSz := uint64(headerDownloadSize)
		if hDiff.Sign() < 0 && hDiff.CmpAbs(big.NewInt(headerDownloadSize)) < 0 {
			downloadSz = new(big.Int).Abs(hDiff).Uint64()
		}
		receivedHeaders, err := downloadHeaders(hash, downloadSz) // TODO: stub
		if err != nil {
			return err
		}
		if err := validateRootBlockHeaderList(receivedHeaders); err != nil { // TODO: stub
			return err
		}
		for _, h := range receivedHeaders {
			if bc.HasBlock(h.Hash()) {
				break
			}
			chain = append(chain, h)
		}
		lastHeader = chain[len(chain)-1]
	}

	logger.Info("Downloading blocks", "length", len(chain), "from", lastHeader.NumberU64(), "to", r.header.NumberU64())

	// Download blocks from lower to higher.
	i := len(chain)
	for i > 0 {
		// Exclusive.
		start, end := i-blockDownloadSize, i
		if start < 0 {
			start = 0
		}
		headersForDownload := chain[start:end]
		blocks, err := downloadRootBlocks(headersForDownload)
		if err != nil {
			return nil
		}
		if len(blocks) != end-start {
			errMsg := "Bad peer missing blocks for given headers"
			logger.Error(errMsg)
			return errors.New(strings.ToLower(errMsg))
		}

		// Again, `blocks` should also be descending.
		for j := len(blocks) - 1; j >= 0; j-- {
			b := blocks[j]
			h := b.Header()
			logger.Info("Syncing root block starts", "height", h.NumberU64(), "hash", h.Hash())
			// Simple profiling.
			ts := time.Now()
			if err := syncMinorBlocks(b); err != nil {
				return err
			}
			if _, err := bc.InsertChain([]types.IBlock{b}); err != nil {
				return err
			}
			elapsed := time.Now().Sub(ts).Seconds()
			logger.Info("Syncing root block finishes", "height", h.NumberU64(), "hash", h.Hash(), "elapsed", elapsed)
		}

		i = start
	}

	return nil
}

func (r *rootChainTask) Priority() uint {
	panic("not implemented")
}

func (r *rootChainTask) Peer() peer {
	return r.peer
}

func downloadHeaders(startHash common.Hash, length uint64) ([]*types.RootBlockHeader, error) {
	// TODO: stub
	return nil, nil
}

func validateRootBlockHeaderList(headers []*types.RootBlockHeader) error {
	// TODO: stub
	return nil
}

func downloadRootBlocks(blockHashesForDownload []*types.RootBlockHeader) ([]*types.RootBlock, error) {
	// TODO: stub
	return nil, nil
}

func syncMinorBlocks(rootBlock *types.RootBlock) error {
	// TODO: stub
	return nil
}
