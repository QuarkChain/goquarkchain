package core

import (
	"github.com/ethereum/go-ethereum/log"

	"github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/rawdb"
	"github.com/QuarkChain/goquarkchain/core/types"
	"reflect"
)
var (
	AccountEnabled  = []byte{1}
	AccountDisabled = []byte{0}
)

func isSameChain(db rawdb.DatabaseReader, longerChainHeader, shorterChainHeader types.IHeader) bool {
	if longerChainHeader.NumberU64() < shorterChainHeader.NumberU64() {
		log.Crit("wrong parameter order", "long.Number", longerChainHeader.NumberU64(), "long.Hash", longerChainHeader.Hash().String(), "short.Number", shorterChainHeader.NumberU64(), "short.hash", shorterChainHeader.Hash().String())
	}
	if shorterChainHeader.NumberU64() == longerChainHeader.NumberU64() {
		return shorterChainHeader.Hash() == longerChainHeader.Hash()
	}

	header := longerChainHeader
	var chainType rawdb.ChainType
	switch {
	case reflect.TypeOf(header) == reflect.TypeOf(new(types.RootBlockHeader)):
		chainType = rawdb.ChainTypeRoot
	case reflect.TypeOf(header) == reflect.TypeOf(new(types.MinorBlockHeader)):
		chainType = rawdb.ChainTypeMinor
	default:
		log.Crit("bad header type", "type", reflect.TypeOf(header))
	}
	diff := longerChainHeader.NumberU64() - shorterChainHeader.NumberU64()
	for i := uint64(0); i < diff-1; i++ {
		if chainType == rawdb.ChainTypeRoot {
			header = rawdb.ReadRootBlockHeader(db, header.GetParentHash())
		} else {
			header = rawdb.ReadMinorBlockHeader(db, header.GetParentHash())
		}
		if common.IsNil(header) {
			log.Crit("mysteriously missing blocks", "long.Number", longerChainHeader.NumberU64(), "long.Hash", longerChainHeader.Hash().String(), "short.Number", shorterChainHeader.NumberU64(), "short.hash", shorterChainHeader.Hash().String())
		}
	}

	return header.GetParentHash() == shorterChainHeader.Hash()
}
