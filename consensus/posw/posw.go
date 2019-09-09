package posw

import (
	"bytes"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	lru "github.com/hashicorp/golang-lru"
	"math/big"
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
	hReader           headReader
}

func NewPoSW(headReader headReader, config *config.POSWConfig) *PoSW {
	cache, _ := lru.New(128)
	return &PoSW{
		hReader:           headReader,
		config:            config,
		coinbaseAddrCache: cache,
	}
}

/*PoSWDiffAdjust PoSW diff calc,already locked by insertChain*/
func (p *PoSW) PoSWDiffAdjust(header types.IHeader, stakes *big.Int) (*big.Int, error) {
	diff := header.GetDifficulty()
	// Evaluate stakes before the to-be-added block
	blockThreshold := new(big.Int).Div(stakes, p.config.TotalStakePerBlock).Uint64()
	if blockThreshold == uint64(0) {
		return diff, nil
	}
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
	if blockCnt < blockThreshold {
		diff = new(big.Int).Div(diff, big.NewInt(int64(p.config.DiffDivider)))
	}
	return diff, nil
}

/*Take an additional recipient parameter and add its block count.*/
func (p *PoSW) BuildSenderDisallowMap(headerHash common.Hash, coinbase *account.Recipient) (map[account.Recipient]*big.Int, error) {
	if !p.config.Enabled {
		return nil, nil
	}
	coinbaseAddrs, err := p.GetCoinbaseAddressUntilBlock(headerHash)
	if err != nil {
		return nil, err
	}
	recipientCountMap := make(map[account.Recipient]uint64)
	for _, ca := range coinbaseAddrs {
		recipientCountMap[ca]++
	}
	if coinbase != nil {
		recipientCountMap[*coinbase]++
	}
	disallowMap := make(map[account.Recipient]*big.Int)
	for k, v := range recipientCountMap {
		disallowMap[k] = new(big.Int).Mul(big.NewInt(int64(v)), p.config.TotalStakePerBlock)
	}
	return disallowMap, nil
}

func (p *PoSW) IsPoSWEnabled() bool {
	return p.config.Enabled
}

func (p *PoSW) countCoinbaseBlockUntil(headerHash common.Hash, coinbase account.Recipient) (uint64, error) {
	coinbases, err := p.GetCoinbaseAddressUntilBlock(headerHash)
	if err != nil {
		return 0, err
	}
	coinbaseBytes := common.Address(coinbase).Bytes()
	var count uint64 = 0
	for _, cb := range coinbases {
		if bytes.Compare(common.Address(cb).Bytes(), coinbaseBytes) == 0 {
			count++
		}
	}
	return count, nil
}

func (p *PoSW) GetCoinbaseAddressUntilBlock(headerHash common.Hash) ([]account.Recipient, error) {
	header := p.hReader.GetHeader(headerHash)
	if qkcCommon.IsNil(header) {
		return nil, fmt.Errorf("curr block not found: hash %x", headerHash)
	}
	length := int(p.config.WindowSize)
	addrs := make([]account.Recipient, 0, length)
	height := header.NumberU64()
	prevHash := header.GetParentHash()
	if p.coinbaseAddrCache.Contains(prevHash) {
		ha, _ := p.coinbaseAddrCache.Get(prevHash)
		haddrs := ha.(heightAndAddrs)
		addrs = append(addrs, haddrs.addrs...)
		if len(addrs) == length {
			addrs = addrs[1:]
		}
		addrs = append(addrs, header.GetCoinbase().Recipient)
	} else { //miss, iterating DB
		for i := 0; i < length; i++ {
			addrsNew := []account.Recipient{header.GetCoinbase().Recipient}
			addrs = append(addrsNew, addrs...)
			if header.NumberU64() == 0 {
				break
			}
			if header = p.hReader.GetHeader(header.GetParentHash()); qkcCommon.IsNil(header) {
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
