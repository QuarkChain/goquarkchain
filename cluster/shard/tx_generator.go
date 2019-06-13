package shard

import (
	"encoding/hex"
	"fmt"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
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
	cfg          *config.QuarkChainConfig
	fullShardId  uint32
	accounts     []*account.Account
	once         sync.Once
	lenAccounts  int
	accountIndex int
	turn         uint64
}

func NewTxGenerator(genesisDir string, fullShardId uint32, cfg *config.QuarkChainConfig) *TxGenerator {
	accounts := config.LoadtestAccounts(genesisDir)
	txG := &TxGenerator{
		cfg:          cfg,
		fullShardId:  fullShardId,
		accounts:     accounts,
		once:         sync.Once{},
		lenAccounts:  len(accounts),
		accountIndex: 0,
		turn:         0,
	}
	return txG
}

func (t *TxGenerator) Generate(genTxs *rpc.GenTxRequest, addTxList func(txs []*types.Transaction) error) error {
	ts := time.Now()
	var (
		batchScale    = uint32(3000)
		txList        = make([]*types.Transaction, 0, batchScale)
		xShardPercent = int(genTxs.XShardPercent)
		total         = uint32(0)
	)
	// return err if accounts is empty.
	if t.accounts == nil {
		return fmt.Errorf("accounts is empty, can't create transactions")
	}

	log.Info("Start Generating transactions", "tx count", genTxs.NumTxPerShard, "cross-shard tx count", xShardPercent)
	for t.accountIndex < t.lenAccounts {
		if total >= genTxs.NumTxPerShard {
			break
		}
		acc := t.accounts[t.accountIndex]
		total++

		tx, err := t.createTransaction(acc, t.turn, xShardPercent, genTxs.Tx)
		if err != nil {
			continue
		}
		txList = append(txList, &types.Transaction{TxType: types.EvmTx, EvmTx: tx})

		if total%batchScale == 0 {
			if err := addTxList(txList); err != nil {
				return err
			}
			txList = make([]*types.Transaction, 0, batchScale)
			time.Sleep(time.Second * 2)
		}

		t.accountIndex++
		if t.accountIndex == t.lenAccounts {
			t.turn++
			t.accountIndex = 0
		}
	}

	if len(txList) != 0 {
		if err := addTxList(txList); err != nil {
			return err
		}
	}

	log.Info("Finish Generating transactions", "fullShardId", t.fullShardId, "tx count", total, "use seconds", time.Now().Sub(ts))
	return nil
}

func (t *TxGenerator) createTransaction(acc *account.Account, nonce uint64,
	xShardPercent int, sampleTx *types.Transaction) (*types.EvmTransaction, error) {
	var (
		fromFullShardKey = sampleTx.EvmTx.FromFullShardKey()
		toFullShardKey   = fromFullShardKey
		recipient        = *sampleTx.EvmTx.To()
	)
	if fromFullShardKey == 0 {
		fromFullShardKey = t.fullShardId
	}
	if recipient == (common.Address{}) {
		idx := t.random(t.lenAccounts)
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
