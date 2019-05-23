package sync

import (
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/core/types"
)

// Task represents a synchronization task for the synchronizer.
type Task interface {
	Run(blockchain) error
	Peer() peer
	Priority() uint
}

type task struct {
	header     types.IHeader
	peer       peer
	name       string
	getHeaders func(common.Hash, uint32) ([]types.IHeader, error)
	getBlocks  func([]common.Hash) ([]types.IBlock, error)
	syncBlock  func(types.IBlock, blockchain) error
}

// Run will execute the synchronization task.
func (t *task) Run(bc blockchain) error {
	if bc.HasBlock(t.header.Hash()) {
		return nil
	}

	logger := log.New("synctask", t.name, "start", t.header.NumberU64())
	headerTip := bc.CurrentHeader()
	tipHeight := headerTip.NumberU64()

	// Prepare for downloading.
	chain := []common.Hash{t.header.Hash()}
	lastHeader := t.header
	for !bc.HasBlock(lastHeader.GetParentHash()) {
		height, hash := lastHeader.NumberU64(), lastHeader.Hash()
		if tipHeight > height && tipHeight-height > uint64(maxSyncStaleness) {
			logger.Warn("Abort synching due to forking at super old block", "currentHeight", tipHeight, "oldHeight", height)
			return nil
		}

		logger.Info("Downloading block header list", "height", height, "hash", hash)
		// Order should be descending. Download size is min(500, h-tip) if h > tip.
		downloadSz := uint32(headerDownloadSize)
		receivedHeaders, err := t.getHeaders(lastHeader.GetParentHash(), downloadSz)
		if err != nil {
			return err
		}
		err = t.validateHeaderList(bc, receivedHeaders)
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

	logger.Info("Downloading blocks", "length", len(chain), "from", lastHeader.NumberU64(), "to", t.header.NumberU64())

	// Download blocks from lower to higher.
	i := len(chain)
	for i > 0 {
		// Exclusive.
		start, end := i-blockDownloadSize, i
		if start < 0 {
			start = 0
		}
		headersForDownload := chain[start:end]
		blocks, err := t.getBlocks(headersForDownload)
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
			h := b.IHeader()
			logger.Info("Syncing block starts", "height", h.NumberU64(), "hash", h.Hash())
			// Simple profiling.
			ts := time.Now()
			if t.syncBlock != nil {
				if err := t.syncBlock(b, bc); err != nil {
					return err
				}
			}
			// TODO: may optimize by batch and insert once?
			if _, err := bc.InsertChain([]types.IBlock{b}); err != nil {
				return err
			}
			elapsed := time.Now().Sub(ts).Seconds()
			logger.Info("Syncing block finishes", "height", h.NumberU64(), "hash", h.Hash(), "elapsed", elapsed)
		}

		i = start
	}

	return nil
}

func (t *task) Peer() peer {
	return t.peer
}

func (t *task) validateHeaderList(bc blockchain, headers []types.IHeader) error {
	var prev types.IHeader
	for _, h := range headers {
		if prev != nil {
			if h.NumberU64()+1 != prev.NumberU64() {
				return errors.New("should have descending order with step 1")
			}
			if prev.GetParentHash() != h.Hash() {
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
