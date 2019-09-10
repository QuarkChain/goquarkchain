package sync

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"

	qkcom "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
)

const (
	RootBlockHeaderListLimit  = 500
	RootBlockBatchSize        = 100
	MinorBlockHeaderListLimit = 100 //TODO 100 50
	MinorBlockBatchSize       = 50
)

// Task represents a synchronization task for the synchronizer.
type Task interface {
	Run(blockchain) error
	Priority() *big.Int
	PeerID() string
}

type task struct {
	name             string
	maxSyncStaleness uint64
	batchSize        int
	findAncestor     func(blockchain) (types.IHeader, error)
	getHeaders       func(types.IHeader) ([]types.IHeader, error)
	getBlocks        func([]common.Hash) ([]types.IBlock, error)
	syncBlock        func(blockchain, types.IBlock) error
	needSkip         func(b blockchain) bool
}

// Run will execute the synchronization task.
func (t *task) Run(bc blockchain) error {
	if t.needSkip(bc) {
		return nil
	}

	ancestor, err := t.findAncestor(bc)
	if err != nil || qkcom.IsNil(ancestor) {
		return err
	}

	logger := log.New("synctask", t.name, "start", ancestor.NumberU64())

	if bc.CurrentHeader().NumberU64()-ancestor.NumberU64() > t.maxSyncStaleness {
		logger.Warn("Abort synching due to forking at super old block", "currentHeight", bc.CurrentHeader().NumberU64(), "oldHeight", ancestor.NumberU64())
		return nil
	}

	for !qkcom.IsNil(ancestor) {
		headers, err := t.getHeaders(ancestor)
		if err != nil {
			return err
		}
		if len(headers) == 0 {
			return nil
		}

		if err := t.validateHeaderList(bc, headers); err != nil {
			return err
		}

		logger.Info("Downloading blocks", "length", len(headers), "from", ancestor.NumberU64(), "to", headers[len(headers)-1].NumberU64())

		hashlist := make([]common.Hash, 0, len(headers))
		for _, hd := range headers {
			hashlist = append(hashlist, hd.Hash())
		}

		for len(hashlist) > 0 {
			var blocks []types.IBlock
			if len(hashlist) > t.batchSize {
				blocks, err = t.getBlocks(hashlist[:t.batchSize])
				if len(blocks) != t.batchSize {
					return fmt.Errorf("unmatched block length, expect: %d, actual: %d", t.batchSize, len(blocks))
				}
				hashlist = hashlist[t.batchSize:]
			} else {
				blocks, err = t.getBlocks(hashlist)
				if len(blocks) != len(hashlist) {
					return fmt.Errorf("unmatched block length, expect: %d, actual: %d", len(hashlist), len(blocks))
				}
				hashlist = nil
			}

			if err != nil {
				return err
			}

			for _, blk := range blocks {
				if t.syncBlock != nil {
					if err := t.syncBlock(bc, blk); err != nil {
						return err
					}
				}
				if err := bc.AddBlock(blk); err != nil {
					return err
				}
				ancestor = blk.IHeader()
			}
		}
	}
	return nil
}

func (t *task) validateHeaderList(bc blockchain, headers []types.IHeader) error {
	fmt.Println("task", len(headers))
	var prev types.IHeader
	for _, h := range headers {
		if !qkcom.IsNil(prev) {
			if h.NumberU64() != prev.NumberU64()+1 {
				return errors.New("should have descending order with step 1")
			}
			if prev.Hash() != h.GetParentHash() {
				return errors.New("should have blocks correctly linked")
			}
		}
		if err := bc.Validator().ValidateSeal(h, false); err != nil { //use diff/20
			return err
		}
		prev = h
	}
	return nil
}
