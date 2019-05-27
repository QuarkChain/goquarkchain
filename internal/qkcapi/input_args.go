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

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data             *hexutil.Bytes `json:"data"`
	FromFullShardKey *hexutil.Uint  `json:"fromFullShardId"`
	ToFullShardKey   *hexutil.Uint  `json:"toFullShardId"`
	V                *hexutil.Big   `json:"v"`
	R                *hexutil.Big   `json:"r"`
	S                *hexutil.Big   `json:"s"`
	NetWorkID        *hexutil.Uint  `json:"networkid"`
}

var (
	DEFAULT_STARTGAS = uint64(100 * 1000)
	DEFAULT_GASPRICE = new(big.Int).Mul(new(big.Int).SetUint64(10), new(big.Int).SetUint64(1000000000))
)

// setDefaults is a helper function that fills in default values for unspecified tx fields.
func (args *SendTxArgs) setDefaults() {
	// ingore nonce
	// ingore to
	if args.Gas == nil {
		args.Gas = (*hexutil.Uint64)(&DEFAULT_STARTGAS)
	}
	if args.GasPrice == nil {
		args.GasPrice = (*hexutil.Big)(DEFAULT_GASPRICE)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
}

func (args *SendTxArgs) toTransaction(networkID uint32, withVRS bool) (*types.Transaction, error) {
	if args.Nonce == nil {
		return nil, errors.New("nonce is missing")
	}
	if args.FromFullShardKey == nil {
		return nil, errors.New("fromFullShardKey is missing")
	}
	if args.ToFullShardKey == nil {
		args.ToFullShardKey = args.FromFullShardKey
	}
	if args.NetWorkID != nil {
		networkID = uint32(*args.NetWorkID)
	}
	evmTx := types.NewEvmTransaction(uint64(*args.Nonce), account.BytesToIdentityRecipient(args.To.Bytes()), (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), uint32(*args.FromFullShardKey), uint32(*args.ToFullShardKey), networkID, 0, *args.Data)
	if withVRS {
		if args.V != nil && args.R != nil && args.S != nil {
			evmTx.SetVRS(args.V.ToInt(), args.R.ToInt(), args.S.ToInt())
		} else {
			return nil, errors.New("need v,r,s")
		}
	}
	return &types.Transaction{
		EvmTx:  evmTx,
		TxType: types.EvmTx,
	}, nil
}
