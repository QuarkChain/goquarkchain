package sync

import (
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/cluster/root"
	"github.com/QuarkChain/goquarkchain/core/types"
)

const (
	// TODO: should use config
	maxSyncStaleness = 60

	// Number of root block headers to download from peers.
	headerDownloadSize = 500
	// Number root blocks to download from peers.
	blockDownloadSize = 100
)

// Task represents a synchronization task for the synchronizer.
type Task interface {
	Run(root.PrimaryServer) error
	Peer() peer
	Priority() uint
}

// All of the sync tasks to are to catch up with the root chain from peers.
type task struct {
	rootBlock *types.RootBlock
	peer
	header *types.RootBlockHeader
}

// Run will execute the synchronization task.
func (t *task) Run(primary root.PrimaryServer) error {
	if primary.RootBlockExists(t.header.Hash()) {
		return nil
	}

	logger := log.New("synctask", t.header.NumberUI64())
	headerTip := primary.Tip()
	tipHeight := headerTip.NumberU64()

	// Prepare for downloading.
	chain := []*types.RootBlockHeader{t.header} // Descending.
	lastHeader := t.header
	for !primary.RootBlockExists(lastHeader.ParentHash) {
		height, hash := lastHeader.NumberUI64(), lastHeader.Hash()
		if tipHeight-height > maxSyncStaleness {
			logger.Warn("Abort synching due to forking at super old block", "currentHeight", tipHeight, "oldHeight", height)
			return nil
		}

		logger.Info("Downloading block header list", "height", height, "hash", hash)
		// Order should be descending.
		receivedHeaders, err := downloadHeaders(hash, headerDownloadSize) // TODO: stub
		if err != nil {
			return err
		}
		if err := validateRootBlockHeaderList(receivedHeaders); err != nil { // TODO: stub
			return err
		}
		for _, h := range receivedHeaders {
			if primary.RootBlockExists(h.Hash()) {
				break
			}
			chain = append(chain, h)
		}
		lastHeader = chain[len(chain)-1]
	}

	logger.Info("Downloading blocks", "length", len(chain), "from", lastHeader.NumberUI64(), "to", t.header.NumberUI64())

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
			logger.Info("Syncing root block starts", "height", h.NumberUI64(), "hash", h.Hash())
			// Simple profiling.
			ts := time.Now()
			if err := syncMinorBlocks(b); err != nil {
				return err
			}
			if err := primary.AddBlock(b); err != nil {
				return err
			}
			logger.Info("Syncing root block finishes", "height", h.NumberUI64(), "hash", h.Hash(), "elapsed", time.Now().Sub(ts))
		}

		i -= blockDownloadSize
	}

	return nil
}

func (t *task) Priority() uint {
	panic("not implemented")
}

func (t *task) Peer() peer {
	return t.peer
}

func downloadHeaders(startHash common.Hash, length int) ([]*types.RootBlockHeader, error) {
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
