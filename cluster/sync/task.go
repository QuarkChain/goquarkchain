package sync

import (
	"errors"
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
	findAncestor     func(blockchain) (types.IHeader, error)
	getHeaders       func(types.IHeader) ([]types.IHeader, error)
	getBlocks        func([]common.Hash) ([]types.IBlock, error)
	syncBlock        func(blockchain, types.IBlock) error
}

// Run will execute the synchronization task.
func (t *task) Run(bc blockchain) error {
	ancestor, err := t.findAncestor(bc)
	if err != nil || ancestor == nil {
		return err
	}

	logger := log.New("synctask", t.name, "start", ancestor.NumberU64())

	if bc.CurrentHeader().NumberU64()-ancestor.NumberU64() > t.maxSyncStaleness {
		logger.Warn("Abort synching due to forking at super old block", "currentHeight", bc.CurrentHeader().NumberU64(), "oldHeight", ancestor.NumberU64())
		return nil
	}

	lastHeader := ancestor
	for !qkcom.IsNil(lastHeader) {
		headers, err := t.getHeaders(lastHeader)
		if err != nil {
			return err
		}
		if len(headers) == 0 {
			return nil
		}

		if err := t.validateHeaderList(bc, headers); err != nil {
			return err
		}

		logger.Info("Downloading blocks", "length", len(headers), "from", lastHeader.NumberU64(), "to", headers[len(headers)-1].NumberU64())

		hashlist := make([]common.Hash, 0, len(headers))
		for _, hd := range headers {
			hashlist = append(hashlist, hd.Hash())
		}

		blocks, err := t.getBlocks(hashlist)
		if err != nil {
			return err
		}

		if t.syncBlock != nil {
			for _, blk := range blocks {
				if t.syncBlock != nil {
					if err := t.syncBlock(bc, blk); err != nil {
						return err
					}
				}
				if err := bc.AddBlock(blk); err != nil {
					return err
				}
			}
		}
		lastHeader = headers[len(headers)-1]
	}
	return nil
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
		if err := bc.Validator().ValidateSeal(h); err != nil {
			return err
		}
		prev = h
	}
	return nil
}
