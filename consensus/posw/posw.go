package posw

import (
	"fmt"
	"math/big"
	"runtime/debug"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
)

type headReader interface {
	GetHeader(hash common.Hash) types.IHeader
}

type heightAndAddrs struct {
	height uint64
	addrs  []account.Recipient
}

type PoSW struct {
	config            *config.POSWConfig
	coinbaseAddrCache *lru.Cache
	minorBCHelper     headReader
}

func NewPoSW(headReader headReader, config *config.POSWConfig) *PoSW {
	cache, _ := lru.New(128)
	return &PoSW{
		minorBCHelper:     headReader,
		config:            config,
		coinbaseAddrCache: cache,
	}
}

/*PoSWDiffAdjust PoSW diff calc,already locked by insertChain*/
func (p *PoSW) PoSWDiffAdjust(header types.IHeader, stakes *big.Int) (*big.Int, error) {
	// Evaluate stakes before the to-be-added block
	blockThreshold := new(big.Int).Div(stakes, p.config.TotalStakePerBlock).Uint64()
	if blockThreshold > p.config.WindowSize {
		blockThreshold = p.config.WindowSize
	}
	// The func is inclusive, so need to fetch block counts until prev block
	// Also only fetch prev window_size - 1 block counts because the
	// new window should count the current block
	blockCnt, err := p.countCoinbaseBlockUntil(header.GetParentHash(), header.GetCoinbase().Recipient)
	if err != nil {
		return nil, err
	}
	diff := header.GetDifficulty()
	if *blockCnt < blockThreshold {
		diff = new(big.Int).Div(diff, big.NewInt(int64(p.config.DiffDivider)))
		log.Info("[PoSW]Adjusted PoSW", "height", header.NumberU64(), "from", header.GetDifficulty(), "to", diff)
	}
	return diff, nil
}

/*Take an additional recipient parameter and add its block count.*/
func (p *PoSW) BuildSenderDisallowMap(headerHash common.Hash, coinbase *account.Recipient) (map[account.Recipient]*big.Int, error) {
	if !p.config.Enabled {
		return nil, nil
	}
	coinbaseAddrs, err := p.getCoinbaseAddressUntilBlock(headerHash)
	if err != nil {
		return nil, err
	}
	recipientCountMap := make(map[account.Recipient]uint64)
	for _, ca := range coinbaseAddrs {
		recipientCountMap[ca]++
	}
	if coinbase != nil {
		recipientCountMap[*coinbase] += 1
	}
	disallowMap := make(map[account.Recipient]*big.Int)
	fmt.Printf("disallowMap:\n")
	for k, v := range recipientCountMap {
		disallowMap[k] = new(big.Int).Mul(big.NewInt(int64(v)), p.config.TotalStakePerBlock)
		fmt.Printf("[%x: %d]\n", k, disallowMap[k])
	}
	return disallowMap, nil
}

func (p *PoSW) IsPoSWEnabled() bool {
	return p.config.Enabled
}

func (p *PoSW) countCoinbaseBlockUntil(headerHash common.Hash, coinbase account.Recipient) (*uint64, error) {
	coinbases, err := p.getCoinbaseAddressUntilBlock(headerHash)
	if err != nil {
		return nil, err
	}
	var count uint64 = 0
	for _, cb := range coinbases {
		if cb == coinbase {
			count++
		}
	}
	return &count, nil
}

func (p *PoSW) getCoinbaseAddressUntilBlock(headerHash common.Hash) ([]account.Recipient, error) {
	var header types.IHeader
	length := int(p.config.WindowSize - 1)
	addrs := make([]account.Recipient, 0, length)
	if header = p.minorBCHelper.GetHeader(headerHash); qkcCommon.IsNil(header) {
		return nil, fmt.Errorf("curr block not found: hash %x, %s", headerHash, string(debug.Stack()))
	}
	height := header.NumberU64()
	prevHash := header.GetParentHash()
	if p.coinbaseAddrCache.Contains(prevHash) {
		ha, _ := p.coinbaseAddrCache.Get(prevHash)
		haddrs := ha.(heightAndAddrs)
		addrs = append(addrs, haddrs.addrs...)
		if len(addrs) == length {
			addrs = addrs[:length-1]
		}
		addrs = append(addrs, header.GetCoinbase().Recipient)
	} else { //miss, iterating DB
		for i := 0; i < length; i++ {
			addrsNew := []account.Recipient{header.GetCoinbase().Recipient}
			addrs = append(addrsNew, addrs...)
			if header.NumberU64() == 0 {
				break
			}
			if header = p.minorBCHelper.GetHeader(header.GetParentHash()); qkcCommon.IsNil(header) {
				return nil, fmt.Errorf("mysteriously missing block %x", header.GetParentHash())
			}
		}
	}
	p.coinbaseAddrCache.Add(headerHash, heightAndAddrs{height, addrs})
	if len(addrs) > length {
		panic("Unexpected result: len(addrs) > length\n")
	}
	return addrs, nil
}

//for test only
func getCoinbaseAddrCache(p *PoSW) *lru.Cache {
	return p.coinbaseAddrCache
}
