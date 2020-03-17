package posw

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	lru "github.com/hashicorp/golang-lru"
)

type headReader interface {
	GetBlock(hash common.Hash) types.IBlock
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
func (p *PoSW) PoSWDiffAdjust(diff *big.Int, parentHash common.Hash, coinbase account.Recipient, stakes *big.Int) (*big.Int, error) {
	if stakes == nil {
		return diff, nil
	}
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
	blockCnt, err := p.countCoinbaseBlockUntil(parentHash, coinbase)
	if err != nil {
		return nil, err
	}
	log.Debug("PoSWDiffAdjust", "blockCnt", blockCnt, "blockThreshold", blockThreshold, "coinbase", coinbase)
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
	coinbaseAddrs, err := p.getCoinbaseAddressUntilBlock(headerHash)
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

func (p *PoSW) IsPoSWEnabled(time uint64, height uint64) bool {
	return p.config.Enabled && time >= p.config.EnableTimestamp && height > 0
}

func (p *PoSW) countCoinbaseBlockUntil(headerHash common.Hash, coinbase account.Recipient) (uint64, error) {
	coinbases, err := p.getCoinbaseAddressUntilBlock(headerHash)
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

func revertArr(arr []account.Recipient) []account.Recipient {
	start := 0
	end := len(arr) - 1

	for start < end {
		arr[start], arr[end] = arr[end], arr[start]
		start++
		end--
	}
	return arr
}

func (p *PoSW) getCoinbaseAddressUntilBlock(headerHash common.Hash) ([]account.Recipient, error) {
	header := p.hReader.GetBlock(headerHash)
	if qkcCommon.IsNil(header) {
		return nil, fmt.Errorf("curr block not found: hash %x", headerHash)
	}
	length := int(p.config.WindowSize) - 1
	addrs := make([]account.Recipient, 0, length+10)
	height := header.NumberU64()
	prevHash := header.ParentHash()
	if p.coinbaseAddrCache.Contains(prevHash) {
		ha, _ := p.coinbaseAddrCache.Get(prevHash)
		haddrs := ha.(heightAndAddrs)
		addrs = append(addrs, haddrs.addrs...)
		if len(addrs) == length {
			addrs = addrs[1:]
		}
		addrs = append(addrs, header.Coinbase().Recipient)
	} else { //miss, iterating DB
		for i := 0; i < length; i++ {
			addrs = append(addrs, header.Coinbase().Recipient)
			if header.NumberU64() == 0 {
				break
			}
			if header = p.hReader.GetBlock(header.ParentHash()); qkcCommon.IsNil(header) {
				return nil, fmt.Errorf("mysteriously missing block %x", header.ParentHash())
			}
		}
		addrs = revertArr(addrs)
	}
	p.coinbaseAddrCache.Add(headerHash, heightAndAddrs{height, addrs})
	if len(addrs) > length {
		panic("Unexpected result: len(addrs) > length\n")
	}
	return addrs, nil
}

func (p *PoSW) GetPoSWInfo(header types.IBlock, stakes *big.Int, address account.Recipient) (effectiveDiff *big.Int, mineable, mined uint64, err error) {
	blockCnt, err := p.countCoinbaseBlockUntil(header.Hash(), address)
	if err != nil {
		return header.Difficulty(), 0, 0, err
	}
	if !p.IsPoSWEnabled(header.Time(), header.NumberU64()) || stakes == nil {
		return header.Difficulty(), 0, blockCnt, nil
	}
	blockThreshold := new(big.Int).Div(stakes, p.config.TotalStakePerBlock).Uint64()
	if blockThreshold > p.config.WindowSize {
		blockThreshold = p.config.WindowSize
	}
	diff := header.Difficulty()
	if blockCnt < blockThreshold {
		diff = new(big.Int).Div(diff, big.NewInt(int64(p.config.DiffDivider)))
	}
	effectiveDiff = diff
	mineable = blockThreshold
	mined = blockCnt
	return
}
