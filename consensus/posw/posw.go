package posw

import (
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/state"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"strconv"
)

type heightAndAddrs struct {
	height uint64
	addrs  []account.Recipient
}

type MinorBlockChainPoSWHelper interface {
	GetEvmStateForNewBlock(header types.IHeader, ephemeral bool) (*state.StateDB, error)
	GetPoSWConfig() *config.POSWConfig
	GetHeaderByHash(hash common.Hash) types.IHeader
}

type PoSW struct {
	Config            *config.POSWConfig
	coinbaseAddrCache map[uint32]map[common.Hash]heightAndAddrs
	minorBCHelper     MinorBlockChainPoSWHelper
}

func NewPoSW(minorBlockChain MinorBlockChainPoSWHelper) *PoSW {
	return &PoSW{
		minorBCHelper:     minorBlockChain,
		Config:            minorBlockChain.GetPoSWConfig(),
		coinbaseAddrCache: make(map[uint32]map[common.Hash]heightAndAddrs),
	}
}

// PoSWDiffAdjust PoSW diff calc,already locked by insertChain
func (p *PoSW) PoSWDiffAdjust(header types.IHeader) (*big.Int, error) {
	// Evaluate stakes before the to-be-added block
	evmState, err := p.minorBCHelper.GetEvmStateForNewBlock(header, false)
	if err != nil {
		return nil, err
	}
	coinBaseRecipient := header.GetCoinbase().Recipient
	stakes := evmState.GetBalance(coinBaseRecipient)
	blockThreshold := new(big.Int).Div(stakes, p.Config.TotalStakePerBlock)
	blockThresholdStr := blockThreshold.Text(10)
	blockThresholdInt64, err := strconv.ParseUint(blockThresholdStr, 10, 32)
	if err != nil {
		log.Error("failed to compute blockThreshold", err)
		return nil, err
	}
	blockThresholdInt32 := uint32(blockThresholdInt64)
	if blockThresholdInt32 > p.Config.WindowSize {
		blockThresholdInt32 = p.Config.WindowSize
	}
	// The func is inclusive, so need to fetch block counts until prev block
	// Also only fetch prev window_size - 1 block counts because the
	// new window should count the current block
	blockCnt, err := p.GetPoSWCoinbaseBlockCnt(header.GetParentHash(), p.Config.WindowSize-1)
	if err != nil {
		return nil, err
	}
	cnt := blockCnt[coinBaseRecipient]
	diff := header.GetDifficulty()
	if cnt < blockThresholdInt32 {
		diff = new(big.Int).Div(diff, big.NewInt(int64(p.Config.DiffDivider)))
		log.Info("[PoSW]Adjusted PoSW ", "from", header.GetDifficulty(), "to", diff)
	}
	return diff, nil
}

/*PoSW needed function: get coinbase addresses up until the given block
hash (inclusive) along with block counts within the PoSW window.
*/
func (p *PoSW) GetPoSWCoinbaseBlockCnt(headerHash common.Hash, length uint32) (map[account.Recipient]uint32, error) {
	coinbaseAddrs, err := p.getCoinbaseAddressUntilBlock(headerHash, length)
	if err != nil {
		return nil, err
	}
	recipientCountMap := make(map[account.Recipient]uint32)
	for _, ca := range coinbaseAddrs {
		if _, ok := recipientCountMap[ca]; ok {
			recipientCountMap[ca]++
		} else {
			recipientCountMap[ca] = 1
		}
	}
	return recipientCountMap, nil
}

/*
*Get coinbase addresses up until block of given hash within the window.
 */
func (p *PoSW) getCoinbaseAddressUntilBlock(headerHash common.Hash, length uint32) ([]account.Recipient, error) {
	var header types.IHeader
	var addrs []account.Recipient
	if header = p.minorBCHelper.GetHeaderByHash(headerHash); qkcCommon.IsNil(header) {
		return nil, fmt.Errorf("curr block not found: hash %x", headerHash)
	}
	height := header.NumberU64()
	prevHash := header.GetParentHash()
	log.Info("[PoSW]Size of p.coinbaseAddrCache:", "size", len(p.coinbaseAddrCache))
	cache := p.coinbaseAddrCache[length]
	if haddrs, ok := cache[prevHash]; ok { //mem cache hit
		addrs = haddrs.addrs
		if uint32(len(addrs)) == length {
			addrs = addrs[1:]
		}
		addrs = append(addrs, header.GetCoinbase().Recipient)
		log.Info("[PoSW]Using Cache of coinbaseAddrCache:", "prevHash", prevHash)
	} else { //miss, iterating DB
		cache = make(map[common.Hash]heightAndAddrs)
		lgth := int(length)
		addrs = make([]account.Recipient, lgth)
		for i := lgth; i > 0; i-- {
			addrs[i-1] = header.GetCoinbase().Recipient
			if header.NumberU64() == 0 {
				break
			}
			if header = p.minorBCHelper.GetHeaderByHash(header.GetParentHash()); qkcCommon.IsNil(header) {
				return nil, fmt.Errorf("mysteriously missing block %x", header.GetParentHash())
			}
		}
		p.coinbaseAddrCache[length] = cache
	}
	log.Info("[PoSW] getCoinbaseAddressUntilBlock", "size of addrs", len(addrs), "last addrs", addrs[len(addrs)-1])
	cache[headerHash] = heightAndAddrs{height, addrs}
	//in case cached too much, clean up
	if len(cache) > 128 { //size around 640KB if window size 256
		//TODO keep most recent ones
		// minorBlockChain.coinbaseAddrCache[length] =

	}
	return addrs, nil
}

/*
*Take an additional recipient parameter and add its block count.
 */
func (p *PoSW) BuildSenderDisallowMap(headerHash common.Hash, recipient account.Recipient) map[account.Recipient]*big.Int {
	if !p.Config.Enabled {
		return nil
	}
	length := p.Config.WindowSize - 1
	stakePerBlock := p.Config.TotalStakePerBlock
	blockCnt, err := p.GetPoSWCoinbaseBlockCnt(headerHash, length)
	if err != nil {
		return nil
	}
	if len(recipient) != 0 {
		blockCnt[recipient] += 1
	}
	disallowMap := make(map[account.Recipient]*big.Int)
	for k, v := range blockCnt {
		disallowMap[k] = new(big.Int).Mul(big.NewInt(int64(v)), stakePerBlock)
	}
	return disallowMap
}

func (p *PoSW) IsPoSWEnabled() bool {
	return p.Config.Enabled
}
