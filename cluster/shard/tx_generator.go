package shard

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/cluster/rpc"
	qkcCommon "github.com/QuarkChain/goquarkchain/common"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
)

type TxGenerator struct {
	cfg             *config.QuarkChainConfig
	fullShardId     uint32
	accounts        []AccountWithPrivateKey
	once            sync.Once
	lenAccounts     int
	accountIndex    int
	turn            uint64
	sender          types.Signer
	fullShardIDList []uint32
}

type AccountWithPrivateKey struct {
	acc        *account.Account
	privateKey *ecdsa.PrivateKey
}

func getAccountWithPrivateKey(data []*account.Account) []AccountWithPrivateKey {
	accounts := make([]AccountWithPrivateKey, len(data))

	for index, v := range data {
		prvKey, err := crypto.HexToECDSA(v.PrivateKey())
		if err != nil {
			panic(err)
		}
		accounts[index] = AccountWithPrivateKey{
			acc:        v,
			privateKey: prvKey,
		}
	}
	return accounts
}

func NewTxGenerator(genesisDir string, fullShardId uint32, cfg *config.QuarkChainConfig) []*TxGenerator {
	tgs := make([]*TxGenerator, params.TPS_Num)
	accounts := getAccountWithPrivateKey(config.LoadtestAccounts(genesisDir))
	interval := len(accounts) / params.TPS_Num
	for index := 0; index < params.TPS_Num; index++ {
		tgs[index] = &TxGenerator{
			cfg:             cfg,
			fullShardId:     fullShardId,
			accounts:        accounts[index*interval : (index+1)*interval],
			once:            sync.Once{},
			lenAccounts:     interval,
			accountIndex:    0,
			turn:            0,
			sender:          types.NewEIP155Signer(cfg.NetworkID),
			fullShardIDList: make([]uint32, 0),
		}
		for _, v := range cfg.GetGenesisShardIds() {
			if v != fullShardId {
				tgs[index].fullShardIDList = append(tgs[index].fullShardIDList, v)
			}
		}

		log.Info("tx-generator", "index", index, "account len", len(tgs[index].accounts))
	}
	return tgs
}

func (t *TxGenerator) random(digit int) int {
	t.once.Do(func() {
		rand.Seed(time.Now().UnixNano())
	})
	return rand.Int() % digit
}

func (t *TxGenerator) sign(evmTx *types.EvmTransaction, key *ecdsa.PrivateKey) (*types.EvmTransaction, error) {
	return types.SignTx(evmTx, t.sender, key)
}

func (t *TxGenerator) Generate(genTxs rpc.GenTxRequest, addTxList func(txs []*types.Transaction) error) error {
	tsa := time.Now()
	var (
		batchScale    = 4000
		txList        = make([]*types.Transaction, batchScale)
		numTx         = genTxs.NumTxPerShard
		xShardPercent = int(genTxs.XShardPercent)
		total         = uint32(0)
		index         = 0
	)
	// return err if accounts is empty.
	if t.accounts == nil {
		return fmt.Errorf("accounts is empty, can't create transactions")
	}
	if numTx == 0 {
		return fmt.Errorf("create txs operation, numTx is zero")
	}
	log.Info("Start Generating transactions", "tx count", numTx, "cross-shard tx count", xShardPercent)
	for t.accountIndex < t.lenAccounts {
		if total >= numTx {
			break
		}

		tx, err := t.createTransaction(t.accounts[t.accountIndex].privateKey, t.turn, xShardPercent, genTxs.Tx)
		if err != nil {
			continue
		}
		total++
		txList[index] = &types.Transaction{TxType: types.EvmTx, EvmTx: tx}
		index++

		if index >= batchScale {
			if err := addTxList(txList); err != nil {
				return err
			}
			index = 0
			txList = make([]*types.Transaction, batchScale)
		}

		t.accountIndex++
		if t.accountIndex == t.lenAccounts {
			t.turn++
			t.accountIndex = 0
		}
	}

	if len(txList) != 0 {
		if err := addTxList(txList[:index]); err != nil {
			return err
		}
	}

	log.Info("Finish Generating transactions", "fullShardId", t.fullShardId, "tx count", total, "use seconds", time.Now().Sub(tsa))
	return nil
}

func (t *TxGenerator) createTransaction(prvKey *ecdsa.PrivateKey, nonce uint64,
	xShardPercent int, sampleTx *types.Transaction) (*types.EvmTransaction, error) {
	var (
		fromFullShardKey = t.fullShardId
		toFullShardKey   = fromFullShardKey
		recipient        = common.Address{}
	)
	if sampleTx.EvmTx.To() != nil {
		recipient = *sampleTx.EvmTx.To()
	} else {
		idx := t.random(t.lenAccounts)
		toAddr := t.accounts[idx].acc
		recipient = toAddr.Identity.GetRecipient()
	}

	if xShardPercent >= 0 && t.random(100) < xShardPercent {
		idx := uint32(t.random(len(t.fullShardIDList)))
		toFullShardKey = t.fullShardIDList[idx]

	}
	value := sampleTx.EvmTx.Value()
	if value.Uint64() == 0 {
		rv := t.random(100)
		value = value.Mul(big.NewInt(int64(rv)), config.QuarkashToJiaozi)
	}

	gasLimit := params.DefaultInShardTxGasLimit.Uint64()
	if fromFullShardKey != toFullShardKey {
		gasLimit = params.DefaultCrossShardTxGasLimit.Uint64()
	}
	gasPrice := new(big.Int).SetUint64(1000000000)
	evmTx := types.NewEvmTransaction(nonce, recipient, value, gasLimit,
		gasPrice, fromFullShardKey, toFullShardKey, t.cfg.NetworkID, 0, sampleTx.EvmTx.Data(), qkcCommon.TokenIDEncode("QKC"), qkcCommon.TokenIDEncode("QKC"))

	return types.SignTx(evmTx, t.sender, prvKey)
}
