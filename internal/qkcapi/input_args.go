package qkcapi

import (
	"errors"
	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"math/big"
)

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     *account.Address `json:"from"`
	To       *account.Address `json:"to"`
	Gas      hexutil.Big      `json:"gas"`
	GasPrice hexutil.Big      `json:"gasPrice"`
	Value    hexutil.Big      `json:"value"`
	Data     hexutil.Bytes    `json:"data"`
}

func (c *CallArgs) setDefaults() {
	if c.From == nil {
		temp := account.CreatEmptyAddress(c.To.FullShardKey)
		c.From = &temp
	}
}
func (c *CallArgs) toTx(config *config.QuarkChainConfig) (*types.Transaction, error) {
	evmTx := types.NewEvmTransaction(0, c.To.Recipient, c.Value.ToInt(), c.Gas.ToInt().Uint64(), c.GasPrice.ToInt(), c.From.FullShardKey, c.To.FullShardKey, config.NetworkID, 0, c.Data)
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	toShardSize := config.GetShardSizeByChainId(tx.EvmTx.ToChainID())
	if err := tx.EvmTx.SetToShardSize(toShardSize); err != nil {
		return nil, errors.New("SetToShardSize err")
	}
	fromShardSize := config.GetShardSizeByChainId(tx.EvmTx.FromChainID())
	if err := tx.EvmTx.SetFromShardSize(fromShardSize); err != nil {
		return nil, errors.New("SetFromShardSize err")
	}
	return tx, nil
}

type CreateTxArgs struct {
	NumTxPreShard    hexutil.Uint   `json:"numTxPerShard"`
	XShardPrecent    hexutil.Uint   `json:"xShardPercent"`
	To               common.Address `json:"to"`
	Gas              *hexutil.Big   `json:"gas"`
	GasPrice         *hexutil.Big   `json:"gasPrice"`
	Value            hexutil.Big    `json:"value"`
	Data             hexutil.Bytes  `json:"data"`
	FromFullShardKey hexutil.Uint   `json:"fromFullShardId"`
}

func (c *CreateTxArgs) setDefaults() {
	if c.Gas == nil {
		c.Gas = (*hexutil.Big)(params.DefaultStartGas)
	}
	if c.GasPrice == nil {
		c.GasPrice = (*hexutil.Big)(new(big.Int).Div(params.DenomsValue.GWei, new(big.Int).SetUint64(10)))
	}
}
func (c *CreateTxArgs) toTx(config *config.QuarkChainConfig) *types.Transaction {
	evmTx := types.NewEvmTransaction(0, c.To, c.Value.ToInt(), c.Gas.ToInt().Uint64(), c.GasPrice.ToInt(), uint32(c.FromFullShardKey), 0, config.NetworkID, 0, c.Data)
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	return tx
}
