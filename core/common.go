package core

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
)

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
