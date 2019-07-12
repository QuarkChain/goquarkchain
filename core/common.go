package core

import (
	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/log"
	"reflect"
)

func isSameChain(db rawdb.DatabaseReader, longerChainHeader, shorterChainHeader types.IHeader) bool {
	if longerChainHeader.NumberU64() < shorterChainHeader.NumberU64() {
		log.Crit("wrong parameter order", "long.Number", longerChainHeader.NumberU64(), "long.Hash", longerChainHeader.Hash().String(), "short.Number", shorterChainHeader.NumberU64(), "short.hash", shorterChainHeader.Hash().String())
	}
	if shorterChainHeader.NumberU64() == longerChainHeader.NumberU64() {
		return shorterChainHeader.Hash() == longerChainHeader.Hash()
	}

	header := longerChainHeader
	diff := longerChainHeader.NumberU64() - shorterChainHeader.NumberU64()
	for i := uint64(0); i < diff-1; i++ {
		switch {
		case reflect.TypeOf(header) == reflect.TypeOf(new(types.RootBlockHeader)):
			header = rawdb.ReadRootBlockHeader(db, header.GetParentHash())
		case reflect.TypeOf(header) == reflect.TypeOf(new(types.MinorBlockHeader)):
			header = rawdb.ReadMinorBlockHeader(db, header.GetParentHash())
		default:
			log.Crit("bad header type", "type", reflect.TypeOf(header))
		}

		if common.IsNil(header) {
			log.Crit("mysteriously missing blocks", "long.Number", longerChainHeader.NumberU64(), "long.Hash", longerChainHeader.Hash().String(), "short.Number", shorterChainHeader.NumberU64(), "short.hash", shorterChainHeader.Hash().String())
		}
	}

	return header.GetParentHash() == shorterChainHeader.Hash()
}
