package core

import (
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/ethereum/go-ethereum/log"
	"math/big"

	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
)

// ShardStatus shard status for api
type ShardStatus struct {
	Branch             account.Branch
	Height             uint64
	Difficulty         *big.Int
	CoinbaseAddress    account.Address
	TimeStamp          uint64
	TxCount60s         uint32
	PendingTxCount     uint32
	TotalTxCount       uint32
	BlockCount60s      uint32
	StaleBlockCount60s uint32
	LastBlockTime      uint32
}

//TODO finish IsSameMinorChain

func isSameRootChain(db rawdb.DatabaseReader, longerChainHeader, shorterChainHeader types.IHeader) bool {
	if longerChainHeader.NumberU64() < shorterChainHeader.NumberU64() {
		log.Crit("wrong parameter order")
	}

	header := longerChainHeader

	for i := uint64(0); i < longerChainHeader.NumberU64()-shorterChainHeader.NumberU64(); i++ {
		header = rawdb.ReadRootBlockHeader(db, header.GetParentHash())
		if common.IsNil(header) {
			log.Crit("mysteriously missing blocks")
		}
	}

	return header.Hash() == shorterChainHeader.Hash()
}
