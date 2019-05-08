package ethash

import (
	qkconsensus "github.com/QuarkChain/goquarkchain/consensus"
	"github.com/ethereum/go-ethereum/common"
)

// QEthash is our wrapper over geth ethash implementation.
type QEthash struct {
	*Ethash
	*qkconsensus.CommonEngine
}

func (q *QEthash) hashAlgo(height uint64, hash []byte, nonce uint64) (ret qkconsensus.MiningResult, err error) {
	size := datasetSize(height)
	cache := q.cache(height)
	digest, result := hashimotoLight(size, cache.cache, hash, nonce)
	return qkconsensus.MiningResult{
		Digest: common.BytesToHash(digest),
		Result: result,
		Nonce:  nonce,
	}, nil
}

// New returns a Ethash scheme.
func New(
	config Config,
	notify []string,
	noverify bool,
	diffCalculator qkconsensus.DifficultyCalculator,
	remote bool,
) *QEthash {
	ethash := NewEthash(config, notify, noverify)
	ret := &QEthash{
		Ethash: ethash,
	}
	spec := qkconsensus.MiningSpec{
		Name:       "Ethash",
		HashAlgo:   ret.hashAlgo,
		VerifySeal: ret.VerifySeal,
	}
	ret.CommonEngine = qkconsensus.NewCommonEngine(spec, diffCalculator, remote)
	return ret
}
