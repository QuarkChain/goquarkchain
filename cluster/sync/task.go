package sync

import (
	"errors"
	"github.com/ethereum/go-ethereum/common"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/core/types"
)

const (
	// Number of root block headers to download from peers.
	headerDownloadSize = 500
	// Number root blocks to download from peers.
	blockDownloadSize = 100
)

var (
	// TODO: should use config.
	maxSyncStaleness = 22500
)

// Task represents a synchronization task for the synchronizer.
type Task interface {
	Run(blockchain) error
	Peer() peer
	Priority() uint
}

// All of the sync tasks to are to catch up with the root chain from peers.
type rootChainTask struct {
	peer
	header *types.RootBlockHeader
}

func NewRootChainTask(p peer, header *types.RootBlockHeader) Task {
	return &rootChainTask{peer: p, header: header}
}

// Run will execute the synchronization task.
func (r *rootChainTask) Run(bc blockchain) error {
	if bc.HasBlock(r.header.Hash()) {
		return nil
	}

	logger := log.New("synctask", r.header.NumberU64())
	peer := r.Peer()
	headerTip := bc.CurrentHeader()
	tipHeight := headerTip.NumberU64()

	// Prepare for downloading.
	chain := []common.Hash{r.header.Hash()}
	lastHeader := r.header
	for !bc.HasBlock(lastHeader.ParentHash) {
		height, hash := lastHeader.NumberU64(), lastHeader.Hash()
		if tipHeight > height && tipHeight-height > uint64(maxSyncStaleness) {
			logger.Warn("Abort synching due to forking at super old block", "currentHeight", tipHeight, "oldHeight", height)
			return nil
		}

		logger.Info("Downloading block header list", "height", height, "hash", hash)
		// Order should be descending. Download size is min(500, h-tip) if h > tip.
		downloadSz := uint32(headerDownloadSize)
		receivedHeaders, err := peer.GetRootBlockHeaderList(lastHeader.ParentHash, downloadSz, true)
		if err != nil {
			return err
		}
		err = r.validateRootBlockHeaderList(bc, receivedHeaders)
		if err != nil {
			return err
		}
		for _, h := range receivedHeaders {
			if bc.HasBlock(h.Hash()) {
				break
			}
			chain = append(chain, h.Hash())
			lastHeader = h
		}
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
		blocks, err := peer.GetRootBlockList(headersForDownload)
		if err != nil {
			return err
		}
		if len(blocks) != end-start {
			errMsg := "Bad peer missing blocks for given headers"
			logger.Error(errMsg)
			return errors.New(strings.ToLower(errMsg))
		}

		// Again, `blocks` should also be descending.
		// TODO: validate block order.
		for j := len(blocks) - 1; j >= 0; j-- {
			b := blocks[j]
			h := b.Header()
			logger.Info("Syncing root block starts", "height", h.NumberU64(), "hash", h.Hash())
			// Simple profiling.
			ts := time.Now()
			if err := syncMinorBlocks(b); err != nil {
				return err
			}
			// TODO: may optimize by batch and insert once?
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
	return uint(r.header.Number)
}

func (r *rootChainTask) Peer() peer {
	return r.peer
}

func (r *rootChainTask) validateRootBlockHeaderList(bc blockchain, headers []*types.RootBlockHeader) error {
	var prev *types.RootBlockHeader
	for _, h := range headers {
		if prev != nil {
			if h.Number+1 != prev.Number {
				return errors.New("should have descending order with step 1")
			}
			if prev.ParentHash != h.Hash() {
				return errors.New("should have blocks correctly linked")
			}
		}
		if err := bc.Validator().ValidateHeader(h); err != nil {
			return err
		}
		prev = h
	}
	return nil
}

func syncMinorBlocks(rootBlock *types.RootBlock) error {
	// TODO: stub
	return nil
}
