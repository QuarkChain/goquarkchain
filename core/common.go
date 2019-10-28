package core

import (
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"runtime/debug"
)

var (
	emptyHash = common.Hash{}
)

func isSameChain(getParentHash func(common.Hash) common.Hash, longerChainHeader, shorterChainHeader types.IHeader) bool {
	if longerChainHeader.NumberU64() < shorterChainHeader.NumberU64() {
		debug.PrintStack()
		log.Crit("wrong parameter order", "long.Number", longerChainHeader.NumberU64(), "long.Hash", longerChainHeader.Hash().String(), "short.Number", shorterChainHeader.NumberU64(), "short.hash", shorterChainHeader.Hash().String())
	}
	if shorterChainHeader.NumberU64() == longerChainHeader.NumberU64() {
		return shorterChainHeader.Hash() == longerChainHeader.Hash()
	}
	if shorterChainHeader.NumberU64()+1 == longerChainHeader.NumberU64() {
		return shorterChainHeader.Hash() == longerChainHeader.GetParentHash()
	}

	diff := longerChainHeader.NumberU64() - shorterChainHeader.NumberU64()
	hash := longerChainHeader.GetParentHash()
	for i := uint64(0); i < diff-1; i++ {
		hash = getParentHash(hash)
		if hash == emptyHash {
			log.Crit("mysteriously missing blocks", "long.Number", longerChainHeader.NumberU64(), "long.Hash", longerChainHeader.Hash().String(), "short.Number", shorterChainHeader.NumberU64(), "short.hash", shorterChainHeader.Hash().String())
		}
	}

	return hash == shorterChainHeader.Hash()
}
