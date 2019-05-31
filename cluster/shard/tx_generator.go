package shard

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"golang.org/x/sync/errgroup"
	"math/big"
	"math/rand"
	"sync"
	"time"
)

type TxGenerator struct {
	cfg         *config.QuarkChainConfig
	fullShardId uint32
	accounts    []*account.Account
	once        sync.Once
}

func NewTxGenerator(genesisDir string, fullShardId uint32, cfg *config.QuarkChainConfig) (*TxGenerator, error) {
	accounts, err := config.LoadtestAccounts(genesisDir)
	if err != nil {
		return nil, err
	}
	return &TxGenerator{
		cfg:         cfg,
		fullShardId: fullShardId,
		accounts:    accounts,
		once:        sync.Once{},
	}, nil
}

func (t *TxGenerator) Generate(genTxs *rpc.GenTxRequest,
	getTxCount func(recipient account.Recipient, height *uint64) (uint64, error),
	addTxList func(txs []*types.Transaction) error) error {
	var (
		batchScale    = 600
		txList        = make([]*types.Transaction, 0, genTxs.NumTxPerShard)
		numTx         = int(genTxs.NumTxPerShard)
		xShardPercent = int(genTxs.XShardPercent)
		total         = 0
		g             errgroup.Group
	)
	if numTx == 0 {
		return fmt.Errorf("Create txs operation, numTx ")
	}
	log.Info("Start Generating transactions", "tx count", numTx, "cross-shard tx count", xShardPercent)

	start := time.Now()
	for total <= numTx {
		for _, acc := range t.accounts {
			nonce, err := getTxCount(acc.Identity.GetRecipient(), nil)
			if err != nil {
				continue
			}
			tx, err := t.createTransaction(acc, nonce, xShardPercent, genTxs.Tx)
			if err != nil {
				continue
			}
			txList = append(txList, &types.Transaction{TxType: types.EvmTx, EvmTx: tx})
			total++
			if total%batchScale == 0 {
				txs := txList[total-batchScale : total]
				g.Go(func() error {
					return addTxList(txs)
				})
				time.Sleep(time.Second * 2)
			}
			if total >= numTx || total >= len(t.accounts) {
				txs := txList[total-total%batchScale : total]
				g.Go(func() error {
					return addTxList(txs)
				})
				break
			}
		}
	}

	log.Info("Finish Generating transactions", "fullShardId", t.fullShardId, "tx count", total, "use seconds", time.Now().Sub(start))
	return g.Wait()
}

func (t *TxGenerator) createTransaction(acc *account.Account, nonce uint64,
	xShardPercent int, sampleTx *types.Transaction) (*types.EvmTransaction, error) {
	var (
		fromFullShardKey = sampleTx.EvmTx.FromFullShardKey()
		toFullShardKey   = fromFullShardKey
		recipient        = *sampleTx.EvmTx.To()
	)
	if t.cfg.GetFullShardIdByFullShardKey(fromFullShardKey) != t.fullShardId {
		return nil, errors.New("fromFullShardId not match")
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
	value := sampleTx.EvmTx.Value()
	if value.Uint64() == 0 {
		rv := t.random(100)
		value = value.Mul(big.NewInt(int64(rv)), config.QuarkashToJiaozi)
	}

	evmTx := types.NewEvmTransaction(nonce, recipient, value, sampleTx.EvmTx.Gas(),
		sampleTx.EvmTx.GasPrice(), fromFullShardKey, toFullShardKey, t.cfg.NetworkID, 0, sampleTx.EvmTx.Data())

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
