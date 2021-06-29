package qkcapi

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/QuarkChain/goquarkchain/account"
	"github.com/QuarkChain/goquarkchain/cluster/config"
	"github.com/QuarkChain/goquarkchain/common/hexutil"
	"github.com/QuarkChain/goquarkchain/core/types"
	"github.com/QuarkChain/goquarkchain/params"
	"github.com/QuarkChain/goquarkchain/rpc"
	"github.com/ethereum/go-ethereum/common"
)

// CallArgs represents the arguments for a call.
type EthCallArgs struct {
	From     account.Address  `json:"from"`
	To       *account.Address `json:"to"`
	Gas      hexutil.Uint64   `json:"gas"`
	GasPrice hexutil.Big      `json:"gasPrice"`
	Value    hexutil.Big      `json:"value"`
	Data     hexutil.Bytes    `json:"data"`
}

func (e *EthCallArgs) UnmarshalJSON(data []byte) error {
	var args EthCallArgs
	if err := json.Unmarshal(data, &args); err != nil {
		return err
	}
	if args.To == nil {
		to := account.CreatEmptyAddress(args.From.FullShardKey)
		args.To = &to
	}
	e = &args
	return nil
}

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From            *account.Address `json:"from"`
	To              *account.Address `json:"to"`
	Gas             hexutil.Big      `json:"gas"`
	GasPrice        hexutil.Big      `json:"gasPrice"`
	Value           hexutil.Big      `json:"value"`
	Data            hexutil.Bytes    `json:"data"`
	GasTokenID      *hexutil.Uint64  `json:"gas_token_id"`
	TransferTokenID *hexutil.Uint64  `json:"transfer_token_id"`
}

type GetAccountDataArgs struct {
	Address       account.Address  `json:"address"`
	IncludeShards *bool            `json:"include_shards"`
	BlockHeight   *rpc.BlockNumber `json:"block_height"`
}

func (c *CallArgs) setDefaults() {
	if c.From == nil {
		temp := account.CreatEmptyAddress(c.To.FullShardKey)
		c.From = &temp
	}
}

func (c *CallArgs) toTx(config *config.QuarkChainConfig) (*types.Transaction, error) {
	gasTokenID, transferTokenID := config.GetDefaultChainTokenID(), config.GetDefaultChainTokenID()
	if c.GasTokenID != nil {
		gasTokenID = uint64(*c.GasTokenID)
	}
	if c.TransferTokenID != nil {
		transferTokenID = uint64(*c.TransferTokenID)
	}
	evmTx := new(types.EvmTransaction)
	if c.To == nil {
		evmTx = types.NewEvmContractCreation(0, c.Value.ToInt(), c.Gas.ToInt().Uint64(), c.GasPrice.ToInt(), c.From.FullShardKey, c.From.FullShardKey, config.NetworkID, 0, c.Data, gasTokenID, transferTokenID)
	} else {
		evmTx = types.NewEvmTransaction(0, c.To.Recipient, c.Value.ToInt(), c.Gas.ToInt().Uint64(),
			c.GasPrice.ToInt(), c.From.FullShardKey, c.To.FullShardKey, config.NetworkID, 0, c.Data, gasTokenID, transferTokenID)
	}
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	return tx, nil
}

type CreateTxArgs struct {
	NumTxPreShard    *uint32         `json:"numTxPerShard"`
	XShardPrecent    *uint32         `json:"xShardPercent"`
	To               *common.Address `json:"to"`
	Gas              *hexutil.Big    `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Value            *hexutil.Big    `json:"value"`
	Data             *hexutil.Bytes  `json:"data"`
	FromFullShardKey *hexutil.Uint   `json:"fromFullShardKey"`
	GasTokenID       *hexutil.Uint64 `json:"gas_token_id"`
	TransferTokenID  *hexutil.Uint64 `json:"transfer_token_id"`
}

func (c *CreateTxArgs) setDefaults(config *config.QuarkChainConfig) error {
	if c.NumTxPreShard == nil {
		return errors.New("must set numTxPerShard")
	}
	if c.XShardPrecent == nil {
		t := uint32(0)
		c.XShardPrecent = &t
	}
	if c.Gas == nil {
		c.Gas = (*hexutil.Big)(params.DefaultStartGas)
	}
	if c.GasPrice == nil {
		c.GasPrice = (*hexutil.Big)(params.DefaultGasPrice.Div(params.DefaultGasPrice, new(big.Int).SetUint64(10)))
	}
	if c.Value == nil {
		t := hexutil.Big{}
		c.Value = &t
	}
	if c.Data == nil {
		t := hexutil.Bytes{}
		c.Data = &t
	}
	if c.FromFullShardKey == nil {
		t := hexutil.Uint(0)
		c.FromFullShardKey = &t
	}
	if c.GasTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		c.GasTokenID = &t
	}
	if c.TransferTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		c.TransferTokenID = &t
	}
	return nil
}
func (c *CreateTxArgs) toTx(config *config.QuarkChainConfig) *types.Transaction {
	var (
		evmTx *types.EvmTransaction
	)
	if c.To == nil {
		evmTx = types.NewEvmContractCreation(0, c.Value.ToInt(), c.Gas.ToInt().Uint64(), c.GasPrice.ToInt(),
			uint32(*c.FromFullShardKey), 0, config.NetworkID, 0, *c.Data, uint64(*c.GasTokenID), uint64(*c.TransferTokenID))
	} else {
		evmTx = types.NewEvmTransaction(0, *c.To, c.Value.ToInt(), c.Gas.ToInt().Uint64(), c.GasPrice.ToInt(),
			uint32(*c.FromFullShardKey), 0, config.NetworkID, 0, *c.Data, uint64(*c.GasTokenID), uint64(*c.TransferTokenID))
	}
	tx := &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}
	return tx
}

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	To       *common.Address `json:"to"`
	Gas      *hexutil.Big    `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data             *hexutil.Bytes  `json:"data"`
	FromFullShardKey *hexutil.Uint   `json:"fromFullShardKey"`
	ToFullShardKey   *hexutil.Uint   `json:"toFullShardKey"`
	V                *hexutil.Big    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
	NetWorkID        *hexutil.Uint   `json:"networkId"`
	GasTokenID       *hexutil.Uint64 `json:"gas_token_id"`
	TransferTokenID  *hexutil.Uint64 `json:"transfer_token_id"`
}

// setDefaults is a helper function that fills in default values for unspecified tx fields.
func (args *SendTxArgs) setDefaults(config *config.QuarkChainConfig) error {
	if args.Gas == nil {
		args.Gas = (*hexutil.Big)(params.DefaultStartGas)
	}
	if args.GasPrice == nil {
		args.GasPrice = (*hexutil.Big)(params.DefaultGasPrice)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Data == nil {
		args.Data = new(hexutil.Bytes)
	}
	if args.Nonce == nil {
		return errors.New("nonce is missing")
	}
	if args.FromFullShardKey == nil {
		return errors.New("fromFullShardKey is missing")
	}
	if args.ToFullShardKey == nil {
		args.ToFullShardKey = args.FromFullShardKey
	}

	if args.NetWorkID == nil {
		t := hexutil.Uint(config.NetworkID)
		args.NetWorkID = &t
	}

	if args.GasTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		args.GasTokenID = &t
	}
	if args.TransferTokenID == nil {
		t := hexutil.Uint64(config.GetDefaultChainTokenID())
		args.TransferTokenID = &t
	}

	if args.V == nil || args.R == nil || args.S == nil {
		return errors.New("missing v r s")
	}
	return nil
}

func (args *SendTxArgs) toTransaction() (*types.Transaction, error) {
	var evmTx *types.EvmTransaction
	if args.To == nil {
		evmTx = types.NewEvmContractCreation(uint64(*args.Nonce), (*big.Int)(args.Value), args.Gas.ToInt().Uint64(),
			(*big.Int)(args.GasPrice), uint32(*args.FromFullShardKey), uint32(*args.ToFullShardKey),
			uint32(*args.NetWorkID), 0, *args.Data, uint64(*args.GasTokenID), uint64(*args.TransferTokenID))
	} else {
		evmTx = types.NewEvmTransaction(uint64(*args.Nonce), account.BytesToIdentityRecipient(args.To.Bytes()),
			(*big.Int)(args.Value), args.Gas.ToInt().Uint64(), (*big.Int)(args.GasPrice), uint32(*args.FromFullShardKey),
			uint32(*args.ToFullShardKey), uint32(*args.NetWorkID), 0, *args.Data, uint64(*args.GasTokenID), uint64(*args.TransferTokenID))
	}

	evmTx.SetVRS(args.V.ToInt(), args.R.ToInt(), args.S.ToInt())

	return &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}, nil
}
