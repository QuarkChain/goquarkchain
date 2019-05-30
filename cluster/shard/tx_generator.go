package shard

import (
	"encoding/hex"
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"math/big"
	"math/rand"
	"sync"
	"time"
)

type TxGenerator struct {
	cfg         *config.QuarkChainConfig
	fullShardId uint32
	getTxCount  func(recipient account.Recipient, height *uint64) (uint64, error)
	accounts    []*account.Account
	once        sync.Once
}

func NewTxGenerator(genesisDir string, fullShardId uint32, cfg *config.QuarkChainConfig, getTxCount func(recipient account.Recipient,
	height *uint64) (uint64, error)) (*TxGenerator, error) {
	accounts, err := config.LoadtestAccounts(genesisDir)
	if err != nil {
		return nil, err
	}
	return &TxGenerator{
		cfg:         cfg,
		fullShardId: fullShardId,
		getTxCount:  getTxCount,
		accounts:    accounts,
		once:        sync.Once{},
	}, nil
}

func (t *TxGenerator) Generate(numTx int, xShardPercent int, sampleTx *types.EvmTransaction) []*types.Transaction {
	var (
		txList = make([]*types.Transaction, 0, numTx)
		total  = 0
	)

	if numTx <= 0 {
		return nil
	}
	log.Info("Start Generating transactions", "tx count", numTx, "cross-shard txs", xShardPercent)
	for _, acc := range t.accounts {
		nonce, err := t.getTxCount(acc.Identity.GetRecipient(), nil)
		if err != nil {
			continue
		}
		tx, err := t.createTransaction(acc, nonce, xShardPercent, sampleTx)
		if err != nil {
			continue
		}
		txList = append(txList, &types.Transaction{TxType: types.EvmTx, EvmTx: tx})
		total++
		if total%600 == 0 {
			time.Sleep(time.Second * 2)
		}
		if total >= numTx {
			break
		}
	}
	return txList
}

func (t *TxGenerator) createTransaction(acc *account.Account, nonce uint64,
	xShardPercent int, sampleTx *types.EvmTransaction) (*types.EvmTransaction, error) {
	var (
		fromFullShardKey = sampleTx.FromFullShardKey()
		toFullShardKey   = fromFullShardKey
		recipient        = *sampleTx.To()
	)
	if t.cfg.GetFullShardIdByFullShardKey(fromFullShardKey) != t.fullShardId {
		return nil, errors.New("fromFullShardKey not match")
	}
	if recipient == (common.Address{}) {
		idx := t.random(len(t.accounts))
		toAddr := t.accounts[idx]
		recipient = toAddr.Identity.GetRecipient()
		toFullShardKey = t.fullShardId
	}

	if t.random(100) < xShardPercent {
		fullShardIds := t.cfg.GetGenesisShardIds()
		idx := uint32(t.random(len(fullShardIds)))
		toFullShardKey = fullShardIds[idx]
	}
	value := sampleTx.Value()
	if value == big.NewInt(0) {
		rv := t.random(100)
		value = value.Mul(big.NewInt(int64(rv)), config.QuarkashToJiaozi)
	}

	evmTx := types.NewEvmTransaction(nonce, recipient, value, sampleTx.Gas(),
		sampleTx.GasPrice(), fromFullShardKey, toFullShardKey, t.cfg.NetworkID, 0, sampleTx.Data())

	return t.sign(evmTx, acc.PrivateKey())
}

func (t *TxGenerator) random(digit int) int {
	t.once.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
	return rand.Int() % digit
}

func (t *TxGenerator) sign(evmTx *types.EvmTransaction, key string) (*types.EvmTransaction, error) {
	prvKey, err := crypto.HexToECDSA(hex.EncodeToString(common.FromHex(key)))
	if err != nil {
		panic(err)
	}
	return types.SignTx(evmTx, types.MakeSigner(evmTx.NetworkId()), prvKey)
}
