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
	turn         uint64
	accountIndex int
}

func NewTxGenerator(genesisDir string, fullShardId uint32, cfg *config.QuarkChainConfig) (*TxGenerator, error) {
	accounts := config.LoadtestAccounts(genesisDir)
	return &TxGenerator{
		cfg:          cfg,
		fullShardId:  fullShardId,
		accounts:     accounts,
		once:         sync.Once{},
		turn:         0,
		accountIndex: -1,
		lenAccounts:  len(accounts),
	}, nil
}

func (t *TxGenerator) Generate(genTxs *rpc.GenTxRequest,
	getTxCount func(recipient account.Recipient, height *uint64) (uint64, error),
	addTxList func(txs []*types.Transaction) error) error {
	var (
		numTx         = int(genTxs.NumTxPerShard)
		txList        = make([]*types.Transaction, 0, numTx)
		xShardPercent = int(genTxs.XShardPercent)
		total         = 0
	)
	// return err if accounts is empty.
	if t.accounts == nil {
		return fmt.Errorf("accounts is empty, can't create transactions")
	}
	if numTx == 0 {
		return fmt.Errorf("create txs operation, numTx is zero")
	}
	log.Info("Start Generating transactions", "tx count", numTx, "cross-shard tx count", xShardPercent)

	start := time.Now()
	timeOneTrun := time.Now()
	for total < numTx {
		for index, acc := range t.accounts {
			if total >= numTx {
				t.accountIndex = index - 1
				t.turn++
				break
			}
			total++
			nonce := t.turn
			if t.accountIndex == -1 { //from head
				nonce = t.turn
			} else if index > t.accountIndex { //not be used in last turn
				nonce--
			}
			tx, err := t.createTransaction(acc, nonce, xShardPercent, genTxs.Tx)
			if err != nil {
				continue
			}
			txList = append(txList, &types.Transaction{TxType: types.EvmTx, EvmTx: tx})
		}
		t.accountIndex = -1
		t.turn++
		log.Info("create_tx-loop", "t-loop", time.Now().Sub(timeOneTrun).Seconds(), "total", total)
		timeOneTrun = time.Now()

	}
	if len(txList) != 0 {
		if err := addTxList(txList); err != nil {
			return err
		}
	}

	log.Info("Finish Generating transactions", "fullShardId", t.fullShardId, "tx count", total, "use seconds", time.Now().Sub(start))
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
