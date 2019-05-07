package consensus

import (
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
)

//TODO finish IsSameMinorChain

func IsSameRootChain(db rawdb.DatabaseReader, longerChainHeader, shorterChainHeader types.IHeader) bool {
	if longerChainHeader.NumberU64() < shorterChainHeader.NumberU64() {
		return false
	}

	header := longerChainHeader

	for i := uint64(0); i < longerChainHeader.NumberU64()-shorterChainHeader.NumberU64(); i++ {
		header = rawdb.ReadRootBlockHeader(db, header.GetParentHash())
		if common.IsNil(header) { // TODO to check here:header==nil error
			return false
		} else {
			//fmt.Println("not nil")
		}
	}

	return header.Hash() == shorterChainHeader.Hash()
}
